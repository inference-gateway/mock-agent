# AGENTS.md

This file describes the agents available in this A2A (Agent-to-Agent) system.

## Agent Overview

### mock-agent
**Version**: 0.1.1  
**Description**: A2A agent server for mocking and testing. Uses a mock LLM client - no API keys required!

This agent is built using the Agent Definition Language (ADL) and provides A2A communication capabilities.

## Agent Capabilities



- **Streaming**: ✅ Real-time response streaming supported


- **Push Notifications**: ❌ Server-sent events not supported


- **State History**: ❌ State transition history not tracked



## AI Configuration





**System Prompt**: You are a mock AI assistant designed for testing and development purposes.

You have access to several mock skills that demonstrate different testing scenarios:
- echo: Simply echo back the input message (useful for basic connectivity tests)
- delay: Simulate slow responses with configurable delays
- error: Simulate error conditions for testing error handling
- random_data: Generate random test data
- validate: Validate input against common patterns

When responding:
- Be clear and predictable in your responses
- Include relevant metadata about the request
- Support both streaming and non-streaming modes
- Handle edge cases gracefully

Your purpose is to provide consistent, reproducible responses for testing A2A protocol implementations.



**Configuration:**




## Skills


This agent provides 5 skills:


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




## Server Configuration

**Port**: 8080

**Debug Mode**: ❌ Disabled



**Authentication**: ❌ Not required


## API Endpoints

The agent exposes the following HTTP endpoints:

- `GET /.well-known/agent-card.json` - Agent metadata and capabilities
- `POST /skills/{skill_name}` - Execute a specific skill
- `GET /skills/{skill_name}/stream` - Stream skill execution results

## Environment Setup

### Required Environment Variables

Key environment variables you'll need to configure:



- `PORT` - Server port (default: 8080)

### Development Environment


**Flox Environment**: ✅ Configured for reproducible development setup




## Usage

### Starting the Agent

```bash
# Install dependencies
go mod download

# Run the agent
go run main.go

# Or use Task
task run
```


### Communicating with the Agent

The agent implements the A2A protocol and can be communicated with via HTTP requests:

```bash
# Get agent information
curl http://localhost:8080/.well-known/agent-card.json



# Execute echo skill
curl -X POST http://localhost:8080/skills/echo \
  -H "Content-Type: application/json" \
  -d '{"input": "your_input_here"}'

# Execute delay skill
curl -X POST http://localhost:8080/skills/delay \
  -H "Content-Type: application/json" \
  -d '{"input": "your_input_here"}'

# Execute error skill
curl -X POST http://localhost:8080/skills/error \
  -H "Content-Type: application/json" \
  -d '{"input": "your_input_here"}'

# Execute random_data skill
curl -X POST http://localhost:8080/skills/random_data \
  -H "Content-Type: application/json" \
  -d '{"input": "your_input_here"}'

# Execute validate skill
curl -X POST http://localhost:8080/skills/validate \
  -H "Content-Type: application/json" \
  -d '{"input": "your_input_here"}'


```

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
├── main.go              # Server entry point
├── skills/              # Business logic skills

│   └── echo.go   # Echo back the input message (useful for basic connectivity tests)

│   └── delay.go   # Simulate slow responses with configurable delays

│   └── error.go   # Simulate error conditions for testing error handling

│   └── random_data.go   # Generate random test data

│   └── validate.go   # Validate input against common patterns

├── .well-known/         # Agent configuration
│   └── agent-card.json  # Agent metadata
├── go.mod               # Go module definition
└── README.md            # Project documentation
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

This agent was generated using ADL CLI v0.1.1 with the following configuration:

- **Language**: Go
- **Template**: Minimal A2A Agent
- **ADL Version**: adl.dev/v1

---

For more information about A2A agents and the ADL specification, visit the [ADL CLI documentation](https://github.com/inference-gateway/adl-cli).
