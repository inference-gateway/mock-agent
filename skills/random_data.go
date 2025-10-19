package skills

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	server "github.com/inference-gateway/adk/server"
)

// RandomDataSkill struct holds the skill with services
type RandomDataSkill struct {
}

// NewRandomDataSkill creates a new random_data skill
func NewRandomDataSkill() server.Tool {
	skill := &RandomDataSkill{}
	return server.NewBasicTool(
		"random_data",
		"Generate random test data",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		skill.RandomDataHandler,
	)
}

// RandomDataHandler handles the random_data skill execution
func (s *RandomDataSkill) RandomDataHandler(ctx context.Context, args map[string]any) (string, error) {
	dataType, ok := args["data_type"].(string)
	if !ok {
		return "", fmt.Errorf("data_type parameter is required and must be a string")
	}

	count := 1
	if val, ok := args["count"]; ok {
		if c, ok := val.(float64); ok {
			count = int(c)
		}
	}

	if count < 1 || count > 100 {
		return "", fmt.Errorf("count must be between 1 and 100")
	}

	var results []string
	for i := 0; i < count; i++ {
		switch dataType {
		case "uuid":
			results = append(results, uuid.New().String())

		case "email":
			results = append(results, fmt.Sprintf("test%d@example.com", i+1))

		case "name":
			names := []string{"Alice Johnson", "Bob Smith", "Carol Williams", "David Brown", "Eve Davis"}
			results = append(results, names[i%len(names)])

		case "number":
			n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
			results = append(results, fmt.Sprintf("%d", n.Int64()))

		case "json":
			obj := map[string]any{
				"id":     i + 1,
				"uuid":   uuid.New().String(),
				"name":   fmt.Sprintf("Item %d", i+1),
				"active": i%2 == 0,
			}
			jsonBytes, _ := json.Marshal(obj)
			results = append(results, string(jsonBytes))

		default:
			return "", fmt.Errorf("unknown data_type: must be one of (uuid, email, name, number, json)")
		}
	}

	resultsJSON, _ := json.Marshal(results)
	return fmt.Sprintf(`{"status": "success", "data_type": %q, "count": %d, "results": %s}`,
		dataType, count, string(resultsJSON)), nil
}
