package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/home"
)

const modesConfigName = "modes.toml"

// ModeConfig represents a single mode loaded from TOML.
type ModeConfig struct {
	ID           string              `toml:"id"`
	Name         string              `toml:"name"`
	Description  string              `toml:"description"`
	Model        string              `toml:"model"` // "large" or "small"
	AllowedTools []string            `toml:"allowed_tools"`
	AllowedMCP   map[string][]string `toml:"allowed_mcp"`
	ContextPaths []string            `toml:"context_paths"`
}

// ModesConfig is the root structure of modes.toml.
type ModesConfig struct {
	Modes map[string]ModeConfig `toml:"modes"`
}

// globalModesConfigPath returns the path to the global modes.toml.
func globalModesConfigPath() string {
	if crushGlobal := os.Getenv("CRUSH_GLOBAL_CONFIG"); crushGlobal != "" {
		return filepath.Join(crushGlobal, modesConfigName)
	}
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, appName, modesConfigName)
	}
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, appName, modesConfigName)
	}
	return filepath.Join(home.Dir(), ".config", appName, modesConfigName)
}

// lookupModesConfigs searches for modes.toml from CWD up to FS root,
// and in global config directories.
func lookupModesConfigs(cwd string) []string {
	configNames := []string{
		modesConfigName,
		"." + appName + "/" + modesConfigName,
	}
	found, err := fsext.Lookup(cwd, configNames...)
	if err != nil {
		return nil
	}
	slices.Reverse(found)

	// Prepend global config path if it exists
	globalPath := globalModesConfigPath()
	if data, err := os.ReadFile(globalPath); err == nil && len(data) > 0 {
		found = append([]string{globalPath}, found...)
	}
	return found
}

// LoadModesFromTOML loads mode configurations from modes.toml files.
// Returns a map of mode ID -> Agent. Modes from later paths override earlier ones.
func LoadModesFromTOML(cwd string, baseAgents map[string]Agent, disabledTools []string) map[string]Agent {
	paths := lookupModesConfigs(cwd)
	if len(paths) == 0 {
		return baseAgents
	}

	result := make(map[string]Agent)
	for k, v := range baseAgents {
		result[k] = v
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var cfg ModesConfig
		if _, err := toml.Decode(string(data), &cfg); err != nil {
			continue
		}

		for id, m := range cfg.Modes {
			if m.ID == "" {
				m.ID = id
			}
			if m.Name == "" {
				m.Name = id
			}
			if m.Model == "" {
				m.Model = string(SelectedModelTypeLarge)
			}
			modelType := SelectedModelTypeLarge
			if m.Model == "small" {
				modelType = SelectedModelTypeSmall
			}

			allowedTools := m.AllowedTools
			if allowedTools == nil {
				allowedTools = allToolNames()
			}
			allowedTools = resolveAllowedTools(allowedTools, disabledTools)

			agent := Agent{
				ID:           m.ID,
				Name:         m.Name,
				Description:  m.Description,
				Model:        modelType,
				AllowedTools: allowedTools,
				AllowedMCP:   m.AllowedMCP,
				ContextPaths: m.ContextPaths,
			}
			result[id] = agent
		}
	}

	return result
}

// DefaultModeConfigs returns built-in mode presets (Git, Rust, Plan).
// These are used when no modes.toml is found, or as fallback.
func DefaultModeConfigs() map[string]Agent {
	return map[string]Agent{
		AgentGit: {
			ID:          AgentGit,
			Name:        "Git",
			Description: "Agent focused on Git operations and version control.",
			Model:       SelectedModelTypeLarge,
			AllowedTools: []string{
				"bash", "edit", "view", "grep", "glob", "ls",
				"fetch", "write", "multiedit",
				"list_mcp_resources", "read_mcp_resource",
			},
			AllowedMCP: nil,
		},
		AgentRust: {
			ID:          AgentRust,
			Name:        "Rust",
			Description: "Agent focused on Rust development with LSP support.",
			Model:       SelectedModelTypeLarge,
			AllowedTools: []string{
				"bash", "edit", "view", "grep", "glob", "ls",
				"lsp_diagnostics", "lsp_references", "lsp_restart",
				"fetch", "write", "multiedit", "todos",
				"list_mcp_resources", "read_mcp_resource",
			},
			AllowedMCP: nil,
		},
		AgentPlan: {
			ID:          AgentPlan,
			Name:        "Plan",
			Description: "Agent focused on planning and task breakdown.",
			Model:       SelectedModelTypeLarge,
			AllowedTools: []string{
				"view", "grep", "glob", "ls", "sourcegraph",
				"list_mcp_resources", "read_mcp_resource",
			},
			AllowedMCP: nil,
		},
	}
}

// ValidateModeID returns an error if the mode ID is unknown.
func (c *Config) ValidateModeID(modeID string) error {
	if _, ok := c.Agents[modeID]; ok {
		return nil
	}
	return fmt.Errorf("unknown mode: %s", modeID)
}
