package aguiserver

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventTypeConstants(t *testing.T) {
	// Test lifecycle events
	if RunStarted != "RUN_STARTED" {
		t.Errorf("Expected RunStarted to be 'RUN_STARTED', got '%s'", RunStarted)
	}
	if RunFinished != "RUN_FINISHED" {
		t.Errorf("Expected RunFinished to be 'RUN_FINISHED', got '%s'", RunFinished)
	}
	if RunError != "RUN_ERROR" {
		t.Errorf("Expected RunError to be 'RUN_ERROR', got '%s'", RunError)
	}
	if StepStarted != "STEP_STARTED" {
		t.Errorf("Expected StepStarted to be 'STEP_STARTED', got '%s'", StepStarted)
	}
	if StepFinished != "STEP_FINISHED" {
		t.Errorf("Expected StepFinished to be 'STEP_FINISHED', got '%s'", StepFinished)
	}

	// Test text message events
	if TextMessageStart != "TEXT_MESSAGE_START" {
		t.Errorf("Expected TextMessageStart to be 'TEXT_MESSAGE_START', got '%s'", TextMessageStart)
	}
	if TextMessageContent != "TEXT_MESSAGE_CONTENT" {
		t.Errorf("Expected TextMessageContent to be 'TEXT_MESSAGE_CONTENT', got '%s'", TextMessageContent)
	}
	if TextMessageEnd != "TEXT_MESSAGE_END" {
		t.Errorf("Expected TextMessageEnd to be 'TEXT_MESSAGE_END', got '%s'", TextMessageEnd)
	}

	// Test tool call events
	if ToolCallStart != "TOOL_CALL_START" {
		t.Errorf("Expected ToolCallStart to be 'TOOL_CALL_START', got '%s'", ToolCallStart)
	}
	if ToolCallArgs != "TOOL_CALL_ARGS" {
		t.Errorf("Expected ToolCallArgs to be 'TOOL_CALL_ARGS', got '%s'", ToolCallArgs)
	}
	if ToolCallEnd != "TOOL_CALL_END" {
		t.Errorf("Expected ToolCallEnd to be 'TOOL_CALL_END', got '%s'", ToolCallEnd)
	}
	if ToolCallResult != "TOOL_CALL_RESULT" {
		t.Errorf("Expected ToolCallResult to be 'TOOL_CALL_RESULT', got '%s'", ToolCallResult)
	}

	// Test state management events
	if StateDelta != "STATE_DELTA" {
		t.Errorf("Expected StateDelta to be 'STATE_DELTA', got '%s'", StateDelta)
	}

	// Test activity events
	if ActivityStart != "ACTIVITY_START" {
		t.Errorf("Expected ActivityStart to be 'ACTIVITY_START', got '%s'", ActivityStart)
	}
	if ActivityEnd != "ACTIVITY_END" {
		t.Errorf("Expected ActivityEnd to be 'ACTIVITY_END', got '%s'", ActivityEnd)
	}
	if ActivityUpdate != "ACTIVITY_UPDATE" {
		t.Errorf("Expected ActivityUpdate to be 'ACTIVITY_UPDATE', got '%s'", ActivityUpdate)
	}

	// Test special events
	if CustomEvent != "CUSTOM_EVENT" {
		t.Errorf("Expected CustomEvent to be 'CUSTOM_EVENT', got '%s'", CustomEvent)
	}
}

func TestAllEventTypesCount(t *testing.T) {
	// Count all event types defined
	eventTypes := []EventType{
		// Lifecycle
		RunStarted, RunFinished, RunError, StepStarted, StepFinished,
		// Text message
		TextMessageStart, TextMessageContent, TextMessageEnd,
		// Tool call
		ToolCallStart, ToolCallArgs, ToolCallEnd, ToolCallResult,
		// State
		StateDelta,
		// Activity
		ActivityStart, ActivityEnd, ActivityUpdate,
		// Special
		CustomEvent,
	}

	if len(eventTypes) != 17 {
		t.Errorf("Expected 17 event types, got %d", len(eventTypes))
	}
}

func TestEventCreation(t *testing.T) {
	data := map[string]string{"key": "value"}
	event := NewEvent(RunStarted, data)

	if event.Type != RunStarted {
		t.Errorf("Expected Type to be 'RunStarted', got '%s'", event.Type)
	}
	if event.Timestamp == nil {
		t.Error("Expected Timestamp to be set")
	}
	if event.Data == nil {
		t.Error("Expected Data to be set")
	}
}

func TestEventSerialization(t *testing.T) {
	data := RunStartedEvent{
		ThreadID: "thread-123",
		RunID:    "run-456",
	}
	event := NewEvent(RunStarted, data)

	jsonData, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	var unmarshaled Event
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	if unmarshaled.Type != event.Type {
		t.Errorf("Expected Type '%s', got '%s'", event.Type, unmarshaled.Type)
	}
	if unmarshaled.Timestamp == nil {
		t.Error("Expected Timestamp to be set after unmarshal")
	}
}

func TestEventString(t *testing.T) {
	data := map[string]string{"key": "value"}
	event := NewEvent(CustomEvent, data)

	str := event.String()
	if str == "" {
		t.Error("Expected String() to return non-empty string")
	}
}

func TestRunStartedEventBuilder(t *testing.T) {
	event := RunStartedBuilder().
		WithThreadID("thread-123").
		WithRunID("run-456").
		Build()

	if event.Type != RunStarted {
		t.Errorf("Expected Type to be 'RunStarted', got '%s'", event.Type)
	}

	runStartedData, ok := event.Data.(RunStartedEvent)
	if !ok {
		t.Fatal("Expected Data to be RunStartedEvent")
	}
	if runStartedData.ThreadID != "thread-123" {
		t.Errorf("Expected ThreadID 'thread-123', got '%s'", runStartedData.ThreadID)
	}
	if runStartedData.RunID != "run-456" {
		t.Errorf("Expected RunID 'run-456', got '%s'", runStartedData.RunID)
	}
}

func TestRunFinishedEventBuilder(t *testing.T) {
	event := RunFinishedBuilder().
		WithThreadID("thread-123").
		WithRunID("run-456").
		Build()

	if event.Type != RunFinished {
		t.Errorf("Expected Type to be 'RunFinished', got '%s'", event.Type)
	}

	runFinishedData, ok := event.Data.(RunFinishedEvent)
	if !ok {
		t.Fatal("Expected Data to be RunFinishedEvent")
	}
	if runFinishedData.ThreadID != "thread-123" {
		t.Errorf("Expected ThreadID 'thread-123', got '%s'", runFinishedData.ThreadID)
	}
	if runFinishedData.RunID != "run-456" {
		t.Errorf("Expected RunID 'run-456', got '%s'", runFinishedData.RunID)
	}
}

func TestRunErrorEventBuilder(t *testing.T) {
	event := RunErrorBuilder().
		WithThreadID("thread-123").
		WithRunID("run-456").
		WithError("something went wrong").
		Build()

	if event.Type != RunError {
		t.Errorf("Expected Type to be 'RunError', got '%s'", event.Type)
	}

	runErrorData, ok := event.Data.(RunErrorEvent)
	if !ok {
		t.Fatal("Expected Data to be RunErrorEvent")
	}
	if runErrorData.ThreadID != "thread-123" {
		t.Errorf("Expected ThreadID 'thread-123', got '%s'", runErrorData.ThreadID)
	}
	if runErrorData.RunID != "run-456" {
		t.Errorf("Expected RunID 'run-456', got '%s'", runErrorData.RunID)
	}
	if runErrorData.Error != "something went wrong" {
		t.Errorf("Expected Error 'something went wrong', got '%s'", runErrorData.Error)
	}
}

func TestTextMessageStartEventBuilder(t *testing.T) {
	event := TextMessageStartBuilder().
		WithMessageID("msg-123").
		Build()

	if event.Type != TextMessageStart {
		t.Errorf("Expected Type to be 'TextMessageStart', got '%s'", event.Type)
	}

	textStartData, ok := event.Data.(TextMessageStartEvent)
	if !ok {
		t.Fatal("Expected Data to be TextMessageStartEvent")
	}
	if textStartData.MessageID != "msg-123" {
		t.Errorf("Expected MessageID 'msg-123', got '%s'", textStartData.MessageID)
	}
}

func TestTextMessageContentEventBuilder(t *testing.T) {
	event := TextMessageContentBuilder().
		WithMessageID("msg-123").
		WithContent("Hello, world!").
		Build()

	if event.Type != TextMessageContent {
		t.Errorf("Expected Type to be 'TextMessageContent', got '%s'", event.Type)
	}

	textContentData, ok := event.Data.(TextMessageContentEvent)
	if !ok {
		t.Fatal("Expected Data to be TextMessageContentEvent")
	}
	if textContentData.MessageID != "msg-123" {
		t.Errorf("Expected MessageID 'msg-123', got '%s'", textContentData.MessageID)
	}
	if textContentData.Content != "Hello, world!" {
		t.Errorf("Expected Content 'Hello, world!', got '%s'", textContentData.Content)
	}
}

func TestTextMessageEndEventBuilder(t *testing.T) {
	event := TextMessageEndBuilder().
		WithMessageID("msg-123").
		Build()

	if event.Type != TextMessageEnd {
		t.Errorf("Expected Type to be 'TextMessageEnd', got '%s'", event.Type)
	}

	textEndData, ok := event.Data.(TextMessageEndEvent)
	if !ok {
		t.Fatal("Expected Data to be TextMessageEndEvent")
	}
	if textEndData.MessageID != "msg-123" {
		t.Errorf("Expected MessageID 'msg-123', got '%s'", textEndData.MessageID)
	}
}

func TestToolCallStartEventBuilder(t *testing.T) {
	event := ToolCallStartBuilder().
		WithToolCallID("tool-call-123").
		WithName("bash").
		Build()

	if event.Type != ToolCallStart {
		t.Errorf("Expected Type to be 'ToolCallStart', got '%s'", event.Type)
	}

	toolStartData, ok := event.Data.(ToolCallStartEvent)
	if !ok {
		t.Fatal("Expected Data to be ToolCallStartEvent")
	}
	if toolStartData.ToolCallID != "tool-call-123" {
		t.Errorf("Expected ToolCallID 'tool-call-123', got '%s'", toolStartData.ToolCallID)
	}
	if toolStartData.Name != "bash" {
		t.Errorf("Expected Name 'bash', got '%s'", toolStartData.Name)
	}
}

func TestToolCallArgsEventBuilder(t *testing.T) {
	event := ToolCallArgsBuilder().
		WithToolCallID("tool-call-123").
		WithArgs(`{"command": "ls -la"}`).
		Build()

	if event.Type != ToolCallArgs {
		t.Errorf("Expected Type to be 'ToolCallArgs', got '%s'", event.Type)
	}

	toolArgsData, ok := event.Data.(ToolCallArgsEvent)
	if !ok {
		t.Fatal("Expected Data to be ToolCallArgsEvent")
	}
	if toolArgsData.ToolCallID != "tool-call-123" {
		t.Errorf("Expected ToolCallID 'tool-call-123', got '%s'", toolArgsData.ToolCallID)
	}
	if toolArgsData.Args != `{"command": "ls -la"}` {
		t.Errorf("Expected Args '{\"command\": \"ls -la\"}', got '%s'", toolArgsData.Args)
	}
}

func TestToolCallEndEventBuilder(t *testing.T) {
	event := ToolCallEndBuilder().
		WithToolCallID("tool-call-123").
		Build()

	if event.Type != ToolCallEnd {
		t.Errorf("Expected Type to be 'ToolCallEnd', got '%s'", event.Type)
	}

	toolEndData, ok := event.Data.(ToolCallEndEvent)
	if !ok {
		t.Fatal("Expected Data to be ToolCallEndEvent")
	}
	if toolEndData.ToolCallID != "tool-call-123" {
		t.Errorf("Expected ToolCallID 'tool-call-123', got '%s'", toolEndData.ToolCallID)
	}
}

func TestToolCallResultEventBuilder(t *testing.T) {
	result := map[string]string{"output": "file1.txt\nfile2.txt"}
	event := ToolCallResultBuilder().
		WithToolCallID("tool-call-123").
		WithResult(result).
		Build()

	if event.Type != ToolCallResult {
		t.Errorf("Expected Type to be 'ToolCallResult', got '%s'", event.Type)
	}

	toolResultData, ok := event.Data.(ToolCallResultEvent)
	if !ok {
		t.Fatal("Expected Data to be ToolCallResultEvent")
	}
	if toolResultData.ToolCallID != "tool-call-123" {
		t.Errorf("Expected ToolCallID 'tool-call-123', got '%s'", toolResultData.ToolCallID)
	}
	if toolResultData.Result == nil {
		t.Error("Expected Result to be set")
	}
}

func TestStateDeltaEventBuilder(t *testing.T) {
	delta := map[string]any{
		"key1": "value1",
		"key2": 42,
	}
	event := StateDeltaBuilder().
		WithDelta(delta).
		Build()

	if event.Type != StateDelta {
		t.Errorf("Expected Type to be 'StateDelta', got '%s'", event.Type)
	}

	stateDeltaData, ok := event.Data.(StateDeltaEvent)
	if !ok {
		t.Fatal("Expected Data to be StateDeltaEvent")
	}
	if len(stateDeltaData.Delta) != 2 {
		t.Errorf("Expected Delta to have 2 entries, got %d", len(stateDeltaData.Delta))
	}
}

func TestGenericEventBuilder(t *testing.T) {
	data := CustomEventData{
		Type:    "custom_type",
		Payload: map[string]string{"key": "value"},
	}
	event := NewEventBuilder(CustomEvent).
		WithData(data).
		Build()

	if event.Type != CustomEvent {
		t.Errorf("Expected Type to be 'CustomEvent', got '%s'", event.Type)
	}

	customData, ok := event.Data.(CustomEventData)
	if !ok {
		t.Fatal("Expected Data to be CustomEventData")
	}
	if customData.Type != "custom_type" {
		t.Errorf("Expected Type 'custom_type', got '%s'", customData.Type)
	}
}

func TestEventTypesAreUnique(t *testing.T) {
	eventTypes := map[EventType]bool{
		RunStarted:         true,
		RunFinished:        true,
		RunError:           true,
		StepStarted:        true,
		StepFinished:       true,
		TextMessageStart:   true,
		TextMessageContent: true,
		TextMessageEnd:     true,
		ToolCallStart:      true,
		ToolCallArgs:       true,
		ToolCallEnd:        true,
		ToolCallResult:     true,
		StateDelta:         true,
		ActivityStart:      true,
		ActivityEnd:        true,
		ActivityUpdate:     true,
		CustomEvent:        true,
	}

	if len(eventTypes) != 17 {
		t.Errorf("Expected 17 unique event types, got %d", len(eventTypes))
	}
}

func TestEventString_AllEventTypes(t *testing.T) {
	tests := []struct {
		name string
		typ  EventType
		data interface{}
	}{
		{"RunStarted", RunStarted, RunStartedEvent{}},
		{"RunFinished", RunFinished, RunFinishedEvent{}},
		{"RunError", RunError, RunErrorEvent{}},
		{"StepStarted", StepStarted, StepStartedEvent{}},
		{"StepFinished", StepFinished, StepFinishedEvent{}},
		{"TextMessageStart", TextMessageStart, TextMessageStartEvent{}},
		{"TextMessageContent", TextMessageContent, TextMessageContentEvent{}},
		{"TextMessageEnd", TextMessageEnd, TextMessageEndEvent{}},
		{"ToolCallStart", ToolCallStart, ToolCallStartEvent{}},
		{"ToolCallArgs", ToolCallArgs, ToolCallArgsEvent{}},
		{"ToolCallEnd", ToolCallEnd, ToolCallEndEvent{}},
		{"ToolCallResult", ToolCallResult, ToolCallResultEvent{}},
		{"StateDelta", StateDelta, StateDeltaEvent{}},
		{"ActivityStart", ActivityStart, ActivityStartEvent{}},
		{"ActivityEnd", ActivityEnd, ActivityEndEvent{}},
		{"ActivityUpdate", ActivityUpdate, ActivityUpdateEvent{}},
		{"CustomEvent", CustomEvent, CustomEventData{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewEvent(tt.typ, tt.data)
			str := event.String()
			if str == "" {
				t.Error("Expected non-empty string representation")
			}
		})
	}
}

func TestEvent_MarshalJSON_Error(t *testing.T) {
	// Create event with unmarshalable data
	event := Event{
		Type:      CustomEvent,
		Timestamp: int64Ptr(time.Now().Unix()),
		Data:      make(chan int), // Cannot be marshaled
	}

	_, err := json.Marshal(event)
	if err == nil {
		t.Error("Expected error for unmarshalable data")
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}

// Table-driven tests for event builders
func TestEventBuilders_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		buildFunc  func() Event
		expectType EventType
	}{
		{
			name: "RunStartedBuilder",
			buildFunc: func() Event {
				return RunStartedBuilder().
					WithThreadID("t1").
					WithRunID("r1").
					Build()
			},
			expectType: RunStarted,
		},
		{
			name: "RunFinishedBuilder",
			buildFunc: func() Event {
				return RunFinishedBuilder().
					WithThreadID("t1").
					WithRunID("r1").
					Build()
			},
			expectType: RunFinished,
		},
		{
			name: "RunErrorBuilder",
			buildFunc: func() Event {
				return RunErrorBuilder().
					WithThreadID("t1").
					WithRunID("r1").
					WithError("error").
					Build()
			},
			expectType: RunError,
		},
		{
			name: "TextMessageStartBuilder",
			buildFunc: func() Event {
				return TextMessageStartBuilder().
					WithMessageID("m1").
					Build()
			},
			expectType: TextMessageStart,
		},
		{
			name: "TextMessageContentBuilder",
			buildFunc: func() Event {
				return TextMessageContentBuilder().
					WithMessageID("m1").
					WithContent("hello").
					Build()
			},
			expectType: TextMessageContent,
		},
		{
			name: "TextMessageEndBuilder",
			buildFunc: func() Event {
				return TextMessageEndBuilder().
					WithMessageID("m1").
					Build()
			},
			expectType: TextMessageEnd,
		},
		{
			name: "ToolCallStartBuilder",
			buildFunc: func() Event {
				return ToolCallStartBuilder().
					WithToolCallID("tc1").
					WithName("tool").
					Build()
			},
			expectType: ToolCallStart,
		},
		{
			name: "ToolCallArgsBuilder",
			buildFunc: func() Event {
				return ToolCallArgsBuilder().
					WithToolCallID("tc1").
					WithArgs("{}").
					Build()
			},
			expectType: ToolCallArgs,
		},
		{
			name: "ToolCallEndBuilder",
			buildFunc: func() Event {
				return ToolCallEndBuilder().
					WithToolCallID("tc1").
					Build()
			},
			expectType: ToolCallEnd,
		},
		{
			name: "ToolCallResultBuilder",
			buildFunc: func() Event {
				return ToolCallResultBuilder().
					WithToolCallID("tc1").
					WithResult(map[string]string{"key": "value"}).
					Build()
			},
			expectType: ToolCallResult,
		},
		{
			name: "StateDeltaBuilder",
			buildFunc: func() Event {
				return StateDeltaBuilder().
					WithDelta(map[string]any{"key": "value"}).
					Build()
			},
			expectType: StateDelta,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tt.buildFunc()
			if event.Type != tt.expectType {
				t.Errorf("Expected type %s, got %s", tt.expectType, event.Type)
			}
			if event.Timestamp == nil {
				t.Error("Expected timestamp to be set")
			}
		})
	}
}

func TestNewEventBuilder_ChainMultipleData(t *testing.T) {
	data1 := map[string]string{"key1": "value1"}
	data2 := map[string]string{"key2": "value2"}

	event := NewEventBuilder(CustomEvent).
		WithData(data1).
		WithData(data2).
		Build()

	// Last data should win
	resultData, ok := event.Data.(map[string]string)
	if !ok {
		t.Fatal("Expected map[string]string data")
	}
	if resultData["key2"] != "value2" {
		t.Error("Expected last data to be used")
	}
}

func TestEventSerialization_WithNilTimestamp(t *testing.T) {
	event := Event{
		Type:      RunStarted,
		Timestamp: nil,
		Data:      RunStartedEvent{ThreadID: "t1", RunID: "r1"},
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	var unmarshaled Event
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	if unmarshaled.Type != event.Type {
		t.Errorf("Expected Type %s, got %s", event.Type, unmarshaled.Type)
	}
}

// Benchmark tests for events
func BenchmarkEventCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewEvent(RunStarted, RunStartedEvent{
			ThreadID: "thread-123",
			RunID:    "run-456",
		})
	}
}

func BenchmarkEventBuilder(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RunStartedBuilder().
			WithThreadID("thread-123").
			WithRunID("run-456").
			Build()
	}
}
