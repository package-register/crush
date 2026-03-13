package model

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// aguiInfo renders the AGUI server status section.
func (m *UI) aguiInfo(width int, isSection bool) string {
	t := m.com.Styles
	title := t.ResourceGroupTitle.Render("AGUI")
	if isSection {
		title = common.Section(t, title, width)
	}
	content := t.ResourceAdditionalText.Render("disabled")
	if m.com.App.AguiServer != nil {
		cfg := m.com.Config()
		if cfg.Options != nil && cfg.Options.AguiServer != nil && cfg.Options.AguiServer.IsEnabled() {
			port := cfg.Options.AguiServer.GetPort()
			base := cfg.Options.AguiServer.GetBasePath()
			content = t.ResourceStatus.Render(fmt.Sprintf("http://localhost:%d%s", port, base))
		}
	}
	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, content))
}
