package common

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

func TestDialogTitle_FillerCapped(t *testing.T) {
	st := styles.DefaultStyles()
	from := color.Black
	to := color.White

	// With very large width, filler should be capped at dialogTitleMaxFiller (24)
	title := "Settings"
	result := DialogTitle(&st, title, 200, from, to)
	if !strings.Contains(result, title) {
		t.Errorf("DialogTitle: result should contain title %q, got %q", title, result)
	}
	// Visible width = lipgloss.Width(result); filler part = total - len(title) - 1
	totalVisible := lipgloss.Width(result)
	titleVisible := lipgloss.Width(title) + 1 // +1 for space
	fillerVisible := totalVisible - titleVisible
	if fillerVisible > 24 {
		t.Errorf("DialogTitle: filler visible width = %d, want <= 24", fillerVisible)
	}
}

func TestDialogTitle_NoFillerWhenNarrow(t *testing.T) {
	st := styles.DefaultStyles()
	from := color.Black
	to := color.White

	// With width <= title length + 1, no filler
	title := "Hi"
	result := DialogTitle(&st, title, 3, from, to)
	if !strings.Contains(result, title) {
		t.Errorf("DialogTitle: result should contain title %q, got %q", title, result)
	}
	// remainingWidth = 3 - 3 = 0, so no filler
	totalVisible := lipgloss.Width(result)
	if totalVisible > lipgloss.Width(title)+1 {
		t.Errorf("DialogTitle: narrow width should produce no filler, totalVisible=%d", totalVisible)
	}
}

func TestDialogTitle_ShortFillerWhenMedium(t *testing.T) {
	st := styles.DefaultStyles()
	from := color.Black
	to := color.White

	// width=20, title="Test" (4 chars) -> remainingWidth=15, not capped
	title := "Test"
	result := DialogTitle(&st, title, 20, from, to)
	if !strings.Contains(result, title) {
		t.Errorf("DialogTitle: result should contain title %q, got %q", title, result)
	}
	totalVisible := lipgloss.Width(result)
	titleVisible := lipgloss.Width(title) + 1
	fillerVisible := totalVisible - titleVisible
	if fillerVisible != 15 {
		t.Errorf("DialogTitle: filler visible width = %d, want 15", fillerVisible)
	}
}
