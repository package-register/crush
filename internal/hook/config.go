package hook

import (
	"encoding/json"
	"fmt"
	"time"
)

// PluginConfig holds configuration for a single hook plugin.
type PluginConfig struct {
	// Name is the plugin name.
	Name string `json:"name"`
	// Path is the path to the plugin (.so file) or built-in hook name.
	Path string `json:"path"`
	// Config is the plugin-specific configuration.
	Config json.RawMessage `json:"config,omitempty"`
}

// RemoteConfig holds configuration for remote hook endpoints.
type RemoteConfig struct {
	// Enabled enables remote hook calls.
	Enabled bool `json:"enabled"`
	// Endpoint is the gRPC/HTTP endpoint URL.
	Endpoint string `json:"endpoint"`
	// Events is the list of events to send (empty = all events).
	Events []EventType `json:"events,omitempty"`
	// Timeout is the timeout for remote calls.
	Timeout time.Duration `json:"timeout,omitempty"`
}

// SystemConfig holds the complete hook system configuration.
type SystemConfig struct {
	// Enabled enables the hook system.
	Enabled bool `json:"enabled"`
	// Async executes hooks asynchronously if true.
	Async bool `json:"async,omitempty"`
	// Timeout is the maximum time to wait for hook execution.
	Timeout time.Duration `json:"timeout,omitempty"`
	// SkipOnError continues execution even if a hook fails.
	SkipOnError bool `json:"skip_on_error,omitempty"`
	// Plugins is the list of hook plugins to load.
	Plugins []PluginConfig `json:"plugins,omitempty"`
	// Remote holds configuration for remote hook endpoints.
	Remote RemoteConfig `json:"remote,omitempty"`
}

// DefaultSystemConfig returns the default hook system configuration.
func DefaultSystemConfig() SystemConfig {
	return SystemConfig{
		Enabled:     false,
		Async:       true,
		Timeout:     5 * time.Second,
		SkipOnError: true,
	}
}

// Validate validates the configuration.
func (c *SystemConfig) Validate() error {
	if c.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}
	if c.Remote.Enabled && c.Remote.Endpoint == "" {
		return fmt.Errorf("remote endpoint is required when remote hooks are enabled")
	}
	return nil
}

// ToManagerConfig converts to hook.Manager config.
func (c *SystemConfig) ToManagerConfig() ManagerConfig {
	return ManagerConfig{
		Enabled:     c.Enabled,
		Async:       c.Async,
		Timeout:     c.Timeout,
		SkipOnError: c.SkipOnError,
	}
}
