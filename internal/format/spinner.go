package format

import (
	"context"
	"errors"
	"fmt"
	"os"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Spinner wraps the bubbles spinner for non-interactive mode
type Spinner struct {
	done chan struct{}
	prog *tea.Program
}

// SpinnerOptions configures the CLI spinner.
type SpinnerOptions struct {
	// Style sets the spinner's lipgloss style (e.g. foreground color).
	Style lipgloss.Style
	// Label is the text shown next to the spinner (e.g. "Generating").
	Label string
}

type model struct {
	cancel  context.CancelFunc
	spinner spinner.Model
	label   string
}

func (m model) Init() tea.Cmd { return m.spinner.Tick }
func (m model) View() tea.View {
	return tea.NewView(m.spinner.View() + " " + m.label)
}

// Update implements tea.Model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancel()
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

// NewSpinner creates a new spinner for non-interactive CLI mode.
func NewSpinner(ctx context.Context, cancel context.CancelFunc, opts SpinnerOptions) *Spinner {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(opts.Style),
	)
	label := opts.Label
	if label == "" {
		label = "Generating"
	}
	m := model{
		spinner: s,
		label:   label,
		cancel:  cancel,
	}

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithContext(ctx))

	return &Spinner{
		prog: p,
		done: make(chan struct{}, 1),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	go func() {
		defer close(s.done)
		_, err := s.prog.Run()
		// ensures line is cleared
		fmt.Fprint(os.Stderr, ansi.EraseEntireLine)
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, tea.ErrInterrupted) {
			fmt.Fprintf(os.Stderr, "Error running spinner: %v\n", err)
		}
	}()
}

// Stop ends the spinner animation
func (s *Spinner) Stop() {
	s.prog.Quit()
	<-s.done
}
