package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
)

// TestA2AMethods_RoundTripWithADKClient boots the real agent and drives the
// A2A v1.0.1 methods (SendMessage, SendStreamingMessage, GetTask, CancelTask)
// through the ADK client, guarding against method-name or wire-format drift
// on ADK upgrades.
func TestA2AMethods_RoundTripWithADKClient(t *testing.T) {
	a2a := startAgent(t)

	t.Run("SendMessage then GetTask", func(t *testing.T) {
		sent := sendMessage(t, a2a, "echo hello")
		got := getTask(t, a2a, sent.ID)
		if got.ID != sent.ID {
			t.Fatalf("GetTask returned task %q, want %q", got.ID, sent.ID)
		}
		completed := waitForState(t, a2a, sent.ID, types.TaskStateCompleted)
		if completed.Status.State != types.TaskStateCompleted {
			t.Fatalf("task %q ended in %q, want %q", sent.ID, completed.Status.State, types.TaskStateCompleted)
		}
	})

	t.Run("SendStreamingMessage", func(t *testing.T) {
		events := streamMessage(t, a2a, "echo hello")
		if len(events) == 0 {
			t.Fatal("SendStreamingMessage produced no events")
		}
		if !sawFinalState(events, types.TaskStateCompleted) {
			t.Fatalf("stream never reported %q: %+v", types.TaskStateCompleted, events)
		}
	})

	t.Run("CancelTask", func(t *testing.T) {
		req := userMessage("delay 10 seconds")
		returnImmediately := true
		req.Configuration = &types.SendMessageConfiguration{ReturnImmediately: &returnImmediately}
		sent := send(t, a2a, req)
		resp, err := a2a.CancelTask(context.Background(), types.CancelTaskRequest{ID: sent.ID})
		if err != nil {
			t.Fatalf("CancelTask: %v", err)
		}
		canceled := decode[types.Task](t, resp.Result)
		if canceled.Status.State != types.TaskStateCanceled {
			t.Fatalf("CancelTask returned state %q, want %q", canceled.Status.State, types.TaskStateCanceled)
		}
	})
}

// TestDocumentedCurlPayloads posts the exact JSON-RPC bodies shown in docs/ and
// examples/ and asserts the outcome each page promises, so the copy-paste
// snippets cannot silently drift from the protocol the ADK speaks.
func TestDocumentedCurlPayloads(t *testing.T) {
	baseURL := startAgent(t).GetBaseURL()

	cases := []struct {
		name      string
		prompt    string
		wantState types.TaskState
		wantText  string
	}{
		{"docs/usage.md", "echo hello", types.TaskStateCompleted, ""},
		{"examples/connectivity-smoke-test", "connectivity check", types.TaskStateCompleted, "Connectivity check complete. Echo round-trip succeeded."},
		{"examples/error-handling-drill", "trigger an error with timeout", types.TaskStateFailed, "timeout error: trigger an error with timeout"},
		{"examples/latency-and-load-simulation", "run load simulation", types.TaskStateCompleted, "Load simulation complete."},
		{"examples/opentelemetry", "read go.mod", types.TaskStateCompleted, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","id":"1","method":"SendMessage","params":{"message":{"role":"ROLE_USER","parts":[{"text":"` + tc.prompt + `"}],"messageId":"m1"}}}`
			resp := postJSONRPC(t, baseURL, body)
			if resp.Error != nil {
				t.Fatalf("SendMessage error: %+v", resp.Error)
			}
			task := decode[types.SendMessageResponse](t, resp.Result).Task
			if task == nil {
				t.Fatalf("SendMessage returned no task: %s", resp.Result)
			}
			if task.Status.State != tc.wantState {
				t.Fatalf("state = %q, want %q", task.Status.State, tc.wantState)
			}
			if got := taskText(*task); !strings.Contains(got, tc.wantText) {
				t.Fatalf("task text %q does not contain %q", got, tc.wantText)
			}
		})
	}

	t.Run("v0.x method name is rejected", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":"1","method":"message/send","params":{"message":{"role":"ROLE_USER","parts":[{"text":"echo hello"}],"messageId":"m1"}}}`
		resp := postJSONRPC(t, baseURL, body)
		if resp.Error == nil || resp.Error.Code != methodNotFound {
			t.Fatalf("message/send returned %+v, want JSON-RPC error %d", resp.Error, methodNotFound)
		}
	})
}

const methodNotFound = -32601

type jsonRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func postJSONRPC(t *testing.T, baseURL, body string) jsonRPCResponse {
	t.Helper()
	httpResp, err := http.Post(baseURL+"/a2a", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /a2a: %v", err)
	}
	defer func() { _ = httpResp.Body.Close() }()
	var resp jsonRPCResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		t.Fatalf("decode JSON-RPC response: %v", err)
	}
	return resp
}

func taskText(task types.Task) string {
	messages := task.History
	if task.Status.Message != nil {
		messages = append(messages, *task.Status.Message)
	}
	var text strings.Builder
	for _, m := range messages {
		for _, p := range m.Parts {
			if p.Text != nil {
				text.WriteString(*p.Text + "\n")
			}
		}
	}
	return text.String()
}

func startAgent(t *testing.T) client.A2AClient {
	t.Helper()
	port := freePort(t)
	t.Setenv("A2A_SERVER_PORT", port)
	t.Setenv("A2A_DEBUG", "false")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = runStart(ctx) }()

	a2a := client.NewClient("http://localhost:" + port)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := a2a.GetHealth(ctx); err == nil {
			return a2a
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("agent did not become healthy")
	return nil
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer func() { _ = l.Close() }()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

func userMessage(text string) types.SendMessageRequest {
	return types.SendMessageRequest{
		Message: types.Message{
			MessageID: "m-" + strconv.FormatInt(time.Now().UnixNano(), 10),
			Role:      types.RoleUser,
			Parts:     []types.Part{types.CreateTextPart(text)},
		},
	}
}

func sendMessage(t *testing.T, a2a client.A2AClient, text string) types.Task {
	t.Helper()
	return send(t, a2a, userMessage(text))
}

func send(t *testing.T, a2a client.A2AClient, req types.SendMessageRequest) types.Task {
	t.Helper()
	resp, err := a2a.SendTask(context.Background(), req)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	sent := decode[types.SendMessageResponse](t, resp.Result)
	if sent.Task == nil || sent.Task.ID == "" {
		t.Fatalf("SendMessage returned no task: %+v", resp.Result)
	}
	return *sent.Task
}

func getTask(t *testing.T, a2a client.A2AClient, id string) types.Task {
	t.Helper()
	resp, err := a2a.GetTask(context.Background(), types.GetTaskRequest{ID: id})
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	return decode[types.Task](t, resp.Result)
}

func waitForState(t *testing.T, a2a client.A2AClient, id string, want types.TaskState) types.Task {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	task := getTask(t, a2a, id)
	for task.Status.State != want && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		task = getTask(t, a2a, id)
	}
	return task
}

func streamMessage(t *testing.T, a2a client.A2AClient, text string) []types.StreamResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := a2a.SendTaskStreaming(ctx, userMessage(text))
	if err != nil {
		t.Fatalf("SendStreamingMessage: %v", err)
	}
	var events []types.StreamResponse
	for resp := range stream {
		events = append(events, decode[types.StreamResponse](t, resp.Result))
	}
	return events
}

func sawFinalState(events []types.StreamResponse, want types.TaskState) bool {
	for _, e := range events {
		if e.StatusUpdate != nil && e.StatusUpdate.Status.State == want {
			return true
		}
		if e.Task != nil && e.Task.Status.State == want {
			return true
		}
	}
	return false
}

func decode[T any](t *testing.T, result any) T {
	t.Helper()
	var out T
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s into %T: %v", raw, out, err)
	}
	return out
}
