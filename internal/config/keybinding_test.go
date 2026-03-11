package config

import (
	"os"
	"testing"
)

func TestEffectiveKeybindingScheme(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		env      map[string]string
		expected string
	}{
		{
			name: "explicit ide",
			cfg: &Config{
				Options: &Options{
					TUI: &TUIOptions{KeybindingScheme: "ide"},
				},
			},
			expected: "ide",
		},
		{
			name: "explicit standalone",
			cfg: &Config{
				Options: &Options{
					TUI: &TUIOptions{KeybindingScheme: "standalone"},
				},
			},
			expected: "standalone",
		},
		{
			name: "auto with vscode",
			cfg: &Config{
				Options: &Options{
					TUI: &TUIOptions{KeybindingScheme: "auto"},
				},
			},
			env:      map[string]string{"TERM_PROGRAM": "vscode"},
			expected: "ide",
		},
		{
			name: "auto without vscode",
			cfg: &Config{
				Options: &Options{
					TUI: &TUIOptions{KeybindingScheme: "auto"},
				},
			},
			env:      map[string]string{"TERM_PROGRAM": "Apple_Terminal"},
			expected: "standalone",
		},
		{
			name:     "nil options defaults to auto then standalone",
			cfg:      &Config{Options: nil},
			env:      map[string]string{"TERM_PROGRAM": ""}, // ensure not vscode
			expected: "standalone",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env for tests that modify it
			oldEnv := make(map[string]string)
			for k, v := range tt.env {
				oldEnv[k] = os.Getenv(k)
				os.Setenv(k, v)
			}
			defer func() {
				for k, v := range oldEnv {
					os.Setenv(k, v)
				}
			}()
			got := tt.cfg.EffectiveKeybindingScheme()
			if got != tt.expected {
				t.Errorf("EffectiveKeybindingScheme() = %q, want %q", got, tt.expected)
			}
		})
	}
}
