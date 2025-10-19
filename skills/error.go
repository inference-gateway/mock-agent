package skills

import (
	"context"
	"errors"
	"fmt"

	server "github.com/inference-gateway/adk/server"
)

// ErrorSkill struct holds the skill with services
type ErrorSkill struct {
}

// NewErrorSkill creates a new error skill
func NewErrorSkill() server.Tool {
	skill := &ErrorSkill{}
	return server.NewBasicTool(
		"error",
		"Simulate error conditions for testing error handling",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		skill.ErrorHandler,
	)
}

// ErrorHandler handles the error skill execution
func (s *ErrorSkill) ErrorHandler(ctx context.Context, args map[string]any) (string, error) {
	errorType, ok := args["error_type"].(string)
	if !ok {
		return "", fmt.Errorf("error_type parameter is required and must be a string")
	}

	customMessage := ""
	if val, ok := args["message"]; ok {
		if msg, ok := val.(string); ok {
			customMessage = msg
		}
	}

	switch errorType {
	case "validation":
		if customMessage == "" {
			customMessage = "Validation failed: invalid input format"
		}
		return "", fmt.Errorf("validation error: %s", customMessage)

	case "timeout":
		if customMessage == "" {
			customMessage = "Operation timed out after 30 seconds"
		}
		return "", fmt.Errorf("timeout error: %s", customMessage)

	case "internal":
		if customMessage == "" {
			customMessage = "Internal server error occurred"
		}
		return "", fmt.Errorf("internal error: %s", customMessage)

	case "not_found":
		if customMessage == "" {
			customMessage = "Resource not found"
		}
		return "", fmt.Errorf("not found error: %s", customMessage)

	default:
		return "", errors.New("unknown error_type: must be one of (validation, timeout, internal, not_found)")
	}
}
