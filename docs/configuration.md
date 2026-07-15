# Configuration

The agent is configured through environment variables (all prefixed `A2A_`).
Defaults come from `agent.yaml`; the environment overrides them at runtime.
This page covers the settings most relevant to the mock agent - see the
[Environment Variables table in the README](../README.md#environment-variables)
for the complete list.

## Server

| Variable | Description | Default |
|----------|-------------|---------|
| `A2A_SERVER_PORT` / `A2A_PORT` | Server port | `8080` |
| `A2A_DEBUG` | Enable debug logging | `false` |
| `A2A_AGENT_URL` | Agent URL for internal references | `http://localhost:8080` |
| `A2A_STREAMING_STATUS_UPDATE_INTERVAL` | Streaming status update frequency | `1s` |

## Mock LLM client

The agent ships with a mock LLM client (`internal/mock`), so **no provider or
API key is required**. `spec.agent.provider` and `spec.agent.model` are
intentionally empty in `agent.yaml`, and the `A2A_AGENT_CLIENT_*` variables are
not needed for normal use. Responses are produced by deterministic keyword
routing rather than a real model (see [Usage](usage.md)).

## Read tool

The `read` built-in is enabled through `spec.config.tools.read` in `agent.yaml`
and exposed as environment overrides:

| Variable | Description | Default |
|----------|-------------|---------|
| `TOOLS_READ_ENABLED` | Enable the Read tool | `true` |
| `TOOLS_READ_MAX_LINES` | Max lines returned per read | `2000` |

## Telemetry

Telemetry is declared in `spec.telemetry` but ships **off by default** in the
ADK, so you opt in at runtime with `A2A_TELEMETRY_ENABLE=true`. The Prometheus
`/metrics` endpoint and OTLP trace export are then available; the exporter
variables are listed in the
[README Environment Variables table](../README.md#environment-variables). For a
ready-to-run stack (agent + OpenTelemetry Collector + Jaeger) see
[`examples/opentelemetry`](../examples/opentelemetry/README.md).
