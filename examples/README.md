# AG-UI Client Examples

This directory contains example clients for connecting to AG-UI (Agentic UI) servers.

## Available Examples

### 1. Go CLI Client (`agui-client/`)

A command-line client demonstrating AG-UI SSE streaming in Go.

**Features:**
- Interactive terminal interface
- Real-time event streaming
- All AG-UI event type support
- Simple and lightweight

**Quick Start:**
```bash
cd agui-client
go run .
```

**Documentation:** [agui-client/README.md](agui-client/README.md)

### 2. React Web Client (`agui-web-client/`)

A modern React web application with a chat interface.

**Features:**
- Beautiful dark theme UI
- Real-time message streaming
- Tool call visualization
- Event debugging panel
- Stop/cancel functionality

**Quick Start:**
```bash
cd agui-web-client
npm install
npm run dev
```

**Documentation:** [agui-web-client/README.md](agui-web-client/README.md)

## AG-UI Protocol Overview

AG-UI (Agentic UI) is a protocol for streaming agent interactions to user interfaces. It uses Server-Sent Events (SSE) for real-time communication.

### Core Concepts

| Concept | Description |
|---------|-------------|
| **Thread** | A conversation session |
| **Run** | A single agent execution within a thread |
| **Event** | A protocol message sent from server to client |
| **SSE** | Server-Sent Events transport |

### Event Types

#### Lifecycle Events
- `RUN_STARTED` - Agent execution began
- `RUN_FINISHED` - Agent execution completed
- `RUN_ERROR` - An error occurred

#### Message Events
- `TEXT_MESSAGE_START` - Text message streaming started
- `TEXT_MESSAGE_CONTENT` - Text content delta
- `TEXT_MESSAGE_END` - Text message completed

#### Tool Events
- `TOOL_CALL_START` - Tool invocation started
- `TOOL_CALL_ARGS` - Tool arguments (streaming)
- `TOOL_CALL_END` - Tool call completed
- `TOOL_CALL_RESULT` - Tool execution result

#### Other Events
- `STEP_STARTED` / `STEP_FINISHED` - Processing steps
- `STATE_DELTA` - State changes
- `ACTIVITY_*` - Long-running activities
- `CUSTOM` - Custom events

### Request Format

```json
{
  "threadId": "unique-thread-id",
  "runId": "unique-run-id",
  "messages": [
    {
      "role": "user",
      "content": "Hello, agent!"
    }
  ]
}
```

### Response Format (SSE)

```
data: {"type":"RUN_STARTED","timestamp":1234567890,"data":{}}

data: {"type":"TEXT_MESSAGE_START","messageId":"msg-1"}

data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"msg-1","content":"Hello"}

data: {"type":"TEXT_MESSAGE_END","messageId":"msg-1"}

data: {"type":"RUN_FINISHED","timestamp":1234567895}
```

## Architecture

```
┌─────────────────┐     SSE      ┌─────────────────┐
│   AG-UI Client  │◄────────────►│   AG-UI Server  │
│                 │   HTTP POST  │                 │
│ - Send message  │─────────────►│ - Process run   │
│ - Stream events │◄─────────────│ - Stream events │
└─────────────────┘              └─────────────────┘
```

## Getting Started

### Prerequisites

For Go client:
- Go 1.21+

For Web client:
- Node.js 18+
- npm or yarn

### Server Setup

You need an AG-UI compatible server. For Crush:

```bash
crush --agui --agui-port 8080
```

### Choose Your Client

**Use Go client if:**
- You want a simple terminal interface
- You're developing in Go
- You need to debug server events

**Use Web client if:**
- You want a modern chat UI
- You're developing a web application
- You need to visualize tool calls

## Reference Implementations

These examples are based on:

1. [trpc-agent-go/examples/agui](https://github.com/trpc-agent-go/examples/agui) - trpc Go examples
2. [ag-ui/sdks/community/go](https://github.com/ag-ui-protocol/ag-ui/sdks/community/go) - AG-UI SDK

## Common Issues

### Connection Refused
- Ensure server is running on the specified port
- Check firewall settings
- Verify endpoint URL

### CORS Errors (Web)
- Server must enable CORS for your origin
- Use `--agui-cors-origins` flag if available

### Stream Disconnects
- Check network stability
- Server may have timeouts configured
- Implement reconnection logic for production

## Contributing

When adding new examples:

1. Follow the existing structure
2. Include comprehensive README
3. Handle all AG-UI event types
4. Provide working code with minimal dependencies

## License

Apache License 2.0
