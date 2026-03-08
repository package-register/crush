// Package aguiserver implements the AG-UI Protocol server for Crush.
// It provides SSE-based streaming of AG-UI events to external clients.
package aguiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/google/uuid"
)

// HistoryEntry represents a single message entry in the history.
type HistoryEntry struct {
	// ID is the unique identifier for this entry.
	ID string `json:"id"`
	// ThreadID is the conversation thread identifier.
	ThreadID string `json:"threadId"`
	// RunID is the run identifier.
	RunID string `json:"runId"`
	// Event is the AG-UI event.
	Event Event `json:"event"`
	// Timestamp is when the event was recorded.
	Timestamp int64 `json:"timestamp"`
}

// HistoryService manages message history for AG-UI threads.
type HistoryService interface {
	// Append adds an event to the thread history.
	Append(threadID string, entry HistoryEntry)
	// GetThreadHistory retrieves history for a thread with pagination.
	GetThreadHistory(threadID string, limit, offset int) ([]HistoryEntry, int)
	// Clear removes all history for a thread.
	Clear(threadID string)
}

// historyService is the default implementation of HistoryService.
type historyService struct {
	// histories maps threadID to a slice of history entries.
	histories *csync.Map[string, *csync.Slice[HistoryEntry]]
}

// NewHistoryService creates a new HistoryService.
func NewHistoryService() HistoryService {
	return &historyService{
		histories: csync.NewMap[string, *csync.Slice[HistoryEntry]](),
	}
}

// Append adds an event to the thread history.
func (s *historyService) Append(threadID string, entry HistoryEntry) {
	if threadID == "" {
		return
	}

	// Get or create the slice for this thread
	slice := s.histories.GetOrSet(threadID, func() *csync.Slice[HistoryEntry] {
		return csync.NewSlice[HistoryEntry]()
	})

	// Append the entry
	slice.Append(entry)
}

// GetThreadHistory retrieves history for a thread with pagination.
// Returns the entries and total count.
func (s *historyService) GetThreadHistory(threadID string, limit, offset int) ([]HistoryEntry, int) {
	if threadID == "" {
		return nil, 0
	}

	// Default limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// Get the slice for this thread
	slice, ok := s.histories.Get(threadID)
	if !ok {
		return nil, 0
	}

	// Get all entries
	entries := slice.Copy()
	total := len(entries)

	// Sort by timestamp
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp < entries[j].Timestamp
	})

	// Apply pagination
	if offset >= total {
		return nil, total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return entries[offset:end], total
}

// Clear removes all history for a thread.
func (s *historyService) Clear(threadID string) {
	if threadID == "" {
		return
	}
	s.histories.Del(threadID)
}

// HistoryQueryParams represents query parameters for history requests.
type HistoryQueryParams struct {
	ThreadID string `json:"threadId"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

// HistoryResponse represents the response for history queries.
type HistoryResponse struct {
	Messages []HistoryEntry `json:"messages"`
	Total    int            `json:"total"`
}

// HandleHistory handles history snapshot requests with SSE streaming.
func (h *Handler) HandleHistory(w http.ResponseWriter, r *http.Request) {
	// Handle OPTIONS preflight
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", http.MethodGet)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Only allow GET method
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	params := HistoryQueryParams{
		ThreadID: r.URL.Query().Get("threadId"),
		Limit:    50,
		Offset:   0,
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			params.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			params.Offset = offset
		}
	}

	// Validate threadId
	if params.ThreadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	// Get history from service
	var entries []HistoryEntry
	var total int

	if h.historyService != nil {
		entries, total = h.historyService.GetThreadHistory(params.ThreadID, params.Limit, params.Offset)
	}

	// Ensure non-nil slices for JSON
	if entries == nil {
		entries = []HistoryEntry{}
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Stream the response
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send RUN_STARTED event
	startEvent := NewEvent(RunStarted, RunStartedEvent{
		ThreadID: params.ThreadID,
		RunID:    uuid.New().String(),
	})
	fmt.Fprintf(w, "data: %s\n\n", mustMarshal(startEvent))
	flusher.Flush()

	// Send MESSAGES_SNAPSHOT event
	response := HistoryResponse{
		Messages: entries,
		Total:    total,
	}
	snapshotEvent := NewEvent(CustomEvent, CustomEventData{
		Type:    "MESSAGES_SNAPSHOT",
		Payload: response,
	})
	fmt.Fprintf(w, "data: %s\n\n", mustMarshal(snapshotEvent))
	flusher.Flush()

	// Send RUN_FINISHED event
	finishEvent := NewEvent(RunFinished, RunFinishedEvent{
		ThreadID: params.ThreadID,
		RunID:    startEvent.Data.(RunStartedEvent).RunID,
	})
	fmt.Fprintf(w, "data: %s\n\n", mustMarshal(finishEvent))
	flusher.Flush()
}

// EmitToHistory emits an event to the history service.
func (h *Handler) EmitToHistory(threadID string, event Event) {
	if h.historyService == nil || threadID == "" {
		return
	}

	entry := HistoryEntry{
		ID:        uuid.New().String(),
		ThreadID:  threadID,
		RunID:     getRunIDFromEvent(event),
		Event:     event,
		Timestamp: time.Now().UnixMilli(),
	}
	h.historyService.Append(threadID, entry)
}

// getRunIDFromEvent extracts the RunID from an event if available.
func getRunIDFromEvent(event Event) string {
	switch data := event.Data.(type) {
	case RunStartedEvent:
		return data.RunID
	case RunFinishedEvent:
		return data.RunID
	case RunErrorEvent:
		return data.RunID
	default:
		return ""
	}
}

// Ensure Handler has historyService field and method to set it.
type handlerWithHistory struct {
	*Handler
	historyService HistoryService
}

// WithHistoryService wraps a handler with history service support.
func WithHistoryService(h *Handler, hs HistoryService) *Handler {
	h.historyService = hs
	return h
}

// historyStore is a thread-safe store for message history using csync.Map.
type historyStore struct {
	mu      sync.RWMutex
	entries map[string][]HistoryEntry
}

// newHistoryStore creates a new history store.
func newHistoryStore() *historyStore {
	return &historyStore{
		entries: make(map[string][]HistoryEntry),
	}
}

// append adds an entry to the store.
func (s *historyStore) append(threadID string, entry HistoryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.entries[threadID] == nil {
		s.entries[threadID] = make([]HistoryEntry, 0)
	}
	s.entries[threadID] = append(s.entries[threadID], entry)
}

// getThreadHistory retrieves entries for a thread with pagination.
func (s *historyStore) getThreadHistory(threadID string, limit, offset int) ([]HistoryEntry, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, ok := s.entries[threadID]
	if !ok {
		return nil, 0
	}

	total := len(entries)

	// Sort by timestamp
	sorted := make([]HistoryEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp < sorted[j].Timestamp
	})

	// Apply pagination
	if offset >= total {
		return nil, total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return sorted[offset:end], total
}

// clear removes all entries for a thread.
func (s *historyStore) clear(threadID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, threadID)
}

// MarshalJSON implements json.Marshaler for HistoryEntry.
func (h HistoryEntry) MarshalJSON() ([]byte, error) {
	type Alias HistoryEntry
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(&h),
	})
}
