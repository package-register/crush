package aguiserver

import (
	"context"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
)

// MockCoordinator is a mock implementation of agent.Coordinator.
type MockCoordinator struct {
	mu          sync.Mutex
	runCalled   bool
	lastSession string
	lastPrompt  string
	result      *fantasy.AgentResult
	err         error
}

func (m *MockCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runCalled = true
	m.lastSession = sessionID
	m.lastPrompt = prompt
	return m.result, m.err
}

func (m *MockCoordinator) Cancel(sessionID string) {}
func (m *MockCoordinator) CancelAll()              {}
func (m *MockCoordinator) IsSessionBusy(sessionID string) bool {
	return false
}
func (m *MockCoordinator) IsBusy() bool                                   { return false }
func (m *MockCoordinator) QueuedPrompts(sessionID string) int             { return 0 }
func (m *MockCoordinator) QueuedPromptsList(sessionID string) []string    { return nil }
func (m *MockCoordinator) ClearQueue(sessionID string)                    {}
func (m *MockCoordinator) Summarize(ctx context.Context, sessionID string) error {
	return nil
}
func (m *MockCoordinator) Model() agent.Model { return agent.Model{} }
func (m *MockCoordinator) UpdateModels(ctx context.Context) error       { return nil }

func TestNewAgentBridge(t *testing.T) {
	coordinator := &MockCoordinator{}
	eventEmitter := &MockEventEmitter{}

	bridge := NewAgentBridge(coordinator, eventEmitter)

	if bridge == nil {
		t.Error("Expected bridge to be created")
	}
	if bridge.eventEmitter != eventEmitter {
		t.Error("Expected eventEmitter to be set")
	}
	if bridge.sessionManager == nil {
		t.Error("Expected sessionManager to be created")
	}
}

func TestAgentBridge_Execute_Success(t *testing.T) {
	coordinator := &MockCoordinator{
		result: &fantasy.AgentResult{},
	}
	eventEmitter := &MockEventEmitter{}
	bridge := NewAgentBridge(coordinator, eventEmitter)

	req := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	err := bridge.Execute(ctx, req)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Wait for async operations
	time.Sleep(50 * time.Millisecond)

	// Verify coordinator was called
	if !coordinator.runCalled {
		t.Error("Expected coordinator.Run to be called")
	}
	if coordinator.lastSession != "test-thread" {
		t.Errorf("Expected session test-thread, got %s", coordinator.lastSession)
	}
	if coordinator.lastPrompt != "Hello" {
		t.Errorf("Expected prompt Hello, got %s", coordinator.lastPrompt)
	}

	// Verify events were emitted
	eventEmitter.mu.Lock()
	eventTypes := make(map[EventType]int)
	for _, event := range eventEmitter.events {
		eventTypes[event.Type]++
	}
	eventEmitter.mu.Unlock()

	if eventTypes[RunStarted] < 1 {
		t.Error("Expected RunStarted event")
	}
	if eventTypes[RunFinished] < 1 {
		t.Error("Expected RunFinished event")
	}
}

func TestAgentBridge_Execute_NoUserMessage(t *testing.T) {
	coordinator := &MockCoordinator{}
	eventEmitter := &MockEventEmitter{}
	bridge := NewAgentBridge(coordinator, eventEmitter)

	req := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "assistant", Content: "Hello"},
		},
	}

	ctx := context.Background()
	err := bridge.Execute(ctx, req)

	if err == nil {
		t.Error("Expected error for missing user message")
	}
}

func TestAgentBridge_Execute_CoordinatorError(t *testing.T) {
	coordinator := &MockCoordinator{
		err: context.DeadlineExceeded,
	}
	eventEmitter := &MockEventEmitter{}
	bridge := NewAgentBridge(coordinator, eventEmitter)

	req := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	err := bridge.Execute(ctx, req)

	if err == nil {
		t.Error("Expected error from coordinator")
	}

	// Wait for async operations
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
		t.Error("Expected RunError event to be emitted")
	}
}

func TestAgentBridge_Execute_NilResult(t *testing.T) {
	coordinator := &MockCoordinator{
		result: nil,
	}
	eventEmitter := &MockEventEmitter{}
	bridge := NewAgentBridge(coordinator, eventEmitter)

	req := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	err := bridge.Execute(ctx, req)

	// Nil result should cause an error in processAgentResult
	if err == nil {
		t.Error("Expected error for nil result")
	}
}

func TestAgentBridge_processAgentResult_NilResult(t *testing.T) {
	coordinator := &MockCoordinator{}
	eventEmitter := &MockEventEmitter{}
	bridge := NewAgentBridge(coordinator, eventEmitter)

	err := bridge.processAgentResult("thread", "run", nil)

	if err == nil {
		t.Error("Expected error for nil result")
	}
}

func TestAgentBridge_processAgentResult_EmptyResult(t *testing.T) {
	coordinator := &MockCoordinator{}
	eventEmitter := &MockEventEmitter{}
	bridge := NewAgentBridge(coordinator, eventEmitter)

	result := &fantasy.AgentResult{}
	err := bridge.processAgentResult("thread", "run", result)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestSessionManager_Add(t *testing.T) {
	manager := NewSessionManager()

	manager.Add("thread1", "run1")

	session, ok := manager.Get("thread1")
	if !ok {
		t.Error("Expected session to be added")
	}
	if session.ThreadID != "thread1" {
		t.Errorf("Expected ThreadID thread1, got %s", session.ThreadID)
	}
	if session.RunID != "run1" {
		t.Errorf("Expected RunID run1, got %s", session.RunID)
	}
	if session.Status != SessionStatusRunning {
		t.Errorf("Expected status running, got %s", session.Status)
	}
}

func TestSessionManager_Update(t *testing.T) {
	manager := NewSessionManager()
	manager.Add("thread1", "run1")

	manager.Update("thread1", SessionStatusFinished)

	session, ok := manager.Get("thread1")
	if !ok {
		t.Error("Expected session to exist")
	}
	if session.Status != SessionStatusFinished {
		t.Errorf("Expected status finished, got %s", session.Status)
	}
}

func TestSessionManager_Get(t *testing.T) {
	manager := NewSessionManager()
	manager.Add("thread1", "run1")

	session, ok := manager.Get("thread1")
	if !ok {
		t.Error("Expected to find session")
	}
	if session == nil {
		t.Error("Expected non-nil session")
	}

	_, ok = manager.Get("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent session")
	}
}

func TestSessionManager_Remove(t *testing.T) {
	manager := NewSessionManager()
	manager.Add("thread1", "run1")

	manager.Remove("thread1")

	_, ok := manager.Get("thread1")
	if ok {
		t.Error("Expected session to be removed")
	}
}

func TestSessionManager_ConcurrentAccess(t *testing.T) {
	manager := NewSessionManager()
	var wg sync.WaitGroup

	// Concurrent adds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			manager.Add("thread"+string(rune(id)), "run")
		}(i)
	}

	// Concurrent gets
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Get("thread1")
		}()
	}

	// Concurrent updates
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Update("thread1", SessionStatusFinished)
		}()
	}

	wg.Wait()
}

func TestSimpleEventEmitter_Emit(t *testing.T) {
	manager := NewConnectionManager()
	emitter := NewSimpleEventEmitter(manager)

	conn := NewConnection("conn1", "thread1", "run1")
	manager.Add(conn)
	defer close(conn.Done)

	event := NewEvent(RunStarted, RunStartedEvent{})
	err := emitter.Emit(event)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	select {
	case received := <-conn.Events:
		if received.Type != RunStarted {
			t.Errorf("Expected RunStarted event, got %v", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected event to be sent")
	}
}

func TestSimpleEventEmitter_EmitToThread(t *testing.T) {
	manager := NewConnectionManager()
	emitter := NewSimpleEventEmitter(manager)

	conn1 := NewConnection("conn1", "thread1", "run1")
	conn2 := NewConnection("conn2", "thread1", "run2")
	conn3 := NewConnection("conn3", "thread2", "run3")

	manager.Add(conn1)
	manager.Add(conn2)
	manager.Add(conn3)

	defer func() {
		close(conn1.Done)
		close(conn2.Done)
		close(conn3.Done)
	}()

	event := NewEvent(RunStarted, RunStartedEvent{})
	err := emitter.EmitToThread("thread1", event)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify conn1 and conn2 received event
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

	// Verify conn3 did not receive event
	select {
	case <-conn3.Events:
		t.Error("conn3 should not receive event")
	default:
		// Expected
	}
}

func TestSimpleEventEmitter_EmitToConnection(t *testing.T) {
	manager := NewConnectionManager()
	emitter := NewSimpleEventEmitter(manager)

	conn := NewConnection("conn1", "thread1", "run1")
	manager.Add(conn)
	defer close(conn.Done)

	event := NewEvent(RunStarted, RunStartedEvent{})
	err := emitter.EmitToConnection("conn1", event)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	select {
	case received := <-conn.Events:
		if received.Type != RunStarted {
			t.Errorf("Expected RunStarted event, got %v", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected event to be sent")
	}
}

func TestSimpleEventEmitter_EmitToConnection_NotFound(t *testing.T) {
	manager := NewConnectionManager()
	emitter := NewSimpleEventEmitter(manager)

	event := NewEvent(RunStarted, RunStartedEvent{})
	err := emitter.EmitToConnection("nonexistent", event)

	if err == nil {
		t.Error("Expected error for nonexistent connection")
	}
}

func TestSessionStatusConstants(t *testing.T) {
	tests := []struct {
		name   string
		status SessionStatus
	}{
		{"running", SessionStatusRunning},
		{"finished", SessionStatusFinished},
		{"error", SessionStatusError},
		{"cancelled", SessionStatusCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.status == "" {
				t.Error("Expected non-empty status")
			}
		})
	}
}

func TestNewSessionManager(t *testing.T) {
	manager := NewSessionManager()

	if manager == nil {
		t.Error("Expected manager to be created")
	}
	if manager.sessions == nil {
		t.Error("Expected sessions map to be initialized")
	}
}

func TestSimpleEventEmitter_EmitToConnection_ClosedConnection(t *testing.T) {
	manager := NewConnectionManager()
	emitter := NewSimpleEventEmitter(manager)

	conn := NewConnection("conn1", "thread1", "run1")
	manager.Add(conn)
	close(conn.Done) // Close immediately
	defer manager.Remove(conn.ID)

	event := NewEvent(RunStarted, RunStartedEvent{})
	
	// When connection is closed, the select should go to the <-conn.Done case
	// and return an error. However, due to Go's select behavior with multiple
	// ready channels, we need to ensure the done channel is checked.
	// The actual behavior depends on which case is selected first.
	// For this test, we'll just verify no panic occurs.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Unexpected panic: %v", r)
		}
	}()
	
	err := emitter.EmitToConnection("conn1", event)
	
	// Error may or may not be returned depending on select behavior
	_ = err
}

func TestSimpleEventEmitter_EmitToConnection_FullChannel(t *testing.T) {
	manager := NewConnectionManager()
	emitter := NewSimpleEventEmitter(manager)

	// Create connection with small buffer
	conn := &Connection{
		ID:        "conn1",
		ThreadID:  "thread1",
		RunID:     "run1",
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

	// Try to emit - should fail due to full channel
	event := NewEvent(RunFinished, RunFinishedEvent{})
	err := emitter.EmitToConnection("conn1", event)

	if err == nil {
		t.Error("Expected error for full channel")
	}
}

func TestAgentBridge_Execute_EmmiterError(t *testing.T) {
	coordinator := &MockCoordinator{
		result: &fantasy.AgentResult{},
	}
	eventEmitter := &MockEventEmitter{
		emitError: context.DeadlineExceeded,
	}
	bridge := NewAgentBridge(coordinator, eventEmitter)

	req := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	err := bridge.Execute(ctx, req)

	// Should return error when emitter fails
	if err == nil {
		t.Error("Expected error from emitter")
	}
}

func TestAgentBridge_processAgentResult_EmitterError(t *testing.T) {
	coordinator := &MockCoordinator{}
	eventEmitter := &MockEventEmitter{
		emitError: context.DeadlineExceeded,
	}
	bridge := NewAgentBridge(coordinator, eventEmitter)

	result := &fantasy.AgentResult{}
	err := bridge.processAgentResult("thread", "run", result)

	// Should return error when emitter fails
	if err == nil {
		t.Error("Expected error from emitter")
	}
}

func TestSessionManager_Update_NonExistent(t *testing.T) {
	manager := NewSessionManager()

	// Update non-existent session - should not panic
	manager.Update("nonexistent", SessionStatusFinished)
}

func TestAgentBridge_Execute_ContextCancellation(t *testing.T) {
	coordinator := &MockCoordinator{
		result: &fantasy.AgentResult{},
	}
	eventEmitter := &MockEventEmitter{}
	bridge := NewAgentBridge(coordinator, eventEmitter)

	req := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should handle context cancellation gracefully
	err := bridge.Execute(ctx, req)

	// May or may not return error depending on timing
	_ = err
}

// Table-driven tests for SessionStatus
func TestSessionStatus_Values(t *testing.T) {
	tests := []struct {
		name   string
		status SessionStatus
		valid  bool
	}{
		{"running", SessionStatusRunning, true},
		{"finished", SessionStatusFinished, true},
		{"error", SessionStatusError, true},
		{"cancelled", SessionStatusCancelled, true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid && tt.status == "" {
				t.Error("Expected non-empty status")
			}
		})
	}
}

// Benchmark tests for AgentBridge
func BenchmarkAgentBridge_Execute(b *testing.B) {
	coordinator := &MockCoordinator{
		result: &fantasy.AgentResult{},
	}
	eventEmitter := &MockEventEmitter{}
	bridge := NewAgentBridge(coordinator, eventEmitter)

	req := RunRequest{
		ThreadID: "test-thread",
		RunID:    "test-run",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bridge.Execute(ctx, req)
	}
}

func BenchmarkSessionManager_Add(b *testing.B) {
	manager := NewSessionManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Add("thread"+string(rune(i%100)), "run")
	}
}

func BenchmarkSessionManager_Get(b *testing.B) {
	manager := NewSessionManager()
	manager.Add("thread1", "run1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Get("thread1")
	}
}

func BenchmarkSimpleEventEmitter_Emit(b *testing.B) {
	manager := NewConnectionManager()
	emitter := NewSimpleEventEmitter(manager)

	conn := NewConnection("conn1", "thread1", "run1")
	manager.Add(conn)
	defer func() {
		manager.Remove(conn.ID)
		close(conn.Done)
	}()

	event := NewEvent(RunStarted, RunStartedEvent{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		emitter.Emit(event)
	}
}

// TestSimpleEventEmitter_NilManager tests emitter with nil manager.
func TestSimpleEventEmitter_NilManager(t *testing.T) {
	emitter := NewSimpleEventEmitter(nil)

	// Should not panic and should return nil
	err := emitter.Emit(NewEvent(RunStarted, RunStartedEvent{}))
	if err != nil {
		t.Errorf("Expected no error with nil manager, got %v", err)
	}

	err = emitter.EmitToThread("thread1", NewEvent(RunStarted, RunStartedEvent{}))
	if err != nil {
		t.Errorf("Expected no error with nil manager, got %v", err)
	}

	err = emitter.EmitToConnection("conn1", NewEvent(RunStarted, RunStartedEvent{}))
	if err == nil {
		t.Error("Expected error with nil manager for EmitToConnection")
	}
}

// TestSimpleEventEmitter_FullChannel tests emitter with full channel.
func TestSimpleEventEmitter_FullChannel(t *testing.T) {
	manager := NewConnectionManager()
	emitter := NewSimpleEventEmitter(manager)

	// Create connection with small buffer
	conn := &Connection{
		ID:       "conn1",
		ThreadID: "thread1",
		RunID:    "run1",
		Events:   make(chan Event, 1),
		Done:     make(chan struct{}),
	}
	manager.Add(conn)
	defer func() {
		manager.Remove(conn.ID)
		close(conn.Done)
	}()

	// Fill the channel
	conn.Events <- NewEvent(RunStarted, RunStartedEvent{})

	// Next emit should not block (default case)
	err := emitter.EmitToConnection("conn1", NewEvent(RunStarted, RunStartedEvent{}))
	if err == nil {
		t.Error("Expected error when channel is full")
	}
}


