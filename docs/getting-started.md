# Getting Started

The mock agent is an [A2A](https://github.com/inference-gateway/adk) server that
answers with **deterministic, canned responses from a mock LLM client** - no
provider API key required. Use it to develop and test A2A clients against a
predictable server.

## Prerequisites

- Go 1.26.7+ (matches `spec.language.go.version` in `agent.yaml`), or Docker
- Optionally [`task`](https://taskfile.dev) to use the generated `Taskfile.yml`

## Run from source

```bash
# Start the A2A server on :8080 (blocks until SIGINT/SIGTERM)
go run . start

# ...or via the generated Taskfile (sets A2A_DEBUG=true, A2A_SERVER_PORT=8080)
task run
```

## Build the binary

```bash
task build            # produces bin/mock-agent
./bin/mock-agent --help
./bin/mock-agent --version
./bin/mock-agent start
```

## Run with Docker

```bash
docker build -t mock-agent .
docker run -p 8080:8080 mock-agent
```

## Verify it is up

```bash
curl -s http://localhost:8080/health
curl -s http://localhost:8080/.well-known/agent-card.json
```

Then send your first message - see [Usage](usage.md). For the settings that
control the server, see [Configuration](configuration.md).
