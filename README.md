<div align="center">

# Mock Agent

[![CI](https://github.com/inference-gateway/mock-agent/workflows/CI/badge.svg)](https://github.com/inference-gateway/mock-agent/actions/workflows/ci.yml)
[![Go Report Card](https://img.shields.io/badge/Go%20Report%20Card-A+-brightgreen?style=flat&logo=go&logoColor=white)](https://goreportcard.com/report/github.com/inference-gateway/mock-agent)
[![Go Version](https://img.shields.io/badge/Go-1.26.4+-00ADD8?style=flat&logo=go)](https://golang.org)
[![A2A Protocol](https://img.shields.io/badge/A2A-Protocol-blue?style=flat)](https://github.com/inference-gateway/adk)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)

**A2A agent server for mocking and testing. Uses a mock LLM client - no API keys required!**

A enterprise-ready [Agent-to-Agent (A2A)](https://github.com/inference-gateway/adk) server that provides AI-powered capabilities through a standardized protocol.

</div>

## Quick Start

The generated binary is a CLI. `start` boots the A2A server; `--help` and
`--version` work as you'd expect.

```bash
# Run the agent
go run . start

# Or build and invoke the CLI directly
task build
./bin/mock-agent start

# Or with Docker
docker build -t mock-agent .
docker run -p 8080:8080 mock-agent
```

### CLI

| Command | Description |
|---------|-------------|
| `mock-agent start` | Start the A2A server (blocks until SIGINT/SIGTERM) |
| `mock-agent --help` | Show top-level help (and per-subcommand with `<cmd> --help`) |
| `mock-agent --version` | Print the embedded version and exit |

## Quick Install

Add this agent to your Inference Gateway CLI:

```bash
infer agents add mock-agent http://localhost:8080 \
  --oci ghcr.io/inference-gateway/mock-agent:latest \
  --run
```

## Features

- ✅ A2A protocol compliant
- ✅ AI-powered capabilities
- ✅ Streaming support
- ✅ OpenTelemetry instrumentation
- ✅ Enterprise-ready
- ✅ Minimal dependencies

## Endpoints

- `GET /.well-known/agent-card.json` - Agent metadata and capabilities
- `GET /health` - Health check endpoint
- `POST /a2a` - A2A protocol endpoint

## Available Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `Read` | Read a file from disk. Returns its contents, optionally sliced by line offset/limit. Use this to load SKILL.md bodies on demand. | file_path, offset, limit |
| `echo` | Echo back the input message (useful for basic connectivity tests) | message |
| `delay` | Simulate slow responses with configurable delays | duration_seconds, message |
| `error` | Simulate error conditions for testing error handling | error_type, message |
| `random_data` | Generate random test data | count, data_type |
| `validate` | Validate input against common patterns | input, validation_type |
| `simulate_tool_call` | Simulate a single tool call for load, latency and failure testing. Emits an instrumented span (gen_ai.tool.name) with a configurable duration and optional error status. The mock LLM drives this once per entry of a multi-tool-call workload. | duration_ms, fail, name |

## Examples

| Example | Description |
|---------|-------------|
| [Connectivity smoke test](examples/connectivity-smoke-test/) | Send a known payload through the echo tool (the connectivity-check skill) and confirm the round-trip, verifying the agent is reachable and responding over A2A. |
| [Error-handling drill](examples/error-handling-drill/) | Drive the error tool across every error_type - validation, timeout, internal, not_found - via the error-injection skill to see how your client reacts to each failure mode. |
| [Latency and load simulation](examples/latency-and-load-simulation/) | Combine the delay tool with random_data (the load-simulation skill) to return realistic test payloads after a configurable delay, exercising client timeouts and streaming under slow responses. |
| [Distributed-tracing demo with nested tool spans](examples/distributed-tracing-demo-with-nested-tool-spans/) | Send "read <path>" to route through the Read built-in and emit a tool.read span nested under a2a.request, producing real per-tool sub-spans for end-to-end OpenTelemetry tracing (see examples/opentelemetry). |

## Skills (loaded into the system prompt)

| Skill | Description | Source |
|-------|-------------|--------|
| `connectivity-check` | Use this when the user wants to verify the agent is reachable and responding correctly. Invokes the echo tool with a known payload and confirms the round-trip succeeded. | bare scaffold (`.agents/skills/connectivity-check/SKILL.md`) |
| `error-injection` | Use this when the user wants to test how their client handles different failure modes. Invokes the error tool across the supported error_type values (validation, timeout, internal, not_found) so the caller can observe each error path. | bare scaffold (`.agents/skills/error-injection/SKILL.md`) |
| `load-simulation` | Use this when the user wants to test client behavior under slow responses with realistic payloads. Combines the delay tool (to introduce latency) with the random_data tool (to produce a test payload of the requested shape). | bare scaffold (`.agents/skills/load-simulation/SKILL.md`) |

## Documentation
- [Getting Started](docs/getting-started.md)
- [Configuration](docs/configuration.md)
- [Usage](docs/usage.md)
- [Simulating Tool Calls](docs/simulating-tool-calls.md)

## Configuration

The agent is configured via environment variables. Defaults are derived
from `agent.yaml`; see [CONFIGURATIONS.md](CONFIGURATIONS.md) for the
full reference of custom and `A2A_*` variables.

## Development

```bash
# Generate code from ADL
task generate

# Run tests
task test

# Build the application
task build

# Run linter
task lint

# Format code
task fmt
```

### Adding Dependencies

The generator owns the baseline toolchain pins (SDK, server framework,
logging, CLI, sandbox utilities). To extend the project without forking
the templates, declare extras in `agent.yaml` - every empty list below
is rendered by `adl init --defaults` precisely so it's discoverable:

| Where | Purpose | Example entry | Rendered into |
|-------|---------|---------------|---------------|
| `spec.language.go.vendor.deps` | Runtime Go modules | `github.com/stretchr/testify@v1.10.0` | `go.mod` `require` block |
| `spec.language.go.vendor.devdeps` | Executable dev tools (Go 1.24 `tool` directive) | `golang.org/x/tools/cmd/stringer@v0.20.0` | `go.mod` `tool` directive |
| `spec.development.deps` | Cross-cutting sandbox tools (not tied to one language) | `kubectl@1.31.0`, `terraform@1.9.5`, `deno@2.1.4` | Flox `manifest.toml` / devcontainer feature |

Entries use the `<package>@<version>` form. Built-in pins always win on
conflict; the generator prints a warning and skips the user entry when
shadowing is attempted. After editing `agent.yaml`, re-run `task generate`
to refresh the manifests.

### Debugging

Use the [A2A Debugger](https://github.com/inference-gateway/a2a-debugger) to test and debug your A2A agent during development. It provides a web interface for sending requests to your agent and inspecting responses, making it easier to troubleshoot issues and validate your implementation.

```bash
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest --server-url http://localhost:8080 tasks submit "What are your skills?"
```

```bash
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest --server-url http://localhost:8080 tasks list
```

```bash
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest --server-url http://localhost:8080 tasks get <task ID>
```

## Deployment

### Docker

The Docker image can be built with custom version information using build arguments:

```bash
docker build \
  --build-arg VERSION=1.2.3 \
  --build-arg AGENT_NAME="My Custom Agent" \
  --build-arg AGENT_DESCRIPTION="Custom agent description" \
  -t mock-agent:1.2.3 .
```

**Available Build Arguments:**

- `VERSION` - Agent version (default: `0.3.4`)
- `AGENT_NAME` - Agent name (default: `mock-agent`)
- `AGENT_DESCRIPTION` - Agent description (default: `A2A agent server for mocking and testing. Uses a mock LLM client - no API keys required!`)

These values are embedded into the binary at build time using linker flags, making them accessible at runtime without requiring environment variables.

## License

Apache 2.0 License - see LICENSE file for details
