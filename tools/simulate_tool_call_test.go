package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	otel "go.opentelemetry.io/otel"
	codes "go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// simulate runs the simulate tool handler directly and returns the decoded
// JSON result.
func simulate(t *testing.T, args map[string]any) map[string]any {
	t.Helper()
	tool := &SimulateToolCallTool{}
	out, err := tool.Handler(context.Background(), args)
	if err != nil {
		t.Fatalf("Handler(%v): %v", args, err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("unmarshal result %q: %v", out, err)
	}
	return m
}

func TestSimulateToolCall_SuccessPayload(t *testing.T) {
	res := simulate(t, map[string]any{"name": "search", "duration_ms": float64(1), "fail": false})

	if res["tool"] != "search" {
		t.Fatalf("tool: want search, got %v", res["tool"])
	}
	if res["outcome"] != "success" {
		t.Fatalf("outcome: want success, got %v", res["outcome"])
	}
	if res["injected_failure"] != false {
		t.Fatalf("injected_failure: want false, got %v", res["injected_failure"])
	}
	// The marker lets the mock client tell a simulated result apart from a real
	// tool error; without it an injected failure would abort the workload.
	if res["mock_simulated"] != true {
		t.Fatalf("mock_simulated marker missing: %v", res)
	}
}

func TestSimulateToolCall_FailurePayloadIsNonFatal(t *testing.T) {
	res := simulate(t, map[string]any{"name": "search", "duration_ms": float64(1), "fail": true})

	// Even for an injected failure the handler returns a nil error and a
	// success-shaped payload so the surrounding workload keeps running.
	if res["outcome"] != "failure" {
		t.Fatalf("outcome: want failure, got %v", res["outcome"])
	}
	if res["injected_failure"] != true {
		t.Fatalf("injected_failure: want true, got %v", res["injected_failure"])
	}
	if res["status"] != "ok" {
		t.Fatalf("status: want ok, got %v", res["status"])
	}
}

func TestSimulateToolCall_DefaultName(t *testing.T) {
	// Missing name defaults to "tool". Uses a 1ms duration so the test is fast.
	res := simulate(t, map[string]any{"duration_ms": float64(1)})
	if res["tool"] != "tool" {
		t.Fatalf("default tool name: want tool, got %v", res["tool"])
	}
}

func TestSimulateToolCall_DurationClampAndDefault(t *testing.T) {
	// Pure helper, so the clamp is verified without actually sleeping 60s.
	cases := []struct {
		name string
		args map[string]any
		want int
	}{
		{"default", map[string]any{}, 100},
		{"explicit", map[string]any{"duration_ms": float64(250)}, 250},
		{"negative floors to 0", map[string]any{"duration_ms": float64(-5)}, 0},
		{"over-max clamps", map[string]any{"duration_ms": float64(maxSimulatedDurationMS + 5000)}, maxSimulatedDurationMS},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := effectiveDurationMS(tc.args); got != tc.want {
				t.Fatalf("effectiveDurationMS(%v): want %d, got %d", tc.args, tc.want, got)
			}
		})
	}
}

// TestSimulateToolCall_EmitsSpan installs an in-memory span recorder as the
// global tracer provider and asserts the handler emits a recording span named
// after the requested tool, carrying gen_ai.tool.name, and that an injected
// failure marks the span with error status. This is the criterion the tracing
// demo depends on.
func TestSimulateToolCall_EmitsSpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)

	tool := &SimulateToolCallTool{}
	if _, err := tool.Handler(context.Background(), map[string]any{"name": "search", "duration_ms": float64(1), "fail": true}); err != nil {
		t.Fatalf("Handler: %v", err)
	}
	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("want exactly 1 span, got %d", len(spans))
	}
	span := spans[0]

	if span.Name() != "tool.search" {
		t.Fatalf("span name: want tool.search, got %q", span.Name())
	}
	if span.Status().Code != codes.Error {
		t.Fatalf("span status: want Error for injected failure, got %v", span.Status().Code)
	}

	var gotToolName string
	for _, a := range span.Attributes() {
		if string(a.Key) == genAIToolNameKey {
			gotToolName = a.Value.AsString()
		}
	}
	if gotToolName != "search" {
		t.Fatalf("%s attribute: want search, got %q", genAIToolNameKey, gotToolName)
	}
}

func TestSimulateToolCall_HonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tool := &SimulateToolCallTool{}
	// A large duration would block, but the already-canceled context must make
	// the handler return promptly with an error.
	done := make(chan error, 1)
	go func() {
		_, err := tool.Handler(ctx, map[string]any{"name": "read", "duration_ms": float64(30000)})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected error on canceled context, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("handler did not honor context cancellation")
	}
}
