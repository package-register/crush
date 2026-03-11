package aguiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// mockCanceler is a mock implementation of Canceler and AgentExecutor for testing.
type mockCanceler struct {
	mu              sync.Mutex
	canceledThreads map[string]bool
	cancelError     error
	executeFunc     func(ctx context.Context, req RunRequest) error
}

func newMockCanceler() *mockCanceler {
	return &mockCanceler{
		canceledThreads: make(map[string]bool),
		executeFunc: func(ctx context.Context, req RunRequest) error {
			return nil
		},
	}
}

func (m *mockCanceler) Cancel(ctx context.Context, threadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancelError != nil {
		return m.cancelError
	}

	m.canceledThreads[threadID] = true
	return nil
}

func (m *mockCanceler) Execute(ctx context.Context, req RunRequest) error {
	return m.executeFunc(ctx, req)
}

func (m *mockCanceler) getCanceledThreads() map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]bool)
	for k, v := range m.canceledThreads {
		result[k] = v
	}
	return result
}

func (m *mockCanceler) setCancelError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelError = err
}

func TestHandleCancel_Success(t *testing.T) {
	// Setup
	mock := newMockCanceler()
	handler := &Handler{
		config:        DefaultServerConfig(),
		eventEmitter:  nil,
		agentExecutor: mock,
	}

	reqBody := RunRequest{
		ThreadID: "test-thread-123",
		RunID:    "test-run-456",
		Messages: []Message{{Role: "user", Content: "test"}},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/agui/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.HandleCancel(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	canceled := mock.getCanceledThreads()
	if !canceled["test-thread-123"] {
		t.Error("Expected thread to be canceled")
	}
}

func TestHandleCancel_SessionNotFound(t *testing.T) {
	// Setup
	mock := newMockCanceler()
	mock.setCancelError(ErrRunNotFound)

	handler := &Handler{
		config:        DefaultServerConfig(),
		eventEmitter:  nil,
		agentExecutor: mock,
	}

	reqBody := RunRequest{
		ThreadID: "non-existent-thread",
		Messages: []Message{{Role: "user", Content: "test"}},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/agui/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.HandleCancel(w, req)

	// Assert
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandleCancel_WrongMethod(t *testing.T) {
	// Setup
	handler := &Handler{
		config:        DefaultServerConfig(),
		eventEmitter:  nil,
		agentExecutor: newMockCanceler(),
	}

	req := httptest.NewRequest(http.MethodGet, "/agui/cancel", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.HandleCancel(w, req)

	// Assert
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandleCancel_Options(t *testing.T) {
	// Setup
	handler := &Handler{
		config:        DefaultServerConfig(),
		eventEmitter:  nil,
		agentExecutor: newMockCanceler(),
	}

	req := httptest.NewRequest(http.MethodOptions, "/agui/cancel", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.HandleCancel(w, req)

	// Assert
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header Access-Control-Allow-Origin to be '*'")
	}
	if w.Header().Get("Access-Control-Allow-Methods") != "POST, OPTIONS" {
		t.Error("Expected CORS header Access-Control-Allow-Methods to be 'POST, OPTIONS'")
	}
}

func TestHandleCancel_MissingThreadID(t *testing.T) {
	// Setup
	handler := &Handler{
		config:        DefaultServerConfig(),
		eventEmitter:  nil,
		agentExecutor: newMockCanceler(),
	}

	reqBody := RunRequest{
		RunID:    "test-run-456",
		Messages: []Message{{Role: "user", Content: "test"}},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/agui/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.HandleCancel(w, req)

	// Assert
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandleCancel_InvalidJSON(t *testing.T) {
	// Setup
	handler := &Handler{
		config:        DefaultServerConfig(),
		eventEmitter:  nil,
		agentExecutor: newMockCanceler(),
	}

	req := httptest.NewRequest(http.MethodPost, "/agui/cancel", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.HandleCancel(w, req)

	// Assert
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleCancel_NoCancelerSupport(t *testing.T) {
	// Setup - use nil agentExecutor to simulate no canceler support
	handler := &Handler{
		config:        DefaultServerConfig(),
		eventEmitter:  nil,
		agentExecutor: nil,
	}

	reqBody := RunRequest{
		ThreadID: "test-thread",
		Messages: []Message{{Role: "user", Content: "test"}},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/agui/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.HandleCancel(w, req)

	// Assert
	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected status %d, got %d: %s", http.StatusNotImplemented, w.Code, w.Body.String())
	}
}

func TestHandleCancel_Concurrent(t *testing.T) {
	// Setup
	mock := newMockCanceler()
	handler := &Handler{
		config:        DefaultServerConfig(),
		eventEmitter:  nil,
		agentExecutor: mock,
	}

	// Create multiple concurrent cancel requests
	numRequests := 10
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			defer wg.Done()

			reqBody := RunRequest{
				ThreadID: "thread-" + string(rune('0'+idx)),
				Messages: []Message{{Role: "user", Content: "test"}},
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/agui/cancel", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.HandleCancel(w, req)
		}(i)
	}

	wg.Wait()

	// Assert - all threads should be canceled
	canceled := mock.getCanceledThreads()
	if len(canceled) != numRequests {
		t.Errorf("Expected %d canceled threads, got %d", numRequests, len(canceled))
	}
}

func TestHandleCancel_ServerError(t *testing.T) {
	// Setup
	mock := newMockCanceler()
	mock.setCancelError(errors.New("internal server error"))

	handler := &Handler{
		config:        DefaultServerConfig(),
		eventEmitter:  nil,
		agentExecutor: mock,
	}

	reqBody := RunRequest{
		ThreadID: "test-thread",
		Messages: []Message{{Role: "user", Content: "test"}},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/agui/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.HandleCancel(w, req)

	// Assert
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

// Test CancelHandler Register and Unregister
func TestCancelHandler_Register(t *testing.T) {
	canceler := newMockCanceler()
	handler := NewCancelHandler(canceler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register a session
	err := handler.Register("thread-1", ctx, cancel)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !handler.IsRunning("thread-1") {
		t.Error("Expected thread-1 to be running")
	}

	if handler.Count() != 1 {
		t.Errorf("Expected count 1, got %d", handler.Count())
	}

	// Try to register same thread again (should fail)
	_, cancel2 := context.WithCancel(context.Background())
	err = handler.Register("thread-1", context.Background(), cancel2)
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}
}

func TestCancelHandler_Unregister(t *testing.T) {
	canceler := newMockCanceler()
	handler := NewCancelHandler(canceler)

	ctx, cancel := context.WithCancel(context.Background())

	// Register and unregister
	handler.Register("thread-1", ctx, cancel)
	handler.Unregister("thread-1")

	if handler.IsRunning("thread-1") {
		t.Error("Expected thread-1 to not be running after unregister")
	}

	if handler.Count() != 0 {
		t.Errorf("Expected count 0, got %d", handler.Count())
	}
}

func TestCancelHandler_ConcurrentAccess(t *testing.T) {
	canceler := newMockCanceler()
	handler := NewCancelHandler(canceler)

	var wg sync.WaitGroup
	numOps := 100

	// Concurrent register/unregister
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			threadID := "thread-" + string(rune(idx%10))

			ctx, cancel := context.WithCancel(context.Background())
			handler.Register(threadID, ctx, cancel)
			handler.Unregister(threadID)
		}(i)
	}

	wg.Wait()

	// Should have no running sessions after all operations
	if handler.Count() != 0 {
		t.Errorf("Expected count 0 after concurrent operations, got %d", handler.Count())
	}
}

// Benchmark tests
func BenchmarkCancelHandler_Register(b *testing.B) {
	canceler := newMockCanceler()
	handler := NewCancelHandler(canceler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		threadID := "thread-" + string(rune(i%100))
		handler.Register(threadID, ctx, cancel)
		handler.Unregister(threadID)
	}
}

func BenchmarkCancelHandler_IsRunning(b *testing.B) {
	canceler := newMockCanceler()
	handler := NewCancelHandler(canceler)

	ctx, cancel := context.WithCancel(context.Background())
	handler.Register("thread-1", ctx, cancel)
	defer handler.Unregister("thread-1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.IsRunning("thread-1")
	}
}

func TestCancelHandler_HandleCancel(t *testing.T) {
	mock := newMockCanceler()
	handler := NewCancelHandler(mock)

	// Test OPTIONS method
	req := httptest.NewRequest(http.MethodOptions, "/cancel", nil)
	w := httptest.NewRecorder()
	handler.HandleCancel(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Test wrong method
	req = httptest.NewRequest(http.MethodGet, "/cancel", nil)
	w = httptest.NewRecorder()
	handler.HandleCancel(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	// Test invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/cancel", bytes.NewReader([]byte("invalid")))
	w = httptest.NewRecorder()
	handler.HandleCancel(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Test missing threadId
	req = httptest.NewRequest(http.MethodPost, "/cancel", bytes.NewReader([]byte(`{"runId":"test"}`)))
	w = httptest.NewRecorder()
	handler.HandleCancel(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Test successful cancel
	reqBody := RunRequest{
		ThreadID: "test-thread",
		Messages: []Message{{Role: "user", Content: "test"}},
	}
	body, _ := json.Marshal(reqBody)
	req = httptest.NewRequest(http.MethodPost, "/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.HandleCancel(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header Access-Control-Allow-Origin to be '*'")
	}
}

func TestCancelHandler_HandleCancel_NotFound(t *testing.T) {
	mock := newMockCanceler()
	mock.setCancelError(ErrRunNotFound)
	handler := NewCancelHandler(mock)

	reqBody := RunRequest{
		ThreadID: "nonexistent",
		Messages: []Message{{Role: "user", Content: "test"}},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleCancel(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestCancelHandler_HandleCancel_InternalError(t *testing.T) {
	mock := newMockCanceler()
	mock.setCancelError(errors.New("internal error"))
	handler := NewCancelHandler(mock)

	reqBody := RunRequest{
		ThreadID: "test-thread",
		Messages: []Message{{Role: "user", Content: "test"}},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleCancel(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestCancelHandler_HandleCancel_NilCanceler(t *testing.T) {
	handler := NewCancelHandler(nil)

	reqBody := RunRequest{
		ThreadID: "test-thread",
		Messages: []Message{{Role: "user", Content: "test"}},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/cancel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleCancel(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected status %d, got %d", http.StatusNotImplemented, w.Code)
	}
}
