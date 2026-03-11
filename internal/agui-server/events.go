package aguiserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// EventType represents the type of an AG-UI event.
type EventType string

// AG-UI event type constants (17 event types).
const (
	// Lifecycle events.
	RunStarted   EventType = "RUN_STARTED"
	RunFinished  EventType = "RUN_FINISHED"
	RunError     EventType = "RUN_ERROR"
	StepStarted  EventType = "STEP_STARTED"
	StepFinished EventType = "STEP_FINISHED"

	// Text message events.
	TextMessageStart   EventType = "TEXT_MESSAGE_START"
	TextMessageContent EventType = "TEXT_MESSAGE_CONTENT"
	TextMessageEnd     EventType = "TEXT_MESSAGE_END"

	// Tool call events.
	ToolCallStart  EventType = "TOOL_CALL_START"
	ToolCallArgs   EventType = "TOOL_CALL_ARGS"
	ToolCallEnd    EventType = "TOOL_CALL_END"
	ToolCallResult EventType = "TOOL_CALL_RESULT"

	// State management events.
	StateDelta EventType = "STATE_DELTA"

	// Activity events.
	ActivityStart  EventType = "ACTIVITY_START"
	ActivityEnd    EventType = "ACTIVITY_END"
	ActivityUpdate EventType = "ACTIVITY_UPDATE"

	// Special events.
	CustomEvent EventType = "CUSTOM_EVENT"
)

// Event represents an AG-UI protocol event.
type Event struct {
	// Type is the event type.
	Type EventType `json:"type"`
	// Timestamp is the Unix timestamp in milliseconds.
	Timestamp *int64 `json:"timestamp,omitempty"`
	// Data is the event-specific payload.
	Data any `json:"data"`
}

// NewEvent creates a new Event with the current timestamp.
func NewEvent(eventType EventType, data any) Event {
	now := time.Now().UnixMilli()
	return Event{
		Type:      eventType,
		Timestamp: &now,
		Data:      data,
	}
}

// MarshalJSON implements custom JSON marshaling for Event.
func (e Event) MarshalJSON() ([]byte, error) {
	buf := getJSONBuffer()
	defer putJSONBuffer(buf)

	type Alias Event
	if err := json.NewEncoder(buf).Encode(&struct {
		*Alias
	}{
		Alias: (*Alias)(&e),
	}); err != nil {
		return nil, err
	}

	// Copy result and trim trailing newline from json.Encoder
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return bytes.TrimSuffix(result, []byte("\n")), nil
}

// String returns a string representation of the Event.
func (e Event) String() string {
	buf := getJSONBuffer()
	defer putJSONBuffer(buf)

	type Alias Event
	if err := json.NewEncoder(buf).Encode(&struct {
		*Alias
	}{
		Alias: (*Alias)(&e),
	}); err != nil {
		return fmt.Sprintf("Event{Type: %s, Error: %v}", e.Type, err)
	}

	// Trim trailing newline from json.Encoder
	result := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	return string(result)
}

// RunStartedEvent represents a run started event.
type RunStartedEvent struct {
	// ThreadID is the conversation thread identifier.
	ThreadID string `json:"threadId"`
	// RunID is the run identifier.
	RunID string `json:"runId"`
}

// RunFinishedEvent represents a run finished event.
type RunFinishedEvent struct {
	// ThreadID is the conversation thread identifier.
	ThreadID string `json:"threadId"`
	// RunID is the run identifier.
	RunID string `json:"runId"`
}

// RunErrorEvent represents a run error event.
type RunErrorEvent struct {
	// ThreadID is the conversation thread identifier.
	ThreadID string `json:"threadId"`
	// RunID is the run identifier.
	RunID string `json:"runId"`
	// Error is the error message.
	Error string `json:"error"`
}

// StepStartedEvent represents a step started event.
type StepStartedEvent struct {
	// ThreadID is the conversation thread identifier.
	ThreadID string `json:"threadId"`
	// RunID is the run identifier.
	RunID string `json:"runId"`
	// StepID is the step identifier.
	StepID string `json:"stepId"`
	// Type is the step type.
	Type string `json:"type"`
}

// StepFinishedEvent represents a step finished event.
type StepFinishedEvent struct {
	// ThreadID is the conversation thread identifier.
	ThreadID string `json:"threadId"`
	// RunID is the run identifier.
	RunID string `json:"runId"`
	// StepID is the step identifier.
	StepID string `json:"stepId"`
	// Type is the step type.
	Type string `json:"type"`
}

// TextMessageStartEvent represents a text message start event.
type TextMessageStartEvent struct {
	// MessageID is the message identifier.
	MessageID string `json:"messageId"`
}

// TextMessageContentEvent represents a text message content event.
type TextMessageContentEvent struct {
	// MessageID is the message identifier.
	MessageID string `json:"messageId"`
	// Content is the message content delta.
	Content string `json:"content"`
}

// TextMessageEndEvent represents a text message end event.
type TextMessageEndEvent struct {
	// MessageID is the message identifier.
	MessageID string `json:"messageId"`
}

// ToolCallStartEvent represents a tool call start event.
type ToolCallStartEvent struct {
	// ToolCallID is the tool call identifier.
	ToolCallID string `json:"toolCallId"`
	// Name is the tool name.
	Name string `json:"name"`
}

// ToolCallArgsEvent represents a tool call arguments event.
type ToolCallArgsEvent struct {
	// ToolCallID is the tool call identifier.
	ToolCallID string `json:"toolCallId"`
	// Args is the arguments delta.
	Args string `json:"args"`
}

// ToolCallEndEvent represents a tool call end event.
type ToolCallEndEvent struct {
	// ToolCallID is the tool call identifier.
	ToolCallID string `json:"toolCallId"`
}

// ToolCallResultEvent represents a tool call result event.
type ToolCallResultEvent struct {
	// ToolCallID is the tool call identifier.
	ToolCallID string `json:"toolCallId"`
	// Result is the tool call result.
	Result any `json:"result"`
}

// StateDeltaEvent represents a state delta event.
type StateDeltaEvent struct {
	// Delta is the state change.
	Delta map[string]any `json:"delta"`
}

// ActivityStartEvent represents an activity start event.
type ActivityStartEvent struct {
	// ActivityID is the activity identifier.
	ActivityID string `json:"activityId"`
	// Name is the activity name.
	Name string `json:"name"`
}

// ActivityEndEvent represents an activity end event.
type ActivityEndEvent struct {
	// ActivityID is the activity identifier.
	ActivityID string `json:"activityId"`
}

// ActivityUpdateEvent represents an activity update event.
type ActivityUpdateEvent struct {
	// ActivityID is the activity identifier.
	ActivityID string `json:"activityId"`
	// Progress is the progress percentage (0-100).
	Progress *int `json:"progress,omitempty"`
	// Status is the activity status.
	Status string `json:"status,omitempty"`
}

// CustomEventData represents custom event data.
type CustomEventData struct {
	// Type is the custom event type.
	Type string `json:"type"`
	// Payload is the custom payload.
	Payload any `json:"payload"`
}

// EventBuilder provides a fluent interface for building AG-UI events.
type EventBuilder struct {
	eventType EventType
	data      any
}

// NewEventBuilder creates a new EventBuilder for the given event type.
func NewEventBuilder(eventType EventType) *EventBuilder {
	return &EventBuilder{
		eventType: eventType,
	}
}

// WithData sets the event data.
func (b *EventBuilder) WithData(data any) *EventBuilder {
	b.data = data
	return b
}

// Build creates the Event.
func (b *EventBuilder) Build() Event {
	return NewEvent(b.eventType, b.data)
}

// RunStartedBuilder creates a builder for RunStarted events.
func RunStartedBuilder() *RunStartedEventBuilder {
	return &RunStartedEventBuilder{}
}

// RunStartedEventBuilder provides a fluent interface for building RunStarted events.
type RunStartedEventBuilder struct {
	threadID string
	runID    string
}

// WithThreadID sets the thread ID.
func (b *RunStartedEventBuilder) WithThreadID(threadID string) *RunStartedEventBuilder {
	b.threadID = threadID
	return b
}

// WithRunID sets the run ID.
func (b *RunStartedEventBuilder) WithRunID(runID string) *RunStartedEventBuilder {
	b.runID = runID
	return b
}

// Build creates the RunStarted event.
func (b *RunStartedEventBuilder) Build() Event {
	return NewEvent(RunStarted, RunStartedEvent{
		ThreadID: b.threadID,
		RunID:    b.runID,
	})
}

// RunFinishedBuilder creates a builder for RunFinished events.
func RunFinishedBuilder() *RunFinishedEventBuilder {
	return &RunFinishedEventBuilder{}
}

// RunFinishedEventBuilder provides a fluent interface for building RunFinished events.
type RunFinishedEventBuilder struct {
	threadID string
	runID    string
}

// WithThreadID sets the thread ID.
func (b *RunFinishedEventBuilder) WithThreadID(threadID string) *RunFinishedEventBuilder {
	b.threadID = threadID
	return b
}

// WithRunID sets the run ID.
func (b *RunFinishedEventBuilder) WithRunID(runID string) *RunFinishedEventBuilder {
	b.runID = runID
	return b
}

// Build creates the RunFinished event.
func (b *RunFinishedEventBuilder) Build() Event {
	return NewEvent(RunFinished, RunFinishedEvent{
		ThreadID: b.threadID,
		RunID:    b.runID,
	})
}

// RunErrorBuilder creates a builder for RunError events.
func RunErrorBuilder() *RunErrorEventBuilder {
	return &RunErrorEventBuilder{}
}

// RunErrorEventBuilder provides a fluent interface for building RunError events.
type RunErrorEventBuilder struct {
	threadID string
	runID    string
	err      string
}

// WithThreadID sets the thread ID.
func (b *RunErrorEventBuilder) WithThreadID(threadID string) *RunErrorEventBuilder {
	b.threadID = threadID
	return b
}

// WithRunID sets the run ID.
func (b *RunErrorEventBuilder) WithRunID(runID string) *RunErrorEventBuilder {
	b.runID = runID
	return b
}

// WithError sets the error message.
func (b *RunErrorEventBuilder) WithError(err string) *RunErrorEventBuilder {
	b.err = err
	return b
}

// Build creates the RunError event.
func (b *RunErrorEventBuilder) Build() Event {
	return NewEvent(RunError, RunErrorEvent{
		ThreadID: b.threadID,
		RunID:    b.runID,
		Error:    b.err,
	})
}

// TextMessageStartBuilder creates a builder for TextMessageStart events.
func TextMessageStartBuilder() *TextMessageStartEventBuilder {
	return &TextMessageStartEventBuilder{}
}

// TextMessageStartEventBuilder provides a fluent interface for building TextMessageStart events.
type TextMessageStartEventBuilder struct {
	messageID string
}

// WithMessageID sets the message ID.
func (b *TextMessageStartEventBuilder) WithMessageID(messageID string) *TextMessageStartEventBuilder {
	b.messageID = messageID
	return b
}

// Build creates the TextMessageStart event.
func (b *TextMessageStartEventBuilder) Build() Event {
	return NewEvent(TextMessageStart, TextMessageStartEvent{
		MessageID: b.messageID,
	})
}

// TextMessageContentBuilder creates a builder for TextMessageContent events.
func TextMessageContentBuilder() *TextMessageContentEventBuilder {
	return &TextMessageContentEventBuilder{}
}

// TextMessageContentEventBuilder provides a fluent interface for building TextMessageContent events.
type TextMessageContentEventBuilder struct {
	messageID string
	content   string
}

// WithMessageID sets the message ID.
func (b *TextMessageContentEventBuilder) WithMessageID(messageID string) *TextMessageContentEventBuilder {
	b.messageID = messageID
	return b
}

// WithContent sets the content.
func (b *TextMessageContentEventBuilder) WithContent(content string) *TextMessageContentEventBuilder {
	b.content = content
	return b
}

// Build creates the TextMessageContent event.
func (b *TextMessageContentEventBuilder) Build() Event {
	return NewEvent(TextMessageContent, TextMessageContentEvent{
		MessageID: b.messageID,
		Content:   b.content,
	})
}

// TextMessageEndBuilder creates a builder for TextMessageEnd events.
func TextMessageEndBuilder() *TextMessageEndEventBuilder {
	return &TextMessageEndEventBuilder{}
}

// TextMessageEndEventBuilder provides a fluent interface for building TextMessageEnd events.
type TextMessageEndEventBuilder struct {
	messageID string
}

// WithMessageID sets the message ID.
func (b *TextMessageEndEventBuilder) WithMessageID(messageID string) *TextMessageEndEventBuilder {
	b.messageID = messageID
	return b
}

// Build creates the TextMessageEnd event.
func (b *TextMessageEndEventBuilder) Build() Event {
	return NewEvent(TextMessageEnd, TextMessageEndEvent{
		MessageID: b.messageID,
	})
}

// ToolCallStartBuilder creates a builder for ToolCallStart events.
func ToolCallStartBuilder() *ToolCallStartEventBuilder {
	return &ToolCallStartEventBuilder{}
}

// ToolCallStartEventBuilder provides a fluent interface for building ToolCallStart events.
type ToolCallStartEventBuilder struct {
	toolCallID string
	name       string
}

// WithToolCallID sets the tool call ID.
func (b *ToolCallStartEventBuilder) WithToolCallID(toolCallID string) *ToolCallStartEventBuilder {
	b.toolCallID = toolCallID
	return b
}

// WithName sets the tool name.
func (b *ToolCallStartEventBuilder) WithName(name string) *ToolCallStartEventBuilder {
	b.name = name
	return b
}

// Build creates the ToolCallStart event.
func (b *ToolCallStartEventBuilder) Build() Event {
	return NewEvent(ToolCallStart, ToolCallStartEvent{
		ToolCallID: b.toolCallID,
		Name:       b.name,
	})
}

// ToolCallArgsBuilder creates a builder for ToolCallArgs events.
func ToolCallArgsBuilder() *ToolCallArgsEventBuilder {
	return &ToolCallArgsEventBuilder{}
}

// ToolCallArgsEventBuilder provides a fluent interface for building ToolCallArgs events.
type ToolCallArgsEventBuilder struct {
	toolCallID string
	args       string
}

// WithToolCallID sets the tool call ID.
func (b *ToolCallArgsEventBuilder) WithToolCallID(toolCallID string) *ToolCallArgsEventBuilder {
	b.toolCallID = toolCallID
	return b
}

// WithArgs sets the arguments.
func (b *ToolCallArgsEventBuilder) WithArgs(args string) *ToolCallArgsEventBuilder {
	b.args = args
	return b
}

// Build creates the ToolCallArgs event.
func (b *ToolCallArgsEventBuilder) Build() Event {
	return NewEvent(ToolCallArgs, ToolCallArgsEvent{
		ToolCallID: b.toolCallID,
		Args:       b.args,
	})
}

// ToolCallEndBuilder creates a builder for ToolCallEnd events.
func ToolCallEndBuilder() *ToolCallEndEventBuilder {
	return &ToolCallEndEventBuilder{}
}

// ToolCallEndEventBuilder provides a fluent interface for building ToolCallEnd events.
type ToolCallEndEventBuilder struct {
	toolCallID string
}

// WithToolCallID sets the tool call ID.
func (b *ToolCallEndEventBuilder) WithToolCallID(toolCallID string) *ToolCallEndEventBuilder {
	b.toolCallID = toolCallID
	return b
}

// Build creates the ToolCallEnd event.
func (b *ToolCallEndEventBuilder) Build() Event {
	return NewEvent(ToolCallEnd, ToolCallEndEvent{
		ToolCallID: b.toolCallID,
	})
}

// ToolCallResultBuilder creates a builder for ToolCallResult events.
func ToolCallResultBuilder() *ToolCallResultEventBuilder {
	return &ToolCallResultEventBuilder{}
}

// ToolCallResultEventBuilder provides a fluent interface for building ToolCallResult events.
type ToolCallResultEventBuilder struct {
	toolCallID string
	result     any
}

// WithToolCallID sets the tool call ID.
func (b *ToolCallResultEventBuilder) WithToolCallID(toolCallID string) *ToolCallResultEventBuilder {
	b.toolCallID = toolCallID
	return b
}

// WithResult sets the result.
func (b *ToolCallResultEventBuilder) WithResult(result any) *ToolCallResultEventBuilder {
	b.result = result
	return b
}

// Build creates the ToolCallResult event.
func (b *ToolCallResultEventBuilder) Build() Event {
	return NewEvent(ToolCallResult, ToolCallResultEvent{
		ToolCallID: b.toolCallID,
		Result:     b.result,
	})
}

// StateDeltaBuilder creates a builder for StateDelta events.
func StateDeltaBuilder() *StateDeltaEventBuilder {
	return &StateDeltaEventBuilder{}
}

// StateDeltaEventBuilder provides a fluent interface for building StateDelta events.
type StateDeltaEventBuilder struct {
	delta map[string]any
}

// WithDelta sets the state delta.
func (b *StateDeltaEventBuilder) WithDelta(delta map[string]any) *StateDeltaEventBuilder {
	b.delta = delta
	return b
}

// Build creates the StateDelta event.
func (b *StateDeltaEventBuilder) Build() Event {
	return NewEvent(StateDelta, StateDeltaEvent{
		Delta: b.delta,
	})
}
