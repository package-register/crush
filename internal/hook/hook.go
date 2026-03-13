// Package hook provides a non-intrusive hook system for monitoring and intercepting
// Crush agent lifecycle events. Hooks are optional and can be used for auditing,
// metrics, compliance checking, and other cross-cutting concerns.
package hook

import (
	"context"
	"time"
)

// EventType identifies the type of hook event.
type EventType string

// Hook event types - lifecycle events for agent execution.
const (
	// User message events.
	UserMessageBefore  EventType = "user_message_before"
	UserMessageAfter   EventType = "user_message_after"

	// Assistant response events.
	AssistantResponseBefore EventType = "assistant_response_before"
	AssistantResponseAfter  EventType = "assistant_response_after"

	// Tool call events.
	ToolCallBefore EventType = "tool_call_before"
	ToolCallAfter  EventType = "tool_call_after"

	// Step lifecycle events.
	StepStart EventType = "step_start"
	StepEnd   EventType = "step_end"

	// Error event.
	Error EventType = "error"
)

// HookContext contains contextual information for hook execution.
type HookContext struct {
	// SessionID is the current session identifier.
	SessionID string
	// MessageID is the current message identifier (if applicable).
	MessageID string
	// ToolCallID is the tool call identifier (if applicable).
	ToolCallID string
	// UserID is the user identifier (if available).
	UserID string
	// Metadata contains additional context-specific data.
	Metadata map[string]any
}

// Event represents a hook event with payload.
type Event struct {
	// Type is the event type.
	Type EventType
	// Context contains execution context.
	Context HookContext
	// Timestamp is when the event occurred.
	Timestamp time.Time
	// Data is the event-specific payload (type varies by event type).
	Data any
}

// Hook defines the interface for hook implementations.
// All methods are optional - implement only what you need.
// Returning an error from Before hooks can prevent the operation.
type Hook interface {
	// User message hooks.
	// OnUserMessageBefore is called before processing user input.
	// Returning an error will prevent the message from being processed.
	// The message can be modified by changing the pointed value.
	OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error
	// OnUserMessageAfter is called after user input is processed.
	OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error

	// Assistant response hooks.
	// OnAssistantResponseBefore is called before sending assistant response.
	// Returning an error will prevent the response from being sent.
	// The response can be modified by changing the pointed value.
	OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error
	// OnAssistantResponseAfter is called after assistant response is sent.
	OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error

	// Tool call hooks.
	// OnToolCallBefore is called before executing a tool.
	// Returning an error will prevent the tool from being executed.
	OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error
	// OnToolCallAfter is called after tool execution completes.
	OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error

	// Step lifecycle hooks.
	// OnStepStart is called when an agent step begins.
	OnStepStart(ctx context.Context, hc HookContext, stepType string) error
	// OnStepEnd is called when an agent step completes.
	OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error

	// Error hook.
	// OnError is called when an error occurs during agent execution.
	OnError(ctx context.Context, hc HookContext, err error) error
}

// NoopHook is a hook implementation that does nothing.
// Use it as a base for embedding in custom hooks.
type NoopHook struct{}

// Compile-time check that NoopHook implements Hook.
var _ Hook = (*NoopHook)(nil)

func (h *NoopHook) OnUserMessageBefore(ctx context.Context, hc HookContext, message *string) error {
	return nil
}

func (h *NoopHook) OnUserMessageAfter(ctx context.Context, hc HookContext, message string) error {
	return nil
}

func (h *NoopHook) OnAssistantResponseBefore(ctx context.Context, hc HookContext, response *string) error {
	return nil
}

func (h *NoopHook) OnAssistantResponseAfter(ctx context.Context, hc HookContext, response string) error {
	return nil
}

func (h *NoopHook) OnToolCallBefore(ctx context.Context, hc HookContext, toolName string, input any) error {
	return nil
}

func (h *NoopHook) OnToolCallAfter(ctx context.Context, hc HookContext, toolName string, result any) error {
	return nil
}

func (h *NoopHook) OnStepStart(ctx context.Context, hc HookContext, stepType string) error {
	return nil
}

func (h *NoopHook) OnStepEnd(ctx context.Context, hc HookContext, stepType string, err error) error {
	return nil
}

func (h *NoopHook) OnError(ctx context.Context, hc HookContext, err error) error {
	return nil
}
