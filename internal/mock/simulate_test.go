package mock

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	sdk "github.com/inference-gateway/sdk"

	tools "github.com/inference-gateway/mock-agent/tools"
)

// simToolset is the runtime toolset plus the simulate tool, so routing tests
// see the same candidate set the real agent registers.
func simToolset() []sdk.ChatCompletionTool {
	set := allMockTools()
	return append(set, sdk.ChatCompletionTool{
		Type:     sdk.Function,
		Function: sdk.FunctionObject{Name: tools.SimulateToolCallName},
	})
}

// stepSimulation drives the mock through a whole workload for userMessage,
// feeding each emitted simulate call back as a (simulated) tool result until
// the mock stops emitting calls. It returns the ordered simulate-call args.
func stepSimulation(t *testing.T, c *MockLLMClient, userMessage string) []map[string]any {
	t.Helper()
	messages := []sdk.Message{{Role: sdk.User, Content: sdk.NewMessageContent(userMessage)}}
	var calls []map[string]any
	for i := 0; i < 64; i++ {
		resp, err := c.CreateChatCompletion(context.Background(), messages, simToolset()...)
		if err != nil {
			t.Fatalf("CreateChatCompletion: %v", err)
		}
		tc := resp.Choices[0].Message.ToolCalls
		if tc == nil || len(*tc) == 0 {
			return calls
		}
		call := (*tc)[0]
		if call.Function.Name != tools.SimulateToolCallName {
			t.Fatalf("step %d: expected %s call, got %q", i, tools.SimulateToolCallName, call.Function.Name)
		}
		var args map[string]any
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			t.Fatalf("unmarshal args %q: %v", call.Function.Arguments, err)
		}
		calls = append(calls, args)
		// Append the assistant tool call + a matching simulated tool result and
		// loop, mirroring how the ADK feeds results back into the mock.
		messages = append(messages,
			sdk.Message{Role: sdk.Assistant, Content: sdk.NewMessageContent(""), ToolCalls: tc},
			sdk.Message{Role: sdk.Tool, Content: sdk.NewMessageContent(simulatedToolResult(args))},
		)
	}
	t.Fatalf("simulation did not terminate for %q", userMessage)
	return nil
}

// simulatedToolResult fabricates what the real simulate tool would return for
// the given call args (the marker is what keeps an injected failure non-fatal).
func simulatedToolResult(args map[string]any) string {
	outcome := "success"
	if f, _ := args["fail"].(bool); f {
		outcome = "failure"
	}
	return `{"status":"ok","tool":"` + toStr(args["name"]) + `","outcome":"` + outcome + `","injected_failure":false,"mock_simulated":true}`
}

func toStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func TestSimulationPrompt_CountDrivesToolCalls(t *testing.T) {
	client, _ := newTestClient()
	calls := stepSimulation(t, client, "simulate 3 tool calls")
	if len(calls) != 3 {
		t.Fatalf("want 3 simulate calls, got %d (%v)", len(calls), calls)
	}
	// Names cycle through the default set for a bare count.
	wantNames := []string{"read", "search", "fetch"}
	for i, want := range wantNames {
		if got := toStr(calls[i]["name"]); got != want {
			t.Fatalf("call %d name: want %q, got %q", i, want, got)
		}
	}
}

func TestSimulationPrompt_ExplicitNames(t *testing.T) {
	client, _ := newTestClient()
	calls := stepSimulation(t, client, "simulate tool calls: read, search, write")
	if len(calls) != 3 {
		t.Fatalf("want 3 calls, got %d", len(calls))
	}
	for i, want := range []string{"read", "search", "write"} {
		if got := toStr(calls[i]["name"]); got != want {
			t.Fatalf("call %d name: want %q, got %q", i, want, got)
		}
	}
}

func TestSimulationPrompt_FailureIntentMarksLastCall(t *testing.T) {
	client, _ := newTestClient()
	calls := stepSimulation(t, client, "simulate 3 tool calls with a failure")
	if len(calls) != 3 {
		t.Fatalf("want 3 calls, got %d", len(calls))
	}
	if calls[0]["fail"] != false || calls[1]["fail"] != false {
		t.Fatalf("only the last call should fail, got %v", calls)
	}
	if calls[2]["fail"] != true {
		t.Fatalf("last call should be marked fail=true, got %v", calls[2])
	}
}

func TestSimulationPrompt_FailAll(t *testing.T) {
	client, _ := newTestClient()
	calls := stepSimulation(t, client, "simulate 2 tool calls, all should fail")
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d", len(calls))
	}
	for i, c := range calls {
		if c["fail"] != true {
			t.Fatalf("call %d should fail, got %v", i, c)
		}
	}
}

func TestSimulationPrompt_SlowRaisesDuration(t *testing.T) {
	client, _ := newTestClient()
	calls := stepSimulation(t, client, "simulate 2 slow tool calls")
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d", len(calls))
	}
	// First call uses the slow base duration (500ms) not the default (100ms).
	if got := calls[0]["duration_ms"].(float64); got < 500 {
		t.Fatalf("slow duration: want >=500, got %v", got)
	}
}

func TestSimulationPrompt_CompletionSummary(t *testing.T) {
	client, _ := newTestClient()
	messages := []sdk.Message{
		{Role: sdk.User, Content: sdk.NewMessageContent("simulate 2 tool calls")},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"tool":"read","mock_simulated":true}`)},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"tool":"search","mock_simulated":true}`)},
	}
	resp, err := client.CreateChatCompletion(context.Background(), messages, simToolset()...)
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	if tc := resp.Choices[0].Message.ToolCalls; tc != nil && len(*tc) > 0 {
		t.Fatalf("expected no further calls after plan complete, got %v", *tc)
	}
	got := contentToString(resp.Choices[0].Message.Content)
	if !strings.Contains(got, "Simulated 2 tool call(s)") {
		t.Fatalf("expected simulation summary, got %q", got)
	}
}

func TestSimulationEnv_MockToolCalls(t *testing.T) {
	t.Setenv("MOCK_TOOL_CALLS", "read,search:300!,read")
	client, _ := newTestClient()
	// A generic prompt (no skill/read/simulate keyword) picks up the env plan.
	calls := stepSimulation(t, client, "do some work")
	if len(calls) != 3 {
		t.Fatalf("want 3 env-driven calls, got %d (%v)", len(calls), calls)
	}
	if toStr(calls[1]["name"]) != "search" {
		t.Fatalf("call 1 name: want search, got %v", calls[1]["name"])
	}
	if calls[1]["duration_ms"].(float64) != 300 {
		t.Fatalf("call 1 duration: want 300, got %v", calls[1]["duration_ms"])
	}
	if calls[1]["fail"] != true {
		t.Fatalf("call 1 should be marked fail (trailing !), got %v", calls[1])
	}
	if calls[0]["fail"] != false || calls[2]["fail"] != false {
		t.Fatalf("only the ! call should fail, got %v", calls)
	}
}

func TestSimulationEnv_DefaultDurationOverride(t *testing.T) {
	t.Setenv("MOCK_TOOL_CALLS", "read,write")
	t.Setenv("MOCK_TOOL_CALL_DURATION_MS", "250")
	client, _ := newTestClient()
	calls := stepSimulation(t, client, "anything")
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d", len(calls))
	}
	for i, c := range calls {
		if c["duration_ms"].(float64) != 250 {
			t.Fatalf("call %d duration: want 250, got %v", i, c["duration_ms"])
		}
	}
}

// TestSimulationEnv_ReadTriggerStillWins verifies an explicit "read <path>"
// request is honored even when a global MOCK_TOOL_CALLS default is set.
func TestSimulationEnv_ReadTriggerStillWins(t *testing.T) {
	t.Setenv("MOCK_TOOL_CALLS", "read,search")
	client, _ := newTestClient()
	call := invoke(t, client, userMsg("read go.mod"))
	if call == nil || call.Function.Name != "Read" {
		t.Fatalf("expected Read tool call to win over env plan, got %+v", call)
	}
}

// TestSimulationEnv_Unset confirms no simulation happens without the env var
// and without a simulate prompt (guards against changing default behavior).
func TestSimulationEnv_Unset(t *testing.T) {
	client, _ := newTestClient()
	call := invoke(t, client, userMsg("hello there"))
	if call != nil && call.Function.Name == tools.SimulateToolCallName {
		t.Fatalf("did not expect a simulate call without env/prompt, got %+v", call)
	}
}

// TestSimulationEnv_InjectedFailureDoesNotAbort proves an injected failure in
// the middle of a workload does not abort the task: a mid-sequence simulated
// failure result must not surface as an error from CreateChatCompletion.
func TestSimulationEnv_InjectedFailureDoesNotAbort(t *testing.T) {
	t.Setenv("MOCK_TOOL_CALLS", "read,search!,write")
	client, _ := newTestClient()
	calls := stepSimulation(t, client, "run the workload")
	if len(calls) != 3 {
		t.Fatalf("failure injection aborted the workload early: got %d calls (%v)", len(calls), calls)
	}
}

// TestGenuineToolErrorStillAborts ensures the marker-based suppression is
// scoped to simulated results: a real tool error (no marker) still fails fast.
func TestGenuineToolErrorStillAborts(t *testing.T) {
	client, _ := newTestClient()
	messages := []sdk.Message{
		{Role: sdk.User, Content: sdk.NewMessageContent("do work")},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"error":"internal error occurred"}`)},
	}
	_, err := client.CreateChatCompletion(context.Background(), messages, simToolset()...)
	if err == nil {
		t.Fatalf("expected a genuine (unmarked) tool error to abort, got nil")
	}
}

func TestParseToolCallSpec(t *testing.T) {
	cases := []struct {
		entry    string
		wantName string
		wantDur  int
		wantFail bool
		wantOK   bool
	}{
		{"read", "read", 100, false, true},
		{"read:150", "read", 150, false, true},
		{"search:300!", "search", 300, true, true},
		{"write!", "write", 100, true, true},
		{" fetch : 42 ", "fetch", 42, false, true},
		{"", "", 0, false, false},
		{"!", "", 0, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.entry, func(t *testing.T) {
			spec, ok := parseToolCallSpec(tc.entry, 100)
			if ok != tc.wantOK {
				t.Fatalf("ok: want %v, got %v", tc.wantOK, ok)
			}
			if !ok {
				return
			}
			if spec.name != tc.wantName || spec.durationMS != tc.wantDur || spec.fail != tc.wantFail {
				t.Fatalf("got %+v, want {name:%q dur:%d fail:%v}", spec, tc.wantName, tc.wantDur, tc.wantFail)
			}
		})
	}
}

// TestSimulation_EndToEnd_ExecutesRealSimulateTool wires the mock LLM client to
// a toolbox holding the same generated simulate_tool_call built-in main.go
// registers, then drives a whole env-configured workload, executing each
// emitted call against the real handler. It proves the chain the multi-tool
// simulation relies on: the mock emits calls under the exact name the toolbox
// registered, the real handler runs (opening its span) and returns the
// "mock_simulated" marker, and the injected failure on the middle call does not
// abort the workload - all three calls run. A name/arg mismatch here would
// silently skip the tool (and its span) at runtime, which no pure-unit test of
// the mock would catch.
func TestSimulation_EndToEnd_ExecutesRealSimulateTool(t *testing.T) {
	t.Setenv("MOCK_TOOL_CALLS", "read:1,search:1!,write:1")

	ctx := context.Background()
	toolBox := server.NewToolBox()
	toolBox.AddTool(tools.NewSimulateToolCallTool())

	client := NewMockLLMClient(zap.NewNop())

	messages := []sdk.Message{{Role: sdk.User, Content: sdk.NewMessageContent("run the workload")}}
	var executed []string
	for i := 0; i < 16; i++ {
		resp, err := client.CreateChatCompletion(ctx, messages, toolBox.GetTools()...)
		if err != nil {
			t.Fatalf("CreateChatCompletion: %v", err)
		}
		calls := resp.Choices[0].Message.ToolCalls
		if calls == nil || len(*calls) == 0 {
			break
		}
		call := (*calls)[0]
		if !toolBox.HasTool(call.Function.Name) {
			t.Fatalf("toolbox has no tool named %q (mock route vs. registration mismatch)", call.Function.Name)
		}
		var args map[string]any
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			t.Fatalf("unmarshal args: %v", err)
		}
		out, err := toolBox.ExecuteTool(ctx, call.Function.Name, args)
		if err != nil {
			t.Fatalf("ExecuteTool(%q): %v", call.Function.Name, err)
		}
		if !strings.Contains(out, `"mock_simulated":true`) {
			t.Fatalf("real handler output missing marker: %q", out)
		}
		executed = append(executed, toStr(args["name"]))
		messages = append(messages,
			sdk.Message{Role: sdk.Assistant, Content: sdk.NewMessageContent(""), ToolCalls: calls},
			sdk.Message{Role: sdk.Tool, Content: sdk.NewMessageContent(out)},
		)
	}

	want := []string{"read", "search", "write"}
	if len(executed) != len(want) {
		t.Fatalf("executed %v, want %v (injected failure must not abort)", executed, want)
	}
	for i, name := range want {
		if executed[i] != name {
			t.Fatalf("call %d: executed %q, want %q", i, executed[i], name)
		}
	}
}

func TestFirstInt(t *testing.T) {
	cases := []struct {
		s    string
		def  int
		want int
	}{
		{"simulate 5 tool calls", 3, 5},
		{"simulate tool calls", 3, 3},
		{"do 12 things", 1, 12},
		{"none here", 7, 7},
	}
	for _, tc := range cases {
		if got := firstInt(tc.s, tc.def); got != tc.want {
			t.Fatalf("firstInt(%q,%d): want %d, got %d", tc.s, tc.def, tc.want, got)
		}
	}
}
