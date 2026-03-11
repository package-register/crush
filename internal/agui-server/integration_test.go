package aguiserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
)

// TestEndToEnd tests the complete flow from request to event streaming.
func TestEndToEnd(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Create event emitter
	emitter := NewSimpleEventEmitter(manager)

	// Create bridge
	bridge := NewAgentBridge(&MockCoordinator{
		result: &fantasy.AgentResult{},
	}, emitter)

	// Start SSE connection
	req := httptest.NewRequest(http.MethodGet, "/sse?threadId=e2e-thread&runId=e2e-run", nil)
	w := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	// Call handler in goroutine
	done := make(chan bool)
	go func() {
		handler.HandleSSE(w, req)
		done <- true
	}()

	// Give time for connection to be established
	time.Sleep(50 * time.Millisecond)

	// Execute agent run
	runReq := RunRequest{
		ThreadID: "e2e-thread",
		RunID:    "e2e-run",
		Messages: []Message{
			{Role: "user", Content: "Hello from E2E test"},
		},
	}

	err := bridge.Execute(context.Background(), runReq)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Wait for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop SSE
	cancel()

	// Wait for handler to finish
	select {
	case <-done:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("SSE handler did not finish within timeout")
	}

	// Verify connection was cleaned up
	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after cleanup, got %d", manager.Count())
	}
}

// TestConcurrentConnections tests multiple simultaneous SSE connections.
func TestConcurrentConnections(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	const numConnections = 10

	var wg sync.WaitGroup
	errors := make(chan error, numConnections)

	for i := range numConnections {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sse?threadId=thread-%d&runId=run-%d", id, id), nil)
			w := httptest.NewRecorder()

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			req = req.WithContext(ctx)

			handler.HandleSSE(w, req)
		}(i)
	}

	// Wait a bit for connections to be established
	time.Sleep(50 * time.Millisecond)

	// Check connection count
	if manager.Count() > numConnections {
		t.Errorf("Expected at most %d connections, got %d", numConnections, manager.Count())
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// All connections should be cleaned up
	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after all done, got %d", manager.Count())
	}

	// Check for errors
	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("Connection error: %v", err)
		}
	}
}

// TestConcurrentEventEmission tests thread-safe event emission.
func TestConcurrentEventEmission(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Create multiple connections
	for i := range 5 {
		conn := NewConnection(fmt.Sprintf("conn-%d", i), "shared-thread", fmt.Sprintf("run-%d", i))
		manager.Add(conn)
	}
	defer manager.CloseAll()

	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "shared-thread",
		RunID:    "shared-run",
	})

	var wg sync.WaitGroup
	const numEmitters = 10

	// Concurrent event emission
	for i := range numEmitters {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			handler.BroadcastToThread("shared-thread", event)
		}(i)
	}

	wg.Wait()

	// Verify no panics occurred
	if manager.Count() != 5 {
		t.Errorf("Expected 5 connections, got %d", manager.Count())
	}
}

// TestErrorRecovery tests that the server recovers from various error conditions.
func TestErrorRecovery(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	t.Run("ConnectionCleanupOnError", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sse?threadId=error-thread&runId=error-run", nil)
		w := httptest.NewRecorder()

		ctx, cancel := context.WithCancel(context.Background())
		req = req.WithContext(ctx)

		done := make(chan bool)
		go func() {
			handler.HandleSSE(w, req)
			done <- true
		}()

		// Cancel immediately
		time.Sleep(10 * time.Millisecond)
		cancel()

		// Wait for cleanup
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Error("Handler did not finish")
		}

		// Verify cleanup
		if manager.Count() != 0 {
			t.Errorf("Expected connection to be cleaned up, got %d", manager.Count())
		}
	})

	t.Run("InvalidRequestRecovery", func(t *testing.T) {
		// Invalid JSON should not crash the server
		req := httptest.NewRequest(http.MethodPost, "/run", nil)
		w := httptest.NewRecorder()

		// Create a handler for testing
		srv := NewServer(config)
		server, ok := srv.(*server)
		if !ok {
			t.Fatal("Expected server to be of type *server")
		}

		// Should not panic
		server.HandleRun(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("EmitterErrorRecovery", func(t *testing.T) {
		emitter := &MockEventEmitter{
			emitError: fmt.Errorf("emitter error"),
		}
		bridge := NewAgentBridge(&MockCoordinator{
			result: &fantasy.AgentResult{},
		}, emitter)

		req := RunRequest{
			ThreadID: "recover-thread",
			RunID:    "recover-run",
			Messages: []Message{
				{Role: "user", Content: "Hello"},
			},
		}

		// Should return error but not crash
		err := bridge.Execute(context.Background(), req)
		if err == nil {
			t.Error("Expected error from failing emitter")
		}
	})
}

// TestReconnection tests that clients can reconnect after disconnection.
func TestReconnection(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	threadID := "reconnect-thread"
	runID := "reconnect-run"

	// First connection
	req1 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sse?threadId=%s&runId=%s", threadID, runID), nil)
	w1 := httptest.NewRecorder()
	ctx1, cancel1 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel1()
	req1 = req1.WithContext(ctx1)

	done1 := make(chan bool)
	go func() {
		handler.HandleSSE(w1, req1)
		done1 <- true
	}()

	time.Sleep(50 * time.Millisecond)

	// Cancel first connection
	cancel1()
	<-done1

	// Verify first connection is cleaned up
	if manager.Count() != 0 {
		t.Errorf("Expected first connection to be cleaned up")
	}

	// Second connection (reconnection)
	req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sse?threadId=%s&runId=%s", threadID, runID), nil)
	w2 := httptest.NewRecorder()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	req2 = req2.WithContext(ctx2)

	done2 := make(chan bool)
	go func() {
		handler.HandleSSE(w2, req2)
		done2 <- true
	}()

	time.Sleep(50 * time.Millisecond)

	cancel2()
	<-done2

	// Verify second connection is also cleaned up
	if manager.Count() != 0 {
		t.Errorf("Expected second connection to be cleaned up")
	}
}

// TestServerLifecycle tests server start and stop.
func TestServerLifecycle(t *testing.T) {
	config := ServerConfig{
		Port:        19091, // Use different port
		BasePath:    "/agui",
		CORSOrigins: []string{"*"},
	}
	srv := NewServer(config)

	// Start server with background context
	startCtx, startCancel := context.WithCancel(context.Background())
	defer startCancel()

	// Start server in goroutine
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(startCtx)
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Stop server with timeout
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()

	err := srv.Stop(stopCtx)
	if err != nil {
		t.Errorf("Expected no error on stop, got %v", err)
	}

	// Cancel start context to ensure server exits
	startCancel()

	// Wait for start to return
	select {
	case err := <-done:
		if err != nil && err != http.ErrServerClosed && err != context.Canceled {
			t.Errorf("Expected server to stop gracefully, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Server did not stop within timeout")
	}
}

// TestBroadcastScalability tests broadcasting to many connections.
func TestBroadcastScalability(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	const numConnections = 50

	// Create many connections
	for i := range numConnections {
		conn := NewConnection(fmt.Sprintf("conn-%d", i), "broadcast-thread", fmt.Sprintf("run-%d", i))
		manager.Add(conn)
	}
	defer manager.CloseAll()

	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "broadcast-thread",
		RunID:    "broadcast-run",
	})

	// Broadcast to all
	count := handler.BroadcastToAll(event)

	if count != numConnections {
		t.Errorf("Expected %d connections to receive event, got %d", numConnections, count)
	}
}

//
// Performance Benchmark Tests
//

// BenchmarkSSEConnectionCreation benchmarks SSE connection creation.
func BenchmarkSSEConnectionCreation(b *testing.B) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sse?threadId=bench-thread-%d&runId=bench-run-%d", i, i), nil)
		w := httptest.NewRecorder()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		req = req.WithContext(ctx)

		handler.HandleSSE(w, req)
		cancel()
	}
}

// BenchmarkEventEmission benchmarks event emission to a single connection.
func BenchmarkEventEmission(b *testing.B) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("bench-conn", "bench-thread", "bench-run")
	manager.Add(conn)
	defer func() {
		manager.Remove(conn.ID)
		close(conn.Done)
	}()

	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "bench-thread",
		RunID:    "bench-run",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.BroadcastToConnection("bench-conn", event)
	}
}

// BenchmarkBroadcastToThread benchmarks broadcasting to multiple connections.
func BenchmarkBroadcastToThread(b *testing.B) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	const numConnections = 100
	for i := range numConnections {
		conn := NewConnection(fmt.Sprintf("bench-conn-%d", i), "bench-thread", fmt.Sprintf("bench-run-%d", i))
		manager.Add(conn)
	}
	defer manager.CloseAll()

	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "bench-thread",
		RunID:    "bench-run",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.BroadcastToThread("bench-thread", event)
	}
}

// BenchmarkBroadcastToAll benchmarks broadcasting to all connections.
func BenchmarkBroadcastToAll(b *testing.B) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	const numConnections = 100
	for i := range numConnections {
		conn := NewConnection(fmt.Sprintf("bench-conn-%d", i), fmt.Sprintf("thread-%d", i), fmt.Sprintf("run-%d", i))
		manager.Add(conn)
	}
	defer manager.CloseAll()

	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "bench-thread",
		RunID:    "bench-run",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.BroadcastToAll(event)
	}
}

// BenchmarkAgentBridgeExecute benchmarks agent bridge execution.
func BenchmarkAgentBridgeExecute(b *testing.B) {
	manager := NewConnectionManager()
	emitter := NewSimpleEventEmitter(manager)
	bridge := NewAgentBridge(&MockCoordinator{
		result: &fantasy.AgentResult{},
	}, emitter)

	req := RunRequest{
		ThreadID: "bench-thread",
		RunID:    "bench-run",
		Messages: []Message{
			{Role: "user", Content: "Benchmark message"},
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bridge.Execute(ctx, req)
	}
}

// BenchmarkSSEThroughput benchmarks SSE event throughput.
func BenchmarkSSEThroughput(b *testing.B) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	conn := NewConnection("bench-conn", "bench-thread", "bench-run")
	manager.Add(conn)
	defer manager.Remove(conn.ID)

	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := NewEvent(RunStarted, RunStartedEvent{
			ThreadID: "bench-thread",
			RunID:    fmt.Sprintf("bench-run-%d", i),
		})

		// Send event directly without streaming
		handler.sendEvent(w, event)
	}
}

// BenchmarkJSONMarshaling benchmarks JSON marshaling of events.
func BenchmarkJSONMarshaling(b *testing.B) {
	event := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: "bench-thread",
		RunID:    "bench-run",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(event)
	}
}

// BenchmarkSSEHandler_HandleSSE benchmarks the full SSE handler.
func BenchmarkSSEHandler_HandleSSE(b *testing.B) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sse?threadId=%d&runId=%d", i, i), nil)
		w := httptest.NewRecorder()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		req = req.WithContext(ctx)

		handler.HandleSSE(w, req)
		cancel()
	}
}

// TestSSEStreamFormat tests that SSE events are properly formatted.
func TestSSEStreamFormat(t *testing.T) {
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

	// Verify SSE format: "data: {...}\n\n"
	if len(body) < 10 {
		t.Fatal("Expected SSE event body")
	}

	// Check prefix
	if body[:6] != "data: " {
		t.Errorf("Expected 'data: ' prefix, got %q", body[:6])
	}

	// Check suffix (double newline)
	if body[len(body)-2:] != "\n\n" {
		t.Errorf("Expected '\\n\\n' suffix, got %q", body[len(body)-2:])
	}

	// Verify JSON is valid
	jsonStr := body[6 : len(body)-2]
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Errorf("Invalid JSON in SSE event: %v", err)
	}
}

// TestConnectionManagerStress stress tests the connection manager.
func TestConnectionManagerStress(t *testing.T) {
	manager := NewConnectionManager()

	var wg sync.WaitGroup
	const numOps = 100

	// Track created connections
	var mu sync.Mutex
	created := make(map[string]bool)

	// Concurrent adds and removes
	for i := range numOps {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			conn := NewConnection(fmt.Sprintf("stress-%d", id), "stress-thread", "stress-run")
			manager.Add(conn)
			mu.Lock()
			created[conn.ID] = true
			mu.Unlock()
		}(i)

		go func(id int) {
			defer wg.Done()
			// Small delay to ensure add happens first
			time.Sleep(time.Millisecond)
			manager.Remove(fmt.Sprintf("stress-%d", id))
		}(i)
	}

	wg.Wait()

	// Due to timing, some connections may remain - just verify no panic
	// and count is reasonable
	count := manager.Count()
	if count > numOps {
		t.Errorf("Expected at most %d connections, got %d", numOps, count)
	}

	// Clean up any remaining connections
	manager.CloseAll()
}

// TestHandlerWithRealHTTPServer tests handler with a real HTTP test server.
func TestHandlerWithRealHTTPServer(t *testing.T) {
	config := DefaultServerConfig()
	manager := NewConnectionManager()
	handler := NewSSEHandler(config, manager)

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", handler.HandleSSE)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Make request
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/sse?threadId=test&runId=test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify SSE content type
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	// Read first event
	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if len(line) < 6 || line[:6] != "data: " {
		t.Errorf("Expected SSE format, got %q", line)
	}
}
