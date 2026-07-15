package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	attribute "go.opentelemetry.io/otel/attribute"
	codes "go.opentelemetry.io/otel/codes"

	server "github.com/inference-gateway/adk/server"
)

// SimulateToolCallName is the registered name of the simulate tool. The mock
// LLM client (internal/mock) emits calls to this name to build configurable
// multi-tool-call workloads for load/latency/failure testing and tracing demos.
const SimulateToolCallName = "simulate_tool_call"

// genAIToolNameKey is the OTel GenAI semantic-convention attribute for the
// invoked tool's name. startToolSpan (tools/telemetry.go) already stamps the
// tool-call id and session id; we add the tool name here so a simulated span
// carries gen_ai.tool.name exactly like a real provider's tool span would.
const genAIToolNameKey = "gen_ai.tool.name"

// maxSimulatedDurationMS caps the injected latency so a misconfigured workload
// (e.g. MOCK_TOOL_CALLS=read:99999999) can't wedge the agent loop.
const maxSimulatedDurationMS = 60000

// SimulateToolCallTool simulates a single tool call. Driven repeatedly by the
// mock LLM client, it turns a task into a realistic multi-tool-call workload:
// each invocation emits its own instrumented span with a configurable
// gen_ai.tool.name, a configurable duration, and an optional error status.
type SimulateToolCallTool struct{}

// NewSimulateToolCallTool creates the simulate_tool_call tool.
func NewSimulateToolCallTool() server.Tool {
	tool := &SimulateToolCallTool{}
	return server.NewBasicTool(
		SimulateToolCallName,
		"Simulate a single tool call for load, latency and failure testing. Emits an instrumented span (gen_ai.tool.name) with a configurable duration and optional error status. The mock LLM drives this once per entry of a multi-tool-call workload.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"description": "Tool name reported as gen_ai.tool.name and the span suffix (tool.<name>), e.g. read or search.",
					"type":        "string",
				},
				"duration_ms": map[string]any{
					"description": "Simulated latency in milliseconds (default 100, capped at 60000).",
					"type":        "number",
				},
				"fail": map[string]any{
					"description": "When true, mark the emitted span with error status. This is non-fatal: the surrounding workload continues so the full trace is produced.",
					"type":        "boolean",
				},
			},
		},
		tool.Handler,
	)
}

// Handler executes one simulated tool call: it opens a span named after the
// requested tool, waits for the configured latency, optionally records an
// error on the span, and returns a JSON payload describing the outcome. The
// payload is tagged with "mock_simulated":true so the mock LLM client can tell
// an injected failure apart from a genuine tool error (and keep the workload
// going rather than aborting the task).
func (t *SimulateToolCallTool) Handler(ctx context.Context, args map[string]any) (string, error) {
	name := "tool"
	if v, ok := args["name"].(string); ok && v != "" {
		name = v
	}

	span := startToolSpan(ctx, name)
	defer span.End()
	span.SetAttributes(attribute.String(genAIToolNameKey, name))

	durationMS := effectiveDurationMS(args)
	span.SetAttributes(attribute.Int("mock.simulated_duration_ms", durationMS))

	fail := false
	if v, ok := args["fail"].(bool); ok {
		fail = v
	}
	span.SetAttributes(attribute.Bool("mock.injected_failure", fail))

	start := time.Now()
	select {
	case <-time.After(time.Duration(durationMS) * time.Millisecond):
	case <-ctx.Done():
		err := fmt.Errorf("simulated tool call %q canceled: %w", name, ctx.Err())
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}
	elapsed := time.Since(start)

	outcome := "success"
	if fail {
		outcome = "failure"
		injected := fmt.Errorf("injected failure for simulated tool %q", name)
		span.RecordError(injected)
		span.SetStatus(codes.Error, injected.Error())
	}

	result := map[string]any{
		"status":           "ok",
		"tool":             name,
		"outcome":          outcome,
		"injected_failure": fail,
		"duration_ms":      durationMS,
		"elapsed_ms":       elapsed.Milliseconds(),
		"mock_simulated":   true,
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("encode simulate result: %w", err)
	}
	return string(payload), nil
}

// effectiveDurationMS resolves the simulated latency from the args, applying
// the default and clamping into [0, maxSimulatedDurationMS]. Kept separate from
// the sleeping Handler so the clamp is unit-testable without a real delay.
func effectiveDurationMS(args map[string]any) int {
	durationMS := 100
	if v, ok := args["duration_ms"]; ok {
		if ms, ok := toMilliseconds(v); ok {
			durationMS = ms
		}
	}
	if durationMS < 0 {
		return 0
	}
	if durationMS > maxSimulatedDurationMS {
		return maxSimulatedDurationMS
	}
	return durationMS
}

// toMilliseconds coerces a JSON-decoded numeric argument (float64 over the
// wire, but int-friendly for direct callers) into an int millisecond value.
func toMilliseconds(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float32:
		return int(x), true
	case float64:
		return int(x), true
	}
	return 0, false
}
