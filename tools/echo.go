package tools

import (
	"context"
	"fmt"

	server "github.com/inference-gateway/adk/server"
)

// EchoTool struct holds the tool with services
type EchoTool struct {
}

// NewEchoTool creates a new echo tool
func NewEchoTool() server.Tool {
	tool := &EchoTool{}
	return server.NewBasicTool(
		"echo",
		"Echo back the input message (useful for basic connectivity tests)",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"description": "The message to echo back",
					"type":        "string",
				},
			},
			"required": []string{"message"},
		},
		tool.EchoHandler,
	)
}

// EchoHandler handles the echo tool execution
func (t *EchoTool) EchoHandler(ctx context.Context, args map[string]any) (string, error) {
	message, ok := args["message"].(string)
	if !ok {
		return "", fmt.Errorf("message parameter is required and must be a string")
	}

	return fmt.Sprintf(`{"status": "success", "echo": %q, "length": %d, "timestamp": %d}`,
		message, len(message), ctx.Value("timestamp")), nil
}
