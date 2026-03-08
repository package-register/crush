/**
 * AG-UI SSE Event Types
 */
export type AguiEventType =
  | 'RUN_STARTED'
  | 'RUN_FINISHED'
  | 'RUN_ERROR'
  | 'TEXT_MESSAGE_START'
  | 'TEXT_MESSAGE_CONTENT'
  | 'TEXT_MESSAGE_END'
  | 'TOOL_CALL_START'
  | 'TOOL_CALL_ARGS'
  | 'TOOL_CALL_END'
  | 'TOOL_CALL_RESULT'
  | 'STEP_STARTED'
  | 'STEP_FINISHED'
  | 'STATE_DELTA'
  | 'ACTIVITY_START'
  | 'ACTIVITY_UPDATE'
  | 'ACTIVITY_END'
  | 'CUSTOM';

/**
 * AG-UI SSE Event
 */
export interface AguiEvent {
  type: AguiEventType;
  timestamp?: number;
  data?: Record<string, unknown>;
  [key: string]: unknown;
}

/**
 * UI Message for display
 */
export interface UIMessage {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  type: 'text' | 'tool-call' | 'tool-result' | 'thinking';
  status: 'pending' | 'streaming' | 'complete' | 'error';
  timestamp: number;
  toolCall?: {
    toolCallId: string;
    toolCallName: string;
    args?: string;
    result?: string;
  };
}

/**
 * Parse SSE frame and extract event data
 */
function parseSseFrame(frame: string): AguiEvent | null {
  const lines = frame.split(/\r?\n/);
  const dataLines: string[] = [];

  for (const line of lines) {
    if (line.startsWith('data:')) {
      dataLines.push(line.slice('data:'.length).trimStart());
    }
  }

  const data = dataLines.join('\n').trim();
  if (!data) {
    return null;
  }

  try {
    return JSON.parse(data) as AguiEvent;
  } catch {
    return { type: 'CUSTOM', raw: data, timestamp: Date.now() };
  }
}

/**
 * Stream AG-UI SSE events from server
 */
export async function streamAguiSse(
  url: string,
  payload: {
    threadId: string;
    runId: string;
    messages: Array<{ role: string; content: string }>;
  },
  options: {
    onEvent: (event: AguiEvent) => void;
    onError?: (error: Error) => void;
    onComplete?: () => void;
    signal?: AbortSignal;
  },
): Promise<void> {
  try {
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
        'Cache-Control': 'no-cache',
      },
      body: JSON.stringify(payload),
      signal: options.signal,
    });

    if (!response.ok) {
      const text = await response.text().catch(() => '');
      throw new Error(`AG-UI request failed (${response.status}): ${text || response.statusText}`);
    }

    if (!response.body) {
      throw new Error('AG-UI response body is empty');
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { value, done } = await reader.read();
      if (done) {
        break;
      }

      buffer += decoder.decode(value, { stream: true });
      buffer = buffer.replace(/\r\n/g, '\n');

      while (true) {
        const boundaryIndex = buffer.indexOf('\n\n');
        if (boundaryIndex === -1) {
          break;
        }

        const frame = buffer.slice(0, boundaryIndex);
        buffer = buffer.slice(boundaryIndex + 2);
        const event = parseSseFrame(frame);

        if (event) {
          options.onEvent(event);
        }
      }
    }

    // Process remaining buffer
    const tail = buffer.trim();
    if (tail) {
      const event = parseSseFrame(tail);
      if (event) {
        options.onEvent(event);
      }
    }

    options.onComplete?.();
  } catch (error) {
    if (options.signal?.aborted) {
      return;
    }
    options.onError?.(error as Error);
    throw error;
  }
}

/**
 * Generate unique ID
 */
export function generateId(prefix: string = 'id'): string {
  const now = Date.now();
  const rand = Math.random().toString(16).slice(2);
  return `${prefix}_${now}_${rand}`;
}

/**
 * Format AG-UI event for display
 */
export function formatEventText(event: AguiEvent): string {
  switch (event.type) {
    case 'RUN_STARTED':
      return '🚀 Agent started';
    case 'RUN_FINISHED':
      return '✅ Agent finished';
    case 'RUN_ERROR':
      return `❌ Error: ${(event as any).error || 'Unknown error'}`;
    case 'TEXT_MESSAGE_CONTENT':
      return (event as any).content || '';
    case 'TOOL_CALL_START':
      return `🔧 Calling tool: ${(event as any).name || 'unknown'}`;
    case 'TOOL_CALL_RESULT':
      return `📦 Tool result: ${JSON.stringify((event as any).result)}`;
    case 'STEP_STARTED':
      return `⏱ Step started: ${(event as any).type || ''}`;
    case 'STEP_FINISHED':
      return `✅ Step finished: ${(event as any).type || ''}`;
    default:
      return `[${event.type}]`;
  }
}
