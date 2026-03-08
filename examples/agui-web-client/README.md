# AG-UI Web Client Example

A React-based web client demonstrating how to connect to an AG-UI server using SSE (Server-Sent Events).

## Features

- Real-time SSE streaming from AG-UI server
- Chat interface with message history
- Tool call visualization
- Event stream debugging panel
- Stop/cancel active streaming
- Dark theme UI

## Prerequisites

- Node.js 18 or later
- npm or yarn

## Quick Start

### 1. Install Dependencies

```bash
cd examples/agui-web-client
npm install
```

### 2. Start the AG-UI Server

Make sure your AG-UI server is running:

```bash
# Example: Start Crush with AG-UI enabled
crush --agui --agui-port 8080
```

### 3. Start the Development Server

```bash
npm run dev
```

The app will be available at `http://localhost:3000`.

### 4. Open in Browser

Navigate to `http://localhost:3000` in your web browser.

## Usage

1. **Configure Endpoint**: Enter your AG-UI server endpoint (default: `http://localhost:8080/agui/sse`)

2. **Send Messages**: Type your message in the input box and press Enter or click Send

3. **View Responses**: Agent responses stream in real-time with typing indicators

4. **Monitor Events**: The events panel shows raw AG-UI events for debugging

5. **Stop Streaming**: Click the Stop button to cancel an active streaming session

## Project Structure

```
agui-web-client/
├── index.html           # HTML entry point
├── package.json         # Dependencies and scripts
├── tsconfig.json        # TypeScript configuration
├── vite.config.ts       # Vite bundler configuration
└── src/
    ├── main.tsx         # React entry point
    ├── App.tsx          # Main chat component
    ├── agui.ts          # AG-UI SSE streaming utilities
    └── index.css        # Global styles
```

## Core Components

### `agui.ts` - SSE Utilities

Provides functions for streaming AG-UI events:

```typescript
// Stream SSE events
await streamAguiSse(endpoint, payload, {
  onEvent: (event) => { /* handle event */ },
  onError: (error) => { /* handle error */ },
  onComplete: () => { /* stream finished */ },
});

// Generate unique IDs
const id = generateId('prefix');

// Format events for display
const text = formatEventText(event);
```

### `App.tsx` - Chat Interface

Main React component with:
- Message list with user/assistant differentiation
- Tool call visualization
- Real-time streaming status
- Event debugging panel

## AG-UI Event Handling

The client handles the following event types:

| Event Type | Description | UI Behavior |
|------------|-------------|-------------|
| `RUN_STARTED` | Agent run begins | Shows streaming indicator |
| `RUN_FINISHED` | Agent run complete | Hides streaming indicator |
| `RUN_ERROR` | Error occurred | Shows error message |
| `TEXT_MESSAGE_START` | Message begins | Creates new assistant message |
| `TEXT_MESSAGE_CONTENT` | Content delta | Appends to current message |
| `TEXT_MESSAGE_END` | Message complete | Marks message as complete |
| `TOOL_CALL_START` | Tool invocation | Shows tool call card |
| `TOOL_CALL_ARGS` | Arguments stream | Updates tool arguments |
| `TOOL_CALL_RESULT` | Tool result | Shows result in tool card |

## Request Format

```typescript
{
  threadId: string;      // Conversation thread ID
  runId: string;         // Unique run identifier
  messages: [{
    role: 'user' | 'assistant' | 'system';
    content: string;
  }];
}
```

## Response Format

SSE frames contain JSON events:

```json
{
  "type": "TEXT_MESSAGE_CONTENT",
  "timestamp": 1234567890,
  "content": "Message delta..."
}
```

## Customization

### Change Default Endpoint

Modify `DEFAULT_ENDPOINT` in `App.tsx`:

```typescript
const DEFAULT_ENDPOINT = 'http://your-server:8080/agui/sse';
```

### Add Custom Event Handlers

Extend the `handleEvent` function in `App.tsx`:

```typescript
case 'CUSTOM':
  // Handle custom events
  const customData = (event as any).value;
  // Your custom logic
  break;
```

### Style Customization

Edit the `styles` object in `App.tsx` or modify `index.css`.

## Build for Production

```bash
npm run build
```

Output will be in the `dist/` directory.

## Preview Production Build

```bash
npm run preview
```

## Troubleshooting

### Connection Issues

- Verify the AG-UI server is running
- Check the endpoint URL is correct
- Ensure CORS is enabled on the server
- Check browser console for errors

### Streaming Not Working

- Ensure server sends proper SSE format
- Check network tab for SSE connection
- Verify event format matches AG-UI spec

## Related Resources

- [AG-UI Protocol Specification](https://github.com/ag-ui-protocol/ag-ui)
- [trpc-agent-go Web Examples](https://github.com/trpc-agent-go/examples/agui/client/tdesign-chat)
- [Server-Sent Events MDN](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)

## License

Apache License 2.0
