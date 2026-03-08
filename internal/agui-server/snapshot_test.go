package aguiserver

import (
	"testing"
	"time"
)

func TestSnapshotManager_CreateSnapshot(t *testing.T) {
	config := DefaultSnapshotManagerConfig()
	config.MaxMessages = 10
	sm := NewSnapshotManager(config)

	tests := []struct {
		name      string
		threadID  string
		messages  []Message
		wantErr   bool
		errMsg    string
	}{
		{
			name:     "valid snapshot",
			threadID: "thread-1",
			messages: []Message{
				{ID: "1", Role: "user", Content: "Hello"},
				{ID: "2", Role: "assistant", Content: "Hi there"},
			},
			wantErr: false,
		},
		{
			name:     "empty thread ID",
			threadID: "",
			messages: []Message{
				{ID: "1", Role: "user", Content: "Hello"},
			},
			wantErr: true,
			errMsg:  "threadID cannot be empty",
		},
		{
			name:     "empty messages",
			threadID: "thread-2",
			messages: []Message{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot, err := sm.CreateSnapshot(tt.threadID, tt.messages)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateSnapshot() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("CreateSnapshot() error = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateSnapshot() unexpected error = %v", err)
				return
			}

			if snapshot == nil {
				t.Errorf("CreateSnapshot() snapshot = nil, want non-nil")
				return
			}

			if snapshot.ThreadID != tt.threadID {
				t.Errorf("CreateSnapshot() ThreadID = %v, want %v", snapshot.ThreadID, tt.threadID)
			}

			if len(snapshot.Messages) != len(tt.messages) {
				t.Errorf("CreateSnapshot() Messages count = %v, want %v", len(snapshot.Messages), len(tt.messages))
			}

			if snapshot.Timestamp.IsZero() {
				t.Errorf("CreateSnapshot() Timestamp = zero, want non-zero")
			}
		})
	}
}

func TestSnapshotManager_RestoreSnapshot(t *testing.T) {
	config := DefaultSnapshotManagerConfig()
	sm := NewSnapshotManager(config)

	tests := []struct {
		name     string
		snapshot *Snapshot
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid snapshot",
			snapshot: &Snapshot{
				ThreadID: "thread-1",
				Messages: []Message{
					{ID: "1", Role: "user", Content: "Hello"},
				},
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name:     "nil snapshot",
			snapshot: nil,
			wantErr:  true,
			errMsg:   "snapshot cannot be nil",
		},
		{
			name: "empty thread ID",
			snapshot: &Snapshot{
				ThreadID: "",
				Messages: []Message{
					{ID: "1", Role: "user", Content: "Hello"},
				},
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "snapshot threadID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.RestoreSnapshot(tt.snapshot)

			if tt.wantErr {
				if err == nil {
					t.Errorf("RestoreSnapshot() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("RestoreSnapshot() error = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("RestoreSnapshot() unexpected error = %v", err)
				return
			}

			// Verify snapshot was restored
			restored, err := sm.GetSnapshot(tt.snapshot.ThreadID)
			if err != nil {
				t.Errorf("RestoreSnapshot() failed to get restored snapshot: %v", err)
				return
			}

			if restored.ThreadID != tt.snapshot.ThreadID {
				t.Errorf("RestoreSnapshot() restored ThreadID = %v, want %v", restored.ThreadID, tt.snapshot.ThreadID)
			}
		})
	}
}

func TestSnapshotManager_MemoryOptimization(t *testing.T) {
	config := DefaultSnapshotManagerConfig()
	config.MaxMessages = 5
	sm := NewSnapshotManager(config)

	// Create more messages than maxMessages
	messages := make([]Message, 20)
	for i := 0; i < 20; i++ {
		messages[i] = Message{
			ID:      string(rune('A' + i)),
			Role:    "user",
			Content: "Message",
		}
	}

	snapshot, err := sm.CreateSnapshot("thread-opt", messages)
	if err != nil {
		t.Fatalf("CreateSnapshot() unexpected error = %v", err)
	}

	// Verify only maxMessages are kept
	if len(snapshot.Messages) != config.MaxMessages {
		t.Errorf("CreateSnapshot() Messages count = %v, want %v (memory optimization failed)",
			len(snapshot.Messages), config.MaxMessages)
	}

	// Verify the most recent messages are kept (last 5)
	if len(snapshot.Messages) > 0 {
		firstMsg := snapshot.Messages[0]
		if firstMsg.ID != string(rune('A'+15)) {
			t.Errorf("CreateSnapshot() first message ID = %v, want %v (should keep most recent)",
				firstMsg.ID, string(rune('A'+15)))
		}

		lastMsg := snapshot.Messages[len(snapshot.Messages)-1]
		if lastMsg.ID != string(rune('A'+19)) {
			t.Errorf("CreateSnapshot() last message ID = %v, want %v",
				lastMsg.ID, string(rune('A'+19)))
		}
	}
}

func TestSnapshotManager_GetSnapshot(t *testing.T) {
	config := DefaultSnapshotManagerConfig()
	sm := NewSnapshotManager(config)

	// Create a snapshot first
	originalMessages := []Message{
		{ID: "1", Role: "user", Content: "Hello"},
		{ID: "2", Role: "assistant", Content: "Hi"},
	}
	_, err := sm.CreateSnapshot("thread-get", originalMessages)
	if err != nil {
		t.Fatalf("CreateSnapshot() unexpected error = %v", err)
	}

	// Get the snapshot
	snapshot, err := sm.GetSnapshot("thread-get")
	if err != nil {
		t.Errorf("GetSnapshot() unexpected error = %v", err)
		return
	}

	if snapshot == nil {
		t.Errorf("GetSnapshot() snapshot = nil, want non-nil")
		return
	}

	if snapshot.ThreadID != "thread-get" {
		t.Errorf("GetSnapshot() ThreadID = %v, want thread-get", snapshot.ThreadID)
	}

	if len(snapshot.Messages) != 2 {
		t.Errorf("GetSnapshot() Messages count = %v, want 2", len(snapshot.Messages))
	}

	// Test non-existent snapshot
	_, err = sm.GetSnapshot("non-existent")
	if err == nil {
		t.Errorf("GetSnapshot() error = nil, want error for non-existent snapshot")
	}
}

func TestSnapshotManager_DeleteSnapshot(t *testing.T) {
	config := DefaultSnapshotManagerConfig()
	sm := NewSnapshotManager(config)

	// Create a snapshot
	_, err := sm.CreateSnapshot("thread-del", []Message{{ID: "1", Role: "user", Content: "Hello"}})
	if err != nil {
		t.Fatalf("CreateSnapshot() unexpected error = %v", err)
	}

	// Delete it
	err = sm.DeleteSnapshot("thread-del")
	if err != nil {
		t.Errorf("DeleteSnapshot() unexpected error = %v", err)
		return
	}

	// Verify it's deleted
	_, err = sm.GetSnapshot("thread-del")
	if err == nil {
		t.Errorf("DeleteSnapshot() snapshot still exists after deletion")
	}

	// Test deleting non-existent snapshot
	err = sm.DeleteSnapshot("non-existent")
	if err == nil {
		t.Errorf("DeleteSnapshot() error = nil, want error for non-existent snapshot")
	}
}

func TestSnapshotManager_ListSnapshots(t *testing.T) {
	config := DefaultSnapshotManagerConfig()
	sm := NewSnapshotManager(config)

	// Create multiple snapshots
	threadIDs := []string{"thread-1", "thread-2", "thread-3"}
	for _, tid := range threadIDs {
		_, err := sm.CreateSnapshot(tid, []Message{{ID: "1", Role: "user", Content: "Hello"}})
		if err != nil {
			t.Fatalf("CreateSnapshot() unexpected error = %v", err)
		}
	}

	list := sm.ListSnapshots()
	if len(list) != len(threadIDs) {
		t.Errorf("ListSnapshots() count = %v, want %v", len(list), len(threadIDs))
	}

	// Verify all thread IDs are present
	threadSet := make(map[string]bool)
	for _, tid := range list {
		threadSet[tid] = true
	}

	for _, tid := range threadIDs {
		if !threadSet[tid] {
			t.Errorf("ListSnapshots() missing thread ID: %s", tid)
		}
	}
}

func TestSnapshotManager_ClearAllSnapshots(t *testing.T) {
	config := DefaultSnapshotManagerConfig()
	sm := NewSnapshotManager(config)

	// Create multiple snapshots
	for i := 0; i < 5; i++ {
		_, err := sm.CreateSnapshot(string(rune('A'+i)), []Message{{ID: "1", Role: "user", Content: "Hello"}})
		if err != nil {
			t.Fatalf("CreateSnapshot() unexpected error = %v", err)
		}
	}

	if sm.GetSnapshotCount() != 5 {
		t.Errorf("GetSnapshotCount() = %v, want 5", sm.GetSnapshotCount())
	}

	// Clear all
	sm.ClearAllSnapshots()

	if sm.GetSnapshotCount() != 0 {
		t.Errorf("ClearAllSnapshots() failed, count = %v, want 0", sm.GetSnapshotCount())
	}
}

func TestSnapshotManager_GetSnapshotCount(t *testing.T) {
	config := DefaultSnapshotManagerConfig()
	sm := NewSnapshotManager(config)

	// Initial count should be 0
	if sm.GetSnapshotCount() != 0 {
		t.Errorf("GetSnapshotCount() initial = %v, want 0", sm.GetSnapshotCount())
	}

	// Add snapshots
	for i := 0; i < 3; i++ {
		_, err := sm.CreateSnapshot(string(rune('A'+i)), []Message{{ID: "1", Role: "user", Content: "Hello"}})
		if err != nil {
			t.Fatalf("CreateSnapshot() unexpected error = %v", err)
		}
	}

	if sm.GetSnapshotCount() != 3 {
		t.Errorf("GetSnapshotCount() after adds = %v, want 3", sm.GetSnapshotCount())
	}

	// Overwrite existing snapshot (count should stay same)
	_, err := sm.CreateSnapshot("A", []Message{{ID: "1", Role: "user", Content: "Updated"}})
	if err != nil {
		t.Fatalf("CreateSnapshot() unexpected error = %v", err)
	}

	if sm.GetSnapshotCount() != 3 {
		t.Errorf("GetSnapshotCount() after overwrite = %v, want 3", sm.GetSnapshotCount())
	}
}

func TestSnapshotManager_StartStop(t *testing.T) {
	config := DefaultSnapshotManagerConfig()
	config.SnapshotInterval = 100 * time.Millisecond
	sm := NewSnapshotManager(config)

	// Start the manager
	sm.Start()

	// Let it run for a bit
	time.Sleep(250 * time.Millisecond)

	// Stop the manager
	sm.Stop()

	// Should not panic or hang
	t.Log("Start/Stop completed successfully")
}

func TestSnapshot_OptimizeMessages_EdgeCases(t *testing.T) {
	config := DefaultSnapshotManagerConfig()
	config.MaxMessages = 3
	sm := NewSnapshotManager(config)

	tests := []struct {
		name     string
		messages []Message
		wantLen  int
	}{
		{
			name:     "nil messages",
			messages: nil,
			wantLen:  0,
		},
		{
			name:     "empty messages",
			messages: []Message{},
			wantLen:  0,
		},
		{
			name: "exact limit",
			messages: []Message{
				{ID: "1", Role: "user", Content: "1"},
				{ID: "2", Role: "user", Content: "2"},
				{ID: "3", Role: "user", Content: "3"},
			},
			wantLen: 3,
		},
		{
			name: "one over limit",
			messages: []Message{
				{ID: "1", Role: "user", Content: "1"},
				{ID: "2", Role: "user", Content: "2"},
				{ID: "3", Role: "user", Content: "3"},
				{ID: "4", Role: "user", Content: "4"},
			},
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optimized := sm.optimizeMessages(tt.messages)
			if len(optimized) != tt.wantLen {
				t.Errorf("optimizeMessages() len = %v, want %v", len(optimized), tt.wantLen)
			}
		})
	}
}
