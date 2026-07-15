package mock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	zap "go.uber.org/zap"
	yaml "gopkg.in/yaml.v3"

	server "github.com/inference-gateway/adk/server"
	sdk "github.com/inference-gateway/sdk"

	tools "github.com/inference-gateway/mock-agent/tools"
)

type MockLLMClient struct {
	logger *zap.Logger
}

func NewMockLLMClient(logger *zap.Logger) server.LLMClient {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MockLLMClient{logger: logger}
}

// skillWorkflow defines the ordered tool sequence the mock executes when a
// user message matches one of the agent's known skills. Single-tool skills
// list one step; multi-tool skills (load-simulation) list each step in the
// order the SKILL.md prescribes.
type skillWorkflow struct {
	name  string
	steps []string
}

var skillWorkflows = map[string]skillWorkflow{
	"connectivity-check": {name: "connectivity-check", steps: []string{"echo"}},
	"error-injection":    {name: "error-injection", steps: []string{"error"}},
	"load-simulation":    {name: "load-simulation", steps: []string{"random_data", "delay"}},
}

var skillTriggers = map[string][]string{
	"connectivity-check": {
		"connectivity-check", "connectivity check", "connectivity",
		"ping", "are you up", "are you there", "healthcheck",
		"health check", "smoke test", "round-trip", "round trip",
	},
	"error-injection": {
		"error-injection", "error injection",
		"test error", "test errors", "test failure",
		"simulate failure", "simulate error", "simulate an error",
		"trigger error", "trigger an error", "throw an error",
		"error handling", "failure mode",
		"timeout error", "internal error", "not_found failure",
		"not found failure",
	},
	"load-simulation": {
		"load-simulation", "load simulation",
		"load test", "slow response", "simulate latency",
		"simulate slow", "delayed payload", "delay with data",
		"delayed response",
	},
}

var skillDetectionOrder = []string{"connectivity-check", "error-injection", "load-simulation"}

// detectSkill returns the skill name implied by the user message and the
// trigger phrase that matched, or empty strings when no skill is detected.
func detectSkill(lowerMsg string) (skill, trigger string) {
	for _, name := range skillDetectionOrder {
		for _, t := range skillTriggers[name] {
			if contains(lowerMsg, t) {
				return name, t
			}
		}
	}
	return "", ""
}

func (m *MockLLMClient) CreateChatCompletion(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (*sdk.CreateChatCompletionResponse, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	lastMessage := messages[len(messages)-1]
	lastContent := contentToString(lastMessage.Content)

	userMessage := ""
	hasToolResults := false
	var toolError string
	for _, msg := range messages {
		if msg.Role == sdk.User {
			userMessage = contentToString(msg.Content)
		}
		if msg.Role == sdk.Tool {
			hasToolResults = true
			msgText := contentToString(msg.Content)
			if contains(toLower(msgText), "error") || contains(toLower(msgText), "failed") {
				toolError = msgText
			}
		}
	}

	if toolError != "" {
		return nil, fmt.Errorf("tool execution failed: %s", toolError)
	}

	var responseContent string
	var toolCalls *[]sdk.ChatCompletionMessageToolCall

	if len(tools) > 0 {
		calls := m.generateMockToolCalls(tools, userMessage, messages)
		if len(calls) > 0 {
			toolCalls = &calls
			responseContent = ""
		} else {
			responseContent = finalTextResponse(lastContent, userMessage, hasToolResults)
		}
	} else {
		responseContent = finalTextResponse(lastContent, userMessage, hasToolResults)
	}

	return &sdk.CreateChatCompletionResponse{
		ID:      "mock-" + generateID(),
		Model:   "mock-model",
		Object:  "chat.completion",
		Created: 1234567890,
		Choices: []sdk.ChatCompletionChoice{
			{
				Index: 0,
				Message: sdk.Message{
					Role:      sdk.Assistant,
					Content:   sdk.NewMessageContent(responseContent),
					ToolCalls: toolCalls,
				},
				FinishReason: sdk.Stop,
			},
		},
		Usage: &sdk.CompletionUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}, nil
}

func (m *MockLLMClient) CreateStreamingChatCompletion(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
	respChan := make(chan *sdk.CreateChatCompletionStreamResponse, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(respChan)
		defer close(errChan)

		if len(messages) == 0 {
			errChan <- fmt.Errorf("no messages provided")
			return
		}

		lastMessage := messages[len(messages)-1]
		lastContent := contentToString(lastMessage.Content)

		userMessage := ""
		hasToolResults := false
		var toolError string
		for _, msg := range messages {
			if msg.Role == sdk.User {
				userMessage = contentToString(msg.Content)
			}
			if msg.Role == sdk.Tool {
				hasToolResults = true
				msgText := contentToString(msg.Content)
				// A simulated tool result may carry an injected failure; it is
				// tagged so we don't mistake it for a genuine tool error and
				// abort the workload mid-way (the span already records the
				// error status).
				if !isSimulatedToolResult(msgText) && (contains(toLower(msgText), "error") || contains(toLower(msgText), "failed")) {
					toolError = msgText
				}
			}
		}

		if toolError != "" {
			errChan <- fmt.Errorf("tool execution failed: %s", toolError)
			return
		}

		if len(tools) > 0 {
			toolCalls := m.generateMockToolCalls(tools, userMessage, messages)
			if len(toolCalls) > 0 {
				for idx, toolCall := range toolCalls {
					toolCallID := toolCall.ID
					toolCallType := string(toolCall.Type)
					chunk := sdk.ChatCompletionMessageToolCallChunk{
						Index: idx,
						ID:    &toolCallID,
						Type:  &toolCallType,
						Function: &sdk.ChatCompletionMessageToolCallFunction{
							Name:      toolCall.Function.Name,
							Arguments: toolCall.Function.Arguments,
						},
					}

					chunks := []sdk.ChatCompletionMessageToolCallChunk{chunk}
					respChan <- &sdk.CreateChatCompletionStreamResponse{
						ID:      "mock-stream-" + generateID(),
						Model:   "mock-model",
						Object:  "chat.completion.chunk",
						Created: 1234567890,
						Choices: []sdk.ChatCompletionStreamChoice{
							{
								Index: 0,
								Delta: sdk.ChatCompletionStreamResponseDelta{
									ToolCalls: &chunks,
								},
								FinishReason: "",
							},
						},
					}
				}

				respChan <- &sdk.CreateChatCompletionStreamResponse{
					ID:      "mock-stream-" + generateID(),
					Model:   "mock-model",
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Choices: []sdk.ChatCompletionStreamChoice{
						{
							Index:        0,
							Delta:        sdk.ChatCompletionStreamResponseDelta{},
							FinishReason: sdk.ToolCalls,
						},
					},
				}
				return
			}
		}

		response := finalTextResponse(lastContent, userMessage, hasToolResults)

		respChan <- &sdk.CreateChatCompletionStreamResponse{
			ID:      "mock-stream-" + generateID(),
			Model:   "mock-model",
			Object:  "chat.completion.chunk",
			Created: 1234567890,
			Choices: []sdk.ChatCompletionStreamChoice{
				{
					Index: 0,
					Delta: sdk.ChatCompletionStreamResponseDelta{
						Content: response,
					},
					FinishReason: "",
				},
			},
		}

		respChan <- &sdk.CreateChatCompletionStreamResponse{
			ID:      "mock-stream-" + generateID(),
			Model:   "mock-model",
			Object:  "chat.completion.chunk",
			Created: 1234567890,
			Choices: []sdk.ChatCompletionStreamChoice{
				{
					Index: 0,
					Delta: sdk.ChatCompletionStreamResponseDelta{
						Content: "",
					},
					FinishReason: sdk.Stop,
				},
			},
		}
	}()

	return respChan, errChan
}

// finalTextResponse builds the text response returned when no tool call is
// emitted. When a skill is detected in the user message AND at least one
// tool result is in history, it returns a skill-specific completion summary
// (much cleaner than the generic mock wrapping). Otherwise it falls back to
// the legacy "This is a mock response to: ..." phrasing.
func finalTextResponse(lastContent, userMessage string, hasToolResults bool) string {
	if hasToolResults {
		if msg, ok := skillCompletionMessage(userMessage); ok {
			return msg
		}
		if plan := effectiveSimulationPlan(userMessage); plan != nil {
			return simulationCompletionMessage(plan)
		}
		if _, ok := readTriggerPath(userMessage); ok {
			return "File read complete. The Read tool returned the requested file contents."
		}
	}
	content := lastContent
	if hasToolResults && userMessage != "" {
		content = "Task completed successfully. I executed the requested operation based on: " + userMessage
	}
	return generateMockResponse(content)
}

// skillCompletionMessage returns a human-readable summary for whichever
// skill the user message matched, or "" / false if no skill matched. Used
// to keep the post-workflow text from being a noisy double-quoted wrapping
// of the previous message.
func skillCompletionMessage(userMessage string) (string, bool) {
	skill, _ := detectSkill(toLower(userMessage))
	switch skill {
	case "connectivity-check":
		return "Connectivity check complete. Echo round-trip succeeded.", true
	case "error-injection":
		return "Error injection complete. The error tool returned the requested failure.", true
	case "load-simulation":
		return "Load simulation complete. Random payload generated and delay applied.", true
	}
	return "", false
}

func generateMockResponse(userMessage string) string {
	return fmt.Sprintf("This is a mock response to: %q. I'm a mock agent designed for testing purposes.", userMessage)
}

// generateMockToolCalls picks the next tool to invoke. It first checks
// whether the user message matches one of the agent's skills; on a match
// it drives the skill's ordered workflow, advancing one step per call as
// tool results accumulate in the message history. Falls back to direct
// tool-keyword routing for ad-hoc requests like "validate this email".
func (m *MockLLMClient) generateMockToolCalls(tools []sdk.ChatCompletionTool, userMessage string, messages []sdk.Message) []sdk.ChatCompletionMessageToolCall {
	if len(tools) == 0 {
		return nil
	}

	lowerMsg := toLower(userMessage)

	if skill, trigger := detectSkill(lowerMsg); skill != "" {
		m.logSkillMatch(skill, trigger, userMessage)
		wf := skillWorkflows[skill]
		completed := countToolResults(messages)
		if completed >= len(wf.steps) {
			return nil
		}
		nextTool := wf.steps[completed]
		if call := buildToolCall(nextTool, userMessage, tools); call != nil {
			return []sdk.ChatCompletionMessageToolCall{*call}
		}
		return nil
	}

	// Multi-tool-call simulation: an explicit "simulate N tool calls" prompt or
	// a MOCK_TOOL_CALLS env default drives a configurable sequence of simulated
	// calls. This runs before the single-shot guard because a workload
	// intentionally spans several iterations (one tool call each), advancing as
	// tool results accumulate - so tool_calls / iterations reflect the plan.
	if plan := effectiveSimulationPlan(userMessage); plan != nil {
		completed := countToolResults(messages)
		if completed >= len(plan) {
			return nil
		}
		if call := buildSimulateToolCall(plan[completed], tools); call != nil {
			m.logSimulationStep(plan, completed, userMessage)
			return []sdk.ChatCompletionMessageToolCall{*call}
		}
		// simulate_tool_call is not registered in this toolset - fall through
		// to a text response rather than looping.
		return nil
	}

	// For non-skill messages, preserve legacy single-shot behavior: once any
	// tool result is in history, don't emit further tool calls - let the
	// caller produce a text response. Without this, fallback branches below
	// (artifact/echo/first-tool) keep firing after every result, looping the
	// agent indefinitely.
	if countToolResults(messages) > 0 {
		return nil
	}

	if path, ok := readTriggerPath(userMessage); ok {
		if call := buildReadToolCall(path, tools); call != nil {
			m.logReadTrigger(path, userMessage)
			return []sdk.ChatCompletionMessageToolCall{*call}
		}
	}

	if contains(lowerMsg, "error") || contains(lowerMsg, "fail") || contains(lowerMsg, "throw") {
		if call := buildToolCall("error", userMessage, tools); call != nil {
			return []sdk.ChatCompletionMessageToolCall{*call}
		}
	}

	if contains(lowerMsg, "delay") || contains(lowerMsg, "wait") || contains(lowerMsg, "sleep") || contains(lowerMsg, "pause") {
		if call := buildToolCall("delay", userMessage, tools); call != nil {
			return []sdk.ChatCompletionMessageToolCall{*call}
		}
	}

	if contains(lowerMsg, "validate") || contains(lowerMsg, "is valid") {
		if call := buildToolCall("validate", userMessage, tools); call != nil {
			return []sdk.ChatCompletionMessageToolCall{*call}
		}
	}

	if contains(lowerMsg, "artifact") || contains(lowerMsg, "create file") || contains(lowerMsg, "save file") {
		if call := buildArtifactCall(userMessage, tools); call != nil {
			return []sdk.ChatCompletionMessageToolCall{*call}
		}
	}

	if contains(lowerMsg, "random") || contains(lowerMsg, "generate") {
		if call := buildToolCall("random_data", userMessage, tools); call != nil {
			return []sdk.ChatCompletionMessageToolCall{*call}
		}
	}

	if call := buildArtifactCall(userMessage, tools); call != nil {
		return []sdk.ChatCompletionMessageToolCall{*call}
	}
	if call := buildToolCall("echo", userMessage, tools); call != nil {
		return []sdk.ChatCompletionMessageToolCall{*call}
	}

	emptyArgs, _ := json.Marshal(map[string]any{})
	return []sdk.ChatCompletionMessageToolCall{*newToolCall(tools[0].Function.Name, string(emptyArgs))}
}

// countToolResults counts how many tool result messages already exist in
// the conversation history. The mock treats this as the index of the next
// workflow step to emit.
func countToolResults(messages []sdk.Message) int {
	n := 0
	for _, msg := range messages {
		if msg.Role == sdk.Tool {
			n++
		}
	}
	return n
}

// buildToolCall constructs a tool call for the given tool name. Returns
// nil when the tool is not registered in the supplied tool list.
func buildToolCall(toolName, userMessage string, tools []sdk.ChatCompletionTool) *sdk.ChatCompletionMessageToolCall {
	if !hasToolNamed(tools, toolName) {
		return nil
	}
	switch toolName {
	case "echo":
		args, _ := json.Marshal(map[string]any{"message": userMessage})
		return newToolCall("echo", string(args))
	case "delay":
		args, _ := json.Marshal(map[string]any{
			"duration_seconds": extractDuration(userMessage),
			"message":          userMessage,
		})
		return newToolCall("delay", string(args))
	case "error":
		args, _ := json.Marshal(map[string]any{
			"error_type": extractErrorType(userMessage),
			"message":    userMessage,
		})
		return newToolCall("error", string(args))
	case "random_data":
		dataType, count := extractRandomDataParams(userMessage)
		args, _ := json.Marshal(map[string]any{
			"data_type": dataType,
			"count":     count,
		})
		return newToolCall("random_data", string(args))
	case "validate":
		args, _ := json.Marshal(map[string]any{
			"validation_type": extractValidationType(userMessage),
			"input":           userMessage,
		})
		return newToolCall("validate", string(args))
	}
	return nil
}

func buildArtifactCall(userMessage string, tools []sdk.ChatCompletionTool) *sdk.ChatCompletionMessageToolCall {
	if !hasToolNamed(tools, "create_artifact") {
		return nil
	}
	lower := toLower(userMessage)
	name := "sample-data.json"
	content := `{"id": 1, "name": "John Doe", "email": "john.doe@example.com"}`
	if contains(lower, "text") || contains(lower, "txt") {
		name = "sample-data.txt"
		content = "This is a sample text artifact created by the mock agent."
	} else if contains(lower, "csv") {
		name = "sample-data.csv"
		content = "id,name,email\n1,John Doe,john.doe@example.com\n2,Jane Smith,jane.smith@example.com"
	}
	args, _ := json.Marshal(map[string]any{
		"name":     name,
		"content":  content,
		"type":     "url",
		"filename": name,
	})
	return newToolCall("create_artifact", string(args))
}

// --- multi-tool-call simulation --------------------------------------------

// simulateToolName is the registered name of the simulate tool the mock drives
// once per entry of a workload plan. It mirrors tools.SimulateToolCallName so
// the mock's emitted call name and the toolbox registration stay in lockstep.
const simulateToolName = tools.SimulateToolCallName

// defaultSimDurationMS is the per-call latency used when neither a MOCK_TOOL_CALLS
// entry nor MOCK_TOOL_CALL_DURATION_MS specifies one.
const defaultSimDurationMS = 100

// slowSimDurationMS is the base latency used when a prompt asks for a "slow"
// workload.
const slowSimDurationMS = 500

// maxPromptSimCalls bounds how many calls a prompt like "simulate 999 tool
// calls" can request, so a stray number can't balloon the workload. Env-driven
// plans are not capped here (the agent loop's MaxChatCompletionIterations is
// the real ceiling).
const maxPromptSimCalls = 20

// toolCallSpec describes one simulated tool call in a workload plan.
type toolCallSpec struct {
	name       string
	durationMS int
	fail       bool
}

// simulateNameCycle supplies varied, realistic tool names when a prompt asks
// for a count but doesn't name the tools (e.g. "simulate 4 tool calls").
var simulateNameCycle = []string{"read", "search", "fetch", "write", "summarize"}

// effectiveSimulationPlan resolves the multi-tool-call workload for a task, or
// nil when none applies. An explicit "simulate N tool calls" prompt wins; then
// the MOCK_TOOL_CALLS env default, which is suppressed for explicit "read
// <path>" requests so the real Read demo still works with a global default set.
func effectiveSimulationPlan(userMessage string) []toolCallSpec {
	if plan := simulationPlanFromPrompt(userMessage); plan != nil {
		return plan
	}
	env := simulationPlanFromEnv()
	if env == nil {
		return nil
	}
	if _, isRead := readTriggerPath(userMessage); isRead {
		return nil
	}
	return env
}

// simulationPlanFromPrompt builds a plan when the user explicitly asks the mock
// to "simulate N tool calls" (or "... workload"), or nil otherwise. Knobs:
//   - count: the first integer in the message (default 3, clamped 1..20);
//   - names: an explicit comma list after a colon ("...calls: read, search")
//     overrides the count and the default name cycle;
//   - latency: "slow"/"latency" raises the base per-call duration;
//   - failures: "fail"/"error"/"failure" fails the last call, or every call
//     when combined with "all"/"every".
func simulationPlanFromPrompt(userMessage string) []toolCallSpec {
	lower := toLower(userMessage)
	if !contains(lower, "simulate") {
		return nil
	}
	if !contains(lower, "tool call") && !contains(lower, "workload") {
		return nil
	}

	names := explicitToolNames(userMessage)
	if len(names) == 0 {
		n := firstInt(userMessage, 3)
		if n < 1 {
			n = 1
		}
		if n > maxPromptSimCalls {
			n = maxPromptSimCalls
		}
		names = make([]string, n)
		for i := 0; i < n; i++ {
			names[i] = simulateNameCycle[i%len(simulateNameCycle)]
		}
	}

	base := defaultSimDurationMS
	if contains(lower, "slow") || contains(lower, "latency") {
		base = slowSimDurationMS
	}

	plan := make([]toolCallSpec, len(names))
	for i, name := range names {
		// Vary durations deterministically so the trace shows a realistic
		// spread rather than N identical spans.
		plan[i] = toolCallSpec{name: name, durationMS: base + (i%3)*(base/2)}
	}

	failIntent := contains(lower, "fail") || contains(lower, "error") || contains(lower, "failure")
	// Whole-word match: bare contains("all") would fire on "tool calls".
	failAll := failIntent && (containsWord(lower, "all") || containsWord(lower, "every"))
	switch {
	case failAll:
		for i := range plan {
			plan[i].fail = true
		}
	case failIntent && len(plan) > 0:
		plan[len(plan)-1].fail = true
	}
	return plan
}

// explicitToolNames returns the comma-separated tool names after the first
// colon in the message ("simulate tool calls: read, search, write"), or nil
// when there is no such list or any entry isn't a bare tool-name token.
func explicitToolNames(userMessage string) []string {
	i := strings.Index(userMessage, ":")
	if i < 0 {
		return nil
	}
	var names []string
	for _, part := range strings.Split(userMessage[i+1:], ",") {
		name := strings.TrimSpace(part)
		if !isSimpleToolName(name) {
			return nil
		}
		names = append(names, name)
	}
	return names
}

// isSimpleToolName reports whether s is a single bare tool-name token
// (letters, digits, '_', '-', '.'), so free-form prose after a colon isn't
// mistaken for a tool list.
func isSimpleToolName(s string) bool {
	if s == "" || len(s) > 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '_' || c == '-' || c == '.':
		default:
			return false
		}
	}
	return true
}

// simulationPlanFromEnv parses the MOCK_TOOL_CALLS env var into a plan, or nil
// when it is unset/empty. Each comma-separated entry is name[:duration_ms][!]:
// a bare name uses the default duration; ":150" sets the per-call latency in
// milliseconds; a trailing "!" injects a failure on that call. Example:
// MOCK_TOOL_CALLS=read,search:300!,read
func simulationPlanFromEnv() []toolCallSpec {
	raw := strings.TrimSpace(os.Getenv("MOCK_TOOL_CALLS"))
	if raw == "" {
		return nil
	}
	base := envDefaultDurationMS()
	var plan []toolCallSpec
	for _, part := range strings.Split(raw, ",") {
		if spec, ok := parseToolCallSpec(part, base); ok {
			plan = append(plan, spec)
		}
	}
	if len(plan) == 0 {
		return nil
	}
	return plan
}

// envDefaultDurationMS reads MOCK_TOOL_CALL_DURATION_MS, falling back to
// defaultSimDurationMS when unset or invalid.
func envDefaultDurationMS() int {
	v := strings.TrimSpace(os.Getenv("MOCK_TOOL_CALL_DURATION_MS"))
	if v == "" {
		return defaultSimDurationMS
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 0 {
		return n
	}
	return defaultSimDurationMS
}

// parseToolCallSpec parses one MOCK_TOOL_CALLS entry of the form
// name[:duration_ms][!]. Returns ok=false for an empty name.
func parseToolCallSpec(entry string, defaultDuration int) (toolCallSpec, bool) {
	entry = strings.TrimSpace(entry)
	fail := false
	if strings.HasSuffix(entry, "!") {
		fail = true
		entry = strings.TrimSpace(strings.TrimSuffix(entry, "!"))
	}
	name := entry
	duration := defaultDuration
	if i := strings.LastIndex(entry, ":"); i >= 0 {
		name = strings.TrimSpace(entry[:i])
		if d, err := strconv.Atoi(strings.TrimSpace(entry[i+1:])); err == nil && d >= 0 {
			duration = d
		}
	}
	if name == "" {
		return toolCallSpec{}, false
	}
	return toolCallSpec{name: name, durationMS: duration, fail: fail}, true
}

// buildSimulateToolCall constructs a call to the simulate tool for spec, or nil
// when the simulate tool is not registered in the supplied toolset.
func buildSimulateToolCall(spec toolCallSpec, tools []sdk.ChatCompletionTool) *sdk.ChatCompletionMessageToolCall {
	if !hasToolNamed(tools, simulateToolName) {
		return nil
	}
	args, _ := json.Marshal(map[string]any{
		"name":        spec.name,
		"duration_ms": spec.durationMS,
		"fail":        spec.fail,
	})
	return newToolCall(simulateToolName, string(args))
}

// simulationCompletionMessage summarizes a finished workload for the final
// assistant text.
func simulationCompletionMessage(plan []toolCallSpec) string {
	names := make([]string, len(plan))
	failures := 0
	for i, s := range plan {
		names[i] = s.name
		if s.fail {
			failures++
		}
	}
	return fmt.Sprintf("Simulated %d tool call(s): %s. Injected failures: %d.",
		len(plan), strings.Join(names, ", "), failures)
}

// isSimulatedToolResult reports whether a tool result came from the simulate
// tool (tagged with "mock_simulated":true). Used to keep an injected failure
// from being treated as a fatal tool error.
func isSimulatedToolResult(msgText string) bool {
	return contains(msgText, `"mock_simulated":true`) || contains(msgText, `"mock_simulated": true`)
}

// firstInt returns the first run of decimal digits in s as an int, or def when
// there is none.
func firstInt(s string, def int) int {
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			if n, err := strconv.Atoi(s[start:i]); err == nil {
				return n
			}
			start = -1
		}
	}
	if start >= 0 {
		if n, err := strconv.Atoi(s[start:]); err == nil {
			return n
		}
	}
	return def
}

// logSimulationStep records each simulated call the mock drives - handy when
// confirming a workload produced the expected span sequence in a trace.
func (m *MockLLMClient) logSimulationStep(plan []toolCallSpec, idx int, userMessage string) {
	spec := plan[idx]
	m.logger.Info("mock LLM simulating tool call",
		zap.String("tool", spec.name),
		zap.Int("step", idx+1),
		zap.Int("total_steps", len(plan)),
		zap.Int("duration_ms", spec.durationMS),
		zap.Bool("inject_failure", spec.fail),
		zap.String("user_message", userMessage),
	)
}

// readToolName is the registered name of the built-in Read tool (see
// tools/read.go). Unlike the mock tools it is PascalCase, so route to it by
// this exact name.
const readToolName = "Read"

// defaultReadPath is the file the mock reads when the agreed "read" phrase
// arrives without an explicit path. README.md always exists at the repo root,
// so the demo always has a real file to open (and a real tool span to emit).
const defaultReadPath = "README.md"

// readFiller are throwaway words the mock skips when hunting for the path in a
// "read the file X" style phrase, so the extracted path is X and not "the".
var readFiller = map[string]bool{
	"the": true, "a": true, "an": true, "file": true, "files": true,
	"contents": true, "content": true, "of": true, "please": true,
	"from": true, "in": true, "at": true, "me": true, "this": true,
	"that": true, "for": true, "and": true, "its": true, "you": true,
	"can": true, "could": true, "now": true,
}

// readTriggerPath implements the mock's agreed "read" phrase: when "read"
// appears as a standalone token in the user message it returns the file path to
// hand to the Read tool (and true). Matching a whole token - not a bare
// substring - keeps words like "already", "thread" and "reading" from firing
// it. An explicit path-like token (one carrying a separator or extension) wins;
// otherwise the first non-filler word after the keyword is used, falling back to
// defaultReadPath when the keyword stands alone. Casing is preserved so the path
// resolves as typed.
func readTriggerPath(userMessage string) (string, bool) {
	tokens := strings.Fields(userMessage)
	readIdx := -1
	for i, tok := range tokens {
		if toLower(trimPathToken(tok)) == "read" {
			readIdx = i
			break
		}
	}
	if readIdx < 0 {
		return "", false
	}

	rest := tokens[readIdx+1:]
	for _, tok := range rest {
		if cand := trimPathToken(tok); looksLikePath(cand) {
			return cand, true
		}
	}
	for _, tok := range rest {
		cand := trimPathToken(tok)
		if cand != "" && !readFiller[toLower(cand)] {
			return cand, true
		}
	}
	return defaultReadPath, true
}

// trimPathToken strips surrounding quotes/brackets and trailing sentence
// punctuation from a token while preserving a leading "./" or "../" and any
// interior dots, so `"README.md",` -> `README.md` and `./main.go` is untouched.
func trimPathToken(tok string) string {
	tok = strings.Trim(tok, "\"'`()[]{}<>")
	tok = strings.TrimRight(tok, ".,;:!?")
	return tok
}

// looksLikePath reports whether a token is confidently a file path: it either
// carries a path separator or has a non-empty extension (a dot with text on
// both sides).
func looksLikePath(s string) bool {
	if s == "" {
		return false
	}
	if strings.ContainsAny(s, "/\\") {
		return true
	}
	if i := strings.LastIndex(s, "."); i > 0 && i < len(s)-1 {
		return true
	}
	return false
}

// buildReadToolCall builds a call to the built-in Read tool for path, or nil
// when the Read tool is not in the supplied toolset (so callers fall through to
// the other routes).
func buildReadToolCall(path string, tools []sdk.ChatCompletionTool) *sdk.ChatCompletionMessageToolCall {
	if !hasToolNamed(tools, readToolName) {
		return nil
	}
	args, _ := json.Marshal(map[string]any{"file_path": path})
	return newToolCall(readToolName, string(args))
}

// logReadTrigger records that the mock routed a request through the real Read
// built-in via the agreed phrase - handy when confirming a tool span was
// exercised in a trace.
func (m *MockLLMClient) logReadTrigger(path, userMessage string) {
	m.logger.Info("mock LLM matched read trigger via pattern",
		zap.String("tool", readToolName),
		zap.String("file_path", path),
		zap.String("user_message", userMessage),
	)
}

func extractErrorType(userMessage string) string {
	lower := toLower(userMessage)
	switch {
	case contains(lower, "timeout"):
		return "timeout"
	case contains(lower, "internal") || contains(lower, "server"):
		return "internal"
	case contains(lower, "not found") || contains(lower, "not_found") || contains(lower, "404"):
		return "not_found"
	default:
		return "validation"
	}
}

func extractDuration(userMessage string) float64 {
	lower := toLower(userMessage)
	switch {
	case contains(lower, "10"):
		return 10.0
	case contains(lower, "5"):
		return 5.0
	case contains(lower, "3"):
		return 3.0
	default:
		return 2.0
	}
}

func extractRandomDataParams(userMessage string) (string, int) {
	lower := toLower(userMessage)
	dataType := "uuid"
	switch {
	case contains(lower, "email"):
		dataType = "email"
	case contains(lower, "name"):
		dataType = "name"
	case contains(lower, "number"):
		dataType = "number"
	case contains(lower, "json"):
		dataType = "json"
	}
	count := 5
	switch {
	case contains(lower, "10"):
		count = 10
	case contains(lower, "3"):
		count = 3
	case contains(lower, "1") && !contains(lower, "10"):
		count = 1
	}
	return dataType, count
}

func extractValidationType(userMessage string) string {
	lower := toLower(userMessage)
	switch {
	case contains(lower, "url") || contains(lower, "http"):
		return "url"
	case contains(lower, "json"):
		return "json"
	case contains(lower, "uuid"):
		return "uuid"
	case contains(lower, "phone"):
		return "phone"
	default:
		return "email"
	}
}

func hasToolNamed(tools []sdk.ChatCompletionTool, name string) bool {
	for _, t := range tools {
		if t.Function.Name == name {
			return true
		}
	}
	return false
}

func newToolCall(name, arguments string) *sdk.ChatCompletionMessageToolCall {
	return &sdk.ChatCompletionMessageToolCall{
		ID:   "call-" + generateID(),
		Type: sdk.Function,
		Function: sdk.ChatCompletionMessageToolCallFunction{
			Name:      name,
			Arguments: arguments,
		},
	}
}

// logSkillMatch emits an info-level log line whenever the mock LLM routes
// a request to a known skill. It also best-effort reads the skill's
// SKILL.md frontmatter so the log carries the skill's own description -
// useful when debugging which skill a request was assigned to.
func (m *MockLLMClient) logSkillMatch(skill, trigger, userMessage string) {
	path := filepath.Join("skills", skill, "SKILL.md")
	desc, readErr := readSkillDescription(path)
	fields := []zap.Field{
		zap.String("skill", skill),
		zap.String("trigger", trigger),
		zap.String("user_message", userMessage),
		zap.String("skill_md_path", path),
		zap.String("skill_description", desc),
	}
	if readErr != nil {
		fields = append(fields, zap.NamedError("skill_md_read_error", readErr))
	}
	m.logger.Info("mock LLM matched skill via pattern", fields...)
}

// readSkillDescription returns the description from the SKILL.md
// frontmatter at path, or an error explaining why it could not. Best-effort:
// callers should treat a non-nil error as a logging hint, not a failure.
func readSkillDescription(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	fm, ok := extractFrontmatter(data)
	if !ok {
		return "", fmt.Errorf("no frontmatter found in %s", path)
	}
	var meta struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal(fm, &meta); err != nil {
		return "", err
	}
	return meta.Description, nil
}

func extractFrontmatter(content []byte) ([]byte, bool) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	buf := bytes.TrimPrefix(content, bom)
	buf = bytes.TrimLeft(buf, "\r\n\t ")
	if !bytes.HasPrefix(buf, []byte("---")) {
		return nil, false
	}
	rest := buf[3:]
	rest = bytes.TrimLeft(rest, "\r\n")
	before, _, found := bytes.Cut(rest, []byte("\n---"))
	if !found {
		return nil, false
	}
	return before, true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

// containsWord reports whether word appears in s bounded by non-word bytes on
// both sides, so "all" matches in "all fail" but not inside "tool calls".
func containsWord(s, word string) bool {
	if word == "" {
		return false
	}
	for idx := 0; idx+len(word) <= len(s); idx++ {
		if s[idx:idx+len(word)] != word {
			continue
		}
		beforeOK := idx == 0 || !isWordByte(s[idx-1])
		afterOK := idx+len(word) == len(s) || !isWordByte(s[idx+len(word)])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

func isWordByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			result[i] = s[i] + 32
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}

func generateID() string {
	return fmt.Sprintf("%d", 1000000+len("mock"))
}

func contentToString(c sdk.MessageContent) string {
	s, err := c.AsMessageContent0()
	if err != nil {
		return ""
	}
	return s
}
