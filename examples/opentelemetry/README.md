# OpenTelemetry example

A self-contained stack that runs the **mock-agent with telemetry enabled**, a
shared **OpenTelemetry Collector** as the trace sink, and **Jaeger** to view the
traces. It's meant for exercising distributed tracing end-to-end — in particular
with the [infer CLI telemetry feature (cli#909)](https://github.com/inference-gateway/cli/pull/909).

```
 infer CLI ─┐                        ┌─ Jaeger UI (:16686)
            ├─(OTLP)─► otel-collector ┤
 mock-agent ┘   :4317/:4318          └─ collector log (debug exporter)
     ▲
     └── W3C traceparent/baggage propagated from the CLI over /a2a
```

Because the CLI propagates W3C trace context to the agent and both export to the
**same collector**, the agent's `a2a.request` span nests under the CLI's tool
span in a single distributed trace. Send the mock the agreed **`read <path>`**
phrase (see the **Nested tool spans** section below) and it emits its own nested
`tool.read` span under `a2a.request`, so the trace shows real per-tool sub-spans
just like a live agent.

## Why this is needed

Telemetry ships **off by default** — that default lives in the ADK
(`A2A_TELEMETRY_ENABLE=false`), and `spec.telemetry` in `agent.yaml` maps
1:1 onto that built-in config. So you turn it on at runtime with the
`A2A_TELEMETRY_*` environment variables, which is exactly what the
`mock-agent` service in this compose file does:

| Variable | Value | Purpose |
|----------|-------|---------|
| `A2A_TELEMETRY_ENABLE` | `true` | Turn on telemetry (Prometheus `/metrics` + tracing) |
| `A2A_TELEMETRY_METRICS_PORT` | `9090` | Prometheus metrics port |
| `A2A_TELEMETRY_TRACE_ENABLE` | `true` | Enable OTLP trace export |
| `A2A_TELEMETRY_TRACE_ENDPOINT` | `http://otel-collector:4318` | Send spans (OTLP/HTTP) to the shared collector |

## Quick start

From the repository root:

```bash
# Build the agent and bring up agent + collector + jaeger
docker compose -f examples/opentelemetry/docker-compose.yaml up --build
```

Then generate a trace. The easiest way (no model or API key required) is the
a2a-debugger, which talks to the mock LLM agent directly over A2A:

```bash
docker compose -f examples/opentelemetry/docker-compose.yaml --profile debugger \
  run --rm debugger --server-url http://mock-agent:8080 tasks submit-streaming "What are your skills?"
```

or a plain HTTP call:

```bash
curl -s http://localhost:8080/a2a \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":"1","method":"message/send","params":{"message":{"role":"user","parts":[{"kind":"text","text":"echo hello"}],"messageId":"m1"}}}'
```

Open **http://localhost:16686**, pick the `mock-agent` service, and you'll see
the `a2a.request` span. You can also confirm spans are flowing without the UI:

```bash
docker compose -f examples/opentelemetry/docker-compose.yaml logs -f otel-collector
```

Prometheus metrics are exposed directly by the agent at
**http://localhost:9090/metrics** (instruments prefixed `a2a.`).

## Nested tool spans (`a2a.request` → `tool.read`)

By default the mock answers with canned text and never runs a tool, so the agent
contributes only the `a2a.request` middleware span. To make it emit a **real
sub-tool span**, send the agreed **`read <path>`** phrase — the mock LLM client
routes it through the generated `Read` built-in, which opens a `tool.read` span
(`tools/telemetry.go`) parented under the inbound request:

```bash
# reads go.mod inside the agent container and emits a tool.read span
docker compose -f examples/opentelemetry/docker-compose.yaml --profile debugger \
  run --rm debugger --server-url http://mock-agent:8080 tasks submit "read go.mod"
```

or over plain HTTP:

```bash
curl -s http://localhost:8080/a2a \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":"1","method":"message/send","params":{"message":{"role":"user","parts":[{"kind":"text","text":"read go.mod"}],"messageId":"m1"}}}'
```

In Jaeger the `mock-agent` trace now shows a `tool.read` span nested under
`a2a.request` (same trace ID). With the CLI wired up (below) the full chain is
`infer` → `a2a.request` → `tool.read` in one distributed trace — the shape
[cli#909](https://github.com/inference-gateway/cli/pull/909) demonstrates.

> Pass any in-container path, e.g. `read README.md` or `read agent.yaml`; a bare
> `read` defaults to `README.md`. Avoid files whose contents include the words
> "error"/"failed" — the mock treats a tool result containing them as a
> simulated failure (the span is still emitted, but the task is marked failed).

## Full CLI e2e (cli#909)

To see the CLI's own spans stitched together with the agent's, use the `cli`
profile. This needs a provider key + model (the CLI runs a real agentic loop
through the gateway) and a build of the CLI that includes
[cli#909](https://github.com/inference-gateway/cli/pull/909):

```bash
# put a provider key + model in the repo-root .env, e.g.:
#   OPENAI_API_KEY=sk-...
#   CLI_PROVIDER=openai
#   CLI_MODEL=gpt-4o
cp ../../.env.example ../../.env   # then edit

docker compose -f examples/opentelemetry/docker-compose.yaml --profile cli up --build -d
docker compose -f examples/opentelemetry/docker-compose.yaml --profile cli run --rm cli chat
```

Ask the CLI to call the agent (e.g. *"use the mock-agent to echo hello"*), then
look in Jaeger for a trace that spans both `infer` and `mock-agent`.

> **Note:** `cli:latest` won't contain the telemetry feature until cli#909 is
> released. Point the image at your cli#909 build, and double-check the CLI-side
> exporter setting — that PR wires `OTEL_EXPORTER_OTLP_ENDPOINT` for child
> processes and also supports a `telemetry.otlp.endpoint` config-file key.

## Cleanup

```bash
docker compose -f examples/opentelemetry/docker-compose.yaml --profile cli --profile debugger down
```

## Ports

| Service | Port | Notes |
|---------|------|-------|
| mock-agent | `8080` | A2A protocol endpoint |
| mock-agent | `9090` | Prometheus `/metrics` |
| otel-collector | `4317` / `4318` | OTLP gRPC / HTTP receivers |
| jaeger | `16686` | Jaeger UI |
