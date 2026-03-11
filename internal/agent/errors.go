package agent

import (
	"errors"
	"fmt"
)

var (
	ErrRequestCancelled = errors.New("request canceled by user")
	ErrSessionBusy      = errors.New("session is currently processing another request")
	ErrEmptyPrompt      = errors.New("prompt is empty")
	ErrSessionMissing   = errors.New("session id is missing")
)

// SubAgentError wraps errors from sub-agent execution with context.
type SubAgentError struct {
	Op      string // operation that failed
	Session string // session ID
	Err     error  // underlying error
}

func (e *SubAgentError) Error() string {
	if e.Session != "" {
		return fmt.Sprintf("sub-agent %s failed (session %s): %v", e.Op, e.Session, e.Err)
	}
	return fmt.Sprintf("sub-agent %s failed: %v", e.Op, e.Err)
}

func (e *SubAgentError) Unwrap() error {
	return e.Err
}

// IsSubAgentError checks if err is a SubAgentError.
func IsSubAgentError(err error) bool {
	var subErr *SubAgentError
	return errors.As(err, &subErr)
}

// NewSubAgentError creates a new SubAgentError.
func NewSubAgentError(op, sessionID string, err error) *SubAgentError {
	return &SubAgentError{
		Op:      op,
		Session: sessionID,
		Err:     err,
	}
}
