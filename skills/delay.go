package skills

import (
	"context"
	"fmt"
	"time"

	server "github.com/inference-gateway/adk/server"
)

// DelaySkill struct holds the skill with services
type DelaySkill struct {
}

// NewDelaySkill creates a new delay skill
func NewDelaySkill() server.Tool {
	skill := &DelaySkill{}
	return server.NewBasicTool(
		"delay",
		"Simulate slow responses with configurable delays",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		skill.DelayHandler,
	)
}

// DelayHandler handles the delay skill execution
func (s *DelaySkill) DelayHandler(ctx context.Context, args map[string]any) (string, error) {
	durationSeconds := 2.0
	if val, ok := args["duration_seconds"]; ok {
		if dur, ok := val.(float64); ok {
			durationSeconds = dur
		}
	}

	message := "Delay completed"
	if val, ok := args["message"]; ok {
		if msg, ok := val.(string); ok {
			message = msg
		}
	}

	startTime := time.Now()

	select {
	case <-time.After(time.Duration(durationSeconds * float64(time.Second))):
	case <-ctx.Done():
		return "", fmt.Errorf("delay canceled: %w", ctx.Err())
	}

	elapsed := time.Since(startTime)

	return fmt.Sprintf(`{"status": "success", "message": %q, "requested_delay_seconds": %.2f, "actual_delay_seconds": %.2f}`,
		message, durationSeconds, elapsed.Seconds()), nil
}
