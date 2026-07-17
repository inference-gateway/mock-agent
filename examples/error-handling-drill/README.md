# Error-handling drill

Drive the error tool across every `error_type` — `validation`, `timeout`,
`internal`, `not_found` — via the `error-injection` skill to see how your client
reacts to each failure mode.

Use this to harden an A2A client's error handling: each request deterministically
produces a **failed task** carrying a known error, so you can assert your
client surfaces, retries, or reports each case correctly.

## What this demonstrates

- The `error-injection` skill: its trigger phrases (`error injection`,
  `trigger an error`, `test error handling`, `failure mode`, …) route to the
  **`error`** tool.
- `error_type` selection from keywords in the message: a bare request defaults to
  `validation`; the words `timeout`, `internal`/`server`, and
  `not found`/`not_found`/`404` select the other three.
- **Failure by design.** The `error` tool returns a Go error, and the mock LLM
  client treats a tool result containing `error`/`failed` as fatal — so the task
  ends in the `failed` state instead of completing. That is the behaviour under
  test.

## 1. Start the agent

```bash
task run                 # listens on :8080
# or: go run . start
```

## 2. Drive each error type

Each row is a separate task submission; the skill drives one `error` call per
task. The phrase both triggers the skill and selects the `error_type`:

| `error_type` | Example prompt | Task fails with |
|--------------|----------------|-----------------|
| `validation` | `test error handling` | `validation error: test error handling` |
| `timeout`    | `trigger an error with timeout` | `timeout error: trigger an error with timeout` |
| `internal`   | `simulate failure: internal server` | `internal error: simulate failure: internal server` |
| `not_found`  | `trigger error - not found failure` | `not found error: trigger error - not found failure` |

> The mock passes your prompt to the `error` tool as its `message` argument, so
> each failure reads `<error_type> error: <your prompt>`. Call the tool with an
> empty `message` (a direct tool call rather than the skill) to get its built-in
> defaults instead — `Operation timed out after 30 seconds`, `Resource not
> found`, and so on (see `tools/error.go`).

With the [a2a-debugger](https://github.com/inference-gateway/a2a-debugger):

```bash
for prompt in \
  "test error handling" \
  "trigger an error with timeout" \
  "simulate failure: internal server" \
  "trigger error - not found failure"; do
  docker run --rm -i --network host ghcr.io/inference-gateway/a2a-debugger:latest \
    --server-url http://localhost:8080 tasks submit "$prompt"
done
```

…or a single case over plain HTTP:

```bash
curl -s http://localhost:8080/a2a \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":"1","method":"message/send","params":{"message":{"role":"user","parts":[{"kind":"text","text":"trigger an error with timeout"}],"messageId":"m1"}}}'
```

## What you should see

Every submission produces a task that transitions to **`failed`**, carrying the
matching message from the table above. List the failed tasks to confirm all four
paths fired:

```bash
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest \
  --server-url http://localhost:8080 tasks list

# inspect one to see the error detail
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest \
  --server-url http://localhost:8080 tasks get <task-id>
```

Assert in your client that a failed task is surfaced (not silently dropped) and
that the error text/type matches what you sent.

## How the routing works

`detectSkill` (`internal/mock/llm_client.go`) matches the `error-injection`
triggers, then `extractErrorType` picks the `error_type` from the same message.
The `error` tool (`tools/error.go`) returns `fmt.Errorf("<type> error: …")`; the
mock's next completion sees a tool result containing `error` and returns a fatal
`tool execution failed: …`, so the ADK marks the task `failed`. This "fail by
design" path is intentional — see the note in
`internal/mock/llm_client_test.go` (`TestSkillCompletion_ErrorInjection`).

> A **non-fatal** failure — an errored tool *span* on a task that still completes
> — is a different scenario: see the injected-failure workloads in the
> [distributed-tracing demo](../distributed-tracing-demo-with-nested-tool-spans/).

## Related

- [Connectivity smoke test](../connectivity-smoke-test/) — the success-path counterpart.
- [Usage](../../docs/usage.md) — full keyword-routing reference.
