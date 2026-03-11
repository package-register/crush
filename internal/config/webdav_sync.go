package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/crush/internal/sync"
	"github.com/charmbracelet/crush/internal/webdav"
)

// WebDAVConfig holds WebDAV synchronization configuration.
type WebDAVConfig struct {
	// Enabled indicates whether WebDAV sync is enabled.
	Enabled bool `json:"enabled,omitempty" jsonschema:"description=Whether WebDAV sync is enabled,default=false"`
	// URL is the WebDAV server URL.
	URL string `json:"url,omitempty" jsonschema:"description=WebDAV server URL,format=uri,example=https://webdav.example.com/"`
	// Username for authentication.
	Username string `json:"username,omitempty" jsonschema:"description=Username for WebDAV authentication"`
	// Password for authentication (can be an environment variable reference).
	Password string `json:"password,omitempty" jsonschema:"description=Password for WebDAV authentication,example=${WEBDAV_PASSWORD}"`
	// Token for bearer token authentication.
	Token string `json:"token,omitempty" jsonschema:"description=Bearer token for WebDAV authentication"`
	// RemotePath is the remote directory path.
	RemotePath string `json:"remote_path,omitempty" jsonschema:"description=Remote directory path on WebDAV server,example=/crush-config"`
	// SyncInterval is how often to sync (e.g., "5m", "1h").
	SyncInterval string `json:"sync_interval,omitempty" jsonschema:"description=Sync interval (e.g., 5m, 1h), default is manual only,example=5m,example=1h"`
	// ConflictStrategy is how to handle conflicts.
	ConflictStrategy string `json:"conflict_strategy,omitempty" jsonschema:"description=Conflict resolution strategy,enum=newer-wins,enum=local-wins,enum=remote-wins,enum=backup,default=newer-wins"`
	// ExcludePatterns are file patterns to exclude from sync.
	ExcludePatterns []string `json:"exclude_patterns,omitempty" jsonschema:"description=File patterns to exclude from sync,example=*.tmp,example=*.lock"`
	// SkipTLSVerify disables TLS certificate verification (for testing only).
	SkipTLSVerify bool `json:"skip_tls_verify,omitempty" jsonschema:"description=Disable TLS certificate verification (for testing only),default=false"`
}

// WebDAVSync manages WebDAV synchronization for Crush configuration.
type WebDAVSync struct {
	config     *WebDAVConfig
	configDir  string
	engine     *sync.Engine
	client     *webdav.Client
	logger     *slog.Logger
	cancelFunc context.CancelFunc
}

// NewWebDAVSync creates a new WebDAV sync manager.
func NewWebDAVSync(config *WebDAVConfig, configDir string, logger *slog.Logger) (*WebDAVSync, error) {
	if config == nil {
		return nil, fmt.Errorf("WebDAV config is required")
	}
	if configDir == "" {
		return nil, fmt.Errorf("config directory is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	ws := &WebDAVSync{
		config:    config,
		configDir: configDir,
		logger:    logger,
	}

	return ws, nil
}

// Start starts WebDAV synchronization.
func (ws *WebDAVSync) Start(ctx context.Context) error {
	if !ws.config.Enabled {
		ws.logger.Info("WebDAV sync is disabled")
		return nil
	}

	ws.logger.Info("Starting WebDAV sync",
		"url", ws.config.URL,
		"remote_path", ws.config.RemotePath,
		"config_dir", ws.configDir,
	)

	// Create WebDAV client
	client, err := webdav.NewClient(ws.config.URL,
		webdav.WithSkipTLSVerify(ws.config.SkipTLSVerify),
	)
	if err != nil {
		return fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	// Set authentication
	if ws.config.Token != "" {
		client.SetToken(ws.config.Token)
	} else if ws.config.Username != "" {
		client.SetAuth(ws.config.Username, ws.config.Password)
	}

	ws.client = client

	// Test connection
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("failed to connect to WebDAV server: %w", err)
	}

	ws.logger.Info("Connected to WebDAV server")

	// Parse sync interval
	var syncInterval time.Duration
	if ws.config.SyncInterval != "" {
		syncInterval, err = time.ParseDuration(ws.config.SyncInterval)
		if err != nil {
			return fmt.Errorf("invalid sync interval: %w", err)
		}
	}

	// Parse conflict strategy
	conflictStrategy := parseConflictStrategy(ws.config.ConflictStrategy)

	// Create sync engine
	engine, err := sync.NewEngine(sync.Config{
		LocalDir:         ws.configDir,
		RemotePath:       ws.config.RemotePath,
		SyncMode:         sync.SyncModeBidirectional,
		ConflictStrategy: conflictStrategy,
		ExcludePatterns:  ws.config.ExcludePatterns,
		SyncInterval:     syncInterval,
		Logger:           ws.logger,
	}, client)
	if err != nil {
		return fmt.Errorf("failed to create sync engine: %w", err)
	}

	ws.engine = engine

	// Subscribe to events
	go ws.handleEvents(engine.SubscribeEvents())

	// Start engine
	if err := engine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start sync engine: %w", err)
	}

	return nil
}

// Stop stops WebDAV synchronization.
func (ws *WebDAVSync) Stop() error {
	if ws.engine == nil {
		return nil
	}

	ws.logger.Info("Stopping WebDAV sync")
	return ws.engine.Stop()
}

// Sync performs a manual sync.
func (ws *WebDAVSync) Sync() error {
	if ws.engine == nil {
		return fmt.Errorf("sync engine not started")
	}

	ws.logger.Info("Manual sync triggered")
	return ws.engine.Sync()
}

// Status returns the current sync status.
func (ws *WebDAVSync) Status() sync.SyncStatus {
	if ws.engine == nil {
		return sync.SyncStatusIdle
	}
	return ws.engine.Status()
}

// LastSyncTime returns the last successful sync time.
func (ws *WebDAVSync) LastSyncTime() time.Time {
	if ws.engine == nil {
		return time.Time{}
	}
	return ws.engine.LastSyncTime()
}

// ResolveConflict resolves a sync conflict.
func (ws *WebDAVSync) ResolveConflict(path string, strategy sync.ConflictStrategy) error {
	if ws.engine == nil {
		return fmt.Errorf("sync engine not started")
	}
	return ws.engine.ResolveConflict(path, strategy)
}

// handleEvents handles sync events.
func (ws *WebDAVSync) handleEvents(events <-chan sync.SyncEvent) {
	for event := range events {
		switch event.Type {
		case "upload":
			ws.logger.Debug("File uploaded", "path", event.Path, "size", event.Size)
		case "download":
			ws.logger.Debug("File downloaded", "path", event.Path, "size", event.Size)
		case "conflict":
			ws.logger.Warn("Sync conflict detected", "path", event.Path)
		case "error":
			ws.logger.Error("Sync error", "path", event.Path, "error", event.Error)
		case "conflict_resolved":
			ws.logger.Info("Conflict resolved", "path", event.Path, "resolution", event.Conflict.Resolution)
		}
	}
}

// parseConflictStrategy parses a conflict strategy string.
func parseConflictStrategy(s string) sync.ConflictStrategy {
	switch s {
	case "local-wins":
		return sync.ConflictStrategyLocalWins
	case "remote-wins":
		return sync.ConflictStrategyRemoteWins
	case "backup":
		return sync.ConflictStrategyBackup
	case "manual":
		return sync.ConflictStrategyManual
	default:
		return sync.ConflictStrategyNewerWins
	}
}

// GetSyncConfigFromEnv creates a WebDAV config from environment variables.
func GetSyncConfigFromEnv() *WebDAVConfig {
	url := os.Getenv("WEBDAV_URL")
	if url == "" {
		return nil
	}

	return &WebDAVConfig{
		Enabled:          true,
		URL:              url,
		Username:         os.Getenv("WEBDAV_USERNAME"),
		Password:         os.Getenv("WEBDAV_PASSWORD"),
		Token:            os.Getenv("WEBDAV_TOKEN"),
		RemotePath:       os.Getenv("WEBDAV_REMOTE_PATH"),
		SyncInterval:     os.Getenv("WEBDAV_SYNC_INTERVAL"),
		ConflictStrategy: os.Getenv("WEBDAV_CONFLICT_STRATEGY"),
		SkipTLSVerify:    os.Getenv("WEBDAV_SKIP_TLS_VERIFY") == "true",
	}
}

// EnsureSyncDir ensures the sync directory exists.
func EnsureSyncDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// GetDefaultSyncDir returns the default sync directory.
func GetDefaultSyncDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".crush"
	}
	return filepath.Join(homeDir, ".crush")
}
