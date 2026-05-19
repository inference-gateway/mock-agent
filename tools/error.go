package tools

import (
	"context"
	"errors"
	"fmt"

	server "github.com/inference-gateway/adk/server"
)

// ErrorTool struct holds the tool with services
type ErrorTool struct {
}

// NewErrorTool creates a new error tool
func NewErrorTool() server.Tool {
	tool := &ErrorTool{}
	return server.NewBasicTool(
		"error",
		"Simulate error conditions for testing error handling",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"error_type": map[string]any{
					"description": "Type of error to simulate",
					"enum":        []string{"validation", "timeout", "internal", "not_found"},
					"type":        "string",
				},
				"message": map[string]any{
					"description": "Custom error message",
					"type":        "string",
				},
			},
			"required": []string{"error_type"},
		},
		tool.ErrorHandler,
	)
}

// ErrorHandler handles the error tool execution
func (t *ErrorTool) ErrorHandler(ctx context.Context, args map[string]any) (string, error) {
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
