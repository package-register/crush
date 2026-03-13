package aguiserver

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

// mockSessionService is a mock session.Service for testing aguiCoordinator.
type mockSessionService struct {
	mu        sync.Mutex
	sessions  map[string]session.Session
	getErr    error // if set, Get returns this error
	createErr error // if set, CreateWithID returns this error
}

func newMockSessionService() *mockSessionService {
	return &mockSessionService{
		sessions: make(map[string]session.Session),
	}
}

func (m *mockSessionService) Get(ctx context.Context, id string) (session.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return session.Session{}, m.getErr
	}
	if s, ok := m.sessions[id]; ok {
		return s, nil
	}
	return session.Session{}, sql.ErrNoRows
}

func (m *mockSessionService) CreateWithID(ctx context.Context, id, title string) (session.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return session.Session{}, m.createErr
	}
	s := session.Session{ID: id, Title: title}
	m.sessions[id] = s
	return s, nil
}

func (m *mockSessionService) Subscribe(ctx context.Context) <-chan pubsub.Event[session.Session] {
	ch := make(chan pubsub.Event[session.Session])
	close(ch)
	return ch
}

// Unused methods to satisfy session.Service interface (not used by agui_coordinator).
func (m *mockSessionService) Create(context.Context, string) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionService) CreateTitleSession(context.Context, string) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionService) CreateTaskSession(context.Context, string, string, string) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionService) List(context.Context) ([]session.Session, error) { return nil, nil }
func (m *mockSessionService) Save(context.Context, session.Session) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionService) UpdateTitleAndUsage(context.Context, string, string, int64, int64, float64) error {
	return nil
}
func (m *mockSessionService) Delete(context.Context, string) error { return nil }
func (m *mockSessionService) CreateAgentToolSessionID(string, string) string {
	return ""
}
func (m *mockSessionService) ParseAgentToolSessionID(string) (string, string, bool) {
	return "", "", false
}
func (m *mockSessionService) IsAgentToolSession(string) bool { return false }

func TestNewAGUICoordinator(t *testing.T) {
	coord := &MockCoordinator{}
	sessions := newMockSessionService()
	wrapped := NewAGUICoordinator(coord, sessions, nil)
	if wrapped == nil {
		t.Fatal("Expected non-nil coordinator")
	}
}

func TestAGUICoordinator_Run_SessionExists(t *testing.T) {
	coord := &MockCoordinator{result: &fantasy.AgentResult{}}
	sessions := newMockSessionService()
	sessions.mu.Lock()
	sessions.sessions["thread-1"] = session.Session{ID: "thread-1", Title: "Existing"}
	sessions.mu.Unlock()
	wrapped := NewAGUICoordinator(coord, sessions, nil)

	result, err := wrapped.Run(context.Background(), "thread-1", "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if !coord.runCalled {
		t.Error("Expected coordinator.Run to be called")
	}
	if coord.lastSession != "thread-1" {
		t.Errorf("Expected session thread-1, got %s", coord.lastSession)
	}
	if coord.lastPrompt != "hello" {
		t.Errorf("Expected prompt hello, got %s", coord.lastPrompt)
	}
}

func TestAGUICoordinator_Run_SessionNotExists_CreateThenRun(t *testing.T) {
	coord := &MockCoordinator{result: &fantasy.AgentResult{}}
	sessions := newMockSessionService()
	wrapped := NewAGUICoordinator(coord, sessions, nil)

	result, err := wrapped.Run(context.Background(), "new-thread", "hi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	// CreateWithID should have been called
	sessions.mu.Lock()
	_, created := sessions.sessions["new-thread"]
	sessions.mu.Unlock()
	if !created {
		t.Error("Expected session to be created")
	}
	if !coord.runCalled {
		t.Error("Expected coordinator.Run to be called")
	}
	if coord.lastSession != "new-thread" {
		t.Errorf("Expected session new-thread, got %s", coord.lastSession)
	}
}

func TestAGUICoordinator_Run_GetErrorPropagated(t *testing.T) {
	coord := &MockCoordinator{}
	sessions := newMockSessionService()
	sessions.getErr = errors.New("db error")
	wrapped := NewAGUICoordinator(coord, sessions, nil)

	_, err := wrapped.Run(context.Background(), "thread", "hi")
	if err == nil {
		t.Fatal("Expected error from Get")
	}
	if err.Error() != "db error" {
		t.Errorf("Expected db error, got %v", err)
	}
	if coord.runCalled {
		t.Error("Expected coordinator.Run not to be called")
	}
}

// TestAGUICoordinator_Run_CreateFailsRetryGetSucceeds tests the race where CreateWithID fails
// (e.g. unique constraint) but a concurrent goroutine created the session; retry Get succeeds.
func TestAGUICoordinator_Run_LongThreadID(t *testing.T) {
	coord := &MockCoordinator{result: &fantasy.AgentResult{}}
	sessions := newMockSessionService()
	wrapped := NewAGUICoordinator(coord, sessions, nil)

	longID := "thread-with-very-long-identifier-that-exceeds-32-chars"
	result, err := wrapped.Run(context.Background(), longID, "hi")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	sessions.mu.Lock()
	_, created := sessions.sessions[longID]
	sessions.mu.Unlock()
	if !created {
		t.Error("Expected session to be created with long ID")
	}
}

func TestAGUICoordinator_Run_CreateFailsRetryGetSucceeds(t *testing.T) {
	coord := &MockCoordinator{result: &fantasy.AgentResult{}}
	// Use a custom mock that creates session then returns error on CreateWithID
	sessions := &mockSessionCreateFailsOnce{sessions: map[string]session.Session{}}
	wrapped := NewAGUICoordinator(coord, sessions, nil)

	result, err := wrapped.Run(context.Background(), "race-thread", "hello")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if !coord.runCalled {
		t.Error("Expected coordinator.Run to be called after retry")
	}
}

// mockSessionCreateFailsOnce: Get returns NoRows until after first CreateWithID attempt.
// CreateWithID adds the session to map then returns error (simulating race).
// Next Get will find it.
type mockSessionCreateFailsOnce struct {
	mu           sync.Mutex
	sessions     map[string]session.Session
	createCalled bool
}

func (m *mockSessionCreateFailsOnce) Get(ctx context.Context, id string) (session.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		return s, nil
	}
	return session.Session{}, sql.ErrNoRows
}

func (m *mockSessionCreateFailsOnce) CreateWithID(ctx context.Context, id, title string) (session.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Simulate: we "create" it (another goroutine would have) but return error (e.g. unique constraint)
	m.sessions[id] = session.Session{ID: id, Title: title}
	m.createCalled = true
	return session.Session{}, errors.New("UNIQUE constraint failed")
}

func (m *mockSessionCreateFailsOnce) Create(context.Context, string) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionCreateFailsOnce) CreateTitleSession(context.Context, string) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionCreateFailsOnce) CreateTaskSession(context.Context, string, string, string) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionCreateFailsOnce) List(context.Context) ([]session.Session, error) {
	return nil, nil
}
func (m *mockSessionCreateFailsOnce) Save(context.Context, session.Session) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionCreateFailsOnce) UpdateTitleAndUsage(context.Context, string, string, int64, int64, float64) error {
	return nil
}
func (m *mockSessionCreateFailsOnce) Delete(context.Context, string) error { return nil }
func (m *mockSessionCreateFailsOnce) CreateAgentToolSessionID(string, string) string {
	return ""
}
func (m *mockSessionCreateFailsOnce) ParseAgentToolSessionID(string) (string, string, bool) {
	return "", "", false
}
func (m *mockSessionCreateFailsOnce) IsAgentToolSession(string) bool { return false }

func (m *mockSessionCreateFailsOnce) Subscribe(ctx context.Context) <-chan pubsub.Event[session.Session] {
	ch := make(chan pubsub.Event[session.Session])
	close(ch)
	return ch
}

func TestAGUICoordinator_Delegation(t *testing.T) {
	coord := &MockCoordinator{}
	sessions := newMockSessionService()
	sessions.mu.Lock()
	sessions.sessions["x"] = session.Session{ID: "x"}
	sessions.mu.Unlock()
	wrapped := NewAGUICoordinator(coord, sessions, nil)

	wrapped.Cancel("x")
	wrapped.CancelAll()
	if got := wrapped.IsSessionBusy("x"); got != false {
		t.Errorf("IsSessionBusy = %v", got)
	}
	if got := wrapped.IsBusy(); got != false {
		t.Errorf("IsBusy = %v", got)
	}
	if got := wrapped.QueuedPrompts("x"); got != 0 {
		t.Errorf("QueuedPrompts = %v", got)
	}
	if got := wrapped.QueuedPromptsList("x"); got != nil {
		t.Errorf("QueuedPromptsList = %v", got)
	}
	wrapped.ClearQueue("x")
	if err := wrapped.Summarize(context.Background(), "x"); err != nil {
		t.Errorf("Summarize: %v", err)
	}
	_ = wrapped.Model()
	if err := wrapped.UpdateModels(context.Background()); err != nil {
		t.Errorf("UpdateModels: %v", err)
	}
}
