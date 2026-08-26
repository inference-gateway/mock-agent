package mock

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"

	tools "github.com/inference-gateway/mock-agent/tools"
)

// TestReadTrigger_EndToEnd_ExecutesRealReadTool wires the mock LLM client to a
// toolbox holding the same generated Read built-in main.go registers, then
// drives one round-trip. It proves the whole chain the tracing feature relies
// on: the mock chooses the Read tool for a "read <path>" message, the ADK
// toolbox finds a tool under the exact name the mock emitted ("Read"), and
// executing it with the mock's emitted args runs the real generated handler
// (the one that opens the tool.read telemetry span) and returns real file
// contents. A name/arg mismatch here would silently skip the tool - and its
// span - at runtime, which no pure-unit test of the mock would catch.
func TestReadTrigger_EndToEnd_ExecutesRealReadTool(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello from a real tool call\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	t.Setenv("TOOLS_READ_ALLOWED_ROOTS", dir)

	readTool, err := tools.NewReadTool(ctx, zap.NewNop())
	if err != nil {
		t.Fatalf("NewReadTool: %v", err)
	}
	toolBox := server.NewToolBox()
	toolBox.AddTool(readTool)

	client := NewMockLLMClient(zap.NewNop())
	resp, err := client.CreateChatCompletion(ctx, userMsg("read "+path), toolBox.GetTools()...)
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}

	calls := resp.Choices[0].Message.ToolCalls
	if calls == nil || len(*calls) == 0 {
		t.Fatalf("expected a tool call, got none")
	}
	call := (*calls)[0]
	if call.Function.Name != readToolName {
		t.Fatalf("expected %q tool call, got %q", readToolName, call.Function.Name)
	}
	if !toolBox.HasTool(call.Function.Name) {
		t.Fatalf("toolbox has no tool named %q (mock route vs. tool registration name mismatch)", call.Function.Name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		t.Fatalf("unmarshal tool args %q: %v", call.Function.Arguments, err)
	}
	if got := args["file_path"]; got != path {
		t.Fatalf("file_path: want %q, got %v", path, got)
	}

	out, err := toolBox.ExecuteTool(ctx, call.Function.Name, args)
	if err != nil {
		t.Fatalf("ExecuteTool(%q): %v", call.Function.Name, err)
	}
	if !strings.Contains(out, "hello from a real tool call") {
		t.Fatalf("expected real file contents in tool result, got: %q", out)
	}
}
