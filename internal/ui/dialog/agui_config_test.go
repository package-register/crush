package dialog

import (
	"context"
	"database/sql"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	_ "modernc.org/sqlite"
)

func TestNewAguiConfig(t *testing.T) {
	com := makeTestCommon(t)
	defer com.App.Shutdown()

	m, cmd := NewAguiConfig(com, nil)
	if m == nil {
		t.Fatal("NewAguiConfig returned nil model")
	}
	if cmd != nil {
		_ = cmd // may be nil
	}
	if m.ID() != AguiConfigID {
		t.Errorf("ID() = %q, want %q", m.ID(), AguiConfigID)
	}
	if len(m.inputs) != 3 {
		t.Errorf("len(inputs) = %d, want 3", len(m.inputs))
	}
}

func TestNewAguiConfigWithOptions(t *testing.T) {
	com := makeTestCommon(t)
	defer com.App.Shutdown()

	enabled := true
	aguiCfg := &config.AguiServerOptions{
		Enabled:     &enabled,
		Port:        5555,
		BasePath:    "/api/agui",
		CORSOrigins: []string{"http://localhost:3000"},
	}

	m, _ := NewAguiConfig(com, aguiCfg)
	if !m.enabled {
		t.Error("expected enabled=true when aguiCfg.IsEnabled()")
	}
	if m.inputs[0].Value() != "5555" {
		t.Errorf("Port input = %q, want 5555", m.inputs[0].Value())
	}
	if m.inputs[1].Value() != "/api/agui" {
		t.Errorf("BasePath input = %q, want /api/agui", m.inputs[1].Value())
	}
}

func TestAguiConfigHandleMsgCancel(t *testing.T) {
	com := makeTestCommon(t)
	defer com.App.Shutdown()

	m, _ := NewAguiConfig(com, nil)
	m.enabled = true
	m.focusInput(0)

	msg := tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	action := m.HandleMsg(msg)
	if action == nil {
		t.Fatal("expected ActionClose on Escape")
	}
	if _, ok := action.(ActionClose); !ok {
		t.Errorf("expected ActionClose, got %T", action)
	}
}

func TestAguiConfigBuildConfigFromInputs(t *testing.T) {
	com := makeTestCommon(t)
	defer com.App.Shutdown()

	m, _ := NewAguiConfig(com, nil)
	m.enabled = true
	m.inputs[0].SetValue("9999")
	m.inputs[1].SetValue("/custom")
	m.inputs[2].SetValue("https://example.com")

	cfg := m.buildConfigFromInputs()
	if cfg.Port != 9999 {
		t.Errorf("Port = %d, want 9999", cfg.Port)
	}
	if cfg.BasePath != "/custom" {
		t.Errorf("BasePath = %q, want /custom", cfg.BasePath)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "https://example.com" {
		t.Errorf("CORSOrigins = %v", cfg.CORSOrigins)
	}
}

func makeTestCommon(t *testing.T) *common.Common {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	disabled := false
	cfg := &config.Config{
		Options: &config.Options{
			AguiServer: &config.AguiServerOptions{Enabled: &disabled},
		},
		Providers: &csync.Map[string, config.ProviderConfig]{},
		Models:    make(map[config.SelectedModelType]config.SelectedModel),
	}
	cfg.SetupAgents()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	a, err := app.New(ctx, db, cfg)
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	return common.DefaultCommon(a)
}
