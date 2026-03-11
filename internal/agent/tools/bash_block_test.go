package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/shell"
)

func TestBlockFuncs_WithAllowedCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		bashOpts    *config.BashOptions
		command     string
		shouldBlock bool
	}{
		{
			name:        "go install blocked by default",
			bashOpts:    nil,
			command:     "go install github.com/some/pkg@latest",
			shouldBlock: true,
		},
		{
			name: "go install allowed via config",
			bashOpts: &config.BashOptions{
				AllowedCommands: []string{"go install"},
			},
			command:     "go install github.com/some/pkg@latest",
			shouldBlock: false,
		},
		{
			name: "go test -exec allowed via config",
			bashOpts: &config.BashOptions{
				AllowedCommands: []string{"go test -exec"},
			},
			command:     "go test -exec 'echo hi' ./...",
			shouldBlock: false,
		},
		{
			name:        "go test -exec blocked by default",
			bashOpts:    nil,
			command:     "go test -exec 'echo hi' ./...",
			shouldBlock: true,
		},
		{
			name: "npm install -g allowed via config",
			bashOpts: &config.BashOptions{
				AllowedCommands: []string{"npm install -g"},
			},
			command:     "npm install -g some-package",
			shouldBlock: false,
		},
		{
			name:        "sudo still blocked",
			bashOpts:    &config.BashOptions{AllowedCommands: []string{"go install"}},
			command:     "sudo apt update",
			shouldBlock: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sh := shell.NewShell(&shell.Options{
				WorkingDir: t.TempDir(),
				BlockFuncs: blockFuncs(tt.bashOpts),
			})
			_, _, err := sh.Exec(t.Context(), tt.command)
			blocked := err != nil && (err.Error() == "command is not allowed for security reasons: \"go\"" ||
				err.Error() == "command is not allowed for security reasons: \"npm\"" ||
				err.Error() == "command is not allowed for security reasons: \"sudo\"")
			if blocked != tt.shouldBlock {
				t.Errorf("command %q: expected blocked=%v, got blocked=%v (err=%v)", tt.command, tt.shouldBlock, blocked, err)
			}
		})
	}
}
