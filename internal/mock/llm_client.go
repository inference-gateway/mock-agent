package mock

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/inference-gateway/adk/server"
	"github.com/inference-gateway/sdk"
)

type MockLLMClient struct{}

func NewMockLLMClient() server.LLMClient {
	return &MockLLMClient{}
}

func (m *MockLLMClient) CreateChatCompletion(ctx context.Context, messages []sdk.Message, tools ...sdk.ChatCompletionTool) (*sdk.CreateChatCompletionResponse, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	lastMessage := messages[len(messages)-1]
	lastContent := lastMessage.Content

	userMessage := ""
	hasToolResults := false
	var toolError string
	for _, msg := range messages {
		if msg.Role == sdk.User {
			userMessage = msg.Content
		}
		if msg.Role == sdk.Tool {
			hasToolResults = true
			if contains(toLower(msg.Content), "error") || contains(toLower(msg.Content), "failed") {
				toolError = msg.Content
			}
		}
	}

	if toolError != "" {
		return nil, fmt.Errorf("tool execution failed: %s", toolError)
	}

	var responseContent string
	var toolCalls *[]sdk.ChatCompletionMessageToolCall

	if len(tools) > 0 && !hasToolResults {
		calls := generateMockToolCalls(tools, lastContent)
		toolCalls = &calls
		responseContent = ""
	} else {
		content := lastContent
		if hasToolResults && userMessage != "" {
			content = "Task completed successfully. I executed the requested operation based on: " + userMessage
		}
		responseContent = generateMockResponse(content)
	}

	return &sdk.CreateChatCompletionResponse{
		Id:      "mock-" + generateID(),
		Model:   "mock-model",
		Object:  "chat.completion",
		Created: 1234567890,
		Choices: []sdk.ChatCompletionChoice{
			{
				Index: 0,
				Message: sdk.Message{
					Role:      sdk.Assistant,
					Content:   responseContent,
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
		lastContent := lastMessage.Content

		userMessage := ""
		hasToolResults := false
		var toolError string
		for _, msg := range messages {
			if msg.Role == sdk.User {
				userMessage = msg.Content
			}
			if msg.Role == sdk.Tool {
				hasToolResults = true
				if contains(toLower(msg.Content), "error") || contains(toLower(msg.Content), "failed") {
					toolError = msg.Content
				}
			}
		}

		if toolError != "" {
			errChan <- fmt.Errorf("tool execution failed: %s", toolError)
			return
		}

		if len(tools) > 0 && !hasToolResults {
			toolCalls := generateMockToolCalls(tools, lastContent)
			if len(toolCalls) > 0 {
				for idx, toolCall := range toolCalls {
					chunk := sdk.ChatCompletionMessageToolCallChunk{
						Index: idx,
						ID:    toolCall.Id,
						Type:  string(toolCall.Type),
						Function: struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						}{
							Name:      toolCall.Function.Name,
							Arguments: toolCall.Function.Arguments,
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
									ToolCalls: []sdk.ChatCompletionMessageToolCallChunk{chunk},
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
							FinishReason: string(sdk.ToolCalls),
						},
					},
				}
				return
			}
		}

		responseContent := lastContent
		if hasToolResults && userMessage != "" {
			responseContent = "Task completed successfully."
		}

		response := generateMockResponse(responseContent)

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
					FinishReason: string(sdk.Stop),
				},
			},
		}
	}()

	return respChan, errChan
}

func generateMockResponse(userMessage string) string {
	return fmt.Sprintf("This is a mock response to: %q. I'm a mock agent designed for testing purposes.", userMessage)
}

func generateMockToolCalls(tools []sdk.ChatCompletionTool, userMessage string) []sdk.ChatCompletionMessageToolCall {
	if len(tools) == 0 {
		return nil
	}

	lowerMsg := toLower(userMessage)

	if contains(lowerMsg, "error") || contains(lowerMsg, "fail") || contains(lowerMsg, "throw") {
		for _, tool := range tools {
			if tool.Function.Name == "error" {
				errorType := "validation"
				if contains(lowerMsg, "timeout") {
					errorType = "timeout"
				} else if contains(lowerMsg, "internal") || contains(lowerMsg, "server") {
					errorType = "internal"
				} else if contains(lowerMsg, "not found") || contains(lowerMsg, "404") {
					errorType = "not_found"
				}

				args, _ := json.Marshal(map[string]any{
					"error_type": errorType,
					"message":    userMessage,
				})

				return []sdk.ChatCompletionMessageToolCall{
					{
						Id:   "call-" + generateID(),
						Type: sdk.Function,
						Function: sdk.ChatCompletionMessageToolCallFunction{
							Name:      "error",
							Arguments: string(args),
						},
					},
				}
			}
		}
	}

	if contains(lowerMsg, "delay") || contains(lowerMsg, "wait") || contains(lowerMsg, "sleep") || contains(lowerMsg, "pause") {
		for _, tool := range tools {
			if tool.Function.Name == "delay" {
				duration := 2.0
				if contains(lowerMsg, "5") {
					duration = 5.0
				} else if contains(lowerMsg, "10") {
					duration = 10.0
				} else if contains(lowerMsg, "3") {
					duration = 3.0
				}

				args, _ := json.Marshal(map[string]any{
					"duration_seconds": duration,
					"message":          userMessage,
				})

				return []sdk.ChatCompletionMessageToolCall{
					{
						Id:   "call-" + generateID(),
						Type: sdk.Function,
						Function: sdk.ChatCompletionMessageToolCallFunction{
							Name:      "delay",
							Arguments: string(args),
						},
					},
				}
			}
		}
	}

	if contains(lowerMsg, "validate") || contains(lowerMsg, "check") {
		for _, tool := range tools {
			if tool.Function.Name == "validate" {
				pattern := "email"
				if contains(lowerMsg, "url") || contains(lowerMsg, "http") {
					pattern = "url"
				} else if contains(lowerMsg, "json") {
					pattern = "json"
				} else if contains(lowerMsg, "uuid") {
					pattern = "uuid"
				} else if contains(lowerMsg, "phone") {
					pattern = "phone"
				}

				args, _ := json.Marshal(map[string]any{
					"pattern": pattern,
					"input":   userMessage,
				})

				return []sdk.ChatCompletionMessageToolCall{
					{
						Id:   "call-" + generateID(),
						Type: sdk.Function,
						Function: sdk.ChatCompletionMessageToolCallFunction{
							Name:      "validate",
							Arguments: string(args),
						},
					},
				}
			}
		}
	}

	if contains(lowerMsg, "artifact") || contains(lowerMsg, "create file") || contains(lowerMsg, "save file") {
		for _, tool := range tools {
			if tool.Function.Name == "create_artifact" {
				name := "sample-data.json"
				content := `{"id": 1, "name": "John Doe", "email": "john.doe@example.com"}`

				if contains(lowerMsg, "text") || contains(lowerMsg, "txt") {
					name = "sample-data.txt"
					content = "This is a sample text artifact created by the mock agent."
				} else if contains(lowerMsg, "csv") {
					name = "sample-data.csv"
					content = "id,name,email\n1,John Doe,john.doe@example.com\n2,Jane Smith,jane.smith@example.com"
				}

				args, _ := json.Marshal(map[string]any{
					"name":     name,
					"content":  content,
					"type":     "url",
					"filename": name,
				})

				return []sdk.ChatCompletionMessageToolCall{
					{
						Id:   "call-" + generateID(),
						Type: sdk.Function,
						Function: sdk.ChatCompletionMessageToolCallFunction{
							Name:      "create_artifact",
							Arguments: string(args),
						},
					},
				}
			}
		}
	}

	if contains(lowerMsg, "random") || contains(lowerMsg, "generate") {
		for _, tool := range tools {
			if tool.Function.Name == "random_data" {
				dataType := "uuid"
				count := 5
				if contains(lowerMsg, "email") {
					dataType = "email"
				} else if contains(lowerMsg, "name") {
					dataType = "name"
				} else if contains(lowerMsg, "number") {
					dataType = "number"
				} else if contains(lowerMsg, "json") {
					dataType = "json"
				}

				if contains(lowerMsg, "10") {
					count = 10
				} else if contains(lowerMsg, "3") {
					count = 3
				} else if contains(lowerMsg, "1") && !contains(lowerMsg, "10") {
					count = 1
				}

				args, _ := json.Marshal(map[string]any{
					"data_type": dataType,
					"count":     count,
				})

				return []sdk.ChatCompletionMessageToolCall{
					{
						Id:   "call-" + generateID(),
						Type: sdk.Function,
						Function: sdk.ChatCompletionMessageToolCallFunction{
							Name:      "random_data",
							Arguments: string(args),
						},
					},
				}
			}
		}
	}

	for _, tool := range tools {
		if tool.Function.Name == "create_artifact" {
			name := "default-file.json"
			content := `{"message": "Default artifact content"}`

			args, _ := json.Marshal(map[string]any{
				"name":     name,
				"content":  content,
				"type":     "url",
				"filename": name,
			})

			return []sdk.ChatCompletionMessageToolCall{
				{
					Id:   "call-" + generateID(),
					Type: sdk.Function,
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "create_artifact",
						Arguments: string(args),
					},
				},
			}
		}
	}

	for _, tool := range tools {
		if tool.Function.Name == "echo" {
			args, _ := json.Marshal(map[string]any{
				"message": userMessage,
			})

			return []sdk.ChatCompletionMessageToolCall{
				{
					Id:   "call-" + generateID(),
					Type: sdk.Function,
					Function: sdk.ChatCompletionMessageToolCallFunction{
						Name:      "echo",
						Arguments: string(args),
					},
				},
			}
		}
	}

	args, _ := json.Marshal(map[string]any{})
	name := tools[0].Function.Name

	return []sdk.ChatCompletionMessageToolCall{
		{
			Id:   "call-" + generateID(),
			Type: sdk.Function,
			Function: sdk.ChatCompletionMessageToolCallFunction{
				Name:      name,
				Arguments: string(args),
			},
		},
	}
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
