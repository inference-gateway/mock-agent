# Simulating multi-tool-call workloads

By default a task makes a single tool call (or none). To make a task look like a
realistic multi-step agent — `a2a.request` → N nested tool spans with varied
durations and the occasional error — drive the `simulate_tool_call` tool. Each
simulated call goes through the real instrumented tool path (`startToolSpan`),
so it emits its own span carrying `gen_ai.tool.name`, a configurable duration,
and an error status when a failure is injected. The task's execution stats
(`tool_calls`, `iterations`) then reflect the calls that actually ran.

There are two ways to control the workload.

## 1. Prompt keywords

Send a message containing **`simulate N tool calls`** (or `... workload`):

| Prompt | Effect |
|--------|--------|
| `simulate 5 tool calls` | 5 calls, names cycled from `read, search, fetch, write, summarize`, varied durations |
| `simulate tool calls: read, search, write` | explicit tool names (one call each) |
| `simulate 4 tool calls with a failure` | last call's span is marked failed |
| `simulate 3 tool calls, all should fail` | every call's span is marked failed |
| `simulate 3 slow tool calls` | larger per-call latency (base 500ms) |

This keyword routing sits alongside the `read <path>` routing documented in
[Usage](usage.md); explicit `read <path>` requests and the skill triggers still
take precedence.

## 2. `MOCK_TOOL_CALLS` env (a default for every task)

Set a workload that applies to any task (explicit `read <path>` requests and the
skill triggers still take precedence). Each comma-separated entry is
`name[:duration_ms][!]`:

```bash
# read (100ms) → search (300ms, span marked failed) → read (100ms)
MOCK_TOOL_CALLS=read,search:300!,read

# override the default per-call latency (used when an entry omits :ms)
MOCK_TOOL_CALL_DURATION_MS=250 MOCK_TOOL_CALLS=read,write,summarize
```

| Variable | Description | Default |
|----------|-------------|---------|
| `MOCK_TOOL_CALLS` | Comma list of `name[:duration_ms][!]` calls to simulate per task | (unset) |
| `MOCK_TOOL_CALL_DURATION_MS` | Per-call latency when an entry omits `:duration_ms` | `100` |

> **Notes**
> - Injected failures (`!` / "with a failure") mark the tool **span** with error
>   status but are **non-fatal**: the workload keeps running so the full N-span
>   trace is produced. For a task that actually fails, use the `error-injection`
>   skill.
> - The number of calls is ultimately bounded by
>   `A2A_AGENT_CLIENT_MAX_CHAT_COMPLETION_ITERATIONS` (one call per iteration,
>   plus a final response).
> - Turn on tracing (`A2A_TELEMETRY_ENABLE=true`, `A2A_OTEL_TRACES_EXPORTER=otlp`)
>   to see the spans — see
>   [`examples/opentelemetry`](../examples/opentelemetry/README.md).
