// Package webdav provides a WebDAV client implementation for Crush configuration synchronization.
package webdav

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Common errors
var (
	ErrUnauthorized     = errors.New("webdav: unauthorized")
	ErrNotFound         = errors.New("webdav: resource not found")
	ErrConflict         = errors.New("webdav: resource conflict")
	ErrForbidden        = errors.New("webdav: forbidden")
	ErrTimeout          = errors.New("webdav: request timeout")
	ErrConnectionFailed = errors.New("webdav: connection failed")
	ErrInvalidResponse  = errors.New("webdav: invalid response")
	ErrSyncConflict     = errors.New("webdav: sync conflict detected")
	ErrCancelled        = errors.New("webdav: operation cancelled")
)

// Error represents a WebDAV error with additional context.
type Error struct {
	Op         string // operation name
	Path       string // file path
	StatusCode int    // HTTP status code
	Err        error  // underlying error
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("webdav: %s %s: %v (status: %d)", e.Op, e.Path, e.Err, e.StatusCode)
	}
	return fmt.Sprintf("webdav: %s: %v (status: %d)", e.Op, e.Err, e.StatusCode)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error is retryable.
func (e *Error) IsRetryable() bool {
	if e == nil {
		return false
	}
	switch e.StatusCode {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	if errors.Is(e.Err, ErrTimeout) || errors.Is(e.Err, ErrConnectionFailed) {
		return true
	}
	return false
}

// IsConflict returns true if the error indicates a conflict.
func (e *Error) IsConflict() bool {
	if e == nil {
		return false
	}
	return e.StatusCode == http.StatusConflict ||
		e.StatusCode == http.StatusPreconditionFailed ||
		errors.Is(e.Err, ErrSyncConflict)
}

// newError creates a new WebDAV error.
func newError(op, path string, statusCode int, err error) *Error {
	return &Error{
		Op:         op,
		Path:       path,
		StatusCode: statusCode,
		Err:        err,
	}
}

// wrapError wraps an existing error with WebDAV context.
func wrapError(op, path string, err error) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e
	}
	return &Error{
		Op:   op,
		Path: path,
		Err:  err,
	}
}
