// Package sync provides synchronization engine for WebDAV-based configuration sync.
package sync

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/webdav"
)

// SyncMode represents the synchronization mode.
type SyncMode int

const (
	// SyncModeBidirectional syncs changes in both directions.
	SyncModeBidirectional SyncMode = iota
	// SyncModeUpload only uploads local changes.
	SyncModeUpload
	// SyncModeDownload only downloads remote changes.
	SyncModeDownload
)

// ConflictStrategy defines how to handle sync conflicts.
type ConflictStrategy int

const (
	// ConflictStrategyNewerWins uses the newer version.
	ConflictStrategyNewerWins ConflictStrategy = iota
	// ConflictStrategyLocalWins always keeps local version.
	ConflictStrategyLocalWins
	// ConflictStrategyRemoteWins always keeps remote version.
	ConflictStrategyRemoteWins
	// ConflictStrategyBackup creates a backup of the conflicting file.
	ConflictStrategyBackup
	// ConflictStrategyManual requires manual resolution.
	ConflictStrategyManual
)

// SyncStatus represents the current sync status.
type SyncStatus int

const (
	SyncStatusIdle SyncStatus = iota
	SyncStatusSyncing
	SyncStatusPaused
	SyncStatusError
	SyncStatusConflict
)

func (s SyncStatus) String() string {
	switch s {
	case SyncStatusIdle:
		return "idle"
	case SyncStatusSyncing:
		return "syncing"
	case SyncStatusPaused:
		return "paused"
	case SyncStatusError:
		return "error"
	case SyncStatusConflict:
		return "conflict"
	default:
		return "unknown"
	}
}

// Config holds the sync engine configuration.
type Config struct {
	// LocalDir is the local directory to sync (e.g., .crush/)
	LocalDir string
	// RemotePath is the remote WebDAV path
	RemotePath string
	// SyncMode is the synchronization mode
	SyncMode SyncMode
	// ConflictStrategy is how to handle conflicts
	ConflictStrategy ConflictStrategy
	// ExcludePatterns are file patterns to exclude from sync
	ExcludePatterns []string
	// SyncInterval is how often to sync (0 = manual only)
	SyncInterval time.Duration
	// MaxRetries is the maximum number of retries per file
	MaxRetries int
	// RetryDelay is the delay between retries
	RetryDelay time.Duration
	// Logger is the logger for sync operations
	Logger *slog.Logger
}

// FileInfo represents information about a file to be synced.
type FileInfo struct {
	Path         string
	RelativePath string
	Size         int64
	ETag         string
	LastModified time.Time
	IsDir        bool
	Hash         string // Local file hash for conflict detection
}

// SyncEvent represents a sync event.
type SyncEvent struct {
	Type      string    // upload, download, delete, conflict, error
	Path      string    // File path
	Direction string    // local->remote, remote->local
	Timestamp time.Time // Event timestamp
	Size      int64     // File size
	Error     error     // Error if any
	Conflict  *ConflictInfo
}

// ConflictInfo contains information about a sync conflict.
type ConflictInfo struct {
	LocalPath     string
	RemotePath    string
	LocalETag     string
	RemoteETag    string
	LocalModTime  time.Time
	RemoteModTime time.Time
	Resolution    string // local, remote, backup, manual
}

// Engine is the sync engine for WebDAV synchronization.
type Engine struct {
	config     Config
	client     *webdav.Client
	status     SyncStatus
	statusMu   sync.RWMutex
	lastSync   time.Time
	lastSyncMu sync.RWMutex

	queue      *SyncQueue
	state      *SyncState
	logger     *slog.Logger
	cancelFunc context.CancelFunc
	ctx        context.Context
	wg         sync.WaitGroup

	eventSubscribers []chan SyncEvent
	eventMu          sync.RWMutex
}

// NewEngine creates a new sync engine.
func NewEngine(config Config, client *webdav.Client) (*Engine, error) {
	if config.LocalDir == "" {
		return nil, fmt.Errorf("local directory is required")
	}
	if config.RemotePath == "" {
		return nil, fmt.Errorf("remote path is required")
	}
	if client == nil {
		return nil, fmt.Errorf("WebDAV client is required")
	}

	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	engine := &Engine{
		config: config,
		client: client,
		status: SyncStatusIdle,
		queue:  NewSyncQueue(),
		state:  NewSyncState(config.LocalDir),
		logger: logger,
	}

	return engine, nil
}

// Start starts the sync engine.
func (e *Engine) Start(ctx context.Context) error {
	e.statusMu.Lock()
	if e.status == SyncStatusSyncing {
		e.statusMu.Unlock()
		return fmt.Errorf("sync already running")
	}
	e.status = SyncStatusSyncing
	e.statusMu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	e.cancelFunc = cancel
	e.ctx = ctx

	e.logger.Info("Starting sync engine",
		"local_dir", e.config.LocalDir,
		"remote_path", e.config.RemotePath,
		"mode", e.config.SyncMode,
	)

	// Load sync state
	if err := e.state.Load(); err != nil {
		e.logger.Warn("Failed to load sync state", "error", err)
	}

	// Start sync loop if interval is set
	if e.config.SyncInterval > 0 {
		e.wg.Add(1)
		go e.syncLoop()
	}

	// Perform initial sync
	go func() {
		if err := e.Sync(); err != nil {
			e.logger.Error("Initial sync failed", "error", err)
			e.setStatus(SyncStatusError)
		}
	}()

	return nil
}

// Stop stops the sync engine.
func (e *Engine) Stop() error {
	e.logger.Info("Stopping sync engine")

	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	e.wg.Wait()
	e.setStatus(SyncStatusIdle)

	// Save sync state
	if err := e.state.Save(); err != nil {
		e.logger.Warn("Failed to save sync state", "error", err)
	}

	return nil
}

// Sync performs a synchronization.
func (e *Engine) Sync() error {
	e.logger.Info("Starting sync")
	e.setStatus(SyncStatusSyncing)
	defer func() {
		if e.getStatus() != SyncStatusConflict && e.getStatus() != SyncStatusError {
			e.setStatus(SyncStatusIdle)
		}
	}()

	// Get local and remote file lists
	localFiles, err := e.getLocalFiles()
	if err != nil {
		return fmt.Errorf("failed to get local files: %w", err)
	}

	remoteFiles, err := e.getRemoteFiles()
	if err != nil {
		return fmt.Errorf("failed to get remote files: %w", err)
	}

	e.logger.Debug("Found files", "local", len(localFiles), "remote", len(remoteFiles))

	// Determine sync actions
	actions, err := e.determineSyncActions(localFiles, remoteFiles)
	if err != nil {
		return fmt.Errorf("failed to determine sync actions: %w", err)
	}

	if len(actions) == 0 {
		e.logger.Info("No sync actions needed")
		e.setLastSync(time.Now())
		return nil
	}

	e.logger.Info("Sync actions determined", "count", len(actions))

	// Execute sync actions
	var hasConflict bool
	for _, action := range actions {
		select {
		case <-e.ctx.Done():
			return context.Canceled
		default:
		}

		if err := e.executeAction(action); err != nil {
			e.logger.Error("Sync action failed", "action", action.Type, "path", action.Path, "error", err)

			if isConflictError(err) {
				hasConflict = true
				e.setStatus(SyncStatusConflict)
				e.broadcastEvent(SyncEvent{
					Type:      "conflict",
					Path:      action.Path,
					Timestamp: time.Now(),
					Conflict:  extractConflictInfo(err),
				})
			} else {
				e.setStatus(SyncStatusError)
				e.broadcastEvent(SyncEvent{
					Type:      "error",
					Path:      action.Path,
					Timestamp: time.Now(),
					Error:     err,
				})
			}
		}
	}

	if !hasConflict {
		e.setLastSync(time.Now())
		e.logger.Info("Sync completed successfully")
	}

	// Save sync state
	if err := e.state.Save(); err != nil {
		e.logger.Warn("Failed to save sync state", "error", err)
	}

	return nil
}

// Status returns the current sync status.
func (e *Engine) Status() SyncStatus {
	return e.getStatus()
}

// LastSyncTime returns the last successful sync time.
func (e *Engine) LastSyncTime() time.Time {
	return e.getLastSync()
}

// ResolveConflict resolves a sync conflict.
func (e *Engine) ResolveConflict(path string, strategy ConflictStrategy) error {
	e.logger.Info("Resolving conflict", "path", path, "strategy", strategy)

	// Get conflict info from state
	conflict, err := e.state.GetConflict(path)
	if err != nil {
		return fmt.Errorf("failed to get conflict info: %w", err)
	}

	var resolution string
	switch strategy {
	case ConflictStrategyLocalWins:
		resolution = "local"
		// Re-upload local file
		if err := e.uploadFile(path); err != nil {
			return err
		}
	case ConflictStrategyRemoteWins:
		resolution = "remote"
		// Re-download remote file
		if err := e.downloadFile(path); err != nil {
			return err
		}
	case ConflictStrategyBackup:
		resolution = "backup"
		// Create backup of local file
		if err := e.createBackup(path); err != nil {
			return err
		}
		// Download remote file
		if err := e.downloadFile(path); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported conflict strategy: %v", strategy)
	}

	// Update conflict resolution in state
	conflict.Resolution = resolution
	if err := e.state.ResolveConflict(path, resolution); err != nil {
		return fmt.Errorf("failed to update conflict resolution: %w", err)
	}

	e.broadcastEvent(SyncEvent{
		Type:      "conflict_resolved",
		Path:      path,
		Timestamp: time.Now(),
		Conflict:  conflict,
	})

	e.logger.Info("Conflict resolved", "path", path, "resolution", resolution)
	return nil
}

// SubscribeEvents subscribes to sync events.
func (e *Engine) SubscribeEvents() chan SyncEvent {
	ch := make(chan SyncEvent, 100)
	e.eventMu.Lock()
	e.eventSubscribers = append(e.eventSubscribers, ch)
	e.eventMu.Unlock()
	return ch
}

// UnsubscribeEvents unsubscribes from sync events.
func (e *Engine) UnsubscribeEvents(ch chan SyncEvent) {
	e.eventMu.Lock()
	defer e.eventMu.Unlock()
	for i, sub := range e.eventSubscribers {
		if sub == ch {
			e.eventSubscribers = append(e.eventSubscribers[:i], e.eventSubscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

// syncLoop runs the periodic sync loop.
func (e *Engine) syncLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			if e.getStatus() == SyncStatusPaused {
				continue
			}
			if err := e.Sync(); err != nil {
				e.logger.Error("Periodic sync failed", "error", err)
			}
		}
	}
}

// getLocalFiles gets all files in the local directory.
func (e *Engine) getLocalFiles() (map[string]FileInfo, error) {
	files := make(map[string]FileInfo)

	err := filepath.WalkDir(e.config.LocalDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded patterns
		if e.isExcluded(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip sync state file
		if strings.HasSuffix(path, ".sync_state.json") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(e.config.LocalDir, path)
		if err != nil {
			return err
		}

		files[relPath] = FileInfo{
			Path:         path,
			RelativePath: relPath,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			IsDir:        d.IsDir(),
		}

		return nil
	})

	return files, err
}

// getRemoteFiles gets all files in the remote directory.
func (e *Engine) getRemoteFiles() (map[string]FileInfo, error) {
	files := make(map[string]FileInfo)

	// Perform PROPFIND to get remote files
	resp, err := e.client.PropFind(e.ctx, e.config.RemotePath, 1, `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:">
	<D:prop>
		<D:resourcetype/>
		<D:displayname/>
		<D:getcontenttype/>
		<D:getcontentlength/>
		<D:getetag/>
		<D:getlastmodified/>
		<D:creationdate/>
	</D:prop>
</D:propfind>`)
	if err != nil {
		return nil, err
	}

	for _, r := range resp.Responses {
		info := r.ToResourceInfo()

		// Skip excluded patterns
		if e.isExcluded(info.Path) {
			continue
		}

		relPath := strings.TrimPrefix(info.Path, e.config.RemotePath)
		relPath = strings.TrimPrefix(relPath, "/")

		if relPath == "" {
			continue
		}

		files[relPath] = FileInfo{
			Path:         info.Path,
			RelativePath: relPath,
			Size:         info.Size,
			ETag:         info.ETag,
			LastModified: info.LastModified,
			IsDir:        info.IsCollection,
		}
	}

	return files, nil
}

// determineSyncActions determines what sync actions are needed.
func (e *Engine) determineSyncActions(localFiles, remoteFiles map[string]FileInfo) ([]SyncAction, error) {
	var actions []SyncAction

	// Check for files to upload (local only or modified)
	for relPath, localInfo := range localFiles {
		if localInfo.IsDir {
			// Create directory if needed
			if _, exists := remoteFiles[relPath]; !exists {
				actions = append(actions, SyncAction{
					Type:      "mkcol",
					Path:      relPath,
					Direction: "local->remote",
				})
			}
			continue
		}

		remoteInfo, exists := remoteFiles[relPath]
		if !exists {
			// New file - upload
			actions = append(actions, SyncAction{
				Type:      "upload",
				Path:      relPath,
				Direction: "local->remote",
			})
		} else {
			// Check if modified
			if e.isModified(localInfo, remoteInfo) {
				actions = append(actions, SyncAction{
					Type:      "upload",
					Path:      relPath,
					Direction: "local->remote",
					Conflict:  true,
				})
			}
		}
	}

	// Check for files to download (remote only or modified)
	if e.config.SyncMode == SyncModeBidirectional || e.config.SyncMode == SyncModeDownload {
		for relPath, remoteInfo := range remoteFiles {
			if remoteInfo.IsDir {
				continue
			}

			localInfo, exists := localFiles[relPath]
			if !exists {
				// New file - download
				actions = append(actions, SyncAction{
					Type:      "download",
					Path:      relPath,
					Direction: "remote->local",
				})
			} else {
				// Check if modified
				if e.isModified(remoteInfo, localInfo) {
					// Only add if not already in actions
					alreadyAdded := false
					for _, a := range actions {
						if a.Path == relPath && a.Type == "upload" {
							alreadyAdded = true
							break
						}
					}
					if !alreadyAdded {
						actions = append(actions, SyncAction{
							Type:      "download",
							Path:      relPath,
							Direction: "remote->local",
							Conflict:  true,
						})
					}
				}
			}
		}
	}

	// Check for files to delete
	// (Implementation depends on delete strategy)

	return actions, nil
}

// executeAction executes a sync action.
func (e *Engine) executeAction(action SyncAction) error {
	switch action.Type {
	case "upload":
		return e.uploadFile(action.Path)
	case "download":
		return e.downloadFile(action.Path)
	case "delete":
		return e.deleteFile(action.Path)
	case "mkcol":
		return e.client.MkCol(e.ctx, filepath.Join(e.config.RemotePath, action.Path))
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// uploadFile uploads a file to the remote server.
func (e *Engine) uploadFile(relPath string) error {
	localPath := filepath.Join(e.config.LocalDir, relPath)
	remotePath := filepath.Join(e.config.RemotePath, relPath)

	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	if err := e.client.Put(e.ctx, remotePath, data); err != nil {
		return err
	}

	// Update state
	e.state.SetLocalETag(relPath, calculateETag(data))
	e.state.SetLastSync(relPath, time.Now())

	e.broadcastEvent(SyncEvent{
		Type:      "upload",
		Path:      relPath,
		Direction: "local->remote",
		Timestamp: time.Now(),
		Size:      int64(len(data)),
	})

	e.logger.Debug("File uploaded", "path", relPath, "size", len(data))
	return nil
}

// downloadFile downloads a file from the remote server.
func (e *Engine) downloadFile(relPath string) error {
	remotePath := filepath.Join(e.config.RemotePath, relPath)
	localPath := filepath.Join(e.config.LocalDir, relPath)

	data, err := e.client.Get(e.ctx, remotePath)
	if err != nil {
		return err
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(localPath, data, 0o600); err != nil {
		return err
	}

	// Update state
	e.state.SetRemoteETag(relPath, calculateETag(data))
	e.state.SetLastSync(relPath, time.Now())

	e.broadcastEvent(SyncEvent{
		Type:      "download",
		Path:      relPath,
		Direction: "remote->local",
		Timestamp: time.Now(),
		Size:      int64(len(data)),
	})

	e.logger.Debug("File downloaded", "path", relPath, "size", len(data))
	return nil
}

// deleteFile deletes a file.
func (e *Engine) deleteFile(relPath string) error {
	// Implementation depends on delete strategy
	return nil
}

// createBackup creates a backup of a file.
func (e *Engine) createBackup(relPath string) error {
	localPath := filepath.Join(e.config.LocalDir, relPath)
	backupPath := localPath + ".backup." + time.Now().Format("20060102150405")

	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	return os.WriteFile(backupPath, data, 0o600)
}

// isModified checks if a file has been modified.
func (e *Engine) isModified(local, remote FileInfo) bool {
	// Check ETag first
	if local.ETag != "" && remote.ETag != "" {
		return local.ETag != remote.ETag
	}

	// Fall back to modification time
	return !local.LastModified.Equal(remote.LastModified)
}

// isExcluded checks if a path should be excluded from sync.
func (e *Engine) isExcluded(path string) bool {
	for _, pattern := range e.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

// getStatus gets the current status.
func (e *Engine) getStatus() SyncStatus {
	e.statusMu.RLock()
	defer e.statusMu.RUnlock()
	return e.status
}

// setStatus sets the current status.
func (e *Engine) setStatus(status SyncStatus) {
	e.statusMu.Lock()
	defer e.statusMu.Unlock()
	e.status = status
}

// getLastSync gets the last sync time.
func (e *Engine) getLastSync() time.Time {
	e.lastSyncMu.RLock()
	defer e.lastSyncMu.RUnlock()
	return e.lastSync
}

// setLastSync sets the last sync time.
func (e *Engine) setLastSync(t time.Time) {
	e.lastSyncMu.Lock()
	defer e.lastSyncMu.Unlock()
	e.lastSync = t
}

// broadcastEvent broadcasts an event to all subscribers.
func (e *Engine) broadcastEvent(event SyncEvent) {
	e.eventMu.RLock()
	defer e.eventMu.RUnlock()
	for _, ch := range e.eventSubscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

// SyncAction represents a sync action to be executed.
type SyncAction struct {
	Type      string // upload, download, delete, mkcol
	Path      string
	Direction string // local->remote, remote->local
	Conflict  bool
}

// Helper functions
func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	var wdErr *webdav.Error
	if errors.As(err, &wdErr) {
		return wdErr.IsConflict()
	}
	return strings.Contains(err.Error(), "conflict")
}

func extractConflictInfo(err error) *ConflictInfo {
	// Extract conflict info from error
	return &ConflictInfo{}
}

func calculateETag(data []byte) string {
	// Simple ETag calculation (in production, use proper hash)
	return fmt.Sprintf("\"%x\"", data)
}
