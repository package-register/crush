package dialog

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
)

// WebDAVConfigID is the identifier for the WebDAV configuration dialog.
const WebDAVConfigID = "webdav_config"

// WebDAVConfig represents the WebDAV configuration dialog.
type WebDAVConfig struct {
	com    *common.Common
	keyMap struct {
		Save    key.Binding
		Cancel  key.Binding
		Next    key.Binding
		Prev    key.Binding
		UpDown  key.Binding
		Confirm key.Binding
	}

	inputs     []textinput.Model
	focusIndex int

	width int
	help  help.Model
}

var _ Dialog = (*WebDAVConfig)(nil)

// NewWebDAVConfig creates a new WebDAV configuration dialog.
func NewWebDAVConfig(com *common.Common, webdavCfg *config.WebDAVConfig) (*WebDAVConfig, tea.Cmd) {
	t := com.Styles

	m := &WebDAVConfig{
		com:        com,
		focusIndex: 0,
		width:      70,
	}

	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize() - 2

	// Initialize text inputs
	inputLabels := []string{
		"URL:",
		"Username:",
		"Password:",
		"Remote Path:",
		"Sync Interval:",
		"Conflict Strategy:",
	}

	defaultValues := []string{
		"",
		"",
		"",
		"/crush-config",
		"5m",
		"newer-wins",
	}

	if webdavCfg != nil {
		if webdavCfg.URL != "" {
			defaultValues[0] = webdavCfg.URL
		}
		if webdavCfg.Username != "" {
			defaultValues[1] = webdavCfg.Username
		}
		if webdavCfg.Password != "" {
			defaultValues[2] = webdavCfg.Password
		}
		if webdavCfg.RemotePath != "" {
			defaultValues[3] = webdavCfg.RemotePath
		}
		if webdavCfg.SyncInterval != "" {
			defaultValues[4] = webdavCfg.SyncInterval
		}
		if webdavCfg.ConflictStrategy != "" {
			defaultValues[5] = webdavCfg.ConflictStrategy
		}
	}

	for i, label := range inputLabels {
		input := textinput.New()
		input.SetVirtualCursor(false)
		input.Placeholder = defaultValues[i]
		input.SetStyles(com.Styles.TextInput)
		input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-len(label)-2))
		input.SetValue(defaultValues[i])
		m.inputs = append(m.inputs, input)
	}

	// Focus the first input
	m.inputs[0].Focus()

	// Setup key bindings
	m.keyMap.Save = key.NewBinding(
		key.WithKeys("ctrl+s", "ctrl+y"),
		key.WithHelp("ctrl+s", "save"),
	)
	m.keyMap.Cancel = CloseKey
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("tab", "down"),
		key.WithHelp("tab/↓", "next field"),
	)
	m.keyMap.Prev = key.NewBinding(
		key.WithKeys("shift+tab", "up"),
		key.WithHelp("shift+tab/↑", "prev field"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "navigate"),
	)
	m.keyMap.Confirm = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "next/save"),
	)

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	m.help = h

	return m, nil
}

// ID implements Dialog.
func (m *WebDAVConfig) ID() string {
	return WebDAVConfigID
}

// HandleMsg implements Dialog.
func (m *WebDAVConfig) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Cancel):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Save):
			return m.saveConfig()
		case key.Matches(msg, m.keyMap.Next):
			m.focusIndex++
			if m.focusIndex >= len(m.inputs) {
				m.focusIndex = 0
			}
			return m.focusInput(m.focusIndex)
		case key.Matches(msg, m.keyMap.Prev):
			m.focusIndex--
			if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs) - 1
			}
			return m.focusInput(m.focusIndex)
		case key.Matches(msg, m.keyMap.Confirm):
			if m.focusIndex < len(m.inputs)-1 {
				m.focusIndex++
				return m.focusInput(m.focusIndex)
			}
			return m.saveConfig()
		default:
			var cmd tea.Cmd
			m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.PasteMsg:
		var cmd tea.Cmd
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
		if cmd != nil {
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Draw implements Dialog.
func (m *WebDAVConfig) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles

	textStyle := t.Dialog.SecondaryText
	dialogStyle := t.Dialog.View.Width(m.width)
	inputStyle := t.Dialog.InputPrompt
	helpStyle := t.Dialog.HelpView
	helpStyle = helpStyle.Width(m.width - dialogStyle.GetHorizontalFrameSize())

	var parts []string
	parts = append(parts, m.headerView())

	labels := []string{
		"URL:",
		"Username:",
		"Password:",
		"Remote Path:",
		"Sync Interval:",
		"Conflict Strategy:",
	}

	for i, input := range m.inputs {
		label := inputStyle.Render(labels[i] + " ")
		inputView := input.View()
		parts = append(parts, label+inputView)
	}

	parts = append(parts, "")
	parts = append(parts, textStyle.Render("• Sync Interval examples: 5m, 1h, 30m"))
	parts = append(parts, textStyle.Render("• Conflict Strategy: newer-wins, local-wins, remote-wins, backup"))
	parts = append(parts, "")
	parts = append(parts, helpStyle.Render(m.help.View(m)))

	content := strings.Join(parts, "\n")
	view := dialogStyle.Render(content)
	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (m *WebDAVConfig) headerView() string {
	var (
		t           = m.com.Styles
		titleStyle  = t.Dialog.Title
		dialogStyle = t.Dialog.View.Width(m.width)
	)
	headerOffset := titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
	return common.DialogTitle(t, titleStyle.Render("WebDAV Configuration"), m.width-headerOffset, m.com.Styles.Primary, m.com.Styles.Secondary)
}

// Cursor returns the cursor position relative to the dialog.
func (m *WebDAVConfig) Cursor() *tea.Cursor {
	return InputCursor(m.com.Styles, m.inputs[m.focusIndex].Cursor())
}

// FullHelp returns the full help view.
func (m *WebDAVConfig) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			m.keyMap.Save,
			m.keyMap.Next,
			m.keyMap.Prev,
			m.keyMap.Confirm,
		},
		{
			m.keyMap.Cancel,
		},
	}
}

// ShortHelp returns the short help view.
func (m *WebDAVConfig) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.Save,
		m.keyMap.Next,
		m.keyMap.Prev,
		m.keyMap.Cancel,
	}
}

func (m *WebDAVConfig) focusInput(index int) tea.Cmd {
	for i := range m.inputs {
		if i == index {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	return nil
}

func (m *WebDAVConfig) saveConfig() Action {
	cfg := m.com.Config()

	webdavCfg := &config.WebDAVConfig{
		Enabled:          true,
		URL:              m.inputs[0].Value(),
		Username:         m.inputs[1].Value(),
		Password:         m.inputs[2].Value(),
		RemotePath:       m.inputs[3].Value(),
		SyncInterval:     m.inputs[4].Value(),
		ConflictStrategy: m.inputs[5].Value(),
	}

	// Validate configuration
	if err := m.validateConfig(webdavCfg); err != nil {
		return ActionCmd{util.ReportError(fmt.Errorf("validation error: %w", err))}
	}

	// Save to config
	if err := cfg.SetConfigField("webdav", webdavCfg); err != nil {
		return ActionCmd{util.ReportError(fmt.Errorf("failed to save WebDAV config: %w", err))}
	}

	return ActionClose{}
}

func (m *WebDAVConfig) validateConfig(cfg *config.WebDAVConfig) error {
	if cfg.URL == "" {
		return fmt.Errorf("URL is required")
	}
	if !strings.HasPrefix(cfg.URL, "http://") && !strings.HasPrefix(cfg.URL, "https://") {
		return fmt.Errorf("URL must start with http:// or https://")
	}
	if cfg.SyncInterval != "" {
		if _, err := parseDuration(cfg.SyncInterval); err != nil {
			return fmt.Errorf("invalid sync interval: %w", err)
		}
	}
	validStrategies := map[string]bool{
		"newer-wins":  true,
		"local-wins":  true,
		"remote-wins": true,
		"backup":      true,
		"manual":      true,
	}
	if cfg.ConflictStrategy != "" && !validStrategies[cfg.ConflictStrategy] {
		return fmt.Errorf("invalid conflict strategy: %s", cfg.ConflictStrategy)
	}
	return nil
}

func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
