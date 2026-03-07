package model

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func TestDefaultKeyMapForScheme(t *testing.T) {
	t.Run("standalone matches ctrl+c", func(t *testing.T) {
		km := DefaultKeyMapForScheme("standalone")
		msg := tea.KeyPressMsg{Key: tea.KeyCtrlC}
		if !key.Matches(msg, km.Quit) {
			t.Error("standalone Quit should match ctrl+c")
		}
	})

	t.Run("ide matches alt+q", func(t *testing.T) {
		km := DefaultKeyMapForScheme("ide")
		msg := tea.KeyPressMsg{Key: tea.KeyAltQ}
		if !key.Matches(msg, km.Quit) {
			t.Error("ide Quit should match alt+q")
		}
	})

	t.Run("ide does not match ctrl+c", func(t *testing.T) {
		km := DefaultKeyMapForScheme("ide")
		msg := tea.KeyPressMsg{Key: tea.KeyCtrlC}
		if key.Matches(msg, km.Quit) {
			t.Error("ide Quit should not match ctrl+c")
		}
	})

	t.Run("unknown scheme falls back to standalone", func(t *testing.T) {
		km := DefaultKeyMapForScheme("unknown")
		msg := tea.KeyPressMsg{Key: tea.KeyCtrlC}
		if !key.Matches(msg, km.Quit) {
			t.Error("unknown scheme should fallback to standalone (ctrl+c)")
		}
	})
}
