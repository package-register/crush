package model

import (
	"testing"

	"charm.land/bubbles/v2/key"
)

// testKey implements fmt.Stringer for key.Matches in tests.
// Bubble Tea v2 KeyPressMsg has no Key field; Matches uses String().
type testKey string

func (k testKey) String() string { return string(k) }

func TestDefaultKeyMapForScheme(t *testing.T) {
	t.Run("standalone matches ctrl+c", func(t *testing.T) {
		km := DefaultKeyMapForScheme("standalone")
		msg := testKey("ctrl+c")
		if !key.Matches(msg, km.Quit) {
			t.Error("standalone Quit should match ctrl+c")
		}
	})

	t.Run("ide matches alt+q", func(t *testing.T) {
		km := DefaultKeyMapForScheme("ide")
		msg := testKey("alt+q")
		if !key.Matches(msg, km.Quit) {
			t.Error("ide Quit should match alt+q")
		}
	})

	t.Run("ide does not match ctrl+c", func(t *testing.T) {
		km := DefaultKeyMapForScheme("ide")
		msg := testKey("ctrl+c")
		if key.Matches(msg, km.Quit) {
			t.Error("ide Quit should not match ctrl+c")
		}
	})

	t.Run("unknown scheme falls back to standalone", func(t *testing.T) {
		km := DefaultKeyMapForScheme("unknown")
		msg := testKey("ctrl+c")
		if !key.Matches(msg, km.Quit) {
			t.Error("unknown scheme should fallback to standalone (ctrl+c)")
		}
	})
}
