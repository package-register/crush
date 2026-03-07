package completions

import (
	"charm.land/bubbles/v2/key"
)

// KeyMap defines the key bindings for the completions component.
type KeyMap struct {
	Down,
	Up,
	Select,
	Cancel key.Binding
	DownInsert,
	UpInsert key.Binding
}

// DefaultKeyMap returns the default (standalone) key bindings for completions.
func DefaultKeyMap() KeyMap {
	return defaultKeyMapForScheme("standalone")
}

// DefaultKeyMapForScheme returns key bindings for the given scheme.
func DefaultKeyMapForScheme(scheme string) KeyMap {
	return defaultKeyMapForScheme(scheme)
}

func defaultKeyMapForScheme(scheme string) KeyMap {
	mod := "ctrl"
	if scheme == "ide" {
		mod = "alt"
	}
	downInsKeys := mod + "+n"
	upInsKeys := mod + "+p"
	var selKeys []string
	if scheme == "ide" {
		selKeys = []string{"enter", "tab", "alt+y"}
	} else {
		selKeys = []string{"enter", "tab", "ctrl+y"}
	}
	return KeyMap{
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("down", "move down"),
		),
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("up", "move up"),
		),
		Select: key.NewBinding(
			key.WithKeys(selKeys...),
			key.WithHelp("enter", "select"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "cancel"),
		),
		DownInsert: key.NewBinding(
			key.WithKeys(downInsKeys),
			key.WithHelp(downInsKeys, "insert next"),
		),
		UpInsert: key.NewBinding(
			key.WithKeys(upInsKeys),
			key.WithHelp(upInsKeys, "insert previous"),
		),
	}
}

// KeyBindings returns all key bindings as a slice.
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Down,
		k.Up,
		k.Select,
		k.Cancel,
	}
}

// FullHelp returns the full help for the key bindings.
func (k KeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := k.KeyBindings()
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp returns the short help for the key bindings.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
	}
}
