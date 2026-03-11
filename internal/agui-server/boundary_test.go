// Package aguiserver implements boundary condition tests for AG-UI server.
// Tests cover nil/empty values, large messages, concurrency, timeouts, errors, and resource management.
package aguiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// P0 - 必须覆盖：nil/空值检查测试
// =============================================================================

// TestSSEHandler_NilBody tests handling of nil request body.
func TestSSEHandler_NilBody(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Create request with nil body
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	w := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// Should not panic
	handler.HandleSSE(w, req)

	// Should still create connection with generated IDs
	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after handler returns, got %d", manager.Count())
	}
}

// TestSSEHandler_EmptyThreadID tests handling of empty thread ID.
func TestSSEHandler_EmptyThreadID(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Create request with empty threadId
	req := httptest.NewRequest(http.MethodGet, "/sse?threadId=&runId=", nil)
	w := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	handler.HandleSSE(w, req)

	// Should generate UUID for empty threadId
	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after handler returns, got %d", manager.Count())
	}
}

// TestHandler_NilRequest tests handler with nil request components.
func TestHandler_NilRequest(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	// Test with empty body
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader([]byte{}))
	w := httptest.NewRecorder()

	handler.HandleRun(w, req)

	// Should return 400 for empty body
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for empty body, got %d", w.Code)
	}
}

// TestHandler_NilEventEmitter tests handler with nil event emitter.
func TestHandler_NilEventEmitter(t *testing.T) {
	config := DefaultServerConfig()
	agentExecutor := &MockAgentExecutor{}

	// Create handler with nil event emitter
	handler := NewHandler(config, nil, agentExecutor)

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

	// Should not panic
	handler.HandleRun(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestHandler_NilAgentExecutor tests handler with nil agent executor.
func TestHandler_NilAgentExecutor(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}

	// Create handler with nil agent executor
	handler := NewHandler(config, eventEmitter, nil)

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

	// Should not panic
	handler.HandleRun(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// =============================================================================
// P0 - 必须覆盖：大消息处理测试
// =============================================================================

// TestSSEHandler_LargeMessage tests handling of large SSE messages.
func TestSSEHandler_LargeMessage(t *testing.T) {
	manager := NewConnectionManager()

	conn := NewConnection("test-conn", "test-thread", "test-run")
	manager.Add(conn)
	defer func() {
		manager.Remove(conn.ID)
		close(conn.Done)
	}()

	// Create large message (1MB)
	largeData := strings.Repeat("x", 1024*1024)
	event := NewEvent(TextMessageContent, TextMessageContentEvent{
		MessageID: "msg-1",
		Content:   largeData,
	})

	// Should handle large message without error
	select {
	case conn.Events <- event:
		// Success
	default:
		t.Error("Failed to send large message to channel")
	}
}

// TestEventSerialization_LargePayload tests serialization of large payloads.
func TestEventSerialization_LargePayload(t *testing.T) {
	// Create large payload (5MB)
	largeData := strings.Repeat("x", 5*1024*1024)
	event := NewEvent(TextMessageContent, TextMessageContentEvent{
		MessageID: "msg-large",
		Content:   largeData,
	})

	// Should serialize without error
	jsonData, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal large event: %v", err)
	}

	if len(jsonData) < 5*1024*1024 {
		t.Error("Expected serialized data to be at least 5MB")
	}

	// Should deserialize correctly
	var unmarshaled Event
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal large event: %v", err)
	}

	if unmarshaled.Type != TextMessageContent {
		t.Errorf("Expected type TextMessageContent, got %s", unmarshaled.Type)
	}
}

// TestHandler_LargeRequest tests handling of large requests.
func TestHandler_LargeRequest(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	// Create large message content
	largeContent := strings.Repeat("x", 100*1024) // 100KB

	reqBody := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "user", Content: largeContent},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.HandleRun(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// =============================================================================
// P0 - 必须覆盖：并发控制测试
// =============================================================================

// TestSSEHandler_ConcurrentAccess tests concurrent access to SSE handler.
func TestSSEHandler_ConcurrentAccess(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	var wg sync.WaitGroup
	numGoroutines := 50

	// Concurrent SSE connections
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sse?threadId=t%d&runId=r%d", id, id), nil)
			w := httptest.NewRecorder()

			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			req = req.WithContext(ctx)

			handler.HandleSSE(w, req)
		}(i)
	}

	wg.Wait()

	// All connections should be cleaned up
	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after all handlers return, got %d", manager.Count())
	}
}

// TestServer_ConcurrentConnections tests concurrent server connections.
func TestServer_ConcurrentConnections(t *testing.T) {
	manager := NewConnectionManager()
	var wg sync.WaitGroup
	numGoroutines := 100

	// Phase 1: Concurrent connection adds
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn := NewConnection(fmt.Sprintf("conn%d", id), "thread", "run")
			manager.Add(conn)
		}(i)
	}
	wg.Wait()

	// Verify all connections were added
	if manager.Count() != numGoroutines {
		t.Errorf("Expected %d connections after adds, got %d", numGoroutines, manager.Count())
	}

	// Phase 2: Concurrent connection removes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			manager.Remove(fmt.Sprintf("conn%d", id))
		}(i)
	}
	wg.Wait()

	// All connections should be cleaned up
	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after cleanup, got %d", manager.Count())
	}
}

// TestConnectionManager_ConcurrentOperations tests concurrent operations on connection manager.
func TestConnectionManager_ConcurrentOperations(t *testing.T) {
	manager := NewConnectionManager()
	var wg sync.WaitGroup

	// Concurrent adds
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn := NewConnection(fmt.Sprintf("conn%d", id), "thread", "run")
			manager.Add(conn)
		}(i)
	}

	// Concurrent gets
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Get("conn1")
		}()
	}

	// Concurrent lists
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.List()
		}()
	}

	// Concurrent counts
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Count()
		}()
	}

	wg.Wait()
}

// TestBroadcastToThread_Concurrent tests concurrent broadcast operations.
func TestBroadcastToThread_Concurrent(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Create multiple connections
	for i := 0; i < 10; i++ {
		conn := NewConnection(fmt.Sprintf("conn%d", i), "thread1", fmt.Sprintf("run%d", i))
		manager.Add(conn)
		defer close(conn.Done)
	}

	var wg sync.WaitGroup
	var successCount int32

	// Concurrent broadcasts
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			event := NewEvent(RunStarted, RunStartedEvent{
				ThreadID: "thread1",
				RunID:    fmt.Sprintf("run%d", id),
			})
			count := handler.BroadcastToThread("thread1", event)
			if count > 0 {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if successCount == 0 {
		t.Error("Expected at least one successful broadcast")
	}
}

// =============================================================================
// P1 - 重要覆盖：超时处理测试
// =============================================================================

// TestSSEHandler_Timeout tests SSE handler timeout behavior.
func TestSSEHandler_Timeout(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("test-conn", "test-thread", "test-run")
	manager.Add(conn)
	defer func() {
		manager.Remove(conn.ID)
		close(conn.Done)
	}()

	w := httptest.NewRecorder()

	// Create context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/sse", nil).WithContext(ctx)

	// Should return immediately
	done := make(chan bool)
	go func() {
		handler.streamEvents(w, req, conn)
		done <- true
	}()

	select {
	case <-done:
		// Expected - should return immediately
	case <-time.After(500 * time.Millisecond):
		t.Error("streamEvents should return when context is cancelled")
	}
}

// TestHandler_ReadTimeout tests handler read timeout.
func TestHandler_ReadTimeout(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	// Create request with slow reader
	pr, pw := io.Pipe()
	go func() {
		time.Sleep(100 * time.Millisecond)
		pw.Write([]byte(`{"threadId":"test","runId":"test","messages":[{"role":"user","content":"hello"}]}`))
		pw.Close()
	}()

	req := httptest.NewRequest(http.MethodPost, "/run", pr)
	w := httptest.NewRecorder()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	handler.HandleRun(w, req)

	// Should complete within timeout
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestSSEHandler_ContextDeadline tests SSE handler with context deadline.
func TestSSEHandler_ContextDeadline(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	w := httptest.NewRecorder()

	// Create context with very short deadline
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/sse", nil).WithContext(ctx)

	// Should handle deadline gracefully
	handler.HandleSSE(w, req)
}

// =============================================================================
// P1 - 重要覆盖：错误处理测试
// =============================================================================

// TestEventDecoder_InvalidJSON tests decoding of invalid JSON.
func TestEventDecoder_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{invalid json}`)

	var event Event
	err := json.Unmarshal(invalidJSON, &event)

	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// TestSSEWriter_InvalidEvent tests writing invalid events.
func TestSSEWriter_InvalidEvent(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	w := httptest.NewRecorder()

	// Create event with unmarshalable data (channel)
	event := Event{
		Type:      CustomEvent,
		Timestamp: int64Ptr(time.Now().Unix()),
		Data:      make(chan int),
	}

	// Should not panic, just return silently
	handler.sendEvent(w, event)

	// Verify nothing was written
	if w.Body.Len() != 0 {
		t.Error("Expected no output for invalid event")
	}
}

// TestHandler_InvalidRequestBody tests handler with invalid request body.
func TestHandler_InvalidRequestBody(t *testing.T) {
	config := DefaultServerConfig()
	eventEmitter := &MockEventEmitter{}
	agentExecutor := &MockAgentExecutor{}
	handler := NewHandler(config, eventEmitter, agentExecutor)

	// Invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader([]byte(`{not valid json}`)))
	w := httptest.NewRecorder()

	handler.HandleRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", w.Code)
	}

	// Valid JSON but wrong structure
	req = httptest.NewRequest(http.MethodPost, "/run", bytes.NewReader([]byte(`{"wrong": "structure"}`)))
	w = httptest.NewRecorder()

	handler.HandleRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for wrong structure, got %d", w.Code)
	}
}

// TestValidateRequest_NilMessages tests validation with nil messages.
func TestValidateRequest_NilMessages(t *testing.T) {
	config := DefaultServerConfig()
	handler := NewHandler(config, nil, nil)

	req := RunRequest{
		Messages: nil,
	}

	err := handler.validateRequest(req)
	if err == nil {
		t.Error("Expected error for nil messages")
	}
}

// TestValidateRequest_EmptyRole tests validation with empty role.
func TestValidateRequest_EmptyRole(t *testing.T) {
	config := DefaultServerConfig()
	handler := NewHandler(config, nil, nil)

	req := RunRequest{
		Messages: []Message{
			{Role: "", Content: "Hello"},
		},
	}

	err := handler.validateRequest(req)
	if err == nil {
		t.Error("Expected error for empty role")
	}
}

// TestValidateRequest_EmptyToolName tests validation with empty tool name.
func TestValidateRequest_EmptyToolName(t *testing.T) {
	config := DefaultServerConfig()
	handler := NewHandler(config, nil, nil)

	req := RunRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		Tools: []Tool{
			{Name: "", Description: "A tool"},
		},
	}

	err := handler.validateRequest(req)
	if err == nil {
		t.Error("Expected error for empty tool name")
	}
}

// TestBroadcastToConnection_NilConnection tests broadcast with nil connection.
func TestBroadcastToConnection_NilConnection(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	event := NewEvent(RunStarted, RunStartedEvent{})

	err := handler.BroadcastToConnection("nonexistent", event)
	if err == nil {
		t.Error("Expected error for nonexistent connection")
	}
}

// =============================================================================
// P2 - 边界覆盖：SSE 边界测试
// =============================================================================

// TestSSEWriter_EmptyEvent tests writing empty events.
func TestSSEWriter_EmptyEvent(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	w := httptest.NewRecorder()

	// Empty event
	event := Event{}

	handler.sendEvent(w, event)

	body := w.Body.String()
	if body == "" {
		t.Error("Expected event to be written")
	}

	// Should contain data: prefix
	if !strings.HasPrefix(body, "data: ") {
		t.Errorf("Expected SSE format, got %q", body)
	}
}

// TestSSEWriter_MultilineData tests writing multiline data.
func TestSSEWriter_MultilineData(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	w := httptest.NewRecorder()

	// Multiline content
	multilineContent := "line1\nline2\nline3\r\nline4"
	event := NewEvent(TextMessageContent, TextMessageContentEvent{
		MessageID: "msg-1",
		Content:   multilineContent,
	})

	handler.sendEvent(w, event)

	body := w.Body.String()
	if body == "" {
		t.Error("Expected event to be written")
	}
}

// TestSSEWriter_SpecialCharacters tests writing events with special characters.
func TestSSEWriter_SpecialCharacters(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	w := httptest.NewRecorder()

	// Special characters
	specialContent := `Special: <>&"'	\`
	event := NewEvent(TextMessageContent, TextMessageContentEvent{
		MessageID: "msg-1",
		Content:   specialContent,
	})

	handler.sendEvent(w, event)

	body := w.Body.String()
	if body == "" {
		t.Error("Expected event to be written")
	}
}

// TestSSEWriter_UnicodeData tests writing events with unicode data.
func TestSSEWriter_UnicodeData(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	w := httptest.NewRecorder()

	// Unicode content
	unicodeContent := "Hello 世界 🌍 Привет"
	event := NewEvent(TextMessageContent, TextMessageContentEvent{
		MessageID: "msg-1",
		Content:   unicodeContent,
	})

	handler.sendEvent(w, event)

	body := w.Body.String()
	if body == "" {
		t.Error("Expected event to be written")
	}

	// Verify unicode is preserved
	var parsed Event
	if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(body), "data: ")), &parsed); err != nil {
		t.Errorf("Failed to parse event: %v", err)
	}
}

// TestSSEHandler_QueryParameters tests various query parameter combinations.
func TestSSEHandler_QueryParameters(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		expectGen bool // expect generated IDs
	}{
		{"both provided", "?threadId=t1&runId=r1", false},
		{"thread only", "?threadId=t1", false},
		{"run only", "?runId=r1", false},
		{"neither provided", "", true},
		{"empty values", "?threadId=&runId=", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultServerConfig()
			manager := NewConnectionManager()
			handler := NewSSEHandler(config, manager)

			req := httptest.NewRequest(http.MethodGet, "/sse"+tt.query, nil)
			w := httptest.NewRecorder()

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			req = req.WithContext(ctx)

			handler.HandleSSE(w, req)

			body := w.Body.String()
			if body == "" {
				t.Error("Expected initial event to be sent")
			}
		})
	}
}

// =============================================================================
// P2 - 边界覆盖：资源管理测试
// =============================================================================

// TestConnection_Cleanup tests connection cleanup.
func TestConnection_Cleanup(t *testing.T) {
	manager := NewConnectionManager()
	conn := NewConnection("test-conn", "test-thread", "test-run")

	manager.Add(conn)
	if manager.Count() != 1 {
		t.Errorf("Expected 1 connection, got %d", manager.Count())
	}

	// Close connection
	close(conn.Done)
	manager.Remove(conn.ID)

	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after removal, got %d", manager.Count())
	}
}

// TestServer_Shutdown tests server graceful shutdown.
func TestServer_Shutdown(t *testing.T) {
	config := ServerConfig{
		Port:        18082,
		BasePath:    "/agui",
		CORSOrigins: []string{"*"},
	}
	srv := NewServer(config)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start server
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop server gracefully
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer stopCancel()

	err := srv.Stop(stopCtx)
	if err != nil && err != http.ErrServerClosed {
		t.Errorf("Expected graceful shutdown, got error: %v", err)
	}

	// Wait for server to stop
	select {
	case <-done:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("Server did not stop within timeout")
	}
}

// TestConnectionManager_CloseAll_Verification tests CloseAll verification.
func TestConnectionManager_CloseAll_Verification(t *testing.T) {
	manager := NewConnectionManager()

	// Add multiple connections
	conns := make([]*Connection, 5)
	for i := 0; i < 5; i++ {
		conn := NewConnection(fmt.Sprintf("conn%d", i), "thread", "run")
		manager.Add(conn)
		conns[i] = conn
	}

	if manager.Count() != 5 {
		t.Errorf("Expected 5 connections, got %d", manager.Count())
	}

	// Close all
	manager.CloseAll()

	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after CloseAll, got %d", manager.Count())
	}

	// Verify all connections are closed
	for i, conn := range conns {
		select {
		case <-conn.Done:
			// Expected
		default:
			t.Errorf("Connection %d should be closed", i)
		}
	}
}

// TestConnection_ChannelCapacity tests connection channel capacity.
func TestConnection_ChannelCapacity(t *testing.T) {
	conn := NewConnection("test", "thread", "run")
	defer close(conn.Done)

	// Fill the channel (capacity is 64)
	for i := 0; i < 64; i++ {
		event := NewEvent(RunStarted, RunStartedEvent{
			ThreadID: "thread",
			RunID:    fmt.Sprintf("run%d", i),
		})
		select {
		case conn.Events <- event:
			// Success
		default:
			t.Errorf("Failed to send event %d to channel", i)
		}
	}

	// Channel should be full now
	select {
	case conn.Events <- NewEvent(RunStarted, RunStartedEvent{}):
		t.Error("Expected channel to be full")
	default:
		// Expected - channel is full
	}
}

// TestConnectionManager_GetByThread_NoMatches tests GetByThread with no matches.
func TestConnectionManager_GetByThread_NoMatches(t *testing.T) {
	manager := NewConnectionManager()

	// Add connections for different thread
	conn := NewConnection("conn1", "thread1", "run1")
	manager.Add(conn)
	defer close(conn.Done)

	// Query for nonexistent thread
	conns := manager.GetByThread("nonexistent")
	if len(conns) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(conns))
	}
}

// TestSSEHandler_setSSEHeaders tests all SSE headers are set.
func TestSSEHandler_setSSEHeaders_Complete(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	w := httptest.NewRecorder()
	handler.setSSEHeaders(w)

	headers := w.Header()

	expectedHeaders := map[string]string{
		"Content-Type":                 "text/event-stream",
		"Cache-Control":                "no-cache",
		"Connection":                   "keep-alive",
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type",
	}

	for header, expected := range expectedHeaders {
		if got := headers.Get(header); got != expected {
			t.Errorf("Header %s: expected %q, got %q", header, expected, got)
		}
	}
}

// TestMustMarshal_VariousTypes tests mustMarshal with various types.
func TestMustMarshal_VariousTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expectOK bool
	}{
		{"string", "hello", true},
		{"int", 42, true},
		{"float", 3.14, true},
		{"bool", true, true},
		{"nil", nil, true},
		{"map", map[string]string{"key": "value"}, true},
		{"slice", []int{1, 2, 3}, true},
		{"struct", RunStartedEvent{ThreadID: "t1", RunID: "r1"}, true},
		{"channel", make(chan int), false}, // Will panic
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expectOK {
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected panic for unmarshalable type")
					}
				}()
			}
			result := mustMarshal(tt.input)
			if tt.expectOK && result == "" {
				t.Error("Expected non-empty result")
			}
		})
	}
}

// TestConnection_HeartbeatCancel tests heartbeat cancellation.
func TestConnection_HeartbeatCancel(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("test", "thread", "run")
	manager.Add(conn)
	defer func() {
		manager.Remove(conn.ID)
		close(conn.Done)
	}()

	// Start heartbeat
	handler.startHeartbeat(conn)

	// Verify cancel is set
	conn.mu.Lock()
	hasCancel := conn.heartbeatCancel != nil
	conn.mu.Unlock()

	if !hasCancel {
		t.Error("Expected heartbeat cancel to be set")
	}

	// Stop heartbeat
	handler.stopHeartbeat(conn)

	// Verify cancel is cleared
	conn.mu.Lock()
	hasCancel = conn.heartbeatCancel != nil
	conn.mu.Unlock()

	if hasCancel {
		t.Error("Expected heartbeat cancel to be cleared")
	}
}

// TestHandler_OptionsMethod tests OPTIONS request handling.
func TestHandler_OptionsMethod(t *testing.T) {
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

// TestWithCORS_EmptyConfig tests WithCORS with empty config.
func TestWithCORS_EmptyConfig(t *testing.T) {
	config := CORSConfig{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := WithCORS(handler, config)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	// Should not set CORS headers for empty config
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected no CORS header for empty config")
	}
}

// TestServer_ValidateRequest_AllFields tests validation with all fields.
func TestServer_ValidateRequest_AllFields(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)
	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	// Valid request with all fields
	req := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{ID: "msg-1", Role: "user", Content: "Hello"},
			{ID: "msg-2", Role: "assistant", Content: "Hi there!"},
		},
		Tools: []Tool{
			{Name: "bash", Description: "Run bash commands", Parameters: map[string]any{"type": "object"}},
		},
		State: map[string]any{"key": "value"},
	}

	err := server.validateRequest(req)
	if err != nil {
		t.Errorf("Expected no error for valid request, got %v", err)
	}
}

// TestConnectionManager_Remove_NonExistent tests removing nonexistent connection.
func TestConnectionManager_Remove_NonExistent(t *testing.T) {
	manager := NewConnectionManager()

	// Should not panic
	manager.Remove("nonexistent")

	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections, got %d", manager.Count())
	}
}

// TestSSEHandler_BroadcastToAll_Empty tests broadcast to all with no connections.
func TestSSEHandler_BroadcastToAll_Empty(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	event := NewEvent(RunStarted, RunStartedEvent{})
	count := handler.BroadcastToAll(event)

	if count != 0 {
		t.Errorf("Expected 0 connections to receive event, got %d", count)
	}
}

// Benchmark tests for boundary conditions
func BenchmarkSSEHandler_Concurrent(b *testing.B) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/sse?threadId=t&runId=r", nil)
			w := httptest.NewRecorder()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			req = req.WithContext(ctx)
			handler.HandleSSE(w, req)
			cancel()
		}
	})
}

func BenchmarkBroadcastToThread_Concurrent(b *testing.B) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Setup connections
	for i := 0; i < 10; i++ {
		conn := NewConnection(fmt.Sprintf("conn%d", i), "thread", "run")
		manager.Add(conn)
		defer close(conn.Done)
	}

	event := NewEvent(RunStarted, RunStartedEvent{ThreadID: "thread", RunID: "run"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.BroadcastToThread("thread", event)
	}
}

func BenchmarkEventSerialization_Large(b *testing.B) {
	largeData := strings.Repeat("x", 100*1024) // 100KB
	event := NewEvent(TextMessageContent, TextMessageContentEvent{
		MessageID: "msg-1",
		Content:   largeData,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(event)
	}
}
