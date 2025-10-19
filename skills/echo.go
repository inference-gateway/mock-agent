package skills

import (
	"context"
	"fmt"

	server "github.com/inference-gateway/adk/server"
)

// EchoSkill struct holds the skill with services
type EchoSkill struct {
}

// NewEchoSkill creates a new echo skill
func NewEchoSkill() server.Tool {
	skill := &EchoSkill{}
	return server.NewBasicTool(
		"echo",
		"Echo back the input message (useful for basic connectivity tests)",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		skill.EchoHandler,
	)
}

// EchoHandler handles the echo skill execution
func (s *EchoSkill) EchoHandler(ctx context.Context, args map[string]any) (string, error) {
	message, ok := args["message"].(string)
	if !ok {
		return "", fmt.Errorf("message parameter is required and must be a string")
	}

	return fmt.Sprintf(`{"status": "success", "echo": %q, "length": %d, "timestamp": %d}`,
		message, len(message), ctx.Value("timestamp")), nil
}
