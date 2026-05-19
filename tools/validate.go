package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"

	"github.com/google/uuid"
	server "github.com/inference-gateway/adk/server"
)

// ValidateTool struct holds the tool with services
type ValidateTool struct {
}

// NewValidateTool creates a new validate tool
func NewValidateTool() server.Tool {
	tool := &ValidateTool{}
	return server.NewBasicTool(
		"validate",
		"Validate input against common patterns",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{
					"description": "The input to validate",
					"type":        "string",
				},
				"validation_type": map[string]any{
					"description": "Type of validation",
					"enum":        []string{"email", "url", "json", "uuid", "phone"},
					"type":        "string",
				},
			},
			"required": []string{"input", "validation_type"},
		},
		tool.ValidateHandler,
	)
}

// ValidateHandler handles the validate tool execution
func (t *ValidateTool) ValidateHandler(ctx context.Context, args map[string]any) (string, error) {
	input, ok := args["input"].(string)
	if !ok {
		return "", fmt.Errorf("input parameter is required and must be a string")
	}

	validationType, ok := args["validation_type"].(string)
	if !ok {
		return "", fmt.Errorf("validation_type parameter is required and must be a string")
	}

	var isValid bool
	var errorMsg string

	switch validationType {
	case "email":
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		isValid = emailRegex.MatchString(input)
		if !isValid {
			errorMsg = "Invalid email format"
		}

	case "url":
		_, err := url.ParseRequestURI(input)
		isValid = err == nil
		if !isValid {
			errorMsg = fmt.Sprintf("Invalid URL format: %v", err)
		}

	case "json":
		var js json.RawMessage
		err := json.Unmarshal([]byte(input), &js)
		isValid = err == nil
		if !isValid {
			errorMsg = fmt.Sprintf("Invalid JSON: %v", err)
		}

	case "uuid":
		_, err := uuid.Parse(input)
		isValid = err == nil
		if !isValid {
			errorMsg = "Invalid UUID format"
		}

	case "phone":
		phoneRegex := regexp.MustCompile(`^[\d\s\-\+\(\)]{10,}$`)
		isValid = phoneRegex.MatchString(input)
		if !isValid {
			errorMsg = "Invalid phone number format"
		}

	default:
		return "", fmt.Errorf("unknown validation_type: must be one of (email, url, json, uuid, phone)")
	}

	return fmt.Sprintf(`{"status": "success", "valid": %t, "validation_type": %q, "input": %q, "error": %q}`,
		isValid, validationType, input, errorMsg), nil
}
