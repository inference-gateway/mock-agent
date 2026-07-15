# AGENTS.md

This file describes the agents available in this A2A (Agent-to-Agent) system.

## Agent Overview

### mock-agent
**Version**: 0.2.1  
**Description**: A2A agent server for mocking and testing. Uses a mock LLM client - no API keys required!

This agent is built using the Agent Definition Language (ADL) and provides A2A communication capabilities.

## Agent Capabilities
- **Streaming**: ✅ Real-time response streaming supported
- **Push Notifications**: ❌ Server-sent events not supported
- **State History**: ❌ State transition history not tracked

## AI Configuration

**System Prompt**: You are a mock AI assistant designed for testing and development purposes.

You have access to several mock tools that demonstrate different testing scenarios:
- echo: Simply echo back the input message (useful for basic connectivity tests)
- delay: Simulate slow responses with configurable delays
- error: Simulate error conditions for testing error handling
- random_data: Generate random test data
- validate: Validate input against common patterns
- Read: Read a file from disk - the mock routes here when a request mentions
  "read <path>", exercising a real tool call (and its telemetry span) so
  distributed traces show a nested sub-tool span under a2a.request
- simulate_tool_call: Simulate one tool call with a configurable name, latency
  and optional error status. The mock drives it repeatedly to build multi
  tool-call workloads for load, latency and failure testing.

The mock routes on deterministic keywords, not model reasoning. Notably,
"read <path>" runs the Read tool against <path> (defaulting to README.md),
which is handy for end-to-end distributed-tracing demos. Saying
"simulate N tool calls" (or setting MOCK_TOOL_CALLS=read,search,read) drives a
sequence of instrumented simulate_tool_call spans - each with its own
gen_ai.tool.name, duration and optional injected failure - so a task can look
like a realistic multi-step agent in a trace.

When responding:
- Be clear and predictable in your responses
- Include relevant metadata about the request
- Support both streaming and non-streaming modes
- Handle edge cases gracefully

Your purpose is to provide consistent, reproducible responses for testing A2A protocol implementations.


**Configuration:**

## Tools

This agent exposes 7 function-call tools:

### Read (built-in)
- **Description**: Read a file from disk. Returns its contents, optionally sliced by line offset/limit. Use this to load SKILL.md bodies on demand.
- **Parameters**: file_path, offset, limit

### echo
- **Description**: Echo back the input message (useful for basic connectivity tests)
- **Tags**: mock, testing, echo
- **Input Schema**: Defined in agent configuration
- **Output Schema**: Defined in agent configuration

### delay
- **Description**: Simulate slow responses with configurable delays
- **Tags**: mock, testing, performance
- **Input Schema**: Defined in agent configuration
- **Output Schema**: Defined in agent configuration

### error
- **Description**: Simulate error conditions for testing error handling
- **Tags**: mock, testing, error-handling
- **Input Schema**: Defined in agent configuration
- **Output Schema**: Defined in agent configuration

### random_data
- **Description**: Generate random test data
- **Tags**: mock, testing, data-generation
- **Input Schema**: Defined in agent configuration
- **Output Schema**: Defined in agent configuration

### validate
- **Description**: Validate input against common patterns
- **Tags**: mock, testing, validation
- **Input Schema**: Defined in agent configuration
- **Output Schema**: Defined in agent configuration

### simulate_tool_call
- **Description**: Simulate a single tool call for load, latency and failure testing. Emits an instrumented span (gen_ai.tool.name) with a configurable duration and optional error status. The mock LLM drives this once per entry of a multi-tool-call workload.
- **Tags**: mock, testing, performance, tracing
- **Input Schema**: Defined in agent configuration
- **Output Schema**: Defined in agent configuration

## Skills

This agent ships 3 markdown skills that are loaded into the system prompt at startup:

### connectivity-check
- **Description**: Use this when the user wants to verify the agent is reachable and responding correctly. Invokes the echo tool with a known payload and confirms the round-trip succeeded.
- **Tags**: mock, testing, connectivity
- **Source**: scaffolded locally (`skills/connectivity-check/SKILL.md`)

### error-injection
- **Description**: Use this when the user wants to test how their client handles different failure modes. Invokes the error tool across the supported error_type values (validation, timeout, internal, not_found) so the caller can observe each error path.
- **Tags**: mock, testing, error-handling
- **Source**: scaffolded locally (`skills/error-injection/SKILL.md`)

### load-simulation
- **Description**: Use this when the user wants to test client behavior under slow responses with realistic payloads. Combines the delay tool (to introduce latency) with the random_data tool (to produce a test payload of the requested shape).
- **Tags**: mock, testing, performance
- **Source**: scaffolded locally (`skills/load-simulation/SKILL.md`)

## Server Configuration

**Port**: 8080
**Debug Mode**: ❌ Disabled
**Authentication**: ❌ Not required

## API Endpoints

The agent exposes the following HTTP endpoints:

- `GET /.well-known/agent-card.json` - Agent metadata and capabilities
- `GET /health` - Health check endpoint
- `POST /a2a` - JSON-RPC endpoint for all A2A operations (skill execution, streaming, etc.)

## Environment Setup

### Required Environment Variables

Key environment variables you'll need to configure:
- `PORT` - Server port (configured: 8080)

### Development Environment
- **Flox Environment**: ✅ Configured for reproducible development setup (`flox activate`)
- **Docker Compose**: ✅ Local service stack defined in `docker-compose.yaml` - brings up the Inference Gateway and this agent (built from `Dockerfile`) with `docker compose up --build`. Opt-in profiles `cli` and `debugger` expose the `infer` CLI and `a2a-debugger` for end-to-end testing.

## Usage

### Starting the Agent

```bash
# Install dependencies
go mod download

# Run the agent (the binary is a CLI; `start` boots the server)
go run . start

# Or use Task
task run
```

### Communicating with the Agent

The agent implements the A2A protocol and can be communicated with via HTTP requests:

```bash
# Get agent information
curl http://localhost:8080/.well-known/agent-card.json
```

Refer to the main README.md for specific skill execution examples and input schemas.

## Deployment

**Deployment Type**: Manual
- Build and run the agent binary directly
- Use provided Dockerfile for containerized deployment

### Docker Deployment

```bash
# Build image
docker build -t mock-agent .

# Run container
docker run -p 8080:8080 mock-agent
```

## Development

### Project Structure

```
.
├── main.go                       # Server entry point
├── tools/                        # Function-call tools
│   └── read.go                   # Read a file from disk. Returns its contents, optionally sliced by line offset/limit. Use this to load SKILL.md bodies on demand.
│   └── echo.go                   # Echo back the input message (useful for basic connectivity tests)
│   └── delay.go                  # Simulate slow responses with configurable delays
│   └── error.go                  # Simulate error conditions for testing error handling
│   └── random_data.go            # Generate random test data
│   └── validate.go               # Validate input against common patterns
│   └── simulate_tool_call.go     # Simulate a single tool call for load, latency and failure testing. Emits an instrumented span (gen_ai.tool.name) with a configurable duration and optional error status. The mock LLM drives this once per entry of a multi-tool-call workload.
├── skills/                       # Skill directories (SKILL.md + optional assets)
│   └── connectivity-check/       # Use this when the user wants to verify the agent is reachable and responding correctly. Invokes the echo tool with a known payload and confirms the round-trip succeeded.
│       └── SKILL.md              # Playbook prepended to the system prompt
│   └── error-injection/          # Use this when the user wants to test how their client handles different failure modes. Invokes the error tool across the supported error_type values (validation, timeout, internal, not_found) so the caller can observe each error path.
│       └── SKILL.md              # Playbook prepended to the system prompt
│   └── load-simulation/          # Use this when the user wants to test client behavior under slow responses with realistic payloads. Combines the delay tool (to introduce latency) with the random_data tool (to produce a test payload of the requested shape).
│       └── SKILL.md              # Playbook prepended to the system prompt
├── .well-known/                  # Agent configuration
│   └── agent-card.json           # Agent metadata
├── go.mod                        # Go module definition
└── README.md                     # Project documentation
```

### Testing

```bash
# Run tests
task test
go test ./...

# Run with coverage
task test:coverage
```

## Contributing

1. Implement business logic in skill files (replace TODO placeholders)
2. Add comprehensive tests for new functionality
3. Follow the established code patterns and conventions
4. Ensure proper error handling throughout
5. Update documentation as needed

## Agent Metadata

This agent was generated using ADL CLI v0.2.1 with the following configuration:

- **Language**: Go
- **Template**: Minimal A2A Agent
- **ADL Version**: adl.inference-gateway.com/v1

---

For more information about A2A agents and the ADL specification, visit the [ADL CLI documentation](https://github.com/inference-gateway/adl-cli).
