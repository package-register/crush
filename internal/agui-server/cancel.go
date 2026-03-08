package aguiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// ErrRunNotFound is returned when a run is not found for cancellation.
var ErrRunNotFound = errors.New("agui: run not found")

// Canceler defines the interface for canceling agent runs.
type Canceler interface {
	// Cancel cancels a running agent session.
	Cancel(ctx context.Context, threadID string) error
}

// runContext holds the context and cancel function for a running session.
type runContext struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// CancelHandler handles cancel requests for AG-UI sessions.
type CancelHandler struct {
	mu       sync.Mutex
	running  map[string]*runContext
	canceler Canceler
}

// NewCancelHandler creates a new CancelHandler.
func NewCancelHandler(canceler Canceler) *CancelHandler {
	return &CancelHandler{
		running:  make(map[string]*runContext),
		canceler: canceler,
	}
}

// Register registers a running session for cancellation.
func (h *CancelHandler) Register(threadID string, ctx context.Context, cancel context.CancelFunc) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.running[threadID]; ok {
		return fmt.Errorf("session %s already running", threadID)
	}

	h.running[threadID] = &runContext{
		ctx:    ctx,
		cancel: cancel,
	}
	return nil
}

// Unregister unregisters a session from cancellation tracking.
func (h *CancelHandler) Unregister(threadID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.running, threadID)
}

// HandleCancel handles incoming cancel requests from clients.
// It validates the request, looks up the session, and cancels the running agent.
func (h *CancelHandler) HandleCancel(w http.ResponseWriter, r *http.Request) {
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

	// Check if canceler is available
	if h.canceler == nil {
		http.Error(w, "canceler not configured", http.StatusNotImplemented)
		return
	}

	// Execute cancel operation
	// Note: We use threadID as the key, ignoring runID as per AG-UI spec
	if err := h.canceler.Cancel(ctx, req.ThreadID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrRunNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	// Set CORS headers and return success
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
}

// IsRunning checks if a session is currently running.
func (h *CancelHandler) IsRunning(threadID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.running[threadID]
	return ok
}

// Count returns the number of running sessions.
func (h *CancelHandler) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.running)
}
