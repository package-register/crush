package dialog

import (
	"strings"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/sahilm/fuzzy"
)

// CommandItem wraps a uicmd.Command to implement the ListItem interface.
type CommandItem struct {
	id       string
	title    string
	shortcut string
	action   Action
	aliases  string // extra words for filtering (e.g. "exit" for quit)
	t        *styles.Styles
	m        fuzzy.Match
	cache    map[int]string
	focused  bool
}

var _ ListItem = &CommandItem{}

// NewCommandItem creates a new CommandItem.
// Aliases are optional extra filter terms (e.g. "exit" for quit).
func NewCommandItem(t *styles.Styles, id, title, shortcut string, action Action, aliases ...string) *CommandItem {
	var aliasStr string
	if len(aliases) > 0 {
		aliasStr = " " + strings.Join(aliases, " ")
	}
	return &CommandItem{
		id:       id,
		t:        t,
		title:    title,
		shortcut: shortcut,
		action:   action,
		aliases:  aliasStr,
	}
}

// Filter implements ListItem.
func (c *CommandItem) Filter() string {
	return c.title + c.aliases
}

// ID implements ListItem.
func (c *CommandItem) ID() string {
	return c.id
}

// SetFocused implements ListItem.
func (c *CommandItem) SetFocused(focused bool) {
	if c.focused != focused {
		c.cache = nil
	}
	c.focused = focused
}

// SetMatch implements ListItem.
func (c *CommandItem) SetMatch(m fuzzy.Match) {
	c.cache = nil
	c.m = m
}

// Action returns the action associated with the command item.
func (c *CommandItem) Action() Action {
	return c.action
}

// Shortcut returns the shortcut associated with the command item.
func (c *CommandItem) Shortcut() string {
	return c.shortcut
}

// Render implements ListItem.
func (c *CommandItem) Render(width int) string {
	styles := ListItemStyles{
		ItemBlurred:     c.t.Dialog.NormalItem,
		ItemFocused:     c.t.Dialog.SelectedItem,
		InfoTextBlurred: c.t.Base,
		InfoTextFocused: c.t.Base,
	}
	return renderItem(styles, c.title, c.shortcut, c.focused, width, c.cache, &c.m)
}
