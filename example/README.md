# Mock Agent Example

This example demonstrates how to use the mock-agent for testing and development. The mock agent provides a complete A2A protocol implementation without requiring any external LLM API keys or configuration.

## Features

- **Zero Configuration** - No API keys or external dependencies required
- **Mock LLM Client** - Simulates LLM responses without making external API calls
- **5 Testing Skills** - Pre-configured skills for various testing scenarios
- **Fast & Reliable** - Instant, predictable responses perfect for testing
- **A2A Protocol Compliant** - Full implementation of the A2A protocol
- **Artifact Server** - MinIO-backed artifact storage for testing file artifacts

## Quick Start

No configuration needed! Just start the services:

```bash
docker compose up --build
```

The mock agent will be available at `http://localhost:8080`

## Testing the Agent

### Using the A2A Debugger

The a2a-debugger provides a CLI interface to interact with the agent.

#### 1. Submit a Test Task

```bash
docker compose run --rm a2a-debugger tasks submit 'Please echo back this message: Hello, Mock Agent!'
```

This will submit a task to the mock agent. The agent will use the `echo` skill to echo back the message.

#### 2. List All Tasks

```bash
docker compose run --rm a2a-debugger tasks list
```

#### 3. Get Task Details

```bash
docker compose run --rm a2a-debugger tasks get <task-id>
```

Replace `<task-id>` with the ID from the task list output.

#### 4. Interactive Mode

For an interactive session with task streaming:

```bash
docker compose run --rm a2a-debugger tasks submit-streaming 'Generate 5 random UUIDs'
```

### Testing Different Skills

The mock agent provides 5 skills for testing various scenarios:

#### Echo Skill (Basic Connectivity)
```bash
docker compose run --rm a2a-debugger tasks submit 'Echo: Testing the mock agent'
```

#### Delay Skill (Performance Testing)
```bash
docker compose run --rm a2a-debugger tasks submit 'Wait for 5 seconds then respond'
```

#### Error Skill (Error Handling)
```bash
docker compose run --rm a2a-debugger tasks submit 'Simulate a validation error'
```

#### Random Data Skill (Data Generation)
```bash
docker compose run --rm a2a-debugger tasks submit 'Generate 10 random email addresses'
```

#### Validate Skill (Input Validation)
```bash
docker compose run --rm a2a-debugger tasks submit 'Validate this email: test@example.com'
```

## Available Skills

| Skill | Description | Example Usage |
|-------|-------------|---------------|
| **echo** | Echo back the input message | Testing basic connectivity |
| **delay** | Simulate slow responses with configurable delays | Testing timeout handling |
| **error** | Simulate error conditions | Testing error recovery |
| **random_data** | Generate random test data | Creating test fixtures |
| **validate** | Validate input against patterns | Testing validation logic |

## Direct API Access

You can also interact with the agent directly via HTTP:

### Check Agent Health

```bash
curl http://localhost:8080/health
```

### Get Agent Card

```bash
curl http://localhost:8080/.well-known/agent-card.json | jq
```

### Submit a Task (A2A Protocol)

```bash
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "req-1",
    "method": "message/send",
    "params": {
      "message": {
        "kind": "message",
        "role": "user",
        "parts": [
          {
            "kind": "text",
            "text": "Echo: Hello from direct API call"
          }
        ]
      }
    }
  }' | jq
```

### Get Task Status

```bash
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "req-2",
    "method": "tasks/get",
    "params": {
      "taskId": "TASK_ID_HERE"
    }
  }' | jq
```

Replace `TASK_ID_HERE` with the actual task ID from the response above.

## Example Workflows

### Testing Streaming Responses

```bash
docker compose run --rm a2a-debugger tasks submit-streaming 'Generate a long response to test streaming'
```

The mock agent will stream the response in chunks, simulating how a real LLM would stream tokens.

### Testing Error Handling

```bash
# Validation error
docker compose run --rm a2a-debugger tasks submit 'Please trigger a validation error'

# Timeout error
docker compose run --rm a2a-debugger tasks submit 'Please simulate a timeout'

# Internal error
docker compose run --rm a2a-debugger tasks submit 'Please simulate an internal server error'
```

### Generating Test Data

```bash
# Generate UUIDs
docker compose run --rm a2a-debugger tasks submit 'Generate 5 UUIDs for testing'

# Generate emails
docker compose run --rm a2a-debugger tasks submit 'Generate 10 test email addresses'

# Generate JSON test data
docker compose run --rm a2a-debugger tasks submit 'Generate 3 JSON objects for testing'
```

### Testing Artifacts

The mock agent includes a fully operational artifact server with MinIO storage backend:

```bash
# Verify artifact server is running
curl http://localhost:8081/health
# Response: {"status":"ok","time":"2025-10-19T17:37:20Z"}

# Verify artifact service is initialized
docker compose logs mock-agent | grep "artifact service initialized"
# Should show: artifact service initialized with MinIO storage

# Submit a task
docker compose run --rm a2a-debugger tasks submit 'Please create an artifact with sample data'
```

**Infrastructure Status:**
- ✅ Artifact server running on port 8081
- ✅ MinIO storage backend connected (minio:9000)
- ✅ Artifact service initialized successfully
- ✅ Download endpoint: `GET /artifacts/:artifactId/:filename`
- ✅ `create_artifact` tool available to LLM

**How Artifacts Work:**
1. The ADK automatically provides a `create_artifact` tool to the LLM
2. When the LLM calls this tool, artifacts are stored in MinIO
3. The artifact metadata (ID, download URL) is returned to the client
4. Clients download artifacts from `:8081/artifacts/:artifactId/:filename`

**Testing with Real LLM:**
The mock LLM is designed for testing A2A protocol infrastructure without API costs. To test actual artifact creation:

1. Replace mock LLM with a real provider in `main.go`:
   ```go
   // Instead of:
   llmClient := mock.NewMockLLMClient()

   // Use:
   llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.A2A.AgentConfig, l)
   ```

2. Set environment variables:
   ```bash
   A2A_AGENT_CLIENT_PROVIDER=openai
   A2A_AGENT_CLIENT_API_KEY=your-key
   A2A_AGENT_CLIENT_MODEL=gpt-4
   ```

3. The LLM will then call `create_artifact` and store files in MinIO

Access MinIO console to view/manage artifacts:
```bash
# Open in browser: http://localhost:9001
# Credentials: minioadmin / minioadmin
```

## Monitoring and Debugging

### View Agent Logs

```bash
docker compose logs -f mock-agent
```

### Check Running Services

```bash
docker compose ps
```

### Restart the Agent

```bash
docker compose restart mock-agent
```

## Cleanup

Stop and remove all containers:

```bash
docker compose down
```

## Architecture

```
┌─────────────┐   A2A Protocol  ┌──────────────┐
│ a2a-debugger│────────────────>│  mock-agent  │
│ (A2A Client)│                 │ (A2A Server) │
│             │                 │   :8080      │
│             │                 └──────┬───────┘
│             │                        │
│             │                        │ Uses
│             │                        v
│             │                 ┌──────────────┐
│             │                 │  Mock LLM    │
│             │                 │   Client     │
│             │                 │ (Generates   │
│             │                 │  Artifacts)  │
│             │                 └──────┬───────┘
│             │                        │
│             │                        │ Stores
│             │                        v
│             │  Download        ┌──────────────┐
│             │  Artifacts       │   MinIO      │
│             │<─────────────────│   :9000      │
└─────────────┘  via :8081       └──────────────┘
                 Artifact Server
```

## Services

This example runs the following services:

1. **mock-agent** (:8080) - The main A2A agent server with Mock LLM client
2. **artifacts-server** (:8081) - HTTP server for clients to download generated artifacts
3. **minio** (:9000, :9001) - Object storage backend where artifacts are stored
4. **a2a-debugger** - CLI tool for interacting with the agent (manual profile)

The flow is:
1. Client sends request to mock-agent via A2A protocol
2. Mock LLM generates artifacts and stores them in MinIO
3. Mock-agent returns artifact metadata to client
4. Client downloads artifacts from artifact server (:8081) which retrieves them from MinIO

## Why Use the Mock Agent?

✅ **No API Costs** - Completely free to run
✅ **Fast** - Instant responses without network latency
✅ **Predictable** - Consistent behavior for testing
✅ **Offline** - Works without internet connection
✅ **CI/CD Ready** - Perfect for automated testing pipelines
✅ **Protocol Testing** - Validate A2A protocol implementations
✅ **Development** - Test integrations without LLM API dependencies

## Troubleshooting

### Agent Not Starting

Check the logs for errors:
```bash
docker compose logs mock-agent
```

### Connection Refused

Make sure the agent is healthy:
```bash
docker compose ps
```

The healthcheck should show "healthy" status.

### Task Submission Fails

Verify the agent is accessible:
```bash
curl http://localhost:8080/health
```

Should return: `{"status":"ok"}`

## Next Steps

- Integrate the mock agent into your CI/CD pipeline
- Use it to test A2A protocol client implementations
- Build test suites that don't require LLM API keys
- Develop against a reliable, fast mock service

## License

MIT License - see LICENSE file for details
