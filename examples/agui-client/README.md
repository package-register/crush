# AG-UI Client Example

This example demonstrates how to connect to an AG-UI server using SSE (Server-Sent Events) streaming.

## Features

- Connect to AG-UI server via SSE
- Send messages to the agent
- Receive and display streaming events
- Handle all AG-UI event types

## Prerequisites

- Go 1.21 or later

## Quick Start

### 1. Start the AG-UI Server

Ensure Crush is running with AG-UI enabled. Add to `crush.json`:

```json
{
  "options": {
    "agui_server": {
      "enabled": true,
      "port": 8080
    }
  }
}
```

Or use TUI: `/` → **Configure AGUI Server**.

### 2. Run the Client

```bash
cd examples/agui-client
go run .
```

Or specify a custom endpoint:

```bash
go run . --endpoint http://localhost:8080/agui/sse
```

### 3. Interact with the Agent

Type your message and press Enter:

```
AG-UI Client Example
Endpoint: http://127.0.0.1:8080/agui/sse
Type your prompt and press Enter (Ctrl+D to exit, or type 'quit').

You> Hello, can you help me with a task?
Agent> [RUN_STARTED]
Agent> [TEXT_MESSAGE_START]
Agent> [TEXT_MESSAGE_CONTENT] Hello! I'd be happy to help you...
Agent> [TEXT_MESSAGE_END]
Agent> [RUN_FINISHED]

You> quit
```

## Event Types

The client handles the following AG-UI event types:

### Lifecycle Events

| Event | Description |
|-------|-------------|
| `RUN_STARTED` | Agent run has started |
| `RUN_FINISHED` | Agent run has completed |
| `RUN_ERROR` | An error occurred during the run |

### Text Message Events

| Event | Description |
|-------|-------------|
| `TEXT_MESSAGE_START` | Text message streaming started |
| `TEXT_MESSAGE_CONTENT` | Text content delta (streaming) |
| `TEXT_MESSAGE_END` | Text message streaming completed |

### Tool Call Events

| Event | Description |
|-------|-------------|
| `TOOL_CALL_START` | Tool call started |
| `TOOL_CALL_ARGS` | Tool arguments (streaming) |
| `TOOL_CALL_END` | Tool call completed |
| `TOOL_CALL_RESULT` | Tool execution result |

### Step Events

| Event | Description |
|-------|-------------|
| `STEP_STARTED` | Processing step started |
| `STEP_FINISHED` | Processing step completed |

### State Events

| Event | Description |
|-------|-------------|
| `STATE_DELTA` | State change notification |

### Activity Events

| Event | Description |
|-------|-------------|
| `ACTIVITY_START` | Long-running activity started |
| `ACTIVITY_UPDATE` | Activity progress update |
| `ACTIVITY_END` | Activity completed |

### Custom Events

| Event | Description |
|-------|-------------|
| `CUSTOM` | Custom event with name and value |

## Code Structure

```
agui-client/
├── main.go          # Main client implementation
├── go.mod           # Go module definition
└── README.md        # This file
```

## How It Works (Crush Dual-Endpoint)

1. **SSE connection**: GET `/agui/sse?threadId=X&runId=Y` – establishes event stream
2. **Trigger run**: POST `/agui/run` with `{threadId, runId, messages}` – starts agent
3. **Streaming**: Events flow through the GET connection in real-time
4. **Parsing**: Parses SSE frames (format: `{type, timestamp, data}`)
5. **Display**: Formats and displays events to the console

## Request Format

```json
{
  "threadId": "unique-thread-id",
  "runId": "unique-run-id",
  "messages": [
    {
      "role": "user",
      "content": "Your message here"
    }
  ]
}
```

## Response Format

Each SSE frame contains:

```
data: {"type":"EVENT_TYPE","timestamp":1234567890,"data":{...}}
```

## Error Handling

The client handles:
- Connection errors
- HTTP errors
- JSON parsing errors
- Stream interruptions

Errors are printed to stderr and the client continues to accept new prompts.

## Customization

### Change Timeouts

Modify the constants at the top of `main.go`:

```go
const (
    requestTimeout   = 2 * time.Minute
    readTimeout      = 5 * time.Minute
)
```

### Add Custom Event Handling

Extend the `formatEvent` function to handle additional event types:

```go
case "YOUR_EVENT_TYPE":
    // Your custom handling
    return []string{fmt.Sprintf("Agent> %s your message", label)}
```

## Related Examples

- [trpc-agent-go/examples/agui/client/raw](https://github.com/trpc-agent-go/examples/agui/client/raw) - Raw SSE client
- [ag-ui/sdks/community/go/example/client](https://github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/client) - AG-UI SDK examples

## License

Apache License 2.0
