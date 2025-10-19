package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"

	"github.com/google/uuid"
	server "github.com/inference-gateway/adk/server"
)

// ValidateSkill struct holds the skill with services
type ValidateSkill struct {
}

// NewValidateSkill creates a new validate skill
func NewValidateSkill() server.Tool {
	skill := &ValidateSkill{}
	return server.NewBasicTool(
		"validate",
		"Validate input against common patterns",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		skill.ValidateHandler,
	)
}

// ValidateHandler handles the validate skill execution
func (s *ValidateSkill) ValidateHandler(ctx context.Context, args map[string]any) (string, error) {
	// Extract input parameter
	input, ok := args["input"].(string)
	if !ok {
		return "", fmt.Errorf("input parameter is required and must be a string")
	}

	// Extract validation_type parameter
	validationType, ok := args["validation_type"].(string)
	if !ok {
		return "", fmt.Errorf("validation_type parameter is required and must be a string")
	}

	// Perform validation based on type
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
		// Simple phone validation (supports various formats)
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
