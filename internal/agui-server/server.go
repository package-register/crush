package aguiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Start starts the AG-UI server.
func (s *server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Register SSE endpoint
	ssePath := fmt.Sprintf("%s/sse", s.config.BasePath)
	mux.HandleFunc(ssePath, s.HandleSSE)

	// Register run endpoint
	runPath := fmt.Sprintf("%s/run", s.config.BasePath)
	mux.HandleFunc(runPath, s.HandleRun)

	// Register cancel endpoint
	cancelPath := fmt.Sprintf("%s/cancel", s.config.BasePath)
	mux.HandleFunc(cancelPath, s.HandleCancel)

	s.mu.Lock()
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.Port),
		Handler: s.withCORS(s.withRateLimit(mux)),
	}
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.mu.RLock()
		hs := s.httpServer
		s.mu.RUnlock()
		if hs != nil {
			hs.Shutdown(context.Background())
		}
	}()

	s.mu.RLock()
	hs := s.httpServer
	s.mu.RUnlock()
	if hs == nil {
		return fmt.Errorf("server not initialized")
	}
	return hs.ListenAndServe()
}

// Stop stops the AG-UI server gracefully.
func (s *server) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.httpServer == nil {
		s.mu.Unlock()
		return nil
	}

	// Close all connections
	if s.connectionManager != nil {
		s.connectionManager.CloseAll()
	}

	hs := s.httpServer
	s.httpServer = nil
	s.mu.Unlock()

	// Stop request tracker to prevent goroutine leak
	s.requestTracker.Close()

	return hs.Shutdown(ctx)
}

// HandleSSE handles incoming SSE connections from clients.
func (s *server) HandleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

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
	if s.connectionManager != nil {
		s.connectionManager.Add(conn)
	}

	// Ensure cleanup
	defer func() {
		if s.connectionManager != nil {
			s.connectionManager.Remove(conn.ID)
		}
		close(conn.Done)
	}()

	// Send initial connection confirmation
	initialEvent := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: threadID,
		RunID:    runID,
	})
	fmt.Fprintf(w, "data: %s\n\n", mustMarshal(initialEvent))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Stream events
	for {
		select {
		case event := <-conn.Events:
			fmt.Fprintf(w, "data: %s\n\n", mustMarshal(event))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-conn.Done:
			return
		case <-r.Context().Done():
			return
		}
	}
}

// HandleRun handles incoming run requests from clients.
func (s *server) HandleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if err := s.validateRequest(req); err != nil {
		http.Error(w, fmt.Sprintf("Validation failed: %v", err), http.StatusBadRequest)
		return
	}

	// Generate IDs if not provided
	if req.ThreadID == "" {
		req.ThreadID = uuid.New().String()
	}
	if req.RunID == "" {
		req.RunID = uuid.New().String()
	}

	// Return response
	resp := RunResponse{
		ThreadID: req.ThreadID,
		RunID:    req.RunID,
		Status:   "started",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)

	// Start agent execution in background
	s.mu.RLock()
	executor := s.agentExecutor
	s.mu.RUnlock()
	if executor != nil {
		go func() {
			ctx := context.WithoutCancel(r.Context())
			if err := executor.Execute(ctx, req); err != nil {
				// Error already emitted by AgentBridge via eventEmitter
				_ = err
			}
		}()
	}
}

// validateRequest validates a run request.
func (s *server) validateRequest(req RunRequest) error {
	if len(req.Messages) == 0 {
		return fmt.Errorf("messages cannot be empty")
	}

	for i, msg := range req.Messages {
		if msg.Role == "" {
			return fmt.Errorf("message[%d]: role is required", i)
		}
	}

	return nil
}

// HandleCancel handles incoming cancel requests from clients.
// It cancels a running agent session by threadID.
func (s *server) HandleCancel(w http.ResponseWriter, r *http.Request) {
	// Use context.WithoutCancel to prevent the request context from being
	// canceled if the client disconnects.
	ctx := context.WithoutCancel(r.Context())

	// CORS preflight handling
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate threadID
	if req.ThreadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	executor := s.agentExecutor
	s.mu.RUnlock()

	canceler, ok := executor.(Canceler)
	if !ok || canceler == nil {
		http.Error(w, "executor does not support cancel", http.StatusNotImplemented)
		return
	}
	if err := canceler.Cancel(ctx, req.ThreadID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrRunNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
}

// HandleHistory handles history snapshot requests with SSE streaming.
func (s *server) HandleHistory(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS preflight
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", http.MethodGet)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Only allow GET method
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	threadID := r.URL.Query().Get("threadId")
	limit := 50
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Validate threadId
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	// Get history from service
	var entries []HistoryEntry
	var total int

	if s.historyService != nil {
		entries, total = s.historyService.GetThreadHistory(threadID, limit, offset)
	}

	// Ensure non-nil slices for JSON
	if entries == nil {
		entries = []HistoryEntry{}
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Stream the response
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send RUN_STARTED event
	startEvent := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: threadID,
		RunID:    uuid.New().String(),
	})
	fmt.Fprintf(w, "data: %s\n\n", mustMarshal(startEvent))
	flusher.Flush()

	// Send MESSAGES_SNAPSHOT event
	response := HistoryResponse{
		Messages: entries,
		Total:    total,
	}
	snapshotEvent := NewEvent(CustomEvent, CustomEventData{
		Type:    "MESSAGES_SNAPSHOT",
		Payload: response,
	})
	fmt.Fprintf(w, "data: %s\n\n", mustMarshal(snapshotEvent))
	flusher.Flush()

	// Send RUN_FINISHED event
	finishEvent := NewEvent(RunFinished, RunFinishedEvent{
		ThreadID: threadID,
		RunID:    startEvent.Data.(RunStartedEvent).RunID,
	})
	fmt.Fprintf(w, "data: %s\n\n", mustMarshal(finishEvent))
	flusher.Flush()
}

// withRateLimit adds rate limiting to the handler.
func (s *server) withRateLimit(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract session ID from request
		sessionID := extractSessionID(r)

		// Check global rate limit
		if !s.globalLimiter.Allow() {
			writeRateLimitError(w, "global", int(s.rateLimitConfig.GlobalQPS), 1*time.Second)
			return
		}

		// Check session rate limit
		if sessionID != "" {
			if !s.sessionLimiters.Allow(sessionID) {
				writeRateLimitError(w, "session", int(s.rateLimitConfig.SessionQPS), 1*time.Second)
				return
			}
		}

		// Check for duplicate request (RequestID deduplication)
		requestID := extractRequestID(r)
		if requestID != "" {
			if !s.requestTracker.Track(requestID) {
				writeDuplicateRequestError(w, requestID)
				return
			}
		}

		handler.ServeHTTP(w, r)
	})
}

// withCORS adds CORS headers to the handler.
func (s *server) withCORS(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(s.config.CORSOrigins) > 0 {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range s.config.CORSOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

// mustMarshal marshals a value to JSON, panicking on error.
func mustMarshal(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Errorf("failed to marshal JSON: %w", err))
	}
	return string(data)
}

// ConnectionManager manages AG-UI connections.
type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]*Connection
}

// NewConnectionManager creates a new ConnectionManager.
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*Connection),
	}
}

// Add adds a connection to the manager.
func (m *ConnectionManager) Add(conn *Connection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connections[conn.ID] = conn
}

// Remove removes a connection from the manager.
func (m *ConnectionManager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.connections, id)
}

// Get retrieves a connection by ID.
func (m *ConnectionManager) Get(id string) (*Connection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conn, ok := m.connections[id]
	return conn, ok
}

// List returns all connections.
func (m *ConnectionManager) List() []*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conns := make([]*Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		conns = append(conns, conn)
	}
	return conns
}

// Count returns the number of connections.
func (m *ConnectionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}

// GetByThread retrieves all connections for a specific thread.
func (m *ConnectionManager) GetByThread(threadID string) []*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var conns []*Connection
	for _, conn := range m.connections {
		if conn.ThreadID == threadID {
			conns = append(conns, conn)
		}
	}
	return conns
}

// CloseAll closes all connections.
func (m *ConnectionManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, conn := range m.connections {
		close(conn.Done)
	}
	m.connections = make(map[string]*Connection)
}
