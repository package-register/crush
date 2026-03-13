// Package aguiserver implements the AG-UI Protocol server for Crush.
package aguiserver

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

// aguiCoordinator wraps an agent.Coordinator and ensures crush sessions exist
// for AG-UI threadIDs before delegating to the real coordinator. AG-UI clients
// send threadIDs (e.g. "demo-thread") that may not exist in crush's session DB;
// this wrapper creates them on demand. AG-UI sessions are auto-approved for
// permissions so the agent does not block waiting for TUI permission dialogs.
type aguiCoordinator struct {
	coordinator agent.Coordinator
	sessions    session.Service
	permissions permission.Service
}

// NewAGUICoordinator returns a Coordinator that creates crush sessions for
// AG-UI threadIDs when they don't exist. If permissions is non-nil, new sessions
// are auto-approved so tools (bash, edit, etc.) run without prompting.
func NewAGUICoordinator(coordinator agent.Coordinator, sessions session.Service, permissions permission.Service) agent.Coordinator {
	return &aguiCoordinator{
		coordinator: coordinator,
		sessions:    sessions,
		permissions: permissions,
	}
}

// getOrCreateSession ensures a crush session exists for the given threadID.
// If it doesn't exist, creates one with that ID.
func (c *aguiCoordinator) getOrCreateSession(ctx context.Context, threadID string) error {
	_, err := c.sessions.Get(ctx, threadID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	// Session doesn't exist; create it with the threadID as ID
	title := "AG-UI: " + threadID
	if len(threadID) > 32 {
		title = "AG-UI: " + threadID[:32] + "..."
	}
	_, err = c.sessions.CreateWithID(ctx, threadID, title)
	if err != nil {
		// Another goroutine may have created it; try get again
		_, getErr := c.sessions.Get(ctx, threadID)
		if getErr == nil {
			return nil
		}
		slog.Debug("AG-UI: failed to create session, retry get failed", "thread_id", threadID, "err", err)
		return err
	}
	return nil
}

// Run implements agent.Coordinator. It ensures a crush session exists for
// the given sessionID (AG-UI threadID) before delegating.
func (c *aguiCoordinator) Run(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	if err := c.getOrCreateSession(ctx, sessionID); err != nil {
		return nil, err
	}
	// Auto-approve permissions for every AG-UI run (idempotent; needed after app restart)
	if c.permissions != nil {
		c.permissions.AutoApproveSession(sessionID)
	}
	return c.coordinator.Run(ctx, sessionID, prompt, attachments...)
}

// Cancel delegates to the underlying coordinator.
func (c *aguiCoordinator) Cancel(sessionID string) {
	c.coordinator.Cancel(sessionID)
}

// CancelAll delegates to the underlying coordinator.
func (c *aguiCoordinator) CancelAll() {
	c.coordinator.CancelAll()
}

// IsSessionBusy delegates to the underlying coordinator.
func (c *aguiCoordinator) IsSessionBusy(sessionID string) bool {
	return c.coordinator.IsSessionBusy(sessionID)
}

// IsBusy delegates to the underlying coordinator.
func (c *aguiCoordinator) IsBusy() bool {
	return c.coordinator.IsBusy()
}

// QueuedPrompts delegates to the underlying coordinator.
func (c *aguiCoordinator) QueuedPrompts(sessionID string) int {
	return c.coordinator.QueuedPrompts(sessionID)
}

// QueuedPromptsList delegates to the underlying coordinator.
func (c *aguiCoordinator) QueuedPromptsList(sessionID string) []string {
	return c.coordinator.QueuedPromptsList(sessionID)
}

// ClearQueue delegates to the underlying coordinator.
func (c *aguiCoordinator) ClearQueue(sessionID string) {
	c.coordinator.ClearQueue(sessionID)
}

// Summarize delegates to the underlying coordinator.
func (c *aguiCoordinator) Summarize(ctx context.Context, sessionID string) error {
	return c.coordinator.Summarize(ctx, sessionID)
}

// Model delegates to the underlying coordinator.
func (c *aguiCoordinator) Model() agent.Model {
	return c.coordinator.Model()
}

// UpdateModels delegates to the underlying coordinator.
func (c *aguiCoordinator) UpdateModels(ctx context.Context) error {
	return c.coordinator.UpdateModels(ctx)
}
