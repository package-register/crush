package dialog

import (
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// ModeItem wraps an agent mode for the modes selection list.
type ModeItem struct {
	modeID  string
	agent   config.Agent
	t       *styles.Styles
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var _ ListItem = (*ModeItem)(nil)

// NewModeItem creates a new ModeItem. modeID is the agent ID from the config map key.
func NewModeItem(t *styles.Styles, modeID string, agent config.Agent) *ModeItem {
	return &ModeItem{
		modeID: modeID,
		agent:  agent,
		t:      t,
		cache:  make(map[int]string),
	}
}

// Filter implements ListItem.
func (m *ModeItem) Filter() string {
	name := m.agent.Name
	if name == "" {
		name = m.modeID
	}
	return name + " " + m.agent.Description
}

// ID implements ListItem.
func (m *ModeItem) ID() string {
	return m.modeID
}

// SetFocused implements ListItem.
func (m *ModeItem) SetFocused(focused bool) {
	if m.focused != focused {
		m.cache = nil
	}
	m.focused = focused
}

// SetMatch implements ListItem.
func (m *ModeItem) SetMatch(fm fuzzy.Match) {
	m.cache = nil
	m.m = fm
}

// ModeID returns the agent mode ID.
func (m *ModeItem) ModeID() string {
	return m.modeID
}

// Render implements ListItem.
func (m *ModeItem) Render(width int) string {
	title := m.agent.Name
	if title == "" {
		title = m.modeID
	}
	info := ansi.Truncate(m.agent.Description, max(0, width/2), "…")
	styles := ListItemStyles{
		ItemBlurred:     m.t.Dialog.NormalItem,
		ItemFocused:     m.t.Dialog.SelectedItem,
		InfoTextBlurred: m.t.Subtle,
		InfoTextFocused: m.t.Base,
	}
	return renderItem(styles, title, info, m.focused, width, m.cache, &m.m)
}
