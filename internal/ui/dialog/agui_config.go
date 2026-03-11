package dialog

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
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
		Save   key.Binding
		Cancel key.Binding
		Next   key.Binding
		Prev   key.Binding
	}

	inputs     []textinput.Model
	focusIndex int
	enabled    bool

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
		width:      70,
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
	if m.enabled {
		m.inputs[0].Focus()
	}

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

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	m.help = h

	return m, nil
}

// ID implements Dialog.
func (m *AguiConfig) ID() string {
	return AguiConfigID
}

// HandleMsg implements Dialog.
func (m *AguiConfig) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Cancel):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Save):
			return m.saveConfig()
		case key.Matches(msg, m.keyMap.Next):
			if !m.enabled {
				m.enabled = true
				m.inputs[0].Focus()
				return nil
			}
			m.focusIndex++
			if m.focusIndex >= len(m.inputs) {
				m.focusIndex = len(m.inputs) - 1
			}
			return m.focusInput(m.focusIndex)
		case key.Matches(msg, m.keyMap.Prev):
			if !m.enabled && m.focusIndex > 0 {
				m.focusIndex--
				return m.focusInput(m.focusIndex)
			}
			if m.enabled && m.focusIndex > 0 {
				m.focusIndex--
				return m.focusInput(m.focusIndex)
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

	textStyle := t.Dialog.SecondaryText
	dialogStyle := t.Dialog.View.Width(m.width)
	inputStyle := t.Dialog.InputPrompt
	helpStyle := t.Dialog.HelpView
	helpStyle = helpStyle.Width(m.width - dialogStyle.GetHorizontalFrameSize())

	var parts []string
	parts = append(parts, m.headerView())

	// Enabled/Disabled toggle
	status := "Disabled"
	statusStyle := t.Dialog.SecondaryText
	if m.enabled {
		status = "Enabled"
		statusStyle = t.Dialog.PrimaryText
	}
	parts = append(parts, inputStyle.Render("Status: ")+statusStyle.Render(status))
	parts = append(parts, "")

	if m.enabled {
		labels := []string{
			"Port:",
			"Base Path:",
			"CORS Origins:",
		}

		for i, input := range m.inputs {
			label := inputStyle.Render(labels[i] + " ")
			inputView := input.View()
			parts = append(parts, label+inputView)
		}

		parts = append(parts, "")
		parts = append(parts, textStyle.Render("• Port: 1-65535 (default: 8080)"))
		parts = append(parts, textStyle.Render("• Base Path: must start with / (default: /agui)"))
		parts = append(parts, textStyle.Render("• CORS Origins: comma-separated list of allowed origins"))
	} else {
		parts = append(parts, textStyle.Render("Press tab to enable configuration"))
	}

	parts = append(parts, "")
	parts = append(parts, helpStyle.Render(m.help.View(m)))

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
	return InputCursor(m.com.Styles, m.inputs[m.focusIndex].Cursor())
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
	for i := range m.inputs {
		if i == index {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	return nil
}

func (m *AguiConfig) saveConfig() Action {
	cfg := m.com.Config()

	var aguiCfg *config.AguiServerOptions
	if m.enabled {
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

		aguiCfg = &config.AguiServerOptions{
			Enabled:     &m.enabled,
			Port:        port,
			BasePath:    basePath,
			CORSOrigins: corsOrigins,
		}

		// Validate configuration
		if err := m.validateConfig(aguiCfg); err != nil {
			return ActionCmd{util.ReportError(fmt.Errorf("validation error: %w", err))}
		}
	} else {
		aguiCfg = &config.AguiServerOptions{
			Enabled: &m.enabled,
		}
	}

	// Save to config
	if err := cfg.SetConfigField("options.agui_server", aguiCfg); err != nil {
		return ActionCmd{util.ReportError(fmt.Errorf("failed to save AGUI config: %w", err))}
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
