package styles

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDefaultStyles_TextInputUsesBarCursor(t *testing.T) {
	s := DefaultStyles()
	if s.TextInput.Cursor.Shape != tea.CursorBar {
		t.Errorf("TextInput cursor shape = %v, want %v (thin bar cursor)", s.TextInput.Cursor.Shape, tea.CursorBar)
	}
}

func TestDefaultStyles_TextAreaUsesBarCursor(t *testing.T) {
	s := DefaultStyles()
	if s.TextArea.Cursor.Shape != tea.CursorBar {
		t.Errorf("TextArea cursor shape = %v, want %v (thin bar cursor)", s.TextArea.Cursor.Shape, tea.CursorBar)
	}
}
