# Usage

The mock agent speaks the A2A protocol at `POST /a2a`. It routes on
**deterministic keywords**, not model reasoning, so every request has a
predictable response - ideal for tests.

## Send a message

Plain HTTP:

```bash
curl -s http://localhost:8080/a2a \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":"1","method":"SendMessage","params":{"message":{"role":"ROLE_USER","parts":[{"text":"echo hello"}],"messageId":"m1"}}}'
```

The endpoint speaks the A2A v1.0.1 JSON-RPC method names (`SendMessage`,
`SendStreamingMessage`, `GetTask`, `ListTasks`, `CancelTask`, ...). The v0.x
slash names such as `message/send` return `-32601` (method not found).
`SendMessage` waits for the task to finish; add
`"configuration":{"returnImmediately":true}` to `params` to get the task back
right away and poll it with `GetTask`.

Or with the [A2A Debugger](https://github.com/inference-gateway/a2a-debugger):

```bash
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest \
  --server-url http://localhost:8080 tasks submit "What are your skills?"
```

## Tools

| Tool | What it does | Key parameters |
|------|--------------|----------------|
| `echo` | Echo back the input message | `message` |
| `delay` | Return after a configurable delay | `duration_seconds`, `message` |
| `error` | Simulate an error condition | `error_type` (`validation`/`timeout`/`internal`/`not_found`), `message` |
| `random_data` | Generate random test data | `data_type` (`uuid`/`email`/`name`/`number`/`json`), `count` |
| `validate` | Validate input against a pattern | `input`, `validation_type` (`email`/`url`/`json`/`uuid`/`phone`) |
| `Read` | Read a file from disk | `file_path`, `offset`, `limit` |

## Skills

The system prompt advertises three skills (playbooks) that combine the tools:

- **connectivity-check** - echo a known payload and confirm the round-trip.
- **error-injection** - drive `error` across every `error_type`.
- **load-simulation** - combine `delay` and `random_data` for slow, realistic payloads.

## Keyword routing: `read <path>`

Because the mock routes on keywords, sending **`read <path>`** runs the `Read`
built-in against `<path>` (a bare `read` defaults to `README.md`). This
exercises a real tool call and emits a nested `tool.read` span under
`a2a.request` - handy for end-to-end distributed-tracing demos:

```bash
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest \
  --server-url http://localhost:8080 tasks submit "read go.mod"
```

> Only paths under the `Read` tool's `allowed_roots` are readable (`README.md`,
> `go.mod`, `agent.yaml`, `.well-known/` by default; override with
> `TOOLS_READ_ALLOWED_ROOTS`). A denied or missing path marks the task failed,
> but the span is still emitted. See [`examples/opentelemetry`](../examples/opentelemetry/README.md) for the
> full tracing stack, including the `infer` -> `a2a.request` -> `tool.read`
> distributed trace.

## Keyword routing: `simulate N tool calls`

Sending **`simulate N tool calls`** drives the `simulate_tool_call` tool a
configurable number of times, so a single task emits several nested tool spans
(with varied durations and the occasional injected failure) and looks like a
realistic multi-step agent in a trace. See
[Simulating Tool Calls](simulating-tool-calls.md) for the full set of prompt
keywords and the `MOCK_TOOL_CALLS` env default.
