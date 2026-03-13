package dialog

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	uv "github.com/charmbracelet/ultraviolet"
)

// AguiConfigID is the identifier for the AGUI server configuration dialog.
const AguiConfigID = "agui_config"

// AguiConfig represents the AGUI server configuration dialog.
type AguiConfig struct {
	com    *common.Common
	keyMap struct {
		Save       key.Binding
		Cancel     key.Binding
		Toggle     key.Binding
		Next       key.Binding
		Prev       key.Binding
		Enter      key.Binding
		LeftRight  key.Binding
		EnterSpace key.Binding
		Yes        key.Binding
		No         key.Binding
	}

	inputs     []textinput.Model
	focusIndex int
	enabled    bool

	confirmingStart    bool
	confirmSelectedNo  bool

	width int
	help  help.Model
}

var _ Dialog = (*AguiConfig)(nil)

// NewAguiConfig creates a new AGUI server configuration dialog.
func NewAguiConfig(com *common.Common, aguiCfg *config.AguiServerOptions) (*AguiConfig, tea.Cmd) {
	t := com.Styles

	m := &AguiConfig{
		com:        com,
		focusIndex: 0,
		width:      85,
		enabled:    false,
	}

	if aguiCfg != nil && aguiCfg.IsEnabled() {
		m.enabled = true
	}

	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize() - 2

	// Initialize text inputs
	inputLabels := []string{
		"Port:",
		"Base Path:",
		"CORS Origins:",
	}

	defaultValues := []string{
		"8080",
		"/agui",
		"http://localhost:3000",
	}

	if aguiCfg != nil {
		if aguiCfg.Port != 0 {
			defaultValues[0] = strconv.Itoa(aguiCfg.Port)
		}
		if aguiCfg.BasePath != "" {
			defaultValues[1] = aguiCfg.BasePath
		}
		if len(aguiCfg.CORSOrigins) > 0 {
			defaultValues[2] = strings.Join(aguiCfg.CORSOrigins, ",")
		}
	}

	for i := range inputLabels {
		input := textinput.New()
		input.SetVirtualCursor(false)
		input.Placeholder = defaultValues[i]
		input.SetStyles(com.Styles.TextInput)
		input.SetWidth(max(0, innerWidth))
		input.SetValue(defaultValues[i])
		if m.enabled && i == 0 {
			input.Focus()
		} else {
			input.Blur()
		}
		m.inputs = append(m.inputs, input)
	}

	// Setup key bindings
	m.keyMap.Save = key.NewBinding(
		key.WithKeys("ctrl+s", "ctrl+y"),
		key.WithHelp("ctrl+s", "save"),
	)
	m.keyMap.Cancel = CloseKey
	m.keyMap.Toggle = key.NewBinding(
		key.WithKeys("space", " "),
		key.WithHelp("space", "toggle"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("tab", "down"),
		key.WithHelp("tab/↓", "next field"),
	)
	m.keyMap.Prev = key.NewBinding(
		key.WithKeys("shift+tab", "up"),
		key.WithHelp("shift+tab/↑", "prev field"),
	)
	m.keyMap.Enter = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	)
	m.keyMap.LeftRight = key.NewBinding(
		key.WithKeys("left", "right"),
		key.WithHelp("←/→", "switch options"),
	)
	m.keyMap.EnterSpace = key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter/space", "confirm"),
	)
	m.keyMap.Yes = key.NewBinding(
		key.WithKeys("y", "Y"),
		key.WithHelp("y/Y", "yes"),
	)
	m.keyMap.No = key.NewBinding(
		key.WithKeys("n", "N"),
		key.WithHelp("n/N", "no"),
	)

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	m.help = h

	return m, nil
}

// ID implements Dialog.
func (m *AguiConfig) ID() string {
	return AguiConfigID
}

// buildConfigFromInputs constructs AguiServerOptions from current input values.
func (m *AguiConfig) buildConfigFromInputs() *config.AguiServerOptions {
	port := 8080
	if m.inputs[0].Value() != "" {
		if p, err := strconv.Atoi(m.inputs[0].Value()); err == nil {
			port = p
		}
	}
	basePath := "/agui"
	if m.inputs[1].Value() != "" {
		basePath = m.inputs[1].Value()
	}
	corsOrigins := []string{"http://localhost:3000"}
	if m.inputs[2].Value() != "" {
		origins := strings.Split(m.inputs[2].Value(), ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		if len(origins) > 0 && origins[0] != "" {
			corsOrigins = origins
		}
	}
	return &config.AguiServerOptions{
		Enabled:     &m.enabled,
		Port:        port,
		BasePath:    basePath,
		CORSOrigins: corsOrigins,
	}
}

// HandleMsg implements Dialog.
func (m *AguiConfig) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case m.confirmingStart:
			switch {
			case key.Matches(msg, m.keyMap.LeftRight, m.keyMap.Next, m.keyMap.Prev):
				m.confirmSelectedNo = !m.confirmSelectedNo
			case key.Matches(msg, m.keyMap.EnterSpace):
				if !m.confirmSelectedNo {
					return m.saveConfig(true)
				}
				m.confirmingStart = false
			case key.Matches(msg, m.keyMap.Yes):
				return m.saveConfig(true)
			case key.Matches(msg, m.keyMap.No, m.keyMap.Cancel):
				m.confirmingStart = false
			}
			return nil
		case key.Matches(msg, m.keyMap.Cancel):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Save):
			return m.saveConfig(false)
		case key.Matches(msg, m.keyMap.Enter):
			if m.enabled && m.focusIndex == len(m.inputs)-1 {
				cfg := m.buildConfigFromInputs()
				if err := m.validateConfig(cfg); err != nil {
					return ActionCmd{util.ReportError(fmt.Errorf("validation error: %w", err))}
				}
				m.confirmingStart = true
				m.confirmSelectedNo = false
				return nil
			}
		case key.Matches(msg, m.keyMap.Toggle):
			// Space toggles Status when disabled (enable AGUI). When enabled, space passes through to input.
			if !m.enabled {
				m.enabled = true
				m.focusIndex = 0
				cmd := m.focusInput(0)
				if cmd != nil {
					return ActionCmd{cmd}
				}
				return nil
			}
		case key.Matches(msg, m.keyMap.Next):
			if !m.enabled {
				m.enabled = true
				for i := range m.inputs {
					if i != 0 {
						m.inputs[i].Blur()
					}
				}
				cmd := m.inputs[0].Focus()
				if cmd != nil {
					return ActionCmd{cmd}
				}
				return nil
			}
			m.focusIndex++
			if m.focusIndex >= len(m.inputs) {
				m.focusIndex = len(m.inputs) - 1
			}
			cmd := m.focusInput(m.focusIndex)
			if cmd != nil {
				return ActionCmd{cmd}
			}
			return nil
		case key.Matches(msg, m.keyMap.Prev):
			if !m.enabled && m.focusIndex > 0 {
				m.focusIndex--
				cmd := m.focusInput(m.focusIndex)
				if cmd != nil {
					return ActionCmd{cmd}
				}
				return nil
			}
			if m.enabled && m.focusIndex > 0 {
				m.focusIndex--
				cmd := m.focusInput(m.focusIndex)
				if cmd != nil {
					return ActionCmd{cmd}
				}
				return nil
			}
			if m.enabled {
				m.enabled = false
				for i := range m.inputs {
					m.inputs[i].Blur()
				}
				m.focusIndex = 0
			}
		default:
			if m.enabled {
				var cmd tea.Cmd
				m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
				if cmd != nil {
					return ActionCmd{cmd}
				}
			}
		}
	case tea.PasteMsg:
		if m.enabled {
			var cmd tea.Cmd
			m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	}
	return nil
}

// Draw implements Dialog.
func (m *AguiConfig) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles

	if m.confirmingStart {
		buttonOpts := []common.ButtonOpts{
			{Text: "启动", Selected: !m.confirmSelectedNo, Padding: 3},
			{Text: "取消", Selected: m.confirmSelectedNo, Padding: 3},
		}
		buttons := common.ButtonGroup(t, buttonOpts, " ")
		content := lipgloss.JoinVertical(lipgloss.Center,
			"Start AGUI server now?",
			"",
			buttons,
		)
		view := t.BorderFocus.Render(t.Base.Render(content))
		DrawCenter(scr, area, view)
		return nil
	}

	// Adaptive width: use area when available, clamp to sensible range
	if area.Dx() > 0 {
		w := min(90, max(70, area.Dx()-20))
		if w != m.width {
			m.width = w
			innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize() - 2
			for i := range m.inputs {
				m.inputs[i].SetWidth(max(0, innerWidth))
			}
		}
	}

	textStyle := t.Dialog.SecondaryText
	dialogStyle := t.Dialog.View.Width(m.width)
	labelStyle := t.Dialog.FormLabel

	var parts []string
	parts = append(parts, m.headerView())

	// Enabled/Disabled toggle - use FormLabel to avoid margin gap
	status := "Disabled"
	statusStyle := t.Dialog.SecondaryText
	if m.enabled {
		status = "Enabled"
		statusStyle = t.Dialog.PrimaryText
	}
	parts = append(parts, labelStyle.Render("Status: ")+statusStyle.Render(status))
	parts = append(parts, "")

	if m.enabled {
		labels := []string{
			"Port:",
			"Base Path:",
			"CORS Origins:",
		}

		for i, input := range m.inputs {
			parts = append(parts, labelStyle.Render(labels[i]))
			parts = append(parts, input.View())
		}

		parts = append(parts, "")
		parts = append(parts, textStyle.Render("• Port: 1-65535 (default: 8080)"))
		parts = append(parts, textStyle.Render("• Base Path: must start with / (default: /agui)"))
		parts = append(parts, textStyle.Render("• CORS Origins: comma-separated list of allowed origins"))
	} else {
		parts = append(parts, textStyle.Render("Press Tab or Space to enable AGUI Server"))
	}

	content := strings.Join(parts, "\n")
	view := dialogStyle.Render(content)
	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (m *AguiConfig) headerView() string {
	var (
		t           = m.com.Styles
		titleStyle  = t.Dialog.Title
		dialogStyle = t.Dialog.View.Width(m.width)
	)
	headerOffset := titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
	return common.DialogTitle(t, titleStyle.Render("AGUI Server Configuration"), m.width-headerOffset, m.com.Styles.Primary, m.com.Styles.Secondary)
}

// Cursor returns the cursor position relative to the dialog.
func (m *AguiConfig) Cursor() *tea.Cursor {
	if !m.enabled {
		return nil
	}
	cur := m.inputs[m.focusIndex].Cursor()
	if cur == nil {
		return nil
	}
	t := m.com.Styles
	dialogStyle := t.Dialog.View.Width(m.width)
	// Layout: header(1) + Status(1) + blank(1) + each field 2 lines (label + input) -> lineIndex = 3 + focusIndex*2 + 1
	lineIndex := 3 + m.focusIndex*2 + 1
	cur.X = dialogStyle.GetBorderLeftSize() +
		dialogStyle.GetPaddingLeft() +
		dialogStyle.GetMarginLeft() +
		cur.X
	cur.Y = dialogStyle.GetBorderTopSize() +
		dialogStyle.GetPaddingTop() +
		dialogStyle.GetMarginTop() +
		lineIndex + cur.Y
	return cur
}

// FullHelp returns the full help view.
func (m *AguiConfig) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			m.keyMap.Save,
			m.keyMap.Next,
			m.keyMap.Prev,
		},
		{
			m.keyMap.Cancel,
		},
	}
}

// ShortHelp returns the short help view.
func (m *AguiConfig) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.Save,
		m.keyMap.Next,
		m.keyMap.Prev,
		m.keyMap.Cancel,
	}
}

func (m *AguiConfig) focusInput(index int) tea.Cmd {
	var cmd tea.Cmd
	for i := range m.inputs {
		if i == index {
			cmd = m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	return cmd
}

func (m *AguiConfig) saveConfig(confirmStart bool) Action {
	cfg := m.com.Config()

	var aguiCfg *config.AguiServerOptions
	if m.enabled {
		aguiCfg = m.buildConfigFromInputs()
		if err := m.validateConfig(aguiCfg); err != nil {
			return ActionCmd{util.ReportError(fmt.Errorf("validation error: %w", err))}
		}
	} else {
		aguiCfg = &config.AguiServerOptions{
			Enabled: &m.enabled,
		}
	}

	if err := cfg.SetConfigField("options.agui_server", aguiCfg); err != nil {
		return ActionCmd{util.ReportError(fmt.Errorf("failed to save AGUI config: %w", err))}
	}

	// Update in-memory config so StartOrRestartAguiServer reads the new values.
	// SetConfigField only writes to file; app.config is not updated otherwise.
	if cfg.Options == nil {
		cfg.Options = &config.Options{}
	}
	cfg.Options.AguiServer = aguiCfg

	if confirmStart {
		return ActionConfirmStartAGUI{}
	}
	return ActionClose{}
}

func (m *AguiConfig) validateConfig(cfg *config.AguiServerOptions) error {
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port %d: must be between 1 and 65535", cfg.Port)
	}
	if cfg.BasePath != "" && !strings.HasPrefix(cfg.BasePath, "/") {
		return fmt.Errorf("invalid base_path %q: must start with /", cfg.BasePath)
	}
	return nil
}
