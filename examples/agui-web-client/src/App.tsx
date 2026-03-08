import React, { useState, useRef, useCallback } from 'react';
import { streamAguiSse, type AguiEvent, type UIMessage, generateId, formatEventText } from './agui';

const DEFAULT_ENDPOINT = 'http://localhost:8080/agui/sse';

export default function App() {
  const [endpoint, setEndpoint] = useState(DEFAULT_ENDPOINT);
  const [messages, setMessages] = useState<UIMessage[]>([]);
  const [input, setInput] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const [events, setEvents] = useState<AguiEvent[]>([]);
  const [error, setError] = useState<string | null>(null);

  const abortControllerRef = useRef<AbortController | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom
  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  React.useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  // Add user message
  const addUserMessage = useCallback((content: string) => {
    const message: UIMessage = {
      id: generateId('user'),
      role: 'user',
      content,
      type: 'text',
      status: 'complete',
      timestamp: Date.now(),
    };
    setMessages((prev) => [...prev, message]);
  }, []);

  // Handle AG-UI events
  const handleEvent = useCallback((event: AguiEvent) => {
    setEvents((prev) => [...prev.slice(-99), event]); // Keep last 100 events

    switch (event.type) {
      case 'RUN_STARTED':
        setIsStreaming(true);
        setError(null);
        break;

      case 'RUN_FINISHED':
        setIsStreaming(false);
        break;

      case 'RUN_ERROR':
        setIsStreaming(false);
        setError((event as any).error || 'Unknown error');
        break;

      case 'TEXT_MESSAGE_START':
        // Start a new assistant message
        setMessages((prev) => [
          ...prev,
          {
            id: generateId('assistant'),
            role: 'assistant',
            content: '',
            type: 'text',
            status: 'streaming',
            timestamp: event.timestamp || Date.now(),
          },
        ]);
        break;

      case 'TEXT_MESSAGE_CONTENT':
        // Append content to the last assistant message
        const delta = (event as any).content || '';
        if (delta) {
          setMessages((prev) => {
            const last = prev[prev.length - 1];
            if (last && last.role === 'assistant' && last.status === 'streaming') {
              return [
                ...prev.slice(0, -1),
                { ...last, content: last.content + delta },
              ];
            }
            return prev;
          });
        }
        break;

      case 'TEXT_MESSAGE_END':
        // Mark the last assistant message as complete
        setMessages((prev) => {
          const last = prev[prev.length - 1];
          if (last && last.role === 'assistant' && last.status === 'streaming') {
            return [...prev.slice(0, -1), { ...last, status: 'complete' }];
          }
          return prev;
        });
        break;

      case 'TOOL_CALL_START':
        setMessages((prev) => [
          ...prev,
          {
            id: generateId('tool'),
            role: 'assistant',
            content: '',
            type: 'tool-call',
            status: 'streaming',
            timestamp: event.timestamp || Date.now(),
            toolCall: {
              toolCallId: (event as any).toolCallId || generateId('tool'),
              toolCallName: (event as any).name || 'unknown',
              args: '',
            },
          },
        ]);
        break;

      case 'TOOL_CALL_ARGS':
        setMessages((prev) => {
          const last = prev[prev.length - 1];
          if (last && last.type === 'tool-call' && last.status === 'streaming') {
            return [
              ...prev.slice(0, -1),
              {
                ...last,
                content: last.content + ((event as any).args || ''),
                toolCall: last.toolCall
                  ? { ...last.toolCall, args: last.toolCall.args + ((event as any).args || '') }
                  : last.toolCall,
              },
            ];
          }
          return prev;
        });
        break;

      case 'TOOL_CALL_RESULT':
        setMessages((prev) => {
          const last = prev[prev.length - 1];
          if (last && last.type === 'tool-call') {
            return [
              ...prev.slice(0, -1),
              {
                ...last,
                status: 'complete',
                toolCall: last.toolCall
                  ? { ...last.toolCall, result: JSON.stringify((event as any).result) }
                  : last.toolCall,
              },
            ];
          }
          return prev;
        });
        break;

      default:
        console.log('AG-UI Event:', event);
    }
  }, []);

  // Send message to agent
  const sendMessage = useCallback(async () => {
    if (!input.trim() || isStreaming) {
      return;
    }

    const userMessage = input.trim();
    setInput('');
    addUserMessage(userMessage);

    const threadId = 'web-demo-thread';
    const runId = generateId('run');

    const payload = {
      threadId,
      runId,
      messages: [{ role: 'user', content: userMessage }],
    };

    abortControllerRef.current = new AbortController();

    try {
      await streamAguiSse(endpoint, payload, {
        onEvent: handleEvent,
        onError: (err) => {
          setError(err.message);
          setIsStreaming(false);
        },
        signal: abortControllerRef.current!.signal,
      });
    } catch (err) {
      // Error already handled in onError
    } finally {
      abortControllerRef.current = null;
    }
  }, [input, isStreaming, endpoint, addUserMessage, handleEvent]);

  // Stop streaming
  const stopStreaming = useCallback(() => {
    abortControllerRef.current?.abort();
    setIsStreaming(false);
  }, []);

  // Clear chat
  const clearChat = useCallback(() => {
    setMessages([]);
    setEvents([]);
    setError(null);
  }, []);

  // Handle Enter key
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendMessage();
      }
    },
    [sendMessage],
  );

  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <h1 style={styles.title}>AG-UI Web Client</h1>
        <div style={styles.controls}>
          <input
            type="text"
            value={endpoint}
            onChange={(e) => setEndpoint(e.target.value)}
            placeholder="AG-UI Endpoint"
            style={styles.endpointInput}
            disabled={isStreaming}
          />
          <button onClick={clearChat} style={styles.button} disabled={isStreaming}>
            Clear
          </button>
        </div>
      </header>

      {error && (
        <div style={styles.error}>
          <strong>Error:</strong> {error}
        </div>
      )}

      <main style={styles.main}>
        <div style={styles.messages}>
          {messages.length === 0 ? (
            <div style={styles.empty}>
              <p>Start a conversation with the agent!</p>
              <p style={styles.hint}>Type a message below and press Enter</p>
            </div>
          ) : (
            messages.map((msg) => (
              <div
                key={msg.id}
                style={{
                  ...styles.message,
                  ...(msg.role === 'user' ? styles.userMessage : styles.assistantMessage),
                }}
              >
                <div style={styles.messageHeader}>
                  <span style={styles.messageRole}>
                    {msg.role === 'user' ? '👤 You' : '🤖 Assistant'}
                  </span>
                  <span style={styles.messageStatus}>
                    {msg.status === 'streaming' && '⏳ '}
                    {new Date(msg.timestamp).toLocaleTimeString()}
                  </span>
                </div>
                <div style={styles.messageContent}>
                  {msg.type === 'tool-call' ? (
                    <div style={styles.toolCall}>
                      <div style={styles.toolCallHeader}>
                        🔧 {msg.toolCall?.toolCallName || 'Tool'}
                      </div>
                      {msg.toolCall?.args && (
                        <pre style={styles.toolArgs}>{msg.toolCall.args}</pre>
                      )}
                      {msg.toolCall?.result && (
                        <div style={styles.toolResult}>
                          <strong>Result:</strong> {msg.toolCall.result}
                        </div>
                      )}
                    </div>
                  ) : (
                    <div style={styles.textContent}>{msg.content}</div>
                  )}
                </div>
              </div>
            ))
          )}
          <div ref={messagesEndRef} />
        </div>
      </main>

      <footer style={styles.footer}>
        <div style={styles.inputContainer}>
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type your message..."
            style={styles.input}
            disabled={isStreaming}
            rows={3}
          />
          {isStreaming ? (
            <button onClick={stopStreaming} style={styles.stopButton}>
              ⏹ Stop
            </button>
          ) : (
            <button onClick={sendMessage} style={styles.sendButton}>
              📤 Send
            </button>
          )}
        </div>
        {events.length > 0 && (
          <div style={styles.events}>
            <strong>Recent Events:</strong>
            <div style={styles.eventsList}>
              {events.slice(-5).map((event, i) => (
                <div key={i} style={styles.eventItem}>
                  {formatEventText(event)}
                </div>
              ))}
            </div>
          </div>
        )}
      </footer>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    height: '100vh',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    backgroundColor: '#1a1a2e',
    color: '#eee',
  },
  header: {
    padding: '16px 24px',
    backgroundColor: '#16213e',
    borderBottom: '1px solid #0f3460',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: '12px',
  },
  title: {
    margin: 0,
    fontSize: '24px',
    fontWeight: 600,
  },
  controls: {
    display: 'flex',
    gap: '12px',
    alignItems: 'center',
  },
  endpointInput: {
    padding: '8px 12px',
    borderRadius: '6px',
    border: '1px solid #0f3460',
    backgroundColor: '#1a1a2e',
    color: '#eee',
    width: '300px',
    fontSize: '14px',
  },
  button: {
    padding: '8px 16px',
    borderRadius: '6px',
    border: 'none',
    backgroundColor: '#e94560',
    color: '#fff',
    cursor: 'pointer',
    fontSize: '14px',
    fontWeight: 500,
  },
  main: {
    flex: 1,
    overflow: 'hidden',
    display: 'flex',
    flexDirection: 'column',
  },
  messages: {
    flex: 1,
    overflowY: 'auto',
    padding: '24px',
    display: 'flex',
    flexDirection: 'column',
    gap: '16px',
  },
  empty: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    flex: 1,
    color: '#666',
    textAlign: 'center',
  },
  hint: {
    fontSize: '14px',
    marginTop: '8px',
  },
  message: {
    maxWidth: '80%',
    padding: '12px 16px',
    borderRadius: '12px',
    alignSelf: 'flex-start',
  },
  userMessage: {
    backgroundColor: '#0f3460',
    alignSelf: 'flex-end',
  },
  assistantMessage: {
    backgroundColor: '#16213e',
  },
  messageHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '8px',
    fontSize: '12px',
    color: '#888',
  },
  messageRole: {
    fontWeight: 500,
  },
  messageStatus: {
    fontSize: '11px',
  },
  messageContent: {
    fontSize: '14px',
    lineHeight: 1.5,
  },
  textContent: {
    whiteSpace: 'pre-wrap',
  },
  toolCall: {
    backgroundColor: '#0f3460',
    borderRadius: '8px',
    padding: '12px',
    fontFamily: 'monospace',
    fontSize: '13px',
  },
  toolCallHeader: {
    fontWeight: 600,
    marginBottom: '8px',
    color: '#e94560',
  },
  toolArgs: {
    backgroundColor: '#1a1a2e',
    padding: '8px',
    borderRadius: '4px',
    overflow: 'auto',
    maxHeight: '150px',
    margin: '8px 0',
  },
  toolResult: {
    marginTop: '8px',
    paddingTop: '8px',
    borderTop: '1px solid #333',
  },
  footer: {
    padding: '16px 24px',
    backgroundColor: '#16213e',
    borderTop: '1px solid #0f3460',
  },
  inputContainer: {
    display: 'flex',
    gap: '12px',
    marginBottom: '12px',
  },
  input: {
    flex: 1,
    padding: '12px',
    borderRadius: '8px',
    border: '1px solid #0f3460',
    backgroundColor: '#1a1a2e',
    color: '#eee',
    fontSize: '14px',
    resize: 'none',
    fontFamily: 'inherit',
  },
  sendButton: {
    padding: '12px 24px',
    borderRadius: '8px',
    border: 'none',
    backgroundColor: '#4caf50',
    color: '#fff',
    cursor: 'pointer',
    fontSize: '14px',
    fontWeight: 500,
    whiteSpace: 'nowrap',
  },
  stopButton: {
    padding: '12px 24px',
    borderRadius: '8px',
    border: 'none',
    backgroundColor: '#e94560',
    color: '#fff',
    cursor: 'pointer',
    fontSize: '14px',
    fontWeight: 500,
    whiteSpace: 'nowrap',
  },
  events: {
    fontSize: '12px',
    color: '#666',
    paddingTop: '12px',
    borderTop: '1px solid #0f3460',
  },
  eventsList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '4px',
    marginTop: '8px',
  },
  eventItem: {
    padding: '4px 8px',
    backgroundColor: '#1a1a2e',
    borderRadius: '4px',
  },
  error: {
    padding: '12px 24px',
    backgroundColor: '#e94560',
    color: '#fff',
    fontSize: '14px',
  },
};
