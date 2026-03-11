package format

import (
	"context"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func TestNewSpinner_DefaultLabel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewSpinner(ctx, cancel, SpinnerOptions{})
	require.NotNil(t, s)
	require.NotNil(t, s.prog)
	require.NotNil(t, s.done)
}

func TestNewSpinner_CustomLabel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewSpinner(ctx, cancel, SpinnerOptions{Label: "Loading..."})
	require.NotNil(t, s)
}

func TestNewSpinner_CustomStyle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sty := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	s := NewSpinner(ctx, cancel, SpinnerOptions{Style: sty, Label: "Working"})
	require.NotNil(t, s)
}

func TestSpinner_StartStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewSpinner(ctx, cancel, SpinnerOptions{Label: "Test"})
	s.Start()

	// Allow spinner to run briefly
	time.Sleep(50 * time.Millisecond)

	s.Stop()
	// Stop should not block; if we get here the test passes
}

func TestSpinner_ContextCancelStopsSpinner(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	s := NewSpinner(ctx, cancel, SpinnerOptions{Label: "Test"})
	s.Start()

	time.Sleep(30 * time.Millisecond)

	// Cancel context - spinner should quit
	cancel()

	// Stop should complete quickly since context is cancelled
	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Spinner.Stop() hung after context cancel")
	}
}

func TestSpinnerOptions_ZeroValue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Empty options should use default label "Generating"
	s := NewSpinner(ctx, cancel, SpinnerOptions{})
	require.NotNil(t, s)
	s.Start()
	time.Sleep(20 * time.Millisecond)
	s.Stop()
}

func TestSpinner_StructFields(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewSpinner(ctx, cancel, SpinnerOptions{})
	require.NotNil(t, s)
	require.NotNil(t, s.prog)
	require.NotNil(t, s.done)
}
