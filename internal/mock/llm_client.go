package mock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	zap "go.uber.org/zap"
	yaml "gopkg.in/yaml.v3"

	server "github.com/inference-gateway/adk/server"
	sdk "github.com/inference-gateway/sdk"
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
				if contains(toLower(msgText), "error") || contains(toLower(msgText), "failed") {
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

	// For non-skill messages, preserve legacy single-shot behavior: once any
	// tool result is in history, don't emit further tool calls - let the
	// caller produce a text response. Without this, fallback branches below
	// (artifact/echo/first-tool) keep firing after every result, looping the
	// agent indefinitely.
	if countToolResults(messages) > 0 {
		return nil
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
