// Package aguiserver implements the AG-UI Protocol server for Crush.
// It provides SSE-based streaming of AG-UI events to external clients.
package aguiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Handler handles HTTP requests for the AG-UI server.
type Handler struct {
	config         ServerConfig
	eventEmitter   EventEmitter
	agentExecutor  AgentExecutor
	historyService HistoryService
}

// EventEmitter defines the interface for emitting AG-UI events.
type EventEmitter interface {
	Emit(event Event) error
	EmitToThread(threadID string, event Event) error
}

// AgentExecutor defines the interface for executing agent runs.
type AgentExecutor interface {
	Execute(ctx context.Context, req RunRequest) error
}

// NewHandler creates a new Handler.
func NewHandler(config ServerConfig, eventEmitter EventEmitter, agentExecutor AgentExecutor) *Handler {
	return &Handler{
		config:         config,
		eventEmitter:   eventEmitter,
		agentExecutor:  agentExecutor,
		historyService: nil,
	}
}

// WithHistoryService sets the history service for the handler.
func (h *Handler) WithHistoryService(hs HistoryService) {
	h.historyService = hs
}

// HandleRun handles incoming run requests from clients.
// It validates the request, generates IDs if needed, and starts agent execution.
func (h *Handler) HandleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.validateRequest(req); err != nil {
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

	// Save fields to local variables before goroutine to avoid race conditions
	executor := h.agentExecutor
	emitter := h.eventEmitter
	threadID := req.ThreadID
	runID := req.RunID

	// Start agent execution in background
	go func() {
		// Check nil before execution
		if executor == nil {
			return
		}
		// Use context.WithoutCancel to prevent request cancellation from affecting execution
		ctx := context.WithoutCancel(r.Context())
		if err := executor.Execute(ctx, req); err != nil {
			// Emit error event (check emitter is not nil)
			if emitter != nil {
				errorEvent := RunErrorBuilder().
					WithThreadID(threadID).
					WithRunID(runID).
					WithError(err.Error()).
					Build()
				emitter.EmitToThread(threadID, errorEvent)
			}
		}
	}()

	// Return response
	resp := RunResponse{
		ThreadID: req.ThreadID,
		RunID:    req.RunID,
		Status:   "started",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// validateRequest validates a run request.
func (h *Handler) validateRequest(req RunRequest) error {
	if len(req.Messages) == 0 {
		return fmt.Errorf("messages cannot be empty")
	}

	for i, msg := range req.Messages {
		if msg.Role == "" {
			return fmt.Errorf("message[%d]: role is required", i)
		}

		// Validate role is one of the allowed values
		role := strings.ToLower(msg.Role)
		if role != "user" && role != "assistant" && role != "system" {
			return fmt.Errorf("message[%d]: invalid role '%s', must be 'user', 'assistant', or 'system'", i, msg.Role)
		}
	}

	// Validate tools if provided
	for i, tool := range req.Tools {
		if tool.Name == "" {
			return fmt.Errorf("tool[%d]: name is required", i)
		}
	}

	return nil
}

// HandleOptions handles preflight OPTIONS requests for CORS.
func (h *Handler) HandleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.WriteHeader(http.StatusNoContent)
}

// HandleCancel handles incoming cancel requests from clients.
// It cancels a running agent session by threadID.
func (h *Handler) HandleCancel(w http.ResponseWriter, r *http.Request) {
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

	// Check if agentExecutor supports cancellation
	canceler, ok := h.agentExecutor.(Canceler)
	if !ok {
		http.Error(w, "executor does not support cancel", http.StatusNotImplemented)
		return
	}

	// Execute cancel operation
	// Note: We use threadID as the key, ignoring runID as per AG-UI spec
	if err := canceler.Cancel(ctx, req.ThreadID); err != nil {
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

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// WithCORS wraps a handler with CORS support.
func WithCORS(handler http.Handler, config CORSConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false

		for _, o := range config.AllowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			if len(config.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
			}
			if len(config.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
			}
			if config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if config.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

// DefaultCORSConfig returns a default CORS configuration.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         86400,
	}
}
