package aguiserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestHistoryService_Append(t *testing.T) {
	hs := NewHistoryService()

	threadID := "test-thread-1"
	event := NewEvent(TextMessageStart, TextMessageStartEvent{MessageID: "msg-1"})

	entry := HistoryEntry{
		ID:        "entry-1",
		ThreadID:  threadID,
		RunID:     "run-1",
		Event:     event,
		Timestamp: time.Now().UnixMilli(),
	}

	hs.Append(threadID, entry)

	// Verify entry was added
	entries, total := hs.GetThreadHistory(threadID, 10, 0)
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID != "entry-1" {
		t.Errorf("Expected entry ID 'entry-1', got '%s'", entries[0].ID)
	}
}

func TestHistoryService_AppendEmptyThreadID(t *testing.T) {
	hs := NewHistoryService()

	event := NewEvent(TextMessageStart, TextMessageStartEvent{MessageID: "msg-1"})
	entry := HistoryEntry{
		ID:        "entry-1",
		ThreadID:  "",
		RunID:     "run-1",
		Event:     event,
		Timestamp: time.Now().UnixMilli(),
	}

	// Should not panic or add entry
	hs.Append("", entry)

	entries, total := hs.GetThreadHistory("", 10, 0)
	if total != 0 {
		t.Errorf("Expected total 0 for empty threadID, got %d", total)
	}
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for empty threadID, got %d", len(entries))
	}
}

func TestHistoryService_GetThreadHistory_Pagination(t *testing.T) {
	hs := NewHistoryService()
	threadID := "test-thread-pagination"

	// Add 10 entries
	for i := range 10 {
		event := NewEvent(TextMessageStart, TextMessageStartEvent{MessageID: "msg-" + strconv.Itoa(i)})
		entry := HistoryEntry{
			ID:        "entry-" + strconv.Itoa(i),
			ThreadID:  threadID,
			RunID:     "run-1",
			Event:     event,
			Timestamp: time.Now().UnixMilli() + int64(i),
		}
		hs.Append(threadID, entry)
	}

	// Test limit
	entries, total := hs.GetThreadHistory(threadID, 5, 0)
	if total != 10 {
		t.Errorf("Expected total 10, got %d", total)
	}
	if len(entries) != 5 {
		t.Errorf("Expected 5 entries with limit 5, got %d", len(entries))
	}

	// Test offset
	entries, total = hs.GetThreadHistory(threadID, 5, 5)
	if total != 10 {
		t.Errorf("Expected total 10, got %d", total)
	}
	if len(entries) != 5 {
		t.Errorf("Expected 5 entries with offset 5, got %d", len(entries))
	}
	if entries[0].ID != "entry-5" {
		t.Errorf("Expected first entry ID 'entry-5', got '%s'", entries[0].ID)
	}

	// Test offset beyond total
	entries, total = hs.GetThreadHistory(threadID, 5, 20)
	if total != 10 {
		t.Errorf("Expected total 10, got %d", total)
	}
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries with offset beyond total, got %d", len(entries))
	}
}

func TestHistoryService_GetThreadHistory_EmptyHistory(t *testing.T) {
	hs := NewHistoryService()

	entries, total := hs.GetThreadHistory("non-existent-thread", 10, 0)
	if total != 0 {
		t.Errorf("Expected total 0 for non-existent thread, got %d", total)
	}
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for non-existent thread, got %d", len(entries))
	}
}

func TestHistoryService_GetThreadHistory_DefaultLimit(t *testing.T) {
	hs := NewHistoryService()
	threadID := "test-thread-default-limit"

	// Add 100 entries
	for i := range 100 {
		event := NewEvent(TextMessageStart, TextMessageStartEvent{MessageID: "msg-" + strconv.Itoa(i)})
		entry := HistoryEntry{
			ID:        "entry-" + strconv.Itoa(i),
			ThreadID:  threadID,
			RunID:     "run-1",
			Event:     event,
			Timestamp: time.Now().UnixMilli() + int64(i),
		}
		hs.Append(threadID, entry)
	}

	// Test default limit (50)
	entries, total := hs.GetThreadHistory(threadID, 0, 0)
	if total != 100 {
		t.Errorf("Expected total 100, got %d", total)
	}
	if len(entries) != 50 {
		t.Errorf("Expected 50 entries with default limit, got %d", len(entries))
	}

	// Test limit > 100 (should be capped at 100)
	entries, total = hs.GetThreadHistory(threadID, 150, 0)
	if len(entries) != 100 {
		t.Errorf("Expected 100 entries with limit 150 (capped), got %d", len(entries))
	}
}

func TestHistoryService_Clear(t *testing.T) {
	hs := NewHistoryService()
	threadID := "test-thread-clear"

	// Add entries
	for i := range 5 {
		event := NewEvent(TextMessageStart, TextMessageStartEvent{MessageID: "msg-" + strconv.Itoa(i)})
		entry := HistoryEntry{
			ID:        "entry-" + strconv.Itoa(i),
			ThreadID:  threadID,
			RunID:     "run-1",
			Event:     event,
			Timestamp: time.Now().UnixMilli() + int64(i),
		}
		hs.Append(threadID, entry)
	}

	// Verify entries exist
	entries, total := hs.GetThreadHistory(threadID, 10, 0)
	if total != 5 {
		t.Errorf("Expected total 5 before clear, got %d", total)
	}

	// Clear history
	hs.Clear(threadID)

	// Verify entries are cleared
	entries, total = hs.GetThreadHistory(threadID, 10, 0)
	if total != 0 {
		t.Errorf("Expected total 0 after clear, got %d", total)
	}
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", len(entries))
	}
}

func TestHistoryService_ClearEmptyThreadID(t *testing.T) {
	hs := NewHistoryService()

	// Should not panic
	hs.Clear("")
}

func TestHistoryService_ChronologicalOrder(t *testing.T) {
	hs := NewHistoryService()
	threadID := "test-thread-order"

	// Add entries with different timestamps (out of order)
	timestamps := []int64{3000, 1000, 2000, 5000, 4000}
	for i, ts := range timestamps {
		event := NewEvent(TextMessageStart, TextMessageStartEvent{MessageID: "msg-" + strconv.Itoa(i)})
		entry := HistoryEntry{
			ID:        "entry-" + strconv.Itoa(i),
			ThreadID:  threadID,
			RunID:     "run-1",
			Event:     event,
			Timestamp: ts,
		}
		hs.Append(threadID, entry)
	}

	// Verify entries are returned in chronological order
	entries, _ := hs.GetThreadHistory(threadID, 10, 0)
	expectedOrder := []int64{1000, 2000, 3000, 4000, 5000}
	for i, entry := range entries {
		if entry.Timestamp != expectedOrder[i] {
			t.Errorf("Expected timestamp %d at index %d, got %d", expectedOrder[i], i, entry.Timestamp)
		}
	}
}

func TestHandleHistory_GET(t *testing.T) {
	hs := NewHistoryService()
	handler := &Handler{
		historyService: hs,
	}

	// Add some history
	threadID := "test-thread-handler"
	for i := range 3 {
		event := NewEvent(TextMessageStart, TextMessageStartEvent{MessageID: "msg-" + strconv.Itoa(i)})
		entry := HistoryEntry{
			ID:        "entry-" + strconv.Itoa(i),
			ThreadID:  threadID,
			RunID:     "run-1",
			Event:     event,
			Timestamp: time.Now().UnixMilli() + int64(i),
		}
		hs.Append(threadID, entry)
	}

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/agui/history?threadId="+threadID+"&limit=10&offset=0", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	// Check status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected content-type 'text/event-stream', got '%s'", contentType)
	}

	// Check SSE events
	body := w.Body.String()
	if body == "" {
		t.Error("Expected non-empty response body")
	}

	// Verify RUN_STARTED event is present
	if !containsString(body, "RUN_STARTED") {
		t.Error("Expected RUN_STARTED event in response")
	}

	// Verify MESSAGES_SNAPSHOT event is present
	if !containsString(body, "MESSAGES_SNAPSHOT") {
		t.Error("Expected MESSAGES_SNAPSHOT event in response")
	}

	// Verify RUN_FINISHED event is present
	if !containsString(body, "RUN_FINISHED") {
		t.Error("Expected RUN_FINISHED event in response")
	}
}

func TestHandleHistory_OPTIONS(t *testing.T) {
	handler := &Handler{}

	req := httptest.NewRequest(http.MethodOptions, "/agui/history", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header Access-Control-Allow-Origin: *")
	}
	if w.Header().Get("Access-Control-Allow-Methods") != http.MethodGet {
		t.Error("Expected CORS header Access-Control-Allow-Methods: GET")
	}
}

func TestHandleHistory_WrongMethod(t *testing.T) {
	handler := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/agui/history", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleHistory_MissingThreadID(t *testing.T) {
	handler := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/agui/history", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleHistory_WithPagination(t *testing.T) {
	hs := NewHistoryService()
	handler := &Handler{
		historyService: hs,
	}

	threadID := "test-thread-pagination-handler"
	for i := range 10 {
		event := NewEvent(TextMessageStart, TextMessageStartEvent{MessageID: "msg-" + strconv.Itoa(i)})
		entry := HistoryEntry{
			ID:        "entry-" + strconv.Itoa(i),
			ThreadID:  threadID,
			RunID:     "run-1",
			Event:     event,
			Timestamp: time.Now().UnixMilli() + int64(i),
		}
		hs.Append(threadID, entry)
	}

	// Test with limit=5
	req := httptest.NewRequest(http.MethodGet, "/agui/history?threadId="+threadID+"&limit=5&offset=0", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	// Should contain 5 messages in the snapshot
	if !containsString(body, `"total":10`) {
		t.Error("Expected total 10 in response")
	}
}

func TestHandleHistory_EmptyHistory(t *testing.T) {
	hs := NewHistoryService()
	handler := &Handler{
		historyService: hs,
	}

	threadID := "empty-thread"
	req := httptest.NewRequest(http.MethodGet, "/agui/history?threadId="+threadID, nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	// Should contain empty messages array
	if !containsString(body, `"messages":[]`) {
		t.Error("Expected empty messages array in response")
	}
	if !containsString(body, `"total":0`) {
		t.Error("Expected total 0 in response")
	}
}

func TestHistoryEntry_MarshalJSON(t *testing.T) {
	event := NewEvent(TextMessageStart, TextMessageStartEvent{MessageID: "msg-1"})
	entry := HistoryEntry{
		ID:        "entry-1",
		ThreadID:  "thread-1",
		RunID:     "run-1",
		Event:     event,
		Timestamp: 1234567890,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal HistoryEntry: %v", err)
	}

	// Verify JSON contains expected fields
	jsonStr := string(data)
	if !containsString(jsonStr, `"id":"entry-1"`) {
		t.Error("Expected 'id' field in JSON")
	}
	if !containsString(jsonStr, `"threadId":"thread-1"`) {
		t.Error("Expected 'threadId' field in JSON")
	}
	if !containsString(jsonStr, `"runId":"run-1"`) {
		t.Error("Expected 'runId' field in JSON")
	}
	if !containsString(jsonStr, `"timestamp":1234567890`) {
		t.Error("Expected 'timestamp' field in JSON")
	}
}

func TestHistoryResponse_MarshalJSON(t *testing.T) {
	response := HistoryResponse{
		Messages: []HistoryEntry{},
		Total:    0,
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal HistoryResponse: %v", err)
	}

	jsonStr := string(data)
	if !containsString(jsonStr, `"messages":[]`) {
		t.Error("Expected 'messages' field in JSON")
	}
	if !containsString(jsonStr, `"total":0`) {
		t.Error("Expected 'total' field in JSON")
	}
}

func TestGetRunIDFromEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		expected string
	}{
		{
			name:     "RunStartedEvent",
			event:    NewEvent(RunStarted, RunStartedEvent{ThreadID: "t1", RunID: "r1"}),
			expected: "r1",
		},
		{
			name:     "RunFinishedEvent",
			event:    NewEvent(RunFinished, RunFinishedEvent{ThreadID: "t1", RunID: "r2"}),
			expected: "r2",
		},
		{
			name:     "RunErrorEvent",
			event:    NewEvent(RunError, RunErrorEvent{ThreadID: "t1", RunID: "r3", Error: "error"}),
			expected: "r3",
		},
		{
			name:     "TextMessageStartEvent",
			event:    NewEvent(TextMessageStart, TextMessageStartEvent{MessageID: "m1"}),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRunIDFromEvent(tt.event)
			if result != tt.expected {
				t.Errorf("Expected RunID '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestEmitToHistory(t *testing.T) {
	hs := NewHistoryService()
	handler := &Handler{
		historyService: hs,
	}

	threadID := "test-thread-emit"
	event := NewEvent(RunStarted, RunStartedEvent{ThreadID: threadID, RunID: "run-1"})

	handler.EmitToHistory(threadID, event)

	entries, total := hs.GetThreadHistory(threadID, 10, 0)
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
	if entries[0].ThreadID != threadID {
		t.Errorf("Expected threadID '%s', got '%s'", threadID, entries[0].ThreadID)
	}
}

func TestEmitToHistory_NilService(t *testing.T) {
	handler := &Handler{
		historyService: nil,
	}

	// Should not panic
	handler.EmitToHistory("thread-1", NewEvent(RunStarted, RunStartedEvent{}))
}

func TestEmitToHistory_EmptyThreadID(t *testing.T) {
	hs := NewHistoryService()
	handler := &Handler{
		historyService: hs,
	}

	// Should not add entry for empty threadID
	handler.EmitToHistory("", NewEvent(RunStarted, RunStartedEvent{}))

	_, total := hs.GetThreadHistory("", 10, 0)
	if total != 0 {
		t.Errorf("Expected total 0, got %d", total)
	}
}

func TestWithHistoryService(t *testing.T) {
	handler := &Handler{}
	hs := NewHistoryService()

	result := WithHistoryService(handler, hs)
	if result != handler {
		t.Error("Expected same handler instance")
	}
	if handler.historyService != hs {
		t.Error("Expected historyService to be set")
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	for i := range s {
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestHistoryStore tests internal history store functions.
func TestHistoryStore(t *testing.T) {
	store := newHistoryStore()

	// Test append
	entry := HistoryEntry{
		ID:        "entry-1",
		ThreadID:  "thread-1",
		RunID:     "run-1",
		Timestamp: 1000,
	}
	store.append("thread-1", entry)

	// Test getThreadHistory
	entries, total := store.getThreadHistory("thread-1", 10, 0)
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	// Test clear
	store.clear("thread-1")
	entries, total = store.getThreadHistory("thread-1", 10, 0)
	if total != 0 {
		t.Errorf("Expected total 0 after clear, got %d", total)
	}
}

func TestHistoryStore_Pagination(t *testing.T) {
	store := newHistoryStore()

	// Add multiple entries
	for i := 0; i < 10; i++ {
		store.append("thread-1", HistoryEntry{
			ID:        "entry-" + string(rune('0'+i)),
			ThreadID:  "thread-1",
			RunID:     "run-1",
			Timestamp: int64(i * 1000),
		})
	}

	// Test pagination
	entries, total := store.getThreadHistory("thread-1", 3, 0)
	if total != 10 {
		t.Errorf("Expected total 10, got %d", total)
	}
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	// Test offset
	entries, total = store.getThreadHistory("thread-1", 3, 5)
	if len(entries) != 3 {
		t.Errorf("Expected 3 entries with offset, got %d", len(entries))
	}

	// Test offset beyond total
	entries, total = store.getThreadHistory("thread-1", 3, 100)
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for large offset, got %d", len(entries))
	}
}

func TestHistoryStore_NonExistentThread(t *testing.T) {
	store := newHistoryStore()

	entries, total := store.getThreadHistory("nonexistent", 10, 0)
	if total != 0 {
		t.Errorf("Expected total 0 for nonexistent thread, got %d", total)
	}
	if entries != nil {
		t.Errorf("Expected nil entries for nonexistent thread")
	}
}

func TestHandler_WithHistoryService(t *testing.T) {
	handler := &Handler{
		config:        DefaultServerConfig(),
		eventEmitter:  nil,
		agentExecutor: nil,
	}

	hs := NewHistoryService()
	handler.WithHistoryService(hs)

	if handler.historyService != hs {
		t.Error("Expected historyService to be set")
	}
}

func TestHandler_HandleHistory(t *testing.T) {
	handler := &Handler{
		config:         DefaultServerConfig(),
		eventEmitter:   nil,
		agentExecutor:  nil,
		historyService: NewHistoryService(),
	}

	// Add some history entries
	threadID := "test-thread"
	handler.historyService.Append(threadID, HistoryEntry{
		ID:        "entry-1",
		ThreadID:  threadID,
		RunID:     "run-1",
		Timestamp: time.Now().UnixMilli(),
	})

	// Test HandleHistory
	req := httptest.NewRequest(http.MethodGet, "/agui/history?threadId="+threadID+"&limit=10&offset=0", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Verify SSE format
	body := w.Body.String()
	if !containsString(body, "data: ") {
		t.Error("Expected SSE format in response")
	}
}

func TestHandler_HandleHistory_Options(t *testing.T) {
	handler := &Handler{
		config:         DefaultServerConfig(),
		eventEmitter:   nil,
		agentExecutor:  nil,
		historyService: NewHistoryService(),
	}

	req := httptest.NewRequest(http.MethodOptions, "/agui/history", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}
}

func TestHandler_HandleHistory_WrongMethod(t *testing.T) {
	handler := &Handler{
		config:         DefaultServerConfig(),
		eventEmitter:   nil,
		agentExecutor:  nil,
		historyService: NewHistoryService(),
	}

	req := httptest.NewRequest(http.MethodPost, "/agui/history", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestHandler_HandleHistory_MissingThreadID(t *testing.T) {
	handler := &Handler{
		config:         DefaultServerConfig(),
		eventEmitter:   nil,
		agentExecutor:  nil,
		historyService: NewHistoryService(),
	}

	req := httptest.NewRequest(http.MethodGet, "/agui/history", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandler_HandleHistory_InvalidLimit(t *testing.T) {
	handler := &Handler{
		config:         DefaultServerConfig(),
		eventEmitter:   nil,
		agentExecutor:  nil,
		historyService: NewHistoryService(),
	}

	req := httptest.NewRequest(http.MethodGet, "/agui/history?threadId=test&limit=invalid", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	// Should use default limit
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHandler_HandleHistory_InvalidOffset(t *testing.T) {
	handler := &Handler{
		config:         DefaultServerConfig(),
		eventEmitter:   nil,
		agentExecutor:  nil,
		historyService: NewHistoryService(),
	}

	req := httptest.NewRequest(http.MethodGet, "/agui/history?threadId=test&offset=invalid", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	// Should use default offset
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHandler_HandleHistory_NilService(t *testing.T) {
	handler := &Handler{
		config:         DefaultServerConfig(),
		eventEmitter:   nil,
		agentExecutor:  nil,
		historyService: nil,
	}

	req := httptest.NewRequest(http.MethodGet, "/agui/history?threadId=test", nil)
	w := httptest.NewRecorder()

	handler.HandleHistory(w, req)

	// Should still return success with empty history
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}
