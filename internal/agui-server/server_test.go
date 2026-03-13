package aguiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestServer_Start(t *testing.T) {
	config := ServerConfig{
		Port:        18080, // Use different port to avoid conflicts
		BasePath:    "/agui",
		CORSOrigins: []string{"*"},
	}
	srv := NewServer(config)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start server in goroutine
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop server
	cancel()

	// Wait for server to stop
	select {
	case err := <-done:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Expected server to stop gracefully, got error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Server did not stop within timeout")
	}
}

func TestServer_Stop(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)

	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	// Create mock http.Server
	server.httpServer = &http.Server{
		Addr:    ":18081",
		Handler: http.NewServeMux(),
	}

	// Add some connections
	conn1 := NewConnection("conn1", "thread1", "run1")
	conn2 := NewConnection("conn2", "thread2", "run2")
	if server.connectionManager != nil {
		server.connectionManager.Add(conn1)
		server.connectionManager.Add(conn2)
	}

	ctx := context.Background()
	err := srv.Stop(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify connections are closed
	select {
	case <-conn1.Done:
		// Expected
	default:
		t.Error("conn1 should be closed")
	}

	select {
	case <-conn2.Done:
		// Expected
	default:
		t.Error("conn2 should be closed")
	}

	// Verify connections are cleared
	if server.connectionManager != nil && server.connectionManager.Count() != 0 {
		t.Errorf("Expected connections to be cleared, got %d entries", server.connectionManager.Count())
	}
}

func TestServer_HandleSSE(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)

	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	// Create test request with query parameters
	req := httptest.NewRequest(http.MethodGet, "/sse?threadId=test-thread&runId=test-run", nil)
	w := httptest.NewRecorder()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// Call handler
	server.HandleSSE(w, req)

	// Verify SSE headers
	headers := w.Header()
	if headers.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", headers.Get("Content-Type"))
	}
	if headers.Get("Cache-Control") != "no-cache" {
		t.Errorf("Expected Cache-Control no-cache, got %s", headers.Get("Cache-Control"))
	}
	if headers.Get("Connection") != "keep-alive" {
		t.Errorf("Expected Connection keep-alive, got %s", headers.Get("Connection"))
	}

	// Verify initial event was sent
	body := w.Body.String()
	if body == "" {
		t.Error("Expected initial event to be sent")
	}
}

func TestServer_HandleSSE_GenerateIDs(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)

	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	// Create test request without query parameters
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	w := httptest.NewRecorder()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// Call handler
	server.HandleSSE(w, req)

	// Verify response contains initial event
	body := w.Body.String()
	if body == "" {
		t.Error("Expected initial event to be sent")
	}

	// Verify it contains RunStarted event
	var event Event
	if err := json.Unmarshal([]byte(body[6:]), &event); err == nil {
		if event.Type != RunStarted {
			t.Errorf("Expected RunStarted event, got %s", event.Type)
		}
	}
}

func TestServer_HandleRun(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)

	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

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

	// Call handler
	server.HandleRun(w, req)

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
}

func TestServer_HandleRun_MethodNotAllowed(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)

	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	req := httptest.NewRequest(http.MethodGet, "/run", nil)
	w := httptest.NewRecorder()

	server.HandleRun(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestServer_HandleRun_InvalidJSON(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)

	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	server.HandleRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestServer_HandleRun_EmptyMessages(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)

	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	reqBody := RunRequest{
		Messages: []Message{},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.HandleRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestServer_HandleRun_GenerateIDs(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)

	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	reqBody := RunRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	server.HandleRun(w, req)

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

func TestServer_withCORS(t *testing.T) {
	tests := []struct {
		name            string
		corsOrigins     []string
		requestOrigin   string
		method          string
		expectCORS      bool
		expectNoContent bool
	}{
		{
			name:          "wildcard origin",
			corsOrigins:   []string{"*"},
			requestOrigin: "http://example.com",
			method:        http.MethodGet,
			expectCORS:    true,
		},
		{
			name:          "allowed origin",
			corsOrigins:   []string{"http://example.com"},
			requestOrigin: "http://example.com",
			method:        http.MethodGet,
			expectCORS:    true,
		},
		{
			name:          "disallowed origin",
			corsOrigins:   []string{"http://example.com"},
			requestOrigin: "http://evil.com",
			method:        http.MethodGet,
			expectCORS:    false,
		},
		{
			name:          "no cors origins",
			corsOrigins:   []string{},
			requestOrigin: "http://example.com",
			method:        http.MethodGet,
			expectCORS:    false,
		},
		{
			name:            "options request",
			corsOrigins:     []string{"*"},
			requestOrigin:   "http://example.com",
			method:          http.MethodOptions,
			expectCORS:      true,
			expectNoContent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ServerConfig{
				CORSOrigins: tt.corsOrigins,
			}
			srv := NewServer(config)
			server, ok := srv.(*server)
			if !ok {
				t.Fatal("Expected server to be of type *server")
			}

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			wrappedHandler := server.withCORS(handler)

			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if tt.expectNoContent {
				if w.Code != http.StatusNoContent {
					t.Errorf("Expected status 204, got %d", w.Code)
				}
			} else {
				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200, got %d", w.Code)
				}
			}

			if tt.expectCORS {
				if w.Header().Get("Access-Control-Allow-Origin") == "" {
					t.Error("Expected CORS header")
				}
			}
		})
	}
}

func TestMustMarshal(t *testing.T) {
	// Test valid JSON marshaling
	data := map[string]string{"key": "value"}
	result := mustMarshal(data)

	if result == "" {
		t.Error("Expected non-empty result")
	}

	expected := `{"key":"value"}`
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}

	// Test with complex data
	complexData := RunStartedEvent{
		ThreadID: "thread-123",
		RunID:    "run-456",
	}
	result = mustMarshal(complexData)
	if result == "" {
		t.Error("Expected non-empty result for complex data")
	}
}

func TestMustMarshalPanic(t *testing.T) {
	// Test that mustMarshal panics on unmarshalable data
	// Create a channel which cannot be marshaled
	ch := make(chan int)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for unmarshalable data")
		}
	}()

	mustMarshal(ch)
}

func TestConnectionManager_GetByThread_Empty(t *testing.T) {
	manager := NewConnectionManager()

	conns := manager.GetByThread("nonexistent")
	if len(conns) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(conns))
	}
}

func TestConnectionManager_List_Empty(t *testing.T) {
	manager := NewConnectionManager()

	conns := manager.List()
	if len(conns) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(conns))
	}
}

func TestConnectionManager_CloseAll_Empty(t *testing.T) {
	manager := NewConnectionManager()

	// Should not panic
	manager.CloseAll()

	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections, got %d", manager.Count())
	}
}

// Table-driven tests for connection manager operations
func TestConnectionManager_Operations(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*ConnectionManager) []*Connection
		operation   func(*ConnectionManager)
		verifyCount int
	}{
		{
			name: "add and remove single",
			setup: func(m *ConnectionManager) []*Connection {
				conn := NewConnection("conn1", "thread1", "run1")
				m.Add(conn)
				return []*Connection{conn}
			},
			operation: func(m *ConnectionManager) {
				m.Remove("conn1")
			},
			verifyCount: 0,
		},
		{
			name: "add multiple",
			setup: func(m *ConnectionManager) []*Connection {
				var conns []*Connection
				for i := range 5 {
					conn := NewConnection("conn"+string(rune(i)), "thread", "run")
					m.Add(conn)
					conns = append(conns, conn)
				}
				return conns
			},
			operation:   func(m *ConnectionManager) {},
			verifyCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewConnectionManager()
			tt.setup(manager)
			tt.operation(manager)

			if manager.Count() != tt.verifyCount {
				t.Errorf("Expected %d connections, got %d", tt.verifyCount, manager.Count())
			}
		})
	}
}

// Table-driven tests for RunRequest validation
func TestServer_ValidateRequest_TableDriven(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)
	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	tests := []struct {
		name    string
		req     RunRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid single message",
			req: RunRequest{
				Messages: []Message{{Role: "user", Content: "Hello"}},
			},
			wantErr: false,
		},
		{
			name: "valid multiple messages",
			req: RunRequest{
				Messages: []Message{
					{Role: "system", Content: "You are helpful"},
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi!"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty messages",
			req: RunRequest{
				Messages: []Message{},
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name: "nil messages",
			req: RunRequest{
				Messages: nil,
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name: "missing role first message",
			req: RunRequest{
				Messages: []Message{{Role: "", Content: "Hello"}},
			},
			wantErr: true,
			errMsg:  "role is required",
		},
		{
			name: "missing role second message",
			req: RunRequest{
				Messages: []Message{
					{Role: "user", Content: "Hello"},
					{Role: "", Content: "World"},
				},
			},
			wantErr: true,
			errMsg:  "role is required",
		},
		{
			name: "empty content allowed",
			req: RunRequest{
				Messages: []Message{{Role: "user", Content: ""}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.validateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestServer_HandleCancel(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)
	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	// Test OPTIONS method
	req := httptest.NewRequest(http.MethodOptions, "/agui/cancel", nil)
	w := httptest.NewRecorder()
	server.HandleCancel(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Test wrong method
	req = httptest.NewRequest(http.MethodGet, "/agui/cancel", nil)
	w = httptest.NewRecorder()
	server.HandleCancel(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	// Test invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/agui/cancel", bytes.NewReader([]byte("invalid")))
	w = httptest.NewRecorder()
	server.HandleCancel(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Test missing threadId
	req = httptest.NewRequest(http.MethodPost, "/agui/cancel", bytes.NewReader([]byte(`{"runId":"test"}`)))
	w = httptest.NewRecorder()
	server.HandleCancel(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Test cancel not implemented (default behavior)
	req = httptest.NewRequest(http.MethodPost, "/agui/cancel", bytes.NewReader([]byte(`{"threadId":"test"}`)))
	w = httptest.NewRecorder()
	server.HandleCancel(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected status %d, got %d", http.StatusNotImplemented, w.Code)
	}
}

func TestServer_HandleHistory(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)
	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	// Test OPTIONS method
	req := httptest.NewRequest(http.MethodOptions, "/agui/history", nil)
	w := httptest.NewRecorder()
	server.HandleHistory(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Test wrong method
	req = httptest.NewRequest(http.MethodPost, "/agui/history", nil)
	w = httptest.NewRecorder()
	server.HandleHistory(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	// Test missing threadId
	req = httptest.NewRequest(http.MethodGet, "/agui/history", nil)
	w = httptest.NewRecorder()
	server.HandleHistory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Test with threadId (nil history service)
	req = httptest.NewRequest(http.MethodGet, "/agui/history?threadId=test", nil)
	w = httptest.NewRecorder()
	server.HandleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Test with invalid limit
	req = httptest.NewRequest(http.MethodGet, "/agui/history?threadId=test&limit=invalid", nil)
	w = httptest.NewRecorder()
	server.HandleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d with invalid limit, got %d", http.StatusOK, w.Code)
	}

	// Test with invalid offset
	req = httptest.NewRequest(http.MethodGet, "/agui/history?threadId=test&offset=invalid", nil)
	w = httptest.NewRecorder()
	server.HandleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d with invalid offset, got %d", http.StatusOK, w.Code)
	}
}
