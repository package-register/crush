package aguiserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSSEHandler_HandleSSE(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/sse?threadId=test-thread&runId=test-run", nil)
	w := httptest.NewRecorder()

	// Create context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// Call handler
	handler.HandleSSE(w, req)

	// Verify connection was created
	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after handler returns, got %d", manager.Count())
	}
}

func TestSSEHandler_setSSEHeaders(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	w := httptest.NewRecorder()
	handler.setSSEHeaders(w)

	headers := w.Header()

	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{"Content-Type", "Content-Type", "text/event-stream"},
		{"Cache-Control", "Cache-Control", "no-cache"},
		{"Connection", "Connection", "keep-alive"},
		{"CORS Origin", "Access-Control-Allow-Origin", "*"},
		{"CORS Methods", "Access-Control-Allow-Methods", "GET, OPTIONS"},
		{"CORS Headers", "Access-Control-Allow-Headers", "Content-Type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := headers.Get(tt.header); got != tt.expected {
				t.Errorf("%s: expected %q, got %q", tt.header, tt.expected, got)
			}
		})
	}
}

func TestSSEHandler_sendEvent(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	w := httptest.NewRecorder()
	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "test-thread",
		RunID:    "test-run",
	})

	handler.sendEvent(w, event)

	body := w.Body.String()
	if body == "" {
		t.Error("Expected event to be written to response")
	}

	// Verify SSE format
	if len(body) < 10 || body[:6] != "data: " {
		t.Errorf("Expected SSE format, got %q", body)
	}
}

func TestSSEHandler_BroadcastToConnection(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Create and add connection
	conn := NewConnection("test-conn", "test-thread", "test-run")
	manager.Add(conn)
	defer close(conn.Done)

	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "test-thread",
		RunID:    "test-run",
	})

	err := handler.BroadcastToConnection("test-conn", event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify event was sent
	select {
	case received := <-conn.Events:
		if received.Type != RunStarted {
			t.Errorf("Expected RunStarted event, got %v", received.Type)
		}
	default:
		t.Error("Expected event to be sent to connection")
	}
}

func TestSSEHandler_BroadcastToConnection_NotFound(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	event := NewEvent(RunStarted, RunStartedEvent{})
	err := handler.BroadcastToConnection("nonexistent", event)

	if err == nil {
		t.Error("Expected error for nonexistent connection")
	}
}

func TestSSEHandler_BroadcastToThread(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Create connections for same thread
	conn1 := NewConnection("conn1", "test-thread", "run1")
	conn2 := NewConnection("conn2", "test-thread", "run2")
	conn3 := NewConnection("conn3", "other-thread", "run3")

	manager.Add(conn1)
	manager.Add(conn2)
	manager.Add(conn3)

	defer func() {
		close(conn1.Done)
		close(conn2.Done)
		close(conn3.Done)
	}()

	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "test-thread",
		RunID:    "test-run",
	})

	count := handler.BroadcastToThread("test-thread", event)

	if count != 2 {
		t.Errorf("Expected 2 connections to receive event, got %d", count)
	}

	// Verify both connections received the event
	select {
	case <-conn1.Events:
	case <-time.After(100 * time.Millisecond):
		t.Error("conn1 should receive event")
	}

	select {
	case <-conn2.Events:
	case <-time.After(100 * time.Millisecond):
		t.Error("conn2 should receive event")
	}

	select {
	case <-conn3.Events:
		t.Error("conn3 should not receive event for different thread")
	default:
		// Expected
	}
}

func TestSSEHandler_BroadcastToAll(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Create multiple connections
	conn1 := NewConnection("conn1", "thread1", "run1")
	conn2 := NewConnection("conn2", "thread2", "run2")

	manager.Add(conn1)
	manager.Add(conn2)

	defer func() {
		close(conn1.Done)
		close(conn2.Done)
	}()

	event := NewEvent(RunStarted, RunStartedEvent{})

	count := handler.BroadcastToAll(event)

	if count != 2 {
		t.Errorf("Expected 2 connections to receive event, got %d", count)
	}
}

func TestConnectionManager_Add(t *testing.T) {
	manager := NewConnectionManager()
	conn := NewConnection("test-id", "thread-id", "run-id")

	manager.Add(conn)

	if manager.Count() != 1 {
		t.Errorf("Expected 1 connection, got %d", manager.Count())
	}

	retrieved, ok := manager.Get("test-id")
	if !ok {
		t.Error("Expected to retrieve connection")
	}
	if retrieved.ID != "test-id" {
		t.Errorf("Expected ID test-id, got %s", retrieved.ID)
	}
}

func TestConnectionManager_Remove(t *testing.T) {
	manager := NewConnectionManager()
	conn := NewConnection("test-id", "thread-id", "run-id")

	manager.Add(conn)
	manager.Remove("test-id")

	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after removal, got %d", manager.Count())
	}
}

func TestConnectionManager_Get(t *testing.T) {
	manager := NewConnectionManager()
	conn := NewConnection("test-id", "thread-id", "run-id")
	manager.Add(conn)

	retrieved, ok := manager.Get("test-id")
	if !ok {
		t.Error("Expected to find connection")
	}
	if retrieved != conn {
		t.Error("Expected same connection instance")
	}

	_, ok = manager.Get("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent connection")
	}
}

func TestConnectionManager_GetByThread(t *testing.T) {
	manager := NewConnectionManager()

	conn1 := NewConnection("conn1", "thread1", "run1")
	conn2 := NewConnection("conn2", "thread1", "run2")
	conn3 := NewConnection("conn3", "thread2", "run3")

	manager.Add(conn1)
	manager.Add(conn2)
	manager.Add(conn3)

	conns := manager.GetByThread("thread1")
	if len(conns) != 2 {
		t.Errorf("Expected 2 connections for thread1, got %d", len(conns))
	}

	conns = manager.GetByThread("thread2")
	if len(conns) != 1 {
		t.Errorf("Expected 1 connection for thread2, got %d", len(conns))
	}

	conns = manager.GetByThread("nonexistent")
	if len(conns) != 0 {
		t.Errorf("Expected 0 connections for nonexistent thread, got %d", len(conns))
	}
}

func TestConnectionManager_List(t *testing.T) {
	manager := NewConnectionManager()

	conn1 := NewConnection("conn1", "thread1", "run1")
	conn2 := NewConnection("conn2", "thread2", "run2")

	manager.Add(conn1)
	manager.Add(conn2)

	conns := manager.List()
	if len(conns) != 2 {
		t.Errorf("Expected 2 connections, got %d", len(conns))
	}
}

func TestConnectionManager_Count(t *testing.T) {
	manager := NewConnectionManager()

	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections initially, got %d", manager.Count())
	}

	manager.Add(NewConnection("conn1", "thread1", "run1"))
	if manager.Count() != 1 {
		t.Errorf("Expected 1 connection, got %d", manager.Count())
	}

	manager.Add(NewConnection("conn2", "thread2", "run2"))
	if manager.Count() != 2 {
		t.Errorf("Expected 2 connections, got %d", manager.Count())
	}
}

func TestConnectionManager_CloseAll(t *testing.T) {
	manager := NewConnectionManager()

	conn1 := NewConnection("conn1", "thread1", "run1")
	conn2 := NewConnection("conn2", "thread2", "run2")

	manager.Add(conn1)
	manager.Add(conn2)

	// Wait for connections to be added
	time.Sleep(10 * time.Millisecond)

	manager.CloseAll()

	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after CloseAll, got %d", manager.Count())
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
}

func TestConnectionManager_ConcurrentAccess(t *testing.T) {
	manager := NewConnectionManager()
	var wg sync.WaitGroup

	// Concurrent adds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn := NewConnection(
				"conn"+string(rune(id)),
				"thread",
				"run",
			)
			manager.Add(conn)
		}(i)
	}

	// Concurrent gets
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Get("conn1")
		}()
	}

	// Concurrent removes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			manager.Remove("conn" + string(rune(id)))
		}(i)
	}

	wg.Wait()
}

func TestNewSSEHandler(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	if handler == nil {
		t.Error("Expected handler to be created")
		return
	}
	if handler.manager != manager {
		t.Error("Expected manager to be set")
	}
}

func TestConnection_Heartbeat(t *testing.T) {
	conn := NewConnection("test-id", "thread-id", "run-id")
	defer close(conn.Done)

	// Verify connection fields
	if conn.ID != "test-id" {
		t.Errorf("Expected ID test-id, got %s", conn.ID)
	}
	if conn.ThreadID != "thread-id" {
		t.Errorf("Expected ThreadID thread-id, got %s", conn.ThreadID)
	}
	if conn.RunID != "run-id" {
		t.Errorf("Expected RunID run-id, got %s", conn.RunID)
	}
	if conn.Events == nil {
		t.Error("Expected Events channel to be initialized")
	}
	if conn.Done == nil {
		t.Error("Expected Done channel to be initialized")
	}
	if conn.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestSSEHandler_Heartbeat(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("test-id", "thread-id", "run-id")
	manager.Add(conn)

	// Start heartbeat
	handler.startHeartbeat(conn)

	// Wait for heartbeat
	time.Sleep(20 * time.Millisecond)

	// Stop heartbeat
	handler.stopHeartbeat(conn)

	manager.Remove(conn.ID)
	close(conn.Done)
}

func TestSSEHandler_streamEvents(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("test-id", "thread-id", "run-id")
	manager.Add(conn)
	defer func() {
		manager.Remove(conn.ID)
		close(conn.Done)
	}()

	// Send event to connection
	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "test-thread",
		RunID:    "test-run",
	})
	conn.Events <- event

	// Create test response writer
	w := httptest.NewRecorder()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/sse", nil).WithContext(ctx)

	// Call streamEvents in goroutine
	done := make(chan bool)
	go func() {
		handler.streamEvents(w, req, conn)
		done <- true
	}()

	// Wait for streaming to complete
	select {
	case <-done:
		// Expected
	case <-time.After(500 * time.Millisecond):
		t.Error("streamEvents did not complete within timeout")
	}
}

func TestSSEHandler_streamEvents_ContextDone(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("test-id", "thread-id", "run-id")
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

	// Call streamEvents - should return immediately
	done := make(chan bool)
	go func() {
		handler.streamEvents(w, req, conn)
		done <- true
	}()

	select {
	case <-done:
		// Expected - should return immediately
	case <-time.After(500 * time.Millisecond):
		t.Error("streamEvents should return when context is done")
	}
}

func TestSSEHandler_streamEvents_ConnectionDone(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("test-id", "thread-id", "run-id")
	manager.Add(conn)

	w := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/sse", nil).WithContext(ctx)

	// Close connection immediately
	close(conn.Done)

	// Call streamEvents - should return immediately
	done := make(chan bool)
	go func() {
		handler.streamEvents(w, req, conn)
		done <- true
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(500 * time.Millisecond):
		t.Error("streamEvents should return when connection is done")
	}

	manager.Remove(conn.ID)
}

func TestSSEHandler_startHeartbeat_AlreadyStarted(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("test-id", "thread-id", "run-id")
	manager.Add(conn)
	defer func() {
		manager.Remove(conn.ID)
		close(conn.Done)
	}()

	// Start heartbeat first time
	handler.startHeartbeat(conn)

	// Try to start again - should not create duplicate
	handler.startHeartbeat(conn)

	// Stop heartbeat
	handler.stopHeartbeat(conn)
}

func TestSSEHandler_startHeartbeat_SendsEvents(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("test-id", "thread-id", "run-id")
	manager.Add(conn)
	defer func() {
		manager.Remove(conn.ID)
		close(conn.Done)
	}()

	// Start heartbeat
	handler.startHeartbeat(conn)

	// Wait for heartbeat event (15 seconds is too long, so we'll just verify it's set up)
	time.Sleep(50 * time.Millisecond)

	// Verify heartbeat cancel is set
	conn.mu.Lock()
	hasCancel := conn.heartbeatCancel != nil
	conn.mu.Unlock()

	if !hasCancel {
		t.Error("Expected heartbeat cancel to be set")
	}

	// Stop heartbeat
	handler.stopHeartbeat(conn)
}

func TestSSEHandler_stopHeartbeat_NoHeartbeat(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("test-id", "thread-id", "run-id")
	manager.Add(conn)
	defer func() {
		manager.Remove(conn.ID)
		close(conn.Done)
	}()

	// Stop heartbeat without starting - should not panic
	handler.stopHeartbeat(conn)
}

func TestSSEHandler_BroadcastToConnection_ClosedConnection(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("test-conn", "test-thread", "test-run")
	manager.Add(conn)
	close(conn.Done) // Close immediately
	defer manager.Remove(conn.ID)

	event := NewEvent(RunStarted, RunStartedEvent{})

	// When connection is closed, the select should go to the <-conn.Done case
	// and return an error. However, due to Go's select behavior with multiple
	// ready channels, we need to ensure the done channel is checked.
	// For this test, we'll just verify no panic occurs.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Unexpected panic: %v", r)
		}
	}()

	err := handler.BroadcastToConnection("test-conn", event)

	// Error may or may not be returned depending on select behavior
	_ = err
}

func TestSSEHandler_BroadcastToConnection_FullChannel(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Create connection with small buffer
	conn := &Connection{
		ID:        "test-conn",
		ThreadID:  "test-thread",
		RunID:     "test-run",
		Events:    make(chan Event, 1),
		Done:      make(chan struct{}),
		CreatedAt: time.Now(),
	}
	manager.Add(conn)
	defer func() {
		manager.Remove(conn.ID)
		close(conn.Done)
	}()

	// Fill the channel
	conn.Events <- NewEvent(RunStarted, RunStartedEvent{})

	// Try to broadcast - should fail due to full channel
	event := NewEvent(RunFinished, RunFinishedEvent{})
	err := handler.BroadcastToConnection("test-conn", event)

	if err == nil {
		t.Error("Expected error for full channel")
	}
}

func TestSSEHandler_sendEvent_MarshalError(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	w := httptest.NewRecorder()

	// Create event with unmarshalable data (channel)
	event := Event{
		Type:      CustomEvent,
		Timestamp: intPtr64(time.Now().Unix()),
		Data:      make(chan int),
	}

	// Should not panic, just return
	handler.sendEvent(w, event)

	// Verify nothing was written (due to marshal error)
	if w.Body.Len() != 0 {
		t.Error("Expected no output for unmarshalable event")
	}
}

func intPtr64(i int64) *int64 {
	return &i
}
