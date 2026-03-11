package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/crush/internal/home"
)

// andyCodeAgentConfig is the TOML structure for a single agent file (andy-code format).
type andyCodeAgentConfig struct {
	Name         string   `toml:"name"`
	Description  string   `toml:"description"`
	Tools        []string `toml:"tools"`
	SystemPrompt string   `toml:"system_prompt"`
}

// andyCodeToolMap maps andy-code tool names to crush tool names.
var andyCodeToolMap = map[string]string{
	"read_file":  "view",
	"list_dir":    "ls",
	"write_file": "write",
	"grep":        "grep",
	"glob":        "glob",
	"edit":        "edit",
	"web_fetch":   "fetch",
	"web_search":  "sourcegraph",
	"todowrite":   "todos",
	"todoread":   "todos",
}

// mapAndyCodeTools maps andy-code tool names to crush tool names.
// Unknown tools are dropped. If result is empty, caller should use allToolNames().
func mapAndyCodeTools(tools []string) []string {
	if len(tools) == 0 {
		return nil // caller interprets as "all tools"
	}
	var result []string
	crushNames := allToolNames()
	crushSet := make(map[string]bool)
	for _, t := range crushNames {
		crushSet[t] = true
	}
	for _, t := range tools {
		if mapped, ok := andyCodeToolMap[t]; ok && crushSet[mapped] {
			result = append(result, mapped)
		}
	}
	return result
}

// resolveAgentsDir returns the agents directory path.
// Priority: CRUSH_AGENTS_DIR env > Options.AgentsDir > ~/.andy-code/agents
func resolveAgentsDir(cfg *Config) string {
	if dir := os.Getenv("CRUSH_AGENTS_DIR"); dir != "" {
		return expandPath(dir)
	}
	if cfg.Options != nil && cfg.Options.AgentsDir != "" {
		return expandPath(cfg.Options.AgentsDir)
	}
	return expandPath(filepath.Join(home.Dir(), ".andy-code", "agents"))
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home.Dir(), p[2:])
	}
	return p
}

// LoadAgentsFromDir loads agent configurations from *.toml files in the given directory.
// Each file defines one agent (andy-code format). Parsing failures are logged and skipped;
// only successfully parsed agents are returned. Never panics.
func LoadAgentsFromDir(dir string, disabledTools []string) map[string]Agent {
	result := make(map[string]Agent)
	if dir == "" {
		return result
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("Failed to read agents directory", "dir", dir, "error", err)
		}
		return result
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".toml") {
			continue
		}

		path := filepath.Join(dir, e.Name())
		id := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if id == "" {
			continue
		}

		agent, err := loadAgentFile(path, id, disabledTools)
		if err != nil {
			slog.Warn("Skipping agent file", "path", path, "error", err)
			continue
		}
		result[id] = agent
	}

	return result
}

// loadAgentFile loads a single agent from a TOML file.
func loadAgentFile(path, id string, disabledTools []string) (Agent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Agent{}, err
	}

	var raw andyCodeAgentConfig
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return Agent{}, err
	}

	name := raw.Name
	if name == "" {
		name = id
	}

	allowedTools := mapAndyCodeTools(raw.Tools)
	if allowedTools == nil {
		allowedTools = allToolNames()
	}
	allowedTools = resolveAllowedTools(allowedTools, disabledTools)
	if len(allowedTools) == 0 {
		allowedTools = allToolNames()
	}

	return Agent{
		ID:           id,
		Name:         name,
		Description:  raw.Description,
		Model:        SelectedModelTypeLarge,
		AllowedTools: allowedTools,
		SystemPrompt: strings.TrimSpace(raw.SystemPrompt),
	}, nil
}
