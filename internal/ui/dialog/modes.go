package dialog

import (
	"slices"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	// ModesID is the identifier for the agent mode selection dialog.
	ModesID               = "modes"
	modesDialogMaxWidth   = 55
	modesDialogMaxHeight = 12
)

// Modes represents a dialog for selecting the active agent mode.
type Modes struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

// NewModes creates a new Modes dialog.
func NewModes(com *common.Common) (*Modes, error) {
	m := &Modes{com: com}

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	m.help = help

	m.list = list.NewFilterableList()
	m.list.Focus()

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Type to filter"
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	m.keyMap.Close = CloseKey

	if err := m.setModeItems(); err != nil {
		return nil, err
	}

	return m, nil
}

// ID implements Dialog.
func (m *Modes) ID() string {
	return ModesID
}

// HandleMsg implements [Dialog].
func (m *Modes) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Previous):
			m.list.Focus()
			if m.list.IsSelectedFirst() {
				m.list.SelectLast()
				m.list.ScrollToBottom()
				break
			}
			m.list.SelectPrev()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Next):
			m.list.Focus()
			if m.list.IsSelectedLast() {
				m.list.SelectFirst()
				m.list.ScrollToTop()
				break
			}
			m.list.SelectNext()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Select):
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			modeItem, ok := selectedItem.(*ModeItem)
			if !ok {
				break
			}
			return ActionSelectMode{ModeID: modeItem.ModeID()}
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			value := m.input.Value()
			m.list.SetFilter(value)
			m.list.ScrollToTop()
			m.list.SetSelected(0)
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (m *Modes) Cursor() *tea.Cursor {
	return InputCursor(m.com.Styles, m.input.Cursor())
}

// Draw implements [Dialog].
func (m *Modes) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(modesDialogMaxWidth, area.Dx()))
	height := max(0, min(modesDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	m.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	m.list.SetSize(innerWidth, height-heightOffset)
	m.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Switch Agent Mode"
	inputView := t.Dialog.InputPrompt.Render(m.input.View())
	rc.AddPart(inputView)

	visibleCount := len(m.list.FilteredItems())
	if m.list.Height() >= visibleCount {
		m.list.ScrollToTop()
	} else {
		m.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
	rc.AddPart(listView)
	rc.Help = m.help.View(m)

	view := rc.Render()

	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (m *Modes) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.UpDown,
		m.keyMap.Select,
		m.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (m *Modes) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{m.keyMap.Select, m.keyMap.Next, m.keyMap.Previous, m.keyMap.Close},
	}
}

func (m *Modes) setModeItems() error {
	cfg := m.com.Config()
	activeMode := config.AgentCoder
	if cfg.Options != nil && cfg.Options.ActiveMode != "" {
		activeMode = cfg.Options.ActiveMode
	}

	ids := make([]string, 0, len(cfg.Agents))
	for id, agent := range cfg.Agents {
		if agent.Disabled {
			continue
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)

	items := make([]list.FilterableItem, 0, len(ids))
	selectedIndex := 0
	for i, id := range ids {
		agent := cfg.Agents[id]
		item := NewModeItem(m.com.Styles, id, agent)
		items = append(items, item)
		if id == activeMode {
			selectedIndex = i
		}
	}

	m.list.SetItems(items...)
	m.list.SetSelected(selectedIndex)
	m.list.ScrollToSelected()
	return nil
}
