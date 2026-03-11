// Package aguiserver implements the AG-UI Protocol server for Crush.
// It provides SSE-based streaming of AG-UI events to external clients.
package aguiserver

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Server defines the interface for an AG-UI Protocol server.
type Server interface {
	// Start starts the AG-UI server.
	Start(ctx context.Context) error
	// Stop stops the AG-UI server gracefully.
	Stop(ctx context.Context) error
	// HandleSSE handles incoming SSE connections from clients.
	HandleSSE(w http.ResponseWriter, r *http.Request)
	// HandleRun handles incoming run requests from clients.
	HandleRun(w http.ResponseWriter, r *http.Request)
	// HandleCancel handles incoming cancel requests from clients.
	HandleCancel(w http.ResponseWriter, r *http.Request)
}

// Connection represents a single AG-UI client connection.
type Connection struct {
	// ID is the unique identifier for this connection.
	ID string
	// ThreadID is the conversation thread identifier.
	ThreadID string
	// RunID is the current run identifier.
	RunID string
	// CreatedAt is the time when the connection was established.
	CreatedAt time.Time
	// Events is the channel for outgoing events.
	Events chan Event
	// Done signals when the connection should be closed.
	Done chan struct{}
	// mu protects concurrent access to connection state.
	mu sync.Mutex
	// heartbeatCancel cancels the heartbeat goroutine.
	heartbeatCancel context.CancelFunc
}

// NewConnection creates a new Connection with the given parameters.
func NewConnection(id, threadID, runID string) *Connection {
	return &Connection{
		ID:        id,
		ThreadID:  threadID,
		RunID:     runID,
		CreatedAt: time.Now(),
		Events:    make(chan Event, 64),
		Done:      make(chan struct{}),
	}
}

// ServerConfig holds the configuration for the AG-UI server.
type ServerConfig struct {
	// Port is the HTTP port to listen on.
	Port int
	// BasePath is the base path for AG-UI endpoints.
	BasePath string
	// CORSOrigins is the list of allowed CORS origins.
	CORSOrigins []string
}

// DefaultServerConfig returns a ServerConfig with default values.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:        8080,
		BasePath:    "/agui",
		CORSOrigins: []string{"*"},
	}
}

// server is the default implementation of the Server interface.
type server struct {
	config          ServerConfig
	rateLimitConfig RateLimitConfig
	mu              sync.RWMutex
	connections     map[string]*Connection
	httpServer      *http.Server
	historyService  HistoryService
	// Rate limiting components
	globalLimiter   *TokenBucket
	sessionLimiters *RateLimiter
	requestTracker  *RequestTracker
}

// NewServer creates a new AG-UI server with the given configuration.
func NewServer(config ServerConfig) Server {
	return NewServerWithRateLimit(config, DefaultRateLimitConfig())
}

// NewServerWithRateLimit creates a new AG-UI server with rate limiting.
func NewServerWithRateLimit(config ServerConfig, rateLimitConfig RateLimitConfig) Server {
	s := &server{
		config:          config,
		rateLimitConfig: rateLimitConfig,
		connections:     make(map[string]*Connection),
		// Initialize rate limiting components
		globalLimiter:   NewTokenBucket(rateLimitConfig.GlobalBurst, rateLimitConfig.GlobalQPS),
		sessionLimiters: NewRateLimiter(rateLimitConfig.SessionQPS, rateLimitConfig.SessionBurst),
		requestTracker:  NewRequestTracker(rateLimitConfig.RequestTimeout, rateLimitConfig.RequestTimeout/2),
	}
	return s
}

// RunRequest represents a request to start a new agent run.
type RunRequest struct {
	// ThreadID is the conversation thread identifier.
	ThreadID string `json:"threadId"`
	// RunID is the unique identifier for this run.
	RunID string `json:"runId"`
	// Messages is the list of messages in the conversation.
	Messages []Message `json:"messages"`
	// Tools is the list of tools available for this run.
	Tools []Tool `json:"tools"`
	// State is the optional initial state.
	State map[string]any `json:"state,omitempty"`
}

// Message represents a message in the conversation.
type Message struct {
	// ID is the unique identifier for the message.
	ID string `json:"id,omitempty"`
	// Role is the role of the message sender.
	Role string `json:"role"`
	// Content is the message content.
	Content string `json:"content"`
}

// Tool represents a tool available for the agent.
type Tool struct {
	// Name is the tool name.
	Name string `json:"name"`
	// Description is the tool description.
	Description string `json:"description"`
	// Parameters is the tool parameters schema.
	Parameters map[string]any `json:"parameters,omitempty"`
}

// RunResponse represents the response to a run request.
type RunResponse struct {
	// ThreadID is the conversation thread identifier.
	ThreadID string `json:"threadId"`
	// RunID is the run identifier.
	RunID string `json:"runId"`
	// Status is the run status.
	Status string `json:"status"`
}
