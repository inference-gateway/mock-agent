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

	var responseContent string
	var toolCalls *[]sdk.ChatCompletionMessageToolCall

	if len(tools) > 0 {
		calls := generateMockToolCalls(tools, lastContent)
		toolCalls = &calls
		responseContent = ""
	} else {
		responseContent = generateMockResponse(lastContent)
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

		response := generateMockResponse(lastContent)
		words := []rune(response)
		chunkSize := 10

		for i := 0; i < len(words); i += chunkSize {
			end := i + chunkSize
			if end > len(words) {
				end = len(words)
			}

			chunk := string(words[i:end])

			respChan <- &sdk.CreateChatCompletionStreamResponse{
				ID:      "mock-stream-" + generateID(),
				Model:   "mock-model",
				Object:  "chat.completion.chunk",
				Created: 1234567890,
				Choices: []sdk.ChatCompletionStreamChoice{
					{
						Index: 0,
						Delta: sdk.ChatCompletionStreamResponseDelta{
							Content: chunk,
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

func generateID() string {
	return fmt.Sprintf("%d", 1000000+len("mock"))
}
