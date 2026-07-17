# Distributed-tracing demo with nested tool spans

Send `read <path>` to route through the Read built-in and emit a `tool.read` span
nested under `a2a.request`, producing real per-tool sub-spans for end-to-end
OpenTelemetry tracing (see [`examples/opentelemetry`](../opentelemetry/)).

By default the mock answers with canned text and runs no tool, so a task
contributes only the ADK's `a2a.request` middleware span. This example shows the
two ways to make it emit **real nested tool spans** — one span for a single
`read`, or a fan-out of N spans for a simulated multi-step agent — so a trace of
the mock looks like a live agent.

## What this demonstrates

- `read <path>` → the built-in **`Read`** tool → a `tool.read` span parented
  under `a2a.request` (same trace ID).
- `simulate N tool calls` (or the `MOCK_TOOL_CALLS` env) → the
  **`simulate_tool_call`** tool driven N times → `a2a.request` → `tool.read` →
  `tool.search` → …, each span carrying `gen_ai.tool.name`, a configurable
  duration, and an optional error status.
- How tool spans correlate with the request span via the OTel semantic-convention
  attributes the ADK and the tools share (`gen_ai.tool.call.id`, `session.id`).

## The fastest path: the OpenTelemetry stack

The [`examples/opentelemetry`](../opentelemetry/) directory ships a self-contained
`docker-compose.yaml` with the agent (telemetry on), an OpenTelemetry Collector,
and Jaeger already wired together. Start there if you want to *see* the spans:

```bash
docker compose -f examples/opentelemetry/docker-compose.yaml up --build
```

Then jump to [1. A single nested tool span](#1-a-single-nested-tool-span-read-path)
below to generate traffic, and open Jaeger at **http://localhost:16686**.

## Enabling tracing on a standalone agent

If you are not using that stack, turn tracing on and point it at your own OTLP
collector. Telemetry is compiled **on** for this agent (`agent.yaml` sets
`telemetry.enabled: true`) and defaults to exporting at `localhost:4318`, but you
still need a collector **listening** there or the spans are silently dropped —
point the exporter at one you are running:

```bash
A2A_TELEMETRY_ENABLE=true \
A2A_OTEL_TRACES_EXPORTER=otlp \
A2A_OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 \
A2A_OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf \
  go run . start
```

| Variable | Purpose | Default |
|----------|---------|---------|
| `A2A_TELEMETRY_ENABLE` | Master switch for telemetry | `true` |
| `A2A_OTEL_TRACES_EXPORTER` | `otlp` to export spans, `none` to disable | `otlp` |
| `A2A_OTEL_EXPORTER_OTLP_ENDPOINT` | Collector endpoint | `http://localhost:4318` |
| `A2A_OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` or `grpc` | `http/protobuf` |

See [CONFIGURATIONS.md](../../CONFIGURATIONS.md) for the complete telemetry table.

## 1. A single nested tool span: `read <path>`

The agreed `read <path>` phrase routes through the real `Read` built-in, which
opens a `tool.read` span (`tools/telemetry.go`):

```bash
# reads go.mod and emits a tool.read span under a2a.request
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest \
  --server-url http://localhost:8080 tasks submit "read go.mod"
```

Any in-container path works (`read README.md`, `read agent.yaml`); a bare `read`
defaults to `README.md`. Matching is whole-token, so `already`, `thread`, and
`reading` do **not** trigger it.

## 2. A multi-tool-call workload

Drive several tool calls in one task so the trace fans out into N nested spans:

```bash
# 4 calls, varied durations, the last marked failed (non-fatal)
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest \
  --server-url http://localhost:8080 tasks submit "simulate 4 tool calls with a failure"
```

…or make it the default for **every** task via `MOCK_TOOL_CALLS`
(`name[:duration_ms][!]` per entry):

```bash
# read (100ms) → search (300ms, failed span) → write (250ms)
MOCK_TOOL_CALLS=read,search:300!,write:250 \
A2A_OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 \
  go run . start
```

See [Simulating Tool Calls](../../docs/simulating-tool-calls.md) for the full set
of prompt keywords and env knobs.

## What you should see

In Jaeger (or your backend), pick the `mock-agent` service and open a trace. The
span tree shows the tool spans nested under the request span:

```
a2a.request
└─ tool.read                     gen_ai.tool.name=read, gen_ai.tool.call.id=…
```

…and for a simulated workload:

```
a2a.request
├─ tool.read       (100ms)
├─ tool.search     (300ms, status=ERROR)   ← injected failure
└─ tool.write      (250ms)
```

Injected failures mark the **span** with an error status but are **non-fatal**:
the workload runs to completion so the whole N-span trace is produced. With the
`infer` CLI wired to the same collector, the full chain becomes
`infer` → `a2a.request` → `tool.read` in one distributed trace — the shape
[cli#909](https://github.com/inference-gateway/cli/pull/909) demonstrates (see
[`examples/opentelemetry`](../opentelemetry/#full-cli-e2e-cli909)).

## Caveats

- A tool result whose **contents** include the words `error`/`failed` is treated
  as a genuine tool failure, so `read`-ing such a file marks the task failed (the
  span is still emitted). Prefer files like `go.mod` or `agent.yaml` for a clean
  success trace.
- For a task that *actually* fails (not just an errored span), use the
  [error-handling drill](../error-handling-drill/).

## Related

- [`examples/opentelemetry`](../opentelemetry/) — the full collector + Jaeger stack.
- [Simulating Tool Calls](../../docs/simulating-tool-calls.md)
- [Usage](../../docs/usage.md) — keyword-routing reference.
