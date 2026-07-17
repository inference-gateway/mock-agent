# Connectivity smoke test

Send a known payload through the echo tool (the `connectivity-check` skill) and
confirm the round-trip, verifying the agent is reachable and responding over
A2A.

This is the first thing to run when wiring a new A2A client against the mock
agent: it proves the transport, the JSON-RPC surface, and the tool-call path all
work end-to-end, with a deterministic response and no API key required.

## What this demonstrates

- The `GET /health` and `GET /.well-known/agent-card.json` liveness endpoints.
- A full `message/send` round-trip over `POST /a2a`.
- The `connectivity-check` skill: the mock routes any of its trigger phrases
  (`ping`, `connectivity check`, `smoke test`, `are you up`, `healthcheck`,
  `round-trip`, …) to the **`echo`** tool, then answers with a fixed completion
  summary — so a passing run always looks identical.

## 1. Start the agent

From the repository root (no provider key needed — it uses the in-repo mock LLM
client):

```bash
task run                 # debug on, listens on :8080
# or
go run . start
# or
docker build -t mock-agent . && docker run -p 8080:8080 mock-agent
```

## 2. Check liveness

```bash
curl -s http://localhost:8080/health
curl -s http://localhost:8080/.well-known/agent-card.json | head
```

`/health` returns `200 OK`; the agent card echoes the name, version, and
capabilities the agent advertises.

## 3. Exercise the round-trip

Any `connectivity-check` trigger phrase drives the `echo` tool. Use the
[a2a-debugger](https://github.com/inference-gateway/a2a-debugger) (talks to the
agent directly — no model or gateway required):

```bash
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest \
  --server-url http://localhost:8080 tasks submit-streaming "ping"
```

…or a plain HTTP call:

```bash
curl -s http://localhost:8080/a2a \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":"1","method":"message/send","params":{"message":{"role":"user","parts":[{"kind":"text","text":"connectivity check"}],"messageId":"m1"}}}'
```

## What you should see

The task completes and the agent's final message is exactly:

```
Connectivity check complete. Echo round-trip succeeded.
```

Behind that summary, the mock invoked `echo` with your message as the `message`
argument and got the payload back verbatim (`{"status":"success","echo":"…"}`),
which is what confirms the round-trip. If you used `tasks submit` instead of
`tasks submit-streaming`, fetch the finished task with:

```bash
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest \
  --server-url http://localhost:8080 tasks get <task-id>
```

## How the routing works

The mock LLM client (`internal/mock/llm_client.go`) routes on **deterministic
keywords**, not model reasoning. `detectSkill` matches the message against the
`connectivity-check` trigger list first; on a match it runs that skill's
single-step workflow (`echo`) and, once the tool result is in history, returns
the skill's completion summary. That is why the response is byte-for-byte
reproducible — ideal for a CI smoke test.

> Tip: send a bare `echo hello` and the mock may route to a different default
> tool, because `echo` is only reached through the `connectivity-check` skill
> triggers above. Stick to a trigger phrase (`ping`, `smoke test`, …) when you
> specifically want to exercise the echo round-trip.

## Related

- [Usage](../../docs/usage.md) — the full keyword-routing reference.
- [Error-handling drill](../error-handling-drill/) — the failure-path counterpart.
- [Distributed-tracing demo](../distributed-tracing-demo-with-nested-tool-spans/)
  — see this round-trip as an `a2a.request` span.
