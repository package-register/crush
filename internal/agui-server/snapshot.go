package aguiserver

import (
	"fmt"
	"sync"
	"time"
)

// Snapshot represents a point-in-time snapshot of messages for a thread.
type Snapshot struct {
	// ThreadID is the conversation thread identifier.
	ThreadID string
	// Messages is the list of messages in the snapshot.
	Messages []Message
	// Timestamp is the time when the snapshot was created.
	Timestamp time.Time
}

// SnapshotManager manages snapshots for all threads.
type SnapshotManager struct {
	mu            sync.RWMutex
	snapshots     map[string]*Snapshot
	maxSnapshots  int
	maxMessages   int
	snapshotInterval time.Duration
	stopChan      chan struct{}
	ticker        *time.Ticker
}

// SnapshotManagerConfig holds configuration for the SnapshotManager.
type SnapshotManagerConfig struct {
	// MaxSnapshots is the maximum number of snapshots to keep per thread.
	MaxSnapshots int
	// MaxMessages is the maximum number of messages to keep in each snapshot.
	MaxMessages int
	// SnapshotInterval is the interval between automatic snapshots.
	SnapshotInterval time.Duration
}

// DefaultSnapshotManagerConfig returns a SnapshotManagerConfig with default values.
func DefaultSnapshotManagerConfig() SnapshotManagerConfig {
	return SnapshotManagerConfig{
		MaxSnapshots:     5,
		MaxMessages:      100,
		SnapshotInterval: 30 * time.Second,
	}
}

// NewSnapshotManager creates a new SnapshotManager with the given configuration.
func NewSnapshotManager(config SnapshotManagerConfig) *SnapshotManager {
	if config.MaxSnapshots <= 0 {
		config.MaxSnapshots = 5
	}
	if config.MaxMessages <= 0 {
		config.MaxMessages = 100
	}
	if config.SnapshotInterval <= 0 {
		config.SnapshotInterval = 30 * time.Second
	}

	sm := &SnapshotManager{
		snapshots:        make(map[string]*Snapshot),
		maxSnapshots:     config.MaxSnapshots,
		maxMessages:      config.MaxMessages,
		snapshotInterval: config.SnapshotInterval,
		stopChan:         make(chan struct{}),
	}

	return sm
}

// Start starts the automatic snapshot ticker.
func (sm *SnapshotManager) Start() {
	sm.ticker = time.NewTicker(sm.snapshotInterval)
	go sm.runSnapshotLoop()
}

// Stop stops the automatic snapshot ticker.
func (sm *SnapshotManager) Stop() {
	close(sm.stopChan)
	if sm.ticker != nil {
		sm.ticker.Stop()
	}
}

// runSnapshotLoop runs the periodic snapshot loop.
func (sm *SnapshotManager) runSnapshotLoop() {
	for {
		select {
		case <-sm.ticker.C:
			// Trigger snapshots for all threads
			sm.mu.RLock()
			threadIDs := make([]string, 0, len(sm.snapshots))
			for threadID := range sm.snapshots {
				threadIDs = append(threadIDs, threadID)
			}
			sm.mu.RUnlock()

			// Note: Actual snapshot creation requires access to message source
			// This is a placeholder for the ticker trigger mechanism
		case <-sm.stopChan:
			return
		}
	}
}

// CreateSnapshot creates a new snapshot for the given thread with the provided messages.
func (sm *SnapshotManager) CreateSnapshot(threadID string, messages []Message) (*Snapshot, error) {
	if threadID == "" {
		return nil, fmt.Errorf("threadID cannot be empty")
	}

	// Apply memory optimization: limit messages
	optimizedMessages := sm.optimizeMessages(messages)

	snapshot := &Snapshot{
		ThreadID:  threadID,
		Messages:  optimizedMessages,
		Timestamp: time.Now(),
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Store snapshot (one per thread, keeping the latest)
	sm.snapshots[threadID] = snapshot

	return snapshot, nil
}

// GetSnapshot retrieves the latest snapshot for a thread.
func (sm *SnapshotManager) GetSnapshot(threadID string) (*Snapshot, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	snapshot, exists := sm.snapshots[threadID]
	if !exists {
		return nil, fmt.Errorf("snapshot not found for thread: %s", threadID)
	}

	return snapshot, nil
}

// RestoreSnapshot restores messages from a snapshot.
func (sm *SnapshotManager) RestoreSnapshot(snapshot *Snapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot cannot be nil")
	}
	if snapshot.ThreadID == "" {
		return fmt.Errorf("snapshot threadID cannot be empty")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Validate and restore
	sm.snapshots[snapshot.ThreadID] = snapshot
	return nil
}

// DeleteSnapshot deletes a snapshot for a thread.
func (sm *SnapshotManager) DeleteSnapshot(threadID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.snapshots[threadID]; !exists {
		return fmt.Errorf("snapshot not found for thread: %s", threadID)
	}

	delete(sm.snapshots, threadID)
	return nil
}

// ListSnapshots returns all snapshot thread IDs.
func (sm *SnapshotManager) ListSnapshots() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	threadIDs := make([]string, 0, len(sm.snapshots))
	for threadID := range sm.snapshots {
		threadIDs = append(threadIDs, threadID)
	}
	return threadIDs
}

// optimizeMessages applies memory optimization by limiting the number of messages.
func (sm *SnapshotManager) optimizeMessages(messages []Message) []Message {
	if len(messages) <= sm.maxMessages {
		return messages
	}

	// Keep only the most recent messages
	startIndex := len(messages) - sm.maxMessages
	optimized := make([]Message, sm.maxMessages)
	copy(optimized, messages[startIndex:])

	return optimized
}

// GetSnapshotCount returns the total number of snapshots.
func (sm *SnapshotManager) GetSnapshotCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.snapshots)
}

// ClearAllSnapshots clears all snapshots.
func (sm *SnapshotManager) ClearAllSnapshots() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.snapshots = make(map[string]*Snapshot)
}
