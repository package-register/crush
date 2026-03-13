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
 * Parse SSE frame and extract event data.
 * Crush sends { type, timestamp, data } - we flatten data for handler convenience.
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
    const raw = JSON.parse(data) as AguiEvent & { data?: Record<string, unknown> };
    // Flatten data onto event so handlers can use event.content, event.error, etc.
    if (raw.data && typeof raw.data === 'object') {
      return { ...raw.data, type: raw.type, timestamp: raw.timestamp } as AguiEvent;
    }
    return raw;
  } catch {
    return { type: 'CUSTOM', raw: data, timestamp: Date.now() };
  }
}

/**
 * Derive run URL from SSE endpoint. Crush uses GET /agui/sse and POST /agui/run.
 */
function getRunUrl(sseUrl: string): string {
  const base = sseUrl.split('?')[0].replace(/\/sse\/?$/, '');
  return base + (base.endsWith('/') ? 'run' : '/run');
}

/**
 * Stream AG-UI events from Crush server.
 * Crush uses dual endpoints: GET /agui/sse for streaming, POST /agui/run to trigger.
 */
export async function streamAguiSse(
  sseUrl: string,
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
  const runUrl = getRunUrl(sseUrl);
  const sep = sseUrl.includes('?') ? '&' : '?';
  const sseUrlWithQuery = `${sseUrl}${sep}threadId=${encodeURIComponent(payload.threadId)}&runId=${encodeURIComponent(payload.runId)}`;

  return new Promise((resolve, reject) => {
    const handleAbort = () => {
      eventSource.close();
      options.onComplete?.();
      resolve();
    };

    if (options.signal?.aborted) {
      handleAbort();
      return;
    }

    const eventSource = new EventSource(sseUrlWithQuery);
    let finished = false;

    const finish = () => {
      if (finished) return;
      finished = true;
      eventSource.close();
      options.onComplete?.();
      resolve();
    };

    options.signal?.addEventListener('abort', handleAbort);

    eventSource.onopen = () => {
      // Connection established - trigger the run
      fetch(runUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
        signal: options.signal,
      })
        .then((r) => {
          if (!r.ok) {
            return r.text().then((t) => {
              throw new Error(`AG-UI run failed (${r.status}): ${t || r.statusText}`);
            });
          }
        })
        .catch((err) => {
          if (options.signal?.aborted) return;
          options.onError?.(err);
          reject(err);
          finish();
        });
    };

    eventSource.onmessage = (e) => {
      const event = parseSseFrame(e.data);
      if (event) {
        options.onEvent(event);
        if (event.type === 'RUN_FINISHED' || event.type === 'RUN_ERROR') {
          finish();
        }
      }
    };

    eventSource.onerror = () => {
      if (options.signal?.aborted) return;
      if (!finished) {
        options.onError?.(new Error('SSE connection error'));
        finish();
      }
    };
  });
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
