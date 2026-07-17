# Latency and load simulation

Combine the delay tool with `random_data` (the `load-simulation` skill) to return
realistic test payloads after a configurable delay, exercising client timeouts
and streaming under slow responses.

Use this to see how an A2A client behaves when the agent is slow: does it stream
partial status, honour its read timeout, keep the connection open, and handle a
non-trivial payload when the response finally lands?

## What this demonstrates

- The `load-simulation` skill, a **two-step** workflow: the mock first calls
  **`random_data`** to build a payload, then **`delay`** to sleep before
  answering.
- Parameter extraction from the prompt: `random_data`'s `data_type`
  (`uuid`/`email`/`name`/`number`/`json`) and `count`, and `delay`'s
  `duration_seconds` (the digits `3`, `5`, or `10`, else the `2s` default).
- Streaming status updates while the task is in-flight, so a slow response is
  observable rather than an opaque hang.

## 1. Start the agent

```bash
task run                 # listens on :8080
# or: go run . start
```

## 2. Drive a slow, payload-bearing response

Trigger phrases include `run load simulation`, `slow response`,
`simulate latency`, `load test`, and `delayed payload`. Streaming makes the
latency visible:

```bash
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest \
  --server-url http://localhost:8080 tasks submit-streaming "slow response with 3 json records"
```

…or over plain HTTP (blocks for the delay, then returns):

```bash
curl -s http://localhost:8080/a2a \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":"1","method":"message/send","params":{"message":{"role":"user","parts":[{"kind":"text","text":"run load simulation"}],"messageId":"m1"}}}'
```

Vary the shape and timing by wording the prompt:

| Prompt | `random_data` | `delay` |
|--------|---------------|---------|
| `run load simulation` | `uuid` × 5 | 2s (default) |
| `slow response with 3 json records` | `json` × 3 | 3s |
| `load test: 10 emails, delay 10` | `email` × 10 | 10s |

## What you should see

While the task runs you get streaming status updates; after the delay elapses the
agent's final message is:

```
Load simulation complete. Random payload generated and delay applied.
```

The two tool calls that ran are reflected in the task's execution stats
(`tool_calls: 2`). The `delay` tool actually sleeps for `duration_seconds`, so
the wall-clock latency is real — point your client's read timeout below the
requested delay to test the timeout path, or above it to test the happy path.

## Tuning the load

- **Bigger payloads:** ask for `json` and a higher count (`… with 10 json
  records`) to make `random_data` return a larger body.
- **Longer latency:** use `3`, `5`, or `10` in the prompt; the mock maps those to
  the `delay` duration.
- **Streaming cadence:** set `A2A_STREAMING_STATUS_UPDATE_INTERVAL` (default `1s`)
  to control how often status updates arrive during the delay.
- **Server timeouts:** `A2A_SERVER_READ_TIMEOUT` / `A2A_SERVER_WRITE_TIMEOUT`
  (default `120s`) bound the longest delay the server will hold a request open.

See [CONFIGURATIONS.md](../../CONFIGURATIONS.md) for the full list.

## How the routing works

`detectSkill` (`internal/mock/llm_client.go`) matches a `load-simulation`
trigger, then drives the skill's ordered steps (`random_data`, then `delay`),
advancing one step per turn as tool results accumulate. `extractRandomDataParams`
and `extractDuration` read the payload shape and delay straight from the prompt,
so the workload is fully deterministic.

> For a **many** tool-call workload (N nested spans with per-call latency and
> injected failures — a different lever), use the `simulate N tool calls`
> keyword or `MOCK_TOOL_CALLS`; see
> [Simulating Tool Calls](../../docs/simulating-tool-calls.md) and the
> [distributed-tracing demo](../distributed-tracing-demo-with-nested-tool-spans/).

## Related

- [Simulating Tool Calls](../../docs/simulating-tool-calls.md)
- [Connectivity smoke test](../connectivity-smoke-test/) — a fast, no-latency round-trip.
