// Package aguiserver implements the AG-UI Protocol server for Crush.
// It provides SSE-based streaming of AG-UI events to external clients.
package aguiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SSEHandler handles Server-Sent Events connections.
type SSEHandler struct {
	config  ServerConfig
	manager *ConnectionManager
	mu      sync.RWMutex
}

// NewSSEHandler creates a new SSEHandler.
func NewSSEHandler(config ServerConfig, manager *ConnectionManager) *SSEHandler {
	return &SSEHandler{
		config:  config,
		manager: manager,
	}
}

// HandleSSE handles incoming SSE connections from clients.
// It sets up SSE headers, creates a connection, and streams events.
func (h *SSEHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	h.setSSEHeaders(w)

	// Get threadId and runId from query parameters or generate new ones
	threadID := r.URL.Query().Get("threadId")
	if threadID == "" {
		threadID = uuid.New().String()
	}

	runID := r.URL.Query().Get("runId")
	if runID == "" {
		runID = uuid.New().String()
	}

	// Create connection
	conn := NewConnection(uuid.New().String(), threadID, runID)

	// Register connection
	h.manager.Add(conn)

	// Ensure cleanup
	defer func() {
		h.manager.Remove(conn.ID)
		close(conn.Done)
	}()

	// Send initial connection confirmation
	initialEvent := RunStartedBuilder().
		WithThreadID(threadID).
		WithRunID(runID).
		Build()
	h.sendEvent(w, initialEvent)

	// Start heartbeat
	h.startHeartbeat(conn)
	defer h.stopHeartbeat(conn)

	// Stream events
	h.streamEvents(w, r, conn)
}

// setSSEHeaders sets the required headers for SSE connections.
func (h *SSEHandler) setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// sendEvent sends an event to the SSE connection.
func (h *SSEHandler) sendEvent(w http.ResponseWriter, event Event) {
	buf := getEventBuffer()
	defer putEventBuffer(buf)
	buf.Reset() // Ensure buffer is clean

	// Write SSE format: "data: <json>\n\n"
	buf.WriteString("data: ")

	// Encode JSON directly to buffer
	if err := json.NewEncoder(buf).Encode(event); err != nil {
		return
	}

	// Remove trailing newline from json.Encoder and write
	data := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	w.Write(data)
	w.Write([]byte("\n\n"))

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// streamEvents streams events from the connection's event channel.
func (h *SSEHandler) streamEvents(w http.ResponseWriter, r *http.Request, conn *Connection) {
	for {
		select {
		case event := <-conn.Events:
			h.sendEvent(w, event)
		case <-conn.Done:
			return
		case <-r.Context().Done():
			return
		}
	}
}

// heartbeatTicker is the interval for heartbeat events.
const heartbeatTicker = 15 * time.Second

// startHeartbeat starts sending heartbeat events to keep the connection alive.
func (h *SSEHandler) startHeartbeat(conn *Connection) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.heartbeatCancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	conn.heartbeatCancel = cancel

	go func() {
		ticker := time.NewTicker(heartbeatTicker)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				select {
				case conn.Events <- NewEvent(CustomEvent, map[string]string{"type": "heartbeat"}):
				case <-conn.Done:
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// stopHeartbeat stops the heartbeat for a connection.
func (h *SSEHandler) stopHeartbeat(conn *Connection) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.heartbeatCancel != nil {
		conn.heartbeatCancel()
		conn.heartbeatCancel = nil
	}
}

// BroadcastToConnection sends an event to a specific connection.
func (h *SSEHandler) BroadcastToConnection(connID string, event Event) error {
	// Check for nil manager
	if h.manager == nil {
		return fmt.Errorf("connection manager is nil")
	}

	conn, ok := h.manager.Get(connID)
	if !ok {
		return fmt.Errorf("connection %s not found", connID)
	}

	select {
	case conn.Events <- event:
		return nil
	case <-conn.Done:
		return fmt.Errorf("connection %s is closed", connID)
	default:
		return fmt.Errorf("connection %s event channel is full", connID)
	}
}

// BroadcastToThread sends an event to all connections for a specific thread.
func (h *SSEHandler) BroadcastToThread(threadID string, event Event) int {
	count := 0
	conns := h.manager.List()

	for _, conn := range conns {
		if conn.ThreadID == threadID {
			select {
			case conn.Events <- event:
				count++
			case <-conn.Done:
			default:
			}
		}
	}

	return count
}

// BroadcastToAll sends an event to all connections.
func (h *SSEHandler) BroadcastToAll(event Event) int {
	count := 0
	conns := h.manager.List()

	for _, conn := range conns {
		select {
		case conn.Events <- event:
			count++
		case <-conn.Done:
		default:
		}
	}

	return count
}
