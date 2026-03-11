package aguiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// MockEventEmitter is a mock implementation of EventEmitter.
type MockEventEmitter struct {
	mu        sync.Mutex
	events    []Event
	threadID  string
	emitError error
}

func (m *MockEventEmitter) Emit(event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return m.emitError
}

func (m *MockEventEmitter) EmitToThread(threadID string, event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	m.threadID = threadID
	return m.emitError
}

// MockAgentExecutor is a mock implementation of AgentExecutor.
type MockAgentExecutor struct {
	mu          sync.Mutex
	executed    bool
	lastRequest RunRequest
	execError   error
}

func (m *MockAgentExecutor) Execute(ctx context.Context, req RunRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executed = true
	m.lastRequest = req
	return m.execError
}

func TestHandler_HandleRun(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	// Create valid request
	reqBody := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleRun(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp RunResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.ThreadID != "test-thread" {
		t.Errorf("Expected ThreadID test-thread, got %s", resp.ThreadID)
	}
	if resp.RunID != "test-run" {
		t.Errorf("Expected RunID test-run, got %s", resp.RunID)
	}
	if resp.Status != "started" {
		t.Errorf("Expected status started, got %s", resp.Status)
	}

	// Wait for async execution and verify with proper synchronization
	time.Sleep(50 * time.Millisecond)

	// Verify agent executor was called (with proper synchronization)
	agentExecutor.mu.Lock()
	executed := agentExecutor.executed
	agentExecutor.mu.Unlock()

	if !executed {
		t.Error("Expected agent executor to be called")
	}
}

func TestHandler_HandleRun_MethodNotAllowed(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	req := httptest.NewRequest(http.MethodGet, "/run", nil)
	w := httptest.NewRecorder()

	handler.HandleRun(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandler_HandleRun_InvalidJSON(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handler.HandleRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandler_HandleRun_EmptyMessages(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	reqBody := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandler_HandleRun_MissingRole(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	reqBody := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandler_HandleRun_InvalidRole(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	reqBody := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "invalid", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandler_HandleRun_ValidRoles(t *testing.T) {
	tests := []struct {
		name string
		role string
	}{
		{"user role", "user"},
		{"assistant role", "assistant"},
		{"system role", "system"},
		{"uppercase USER", "USER"},
		{"mixed case User", "User"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultServerConfig()
			eventEmitter := &MockEventEmitter{}
			agentExecutor := &MockAgentExecutor{}
			handler := NewHandler(config, eventEmitter, agentExecutor)

			reqBody := RunRequest{
				Messages: []Message{
					{Role: tt.role, Content: "Hello"},
				},
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handler.HandleRun(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200 for role %s, got %d", tt.role, w.Code)
			}
		})
	}
}

func TestHandler_HandleRun_GenerateThreadID(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	reqBody := RunRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleRun(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp RunResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.ThreadID == "" {
		t.Error("Expected ThreadID to be generated")
	}
	if resp.RunID == "" {
		t.Error("Expected RunID to be generated")
	}
}

func TestHandler_HandleRun_AgentError(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{
		execError: context.DeadlineExceeded,
	}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	reqBody := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleRun(w, req)

	// Response should still be 200 (async execution)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Wait for async execution
	time.Sleep(50 * time.Millisecond)

	// Verify error event was emitted
	eventEmitter.mu.Lock()
	hasErrorEvent := false
	for _, event := range eventEmitter.events {
		if event.Type == RunError {
			hasErrorEvent = true
			break
		}
	}
	eventEmitter.mu.Unlock()

	if !hasErrorEvent {
		t.Error("Expected error event to be emitted")
	}
}

func TestHandler_validateRequest(t *testing.T) {
	config := DefaultServerConfig()
	handler := NewHandler(config, nil, nil)

	tests := []struct {
		name    string
		req     RunRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: RunRequest{
				Messages: []Message{{Role: "user", Content: "Hello"}},
			},
			wantErr: false,
		},
		{
			name: "empty messages",
			req: RunRequest{
				Messages: []Message{},
			},
			wantErr: true,
		},
		{
			name: "missing role",
			req: RunRequest{
				Messages: []Message{{Role: "", Content: "Hello"}},
			},
			wantErr: true,
		},
		{
			name: "invalid role",
			req: RunRequest{
				Messages: []Message{{Role: "invalid", Content: "Hello"}},
			},
			wantErr: true,
		},
		{
			name: "multiple valid messages",
			req: RunRequest{
				Messages: []Message{
					{Role: "system", Content: "You are helpful"},
					{Role: "user", Content: "Hello"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid tools",
			req: RunRequest{
				Messages: []Message{{Role: "user", Content: "Hello"}},
				Tools: []Tool{
					{Name: "test-tool", Description: "A test tool"},
				},
			},
			wantErr: false,
		},
		{
			name: "tool missing name",
			req: RunRequest{
				Messages: []Message{{Role: "user", Content: "Hello"}},
				Tools:    []Tool{{Name: "", Description: "A test tool"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.validateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_HandleOptions(t *testing.T) {
	config := DefaultServerConfig()
	handler := NewHandler(config, nil, nil)

	req := httptest.NewRequest(http.MethodOptions, "/run", nil)
	w := httptest.NewRecorder()

	handler.HandleOptions(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	headers := w.Header()
	if headers.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header")
	}
	if headers.Get("Access-Control-Allow-Methods") != "GET, POST, OPTIONS" {
		t.Error("Expected allowed methods")
	}
}

func TestWithCORS(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins: []string{"http://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
		MaxAge:         3600,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := WithCORS(handler, config)

	// Test with allowed origin
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	headers := w.Header()
	if headers.Get("Access-Control-Allow-Origin") == "" {
		t.Error("Expected CORS header for allowed origin")
	}
}

func TestWithCORS_Wildcard(t *testing.T) {
	config := DefaultCORSConfig()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := WithCORS(handler, config)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected wildcard CORS header")
	}
}

func TestDefaultCORSConfig(t *testing.T) {
	config := DefaultCORSConfig()

	if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "*" {
		t.Error("Expected wildcard origin")
	}
	if len(config.AllowedMethods) != 3 {
		t.Error("Expected 3 allowed methods")
	}
}

func TestNewHandler(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}

	handler := NewHandler(config, eventEmitter, agentExecutor)

	if handler == nil {
		t.Error("Expected handler to be created")
		return
	}
	if handler.eventEmitter != eventEmitter {
		t.Error("Expected eventEmitter to be set")
	}
	if handler.agentExecutor != agentExecutor {
		t.Error("Expected agentExecutor to be set")
	}
}
