package agent

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"charm.land/fantasy"
)

// TestSubAgentError tests the SubAgentError type.
func TestSubAgentError(t *testing.T) {
	t.Run("Error message with session", func(t *testing.T) {
		err := NewSubAgentError("test_op", "session-123", errors.New("underlying error"))
		expected := "sub-agent test_op failed (session session-123): underlying error"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("Error message without session", func(t *testing.T) {
		err := NewSubAgentError("test_op", "", errors.New("underlying error"))
		expected := "sub-agent test_op failed: underlying error"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("Unwrap", func(t *testing.T) {
		underlying := errors.New("underlying error")
		err := NewSubAgentError("test_op", "session-123", underlying)
		if !errors.Is(err, underlying) {
			t.Error("expected errors.Is to return true")
		}
	})

	t.Run("IsSubAgentError", func(t *testing.T) {
		err := NewSubAgentError("test_op", "session-123", errors.New("underlying error"))
		if !IsSubAgentError(err) {
			t.Error("expected IsSubAgentError to return true")
		}

		// Test with wrapped error
		wrappedErr := NewSubAgentError("outer", "", err)
		if !IsSubAgentError(wrappedErr) {
			t.Error("expected IsSubAgentError to return true for wrapped error")
		}
	})
}

// TestIsRetryableError tests the retryable error detection logic.
func TestIsRetryableError(t *testing.T) {
	c := &coordinator{}

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "timeout network error",
			err:      &mockNetError{temporary: false, timeout: true},
			expected: true,
		},
		{
			name:     "permanent network error",
			err:      &mockNetError{temporary: false, timeout: false},
			expected: false,
		},
		{
			name:     "HTTP 500 error",
			err:      &fantasy.ProviderError{StatusCode: http.StatusInternalServerError},
			expected: true,
		},
		{
			name:     "HTTP 503 error",
			err:      &fantasy.ProviderError{StatusCode: http.StatusServiceUnavailable},
			expected: true,
		},
		{
			name:     "HTTP 429 error",
			err:      &fantasy.ProviderError{StatusCode: http.StatusTooManyRequests},
			expected: true,
		},
		{
			name:     "HTTP 400 error",
			err:      &fantasy.ProviderError{StatusCode: http.StatusBadRequest},
			expected: false,
		},
		{
			name:     "HTTP 401 error",
			err:      &fantasy.ProviderError{StatusCode: http.StatusUnauthorized},
			expected: false,
		},
		{
			name:     "connection reset error",
			err:      errors.New("connection reset by peer"),
			expected: true,
		},
		{
			name:     "broken pipe error",
			err:      errors.New("broken pipe"),
			expected: true,
		},
		{
			name:     "timeout in error message",
			err:      errors.New("i/o timeout"),
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestFormatSubAgentError tests error formatting for user display.
func TestFormatSubAgentError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "SubAgentError",
			err:      NewSubAgentError("test_op", "session-123", errors.New("underlying error")),
			expected: "Sub-agent failed to test_op: underlying error",
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: "Sub-agent timed out after 10m0s",
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: "Sub-agent was canceled by user",
		},
		{
			name:     "generic error",
			err:      errors.New("something went wrong"),
			expected: "Sub-agent error: something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSubAgentError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// mockNetError is a mock implementation of net.Error for testing.
type mockNetError struct {
	temporary bool
	timeout   bool
}

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Temporary() bool { return e.temporary }
func (e *mockNetError) Timeout() bool   { return e.timeout }

// TestSubAgentTimeoutConstant tests that the timeout constant is reasonable.
func TestSubAgentTimeoutConstant(t *testing.T) {
	if subAgentTimeout < 1*time.Minute {
		t.Error("subAgentTimeout should be at least 1 minute")
	}
	if subAgentTimeout > 30*time.Minute {
		t.Error("subAgentTimeout should not exceed 30 minutes")
	}
}

// TestSubAgentRetryConstants tests that retry constants are reasonable.
func TestSubAgentRetryConstants(t *testing.T) {
	if subAgentMaxRetries < 0 {
		t.Error("subAgentMaxRetries should be non-negative")
	}
	if subAgentMaxRetries > 10 {
		t.Error("subAgentMaxRetries should not exceed 10")
	}
	if subAgentRetryDelay < 100*time.Millisecond {
		t.Error("subAgentRetryDelay should be at least 100ms")
	}
}
