package tools

import (
	"context"
	"fmt"
	"time"

	server "github.com/inference-gateway/adk/server"
)

// DelayTool struct holds the tool with services
type DelayTool struct {
}

// NewDelayTool creates a new delay tool
func NewDelayTool() server.Tool {
	tool := &DelayTool{}
	return server.NewBasicTool(
		"delay",
		"Simulate slow responses with configurable delays",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"duration_seconds": map[string]any{
					"description": "Number of seconds to delay (default 2)",
					"type":        "number",
				},
				"message": map[string]any{
					"description": "Message to return after delay",
					"type":        "string",
				},
			},
		},
		tool.DelayHandler,
	)
}

// DelayHandler handles the delay tool execution
func (t *DelayTool) DelayHandler(ctx context.Context, args map[string]any) (string, error) {
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
