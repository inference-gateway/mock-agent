package mock

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	zap "go.uber.org/zap"
	observer "go.uber.org/zap/zaptest/observer"

	sdk "github.com/inference-gateway/sdk"
)

// newTestClient builds a MockLLMClient with an observable logger so tests
// can assert on emitted log fields.
func newTestClient() (*MockLLMClient, *observer.ObservedLogs) {
	core, recorded := observer.New(zap.InfoLevel)
	return &MockLLMClient{logger: zap.New(core)}, recorded
}

// allMockTools returns the toolset the agent registers at runtime (plus
// create_artifact from the default toolbox), so each test sees the same
// candidate set the real agent would see.
func allMockTools() []sdk.ChatCompletionTool {
	names := []string{"echo", "delay", "error", "random_data", "validate", "create_artifact"}
	tools := make([]sdk.ChatCompletionTool, 0, len(names))
	for _, name := range names {
		tools = append(tools, sdk.ChatCompletionTool{
			Type:     sdk.Function,
			Function: sdk.FunctionObject{Name: name},
		})
	}
	return tools
}

func userMsg(text string) []sdk.Message {
	return []sdk.Message{{
		Role:    sdk.User,
		Content: sdk.NewMessageContent(text),
	}}
}

// invoke runs CreateChatCompletion and returns the (single) tool call the
// mock chose, or nil when none was emitted.
func invoke(t *testing.T, c *MockLLMClient, messages []sdk.Message) *sdk.ChatCompletionMessageToolCall {
	t.Helper()
	resp, err := c.CreateChatCompletion(context.Background(), messages, allMockTools()...)
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	if len(resp.Choices) == 0 {
		t.Fatalf("no choices in response")
	}
	calls := resp.Choices[0].Message.ToolCalls
	if calls == nil || len(*calls) == 0 {
		return nil
	}
	return &(*calls)[0]
}

// argsMap unmarshals the JSON arguments of a tool call.
func argsMap(t *testing.T, call *sdk.ChatCompletionMessageToolCall) map[string]any {
	t.Helper()
	if call == nil {
		t.Fatalf("expected tool call, got nil")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &m); err != nil {
		t.Fatalf("unmarshal args %q: %v", call.Function.Arguments, err)
	}
	return m
}

// --- Group A: skill intent routing -----------------------------------------

func TestSkillRouting_ConnectivityCheck_RoutesToEcho(t *testing.T) {
	t.Parallel()
	cases := []string{
		"ping",
		"run a connectivity check",
		"connectivity-check",
		"healthcheck please",
		"are you up",
		"smoke test",
	}
	for _, msg := range cases {
		t.Run(msg, func(t *testing.T) {
			t.Parallel()
			client, logs := newTestClient()
			call := invoke(t, client, userMsg(msg))
			if call == nil {
				t.Fatalf("expected echo tool call, got nil")
			}
			if call.Function.Name != "echo" {
				t.Fatalf("expected echo, got %s (args=%s)", call.Function.Name, call.Function.Arguments)
			}
			args := argsMap(t, call)
			if _, ok := args["message"]; !ok {
				t.Fatalf("echo args missing 'message': %v", args)
			}
			assertSkillLogged(t, logs, "connectivity-check")
		})
	}
}

func TestSkillRouting_ErrorInjection_RoutesToError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		msg         string
		wantErrType string
	}{
		{"test error handling", "validation"},
		{"trigger an error with timeout", "timeout"},
		{"trigger error - not found failure", "not_found"},
		{"simulate failure: internal server", "internal"},
		{"error injection please", "validation"},
	}
	for _, tc := range cases {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			client, logs := newTestClient()
			call := invoke(t, client, userMsg(tc.msg))
			if call == nil || call.Function.Name != "error" {
				t.Fatalf("expected error tool call, got %+v", call)
			}
			args := argsMap(t, call)
			if got := args["error_type"]; got != tc.wantErrType {
				t.Fatalf("error_type: want %q, got %v", tc.wantErrType, got)
			}
			assertSkillLogged(t, logs, "error-injection")
		})
	}
}

func TestSkillRouting_LoadSimulation_FirstStepIsRandomData(t *testing.T) {
	t.Parallel()
	cases := []string{
		"run load simulation",
		"slow response with json",
		"simulate latency",
	}
	for _, msg := range cases {
		t.Run(msg, func(t *testing.T) {
			t.Parallel()
			client, logs := newTestClient()
			call := invoke(t, client, userMsg(msg))
			if call == nil || call.Function.Name != "random_data" {
				t.Fatalf("expected random_data tool call, got %+v", call)
			}
			args := argsMap(t, call)
			if _, ok := args["data_type"]; !ok {
				t.Fatalf("random_data args missing 'data_type': %v", args)
			}
			assertSkillLogged(t, logs, "load-simulation")
		})
	}
}

// --- Group B: load-simulation continuation ---------------------------------

func TestSkillRouting_LoadSimulation_SecondStepIsDelay(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient()
	messages := []sdk.Message{
		{Role: sdk.User, Content: sdk.NewMessageContent("run load simulation with 5 json records")},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"status":"success","data":[1,2,3]}`)},
	}
	call := invoke(t, client, messages)
	if call == nil || call.Function.Name != "delay" {
		t.Fatalf("expected delay as step 2, got %+v", call)
	}
	args := argsMap(t, call)
	if _, ok := args["duration_seconds"]; !ok {
		t.Fatalf("delay args missing 'duration_seconds': %v", args)
	}
}

// --- Group C: direct tool intent (fallback path) ---------------------------

func TestDirectIntent_Validate_UsesValidationTypeNotPattern(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient()
	call := invoke(t, client, userMsg("validate this email foo@bar.com"))
	if call == nil || call.Function.Name != "validate" {
		t.Fatalf("expected validate tool call, got %+v", call)
	}
	args := argsMap(t, call)
	if got := args["validation_type"]; got != "email" {
		t.Fatalf("validation_type: want \"email\", got %v (full args=%v)", got, args)
	}
	if _, present := args["pattern"]; present {
		t.Fatalf("validate args must not carry legacy 'pattern' key: %v", args)
	}
	if _, ok := args["input"]; !ok {
		t.Fatalf("validate args missing 'input': %v", args)
	}
}

func TestDirectIntent_Validate_UrlVariant(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient()
	call := invoke(t, client, userMsg("is valid url https://example.io"))
	if call == nil || call.Function.Name != "validate" {
		t.Fatalf("expected validate tool call, got %+v", call)
	}
	args := argsMap(t, call)
	if got := args["validation_type"]; got != "url" {
		t.Fatalf("validation_type: want \"url\", got %v", got)
	}
}

func TestDirectIntent_Delay(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient()
	call := invoke(t, client, userMsg("delay 5 seconds"))
	if call == nil || call.Function.Name != "delay" {
		t.Fatalf("expected delay tool call, got %+v", call)
	}
	args := argsMap(t, call)
	if got := args["duration_seconds"]; got != 5.0 {
		t.Fatalf("duration_seconds: want 5, got %v", got)
	}
}

func TestDirectIntent_RandomData(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient()
	call := invoke(t, client, userMsg("generate 3 uuids"))
	if call == nil || call.Function.Name != "random_data" {
		t.Fatalf("expected random_data tool call, got %+v", call)
	}
	args := argsMap(t, call)
	if got := args["data_type"]; got != "uuid" {
		t.Fatalf("data_type: want \"uuid\", got %v", got)
	}
	if got := args["count"]; got != 3.0 {
		t.Fatalf("count: want 3, got %v", got)
	}
}

// --- Group D: workflow completion -------------------------------------------

func TestSkillRouting_ConnectivityCheck_CompletesAfterEcho(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient()
	messages := []sdk.Message{
		{Role: sdk.User, Content: sdk.NewMessageContent("ping")},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"status":"success","echo":"ping"}`)},
	}
	call := invoke(t, client, messages)
	if call != nil {
		t.Fatalf("expected no tool calls after workflow complete, got %+v", call)
	}
}

func TestSkillRouting_LoadSimulation_CompletesAfterBothTools(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient()
	messages := []sdk.Message{
		{Role: sdk.User, Content: sdk.NewMessageContent("run load simulation")},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"status":"success","data":[]}`)},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"status":"success","elapsed":2}`)},
	}
	call := invoke(t, client, messages)
	if call != nil {
		t.Fatalf("expected no tool calls after both load-sim steps, got %+v", call)
	}
}

// --- detectSkill unit tests -------------------------------------------------

func TestDetectSkill_KnownTriggers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		msg       string
		wantSkill string
	}{
		{"ping", "connectivity-check"},
		{"healthcheck", "connectivity-check"},
		{"connectivity-check", "connectivity-check"},
		{"test error handling", "error-injection"},
		{"trigger error", "error-injection"},
		{"load simulation", "load-simulation"},
		{"slow response", "load-simulation"},
		{"hello world", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			got, _ := detectSkill(toLower(tc.msg))
			if got != tc.wantSkill {
				t.Fatalf("detectSkill(%q): want %q, got %q", tc.msg, tc.wantSkill, got)
			}
		})
	}
}

// --- SKILL.md read --------------------------------------------------------

func TestReadSkillDescription_ReturnsFrontmatterDescription(t *testing.T) {
	t.Parallel()
	// Tests run from the package directory (internal/mock), so reach back
	// to the repo root's skills/ tree for a real SKILL.md.
	desc, err := readSkillDescription("../../skills/connectivity-check/SKILL.md")
	if err != nil {
		t.Fatalf("readSkillDescription: %v", err)
	}
	if desc == "" {
		t.Fatalf("expected non-empty description, got empty")
	}
}

func TestReadSkillDescription_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := readSkillDescription("/nonexistent/path/SKILL.md")
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}

// --- Streaming response shape ---------------------------------------------

// drainStream consumes a streaming completion channel and returns the
// concatenated Delta.Content across all chunks.
func drainStream(t *testing.T, respChan <-chan *sdk.CreateChatCompletionStreamResponse, errChan <-chan error) string {
	t.Helper()
	var content string
	for {
		select {
		case resp, ok := <-respChan:
			if !ok {
				return content
			}
			if len(resp.Choices) > 0 {
				content += resp.Choices[0].Delta.Content
			}
		case err, ok := <-errChan:
			if ok && err != nil {
				t.Fatalf("stream error: %v", err)
			}
			return content
		}
	}
}

func TestStreaming_ConnectivityCheck_NotDoubleWrapped(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient()
	messages := []sdk.Message{
		{Role: sdk.User, Content: sdk.NewMessageContent("ping")},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"status":"success","echo":"ping"}`)},
	}
	respChan, errChan := client.CreateStreamingChatCompletion(context.Background(), messages, allMockTools()...)
	content := drainStream(t, respChan, errChan)
	if content == "" {
		t.Fatalf("expected non-empty streamed content")
	}
	// The bug produced two nested "This is a mock response to:" wrappers.
	// Either the skill-specific summary (preferred) or single-wrap is fine;
	// nested wrapping is the regression we're locking out.
	if strings.Contains(content, `This is a mock response to: "This is a mock response to:`) {
		t.Fatalf("streamed content is double-wrapped: %q", content)
	}
}

// --- Skill-aware completion summaries -------------------------------------

func TestSkillCompletion_ConnectivityCheck(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient()
	messages := []sdk.Message{
		{Role: sdk.User, Content: sdk.NewMessageContent("ping")},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"status":"success","echo":"ping"}`)},
	}
	resp, err := client.CreateChatCompletion(context.Background(), messages, allMockTools()...)
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	got := contentToString(resp.Choices[0].Message.Content)
	if !strings.Contains(got, "Connectivity check complete") {
		t.Fatalf("expected skill summary, got: %q", got)
	}
}

// TestSkillCompletion_ErrorInjection tests skillCompletionMessage directly
// because the error tool's actual tool result message contains the word
// "error", which the mock LLM client correctly treats as a tool failure
// (so end-to-end error-injection legitimately surfaces as a failed task,
// matching the skill's "fail by design" intent). The completion summary
// helper itself is still worth covering.
func TestSkillCompletion_ErrorInjection(t *testing.T) {
	t.Parallel()
	msg, ok := skillCompletionMessage("trigger an error")
	if !ok {
		t.Fatalf("expected skillCompletionMessage to match error-injection")
	}
	if !strings.Contains(msg, "Error injection complete") {
		t.Fatalf("unexpected summary text: %q", msg)
	}
}

func TestSkillCompletion_LoadSimulation(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient()
	messages := []sdk.Message{
		{Role: sdk.User, Content: sdk.NewMessageContent("run load simulation")},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"status":"success","data":[]}`)},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"status":"success","elapsed":2}`)},
	}
	resp, err := client.CreateChatCompletion(context.Background(), messages, allMockTools()...)
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	got := contentToString(resp.Choices[0].Message.Content)
	if !strings.Contains(got, "Load simulation complete") {
		t.Fatalf("expected skill summary, got: %q", got)
	}
}

func TestSkillCompletion_NoSkillFallback(t *testing.T) {
	t.Parallel()
	client, _ := newTestClient()
	// Generic message that doesn't match any skill, but has a tool result.
	messages := []sdk.Message{
		{Role: sdk.User, Content: sdk.NewMessageContent("hello world")},
		{Role: sdk.Assistant, Content: sdk.NewMessageContent("")},
		{Role: sdk.Tool, Content: sdk.NewMessageContent(`{"status":"success"}`)},
	}
	resp, err := client.CreateChatCompletion(context.Background(), messages, allMockTools()...)
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}
	got := contentToString(resp.Choices[0].Message.Content)
	if !strings.Contains(got, "This is a mock response to:") {
		t.Fatalf("expected legacy mock wrapping, got: %q", got)
	}
	if strings.Contains(got, `This is a mock response to: "This is a mock response to:`) {
		t.Fatalf("legacy fallback must still be single-wrap, got double-wrap: %q", got)
	}
}

// --- log assertion helper -------------------------------------------------

func assertSkillLogged(t *testing.T, logs *observer.ObservedLogs, wantSkill string) {
	t.Helper()
	entries := logs.FilterMessage("mock LLM matched skill via pattern").All()
	if len(entries) == 0 {
		t.Fatalf("expected at least one skill-match log entry, got none. all logs: %v", logs.All())
	}
	for _, e := range entries {
		for _, f := range e.Context {
			if f.Key == "skill" && f.String == wantSkill {
				return
			}
		}
	}
	t.Fatalf("no log entry carried skill=%q; entries: %v", wantSkill, entries)
}
