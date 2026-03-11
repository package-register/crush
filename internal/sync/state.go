package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileState represents the sync state of a file.
type FileState struct {
	LocalETag    string    `json:"local_etag,omitempty"`
	RemoteETag   string    `json:"remote_etag,omitempty"`
	LastSync     time.Time `json:"last_sync,omitempty"`
	LastModified time.Time `json:"last_modified,omitempty"`
	Size         int64     `json:"size,omitempty"`
}

// ConflictState represents the state of a conflict.
type ConflictState struct {
	LocalETag     string    `json:"local_etag"`
	RemoteETag    string    `json:"remote_etag"`
	LocalModTime  time.Time `json:"local_mod_time"`
	RemoteModTime time.Time `json:"remote_mod_time"`
	Resolution    string    `json:"resolution,omitempty"`
	ResolvedAt    time.Time `json:"resolved_at,omitempty"`
}

// SyncState represents the overall sync state.
type SyncState struct {
	mu           sync.RWMutex
	localDir     string
	Files        map[string]FileState     `json:"files"`
	Conflicts    map[string]ConflictState `json:"conflicts,omitempty"`
	LastFullSync time.Time                `json:"last_full_sync,omitempty"`
	SyncVersion  int                      `json:"sync_version"`
}

// NewSyncState creates a new sync state.
func NewSyncState(localDir string) *SyncState {
	return &SyncState{
		localDir:    localDir,
		Files:       make(map[string]FileState),
		Conflicts:   make(map[string]ConflictState),
		SyncVersion: 1,
	}
}

// Load loads the sync state from disk.
func (s *SyncState) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stateFile := s.getStateFile()
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, s)
}

// Save saves the sync state to disk.
func (s *SyncState) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stateFile := s.getStateFile()

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(stateFile, data, 0o600)
}

// GetFileState gets the state of a file.
func (s *SyncState) GetFileState(path string) (FileState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.Files[path]
	return state, ok
}

// SetFileState sets the state of a file.
func (s *SyncState) SetFileState(path string, state FileState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Files[path] = state
}

// SetLocalETag sets the local ETag for a file.
func (s *SyncState) SetLocalETag(path, etag string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.Files[path]
	state.LocalETag = etag
	s.Files[path] = state
}

// SetRemoteETag sets the remote ETag for a file.
func (s *SyncState) SetRemoteETag(path, etag string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.Files[path]
	state.RemoteETag = etag
	s.Files[path] = state
}

// SetLastSync sets the last sync time for a file.
func (s *SyncState) SetLastSync(path string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.Files[path]
	state.LastSync = t
	s.Files[path] = state
}

// GetConflict gets conflict information for a file.
func (s *SyncState) GetConflict(path string) (*ConflictInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conflict, ok := s.Conflicts[path]
	if !ok {
		return nil, nil
	}

	return &ConflictInfo{
		LocalPath:     path,
		RemotePath:    path,
		LocalETag:     conflict.LocalETag,
		RemoteETag:    conflict.RemoteETag,
		LocalModTime:  conflict.LocalModTime,
		RemoteModTime: conflict.RemoteModTime,
		Resolution:    conflict.Resolution,
	}, nil
}

// AddConflict adds a conflict to the state.
func (s *SyncState) AddConflict(path string, info *ConflictInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Conflicts[path] = ConflictState{
		LocalETag:     info.LocalETag,
		RemoteETag:    info.RemoteETag,
		LocalModTime:  info.LocalModTime,
		RemoteModTime: info.RemoteModTime,
	}
}

// ResolveConflict marks a conflict as resolved.
func (s *SyncState) ResolveConflict(path, resolution string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conflict, ok := s.Conflicts[path]; ok {
		conflict.Resolution = resolution
		conflict.ResolvedAt = time.Now()
		s.Conflicts[path] = conflict
	}

	return nil
}

// RemoveConflict removes a conflict from the state.
func (s *SyncState) RemoveConflict(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Conflicts, path)
}

// GetConflicts returns all unresolved conflicts.
func (s *SyncState) GetConflicts() map[string]ConflictState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conflicts := make(map[string]ConflictState)
	for path, conflict := range s.Conflicts {
		if conflict.Resolution == "" {
			conflicts[path] = conflict
		}
	}
	return conflicts
}

// ClearConflicts removes all resolved conflicts.
func (s *SyncState) ClearConflicts() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for path, conflict := range s.Conflicts {
		if conflict.Resolution != "" {
			delete(s.Conflicts, path)
		}
	}
}

// getStateFile returns the path to the state file.
func (s *SyncState) getStateFile() string {
	return filepath.Join(s.localDir, ".sync_state.json")
}
