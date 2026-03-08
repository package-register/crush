// Package aguiserver implements the AG-UI Protocol server for Crush.
// It provides SSE-based streaming of AG-UI events to external clients.
package aguiserver

import (
	"context"
	"fmt"
	"sync"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
)

// AgentBridge bridges AG-UI events with the Crush Agent system.
// It converts Agent events to AG-UI protocol events and manages state synchronization.
type AgentBridge struct {
	coordinator    agent.Coordinator
	eventEmitter   EventEmitter
	sessionManager *SessionManager
	mu             sync.RWMutex
}

// SessionManager manages active agent sessions.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*SessionInfo
}

// SessionInfo holds information about an active session.
type SessionInfo struct {
	ThreadID string
	RunID    string
	Status   SessionStatus
}

// SessionStatus represents the status of a session.
type SessionStatus string

const (
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusFinished  SessionStatus = "finished"
	SessionStatusError     SessionStatus = "error"
	SessionStatusCancelled SessionStatus = "cancelled"
)

// NewSessionManager creates a new SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*SessionInfo),
	}
}

// Add adds a session to the manager.
func (m *SessionManager) Add(threadID, runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[threadID] = &SessionInfo{
		ThreadID: threadID,
		RunID:    runID,
		Status:   SessionStatusRunning,
	}
}

// Update updates the status of a session.
func (m *SessionManager) Update(threadID string, status SessionStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if session, ok := m.sessions[threadID]; ok {
		session.Status = status
	}
}

// Get retrieves a session by thread ID.
func (m *SessionManager) Get(threadID string) (*SessionInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[threadID]
	return session, ok
}

// Remove removes a session from the manager.
func (m *SessionManager) Remove(threadID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, threadID)
}

// NewAgentBridge creates a new AgentBridge.
func NewAgentBridge(coordinator agent.Coordinator, eventEmitter EventEmitter) *AgentBridge {
	return &AgentBridge{
		coordinator:    coordinator,
		eventEmitter:   eventEmitter,
		sessionManager: NewSessionManager(),
	}
}

// Execute executes an agent run and streams AG-UI events.
func (b *AgentBridge) Execute(ctx context.Context, req RunRequest) error {
	// Save fields to local variables to avoid race conditions
	emitter := b.eventEmitter
	sessionMgr := b.sessionManager

	// Register session
	sessionMgr.Add(req.ThreadID, req.RunID)
	defer sessionMgr.Remove(req.ThreadID)

	// Emit run started event (check for nil emitter)
	if emitter != nil {
		startEvent := RunStartedBuilder().
			WithThreadID(req.ThreadID).
			WithRunID(req.RunID).
			Build()
		if err := emitter.EmitToThread(req.ThreadID, startEvent); err != nil {
			return fmt.Errorf("failed to emit run started event: %w", err)
		}
	}

	// Get the last user message as the prompt
	var prompt string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			prompt = req.Messages[i].Content
			break
		}
	}

	if prompt == "" {
		return fmt.Errorf("no user message found in request")
	}

	// Execute agent (use context.WithoutCancel to prevent cancellation)
	execCtx := context.WithoutCancel(ctx)
	result, err := b.coordinator.Run(execCtx, req.ThreadID, prompt)
	if err != nil {
		// Emit error event (check for nil emitter)
		if emitter != nil {
			errorEvent := RunErrorBuilder().
				WithThreadID(req.ThreadID).
				WithRunID(req.RunID).
				WithError(err.Error()).
				Build()
			emitter.EmitToThread(req.ThreadID, errorEvent)
		}
		sessionMgr.Update(req.ThreadID, SessionStatusError)
		return fmt.Errorf("agent execution failed: %w", err)
	}

	// Process agent result and emit AG-UI events
	if err := b.processAgentResult(req.ThreadID, req.RunID, result); err != nil {
		return fmt.Errorf("failed to process agent result: %w", err)
	}

	// Emit run finished event (check for nil emitter)
	if emitter != nil {
		finishEvent := RunFinishedBuilder().
			WithThreadID(req.ThreadID).
			WithRunID(req.RunID).
			Build()
		if err := emitter.EmitToThread(req.ThreadID, finishEvent); err != nil {
			return fmt.Errorf("failed to emit run finished event: %w", err)
		}
	}

	sessionMgr.Update(req.ThreadID, SessionStatusFinished)
	return nil
}

// processAgentResult processes the agent result and emits corresponding AG-UI events.
// Note: This is a simplified implementation. The actual fantasy.AgentResult structure
// may have different fields that need to be handled.
func (b *AgentBridge) processAgentResult(threadID, runID string, result *fantasy.AgentResult) error {
	if result == nil {
		return fmt.Errorf("agent result is nil")
	}

	// Note: The actual AgentResult structure from fantasy package may have different fields.
	// This is a placeholder implementation that emits a simple completion event.
	// TODO: Update this when the actual AgentResult structure is known.

	// Emit activity events to show processing
	activityStartEvent := NewEvent(ActivityStart, ActivityStartEvent{
		ActivityID: fmt.Sprintf("activity-%s", runID),
		Name:       "Processing",
	})
	if err := b.eventEmitter.EmitToThread(threadID, activityStartEvent); err != nil {
		return err
	}

	// Emit activity update
	activityUpdateEvent := NewEvent(ActivityUpdate, ActivityUpdateEvent{
		ActivityID: fmt.Sprintf("activity-%s", runID),
		Progress:   intPtr(100),
		Status:     "completed",
	})
	if err := b.eventEmitter.EmitToThread(threadID, activityUpdateEvent); err != nil {
		return err
	}

	// Emit activity end
	activityEndEvent := NewEvent(ActivityEnd, ActivityEndEvent{
		ActivityID: fmt.Sprintf("activity-%s", runID),
	})
	if err := b.eventEmitter.EmitToThread(threadID, activityEndEvent); err != nil {
		return err
	}

	return nil
}

// intPtr returns a pointer to an int.
func intPtr(i int) *int {
	return &i
}

// EventEmitter implementations.

// SimpleEventEmitter is a simple implementation of EventEmitter.
type SimpleEventEmitter struct {
	manager *ConnectionManager
}

// NewSimpleEventEmitter creates a new SimpleEventEmitter.
func NewSimpleEventEmitter(manager *ConnectionManager) *SimpleEventEmitter {
	return &SimpleEventEmitter{
		manager: manager,
	}
}

// Emit emits an event to all connections.
func (e *SimpleEventEmitter) Emit(event Event) error {
	// Check for nil manager
	if e.manager == nil {
		return nil
	}
	conns := e.manager.List()
	for _, conn := range conns {
		select {
		case conn.Events <- event:
		case <-conn.Done:
		default:
		}
	}
	return nil
}

// EmitToThread emits an event to all connections for a specific thread.
func (e *SimpleEventEmitter) EmitToThread(threadID string, event Event) error {
	// Check for nil manager
	if e.manager == nil {
		return nil
	}
	conns := e.manager.GetByThread(threadID)
	for _, conn := range conns {
		select {
		case conn.Events <- event:
		case <-conn.Done:
		default:
		}
	}
	return nil
}

// EmitToConnection emits an event to a specific connection.
func (e *SimpleEventEmitter) EmitToConnection(connID string, event Event) error {
	// Check for nil manager
	if e.manager == nil {
		return fmt.Errorf("connection manager is nil")
	}
	conn, ok := e.manager.Get(connID)
	if !ok {
		return fmt.Errorf("connection %s not found", connID)
	}

	select {
	case conn.Events <- event:
		return nil
	case <-conn.Done:
		return fmt.Errorf("connection %s is closed", connID)
	default:
		return fmt.Errorf("connection %s event channel is full", connID)
	}
}
