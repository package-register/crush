package app

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"charm.land/fantasy"
	aguiserver "github.com/charmbracelet/crush/internal/agui-server"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	_ "modernc.org/sqlite"
)

// mockCoordinator is a minimal Coordinator for testing AGUI server initialization.
type mockCoordinator struct{}

func (m *mockCoordinator) Run(_ context.Context, _, _ string, _ ...message.Attachment) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}
func (m *mockCoordinator) Cancel(_ string)             {}
func (m *mockCoordinator) CancelAll()                 {}
func (m *mockCoordinator) IsSessionBusy(_ string) bool { return false }
func (m *mockCoordinator) IsBusy() bool               { return false }
func (m *mockCoordinator) QueuedPrompts(_ string) int  { return 0 }
func (m *mockCoordinator) QueuedPromptsList(_ string) []string { return nil }
func (m *mockCoordinator) ClearQueue(_ string)        {}
func (m *mockCoordinator) Summarize(context.Context, string) error { return nil }
func (m *mockCoordinator) Model() agent.Model         { return agent.Model{} }
func (m *mockCoordinator) UpdateModels(context.Context) error { return nil }

// Ensure mockCoordinator implements agent.Coordinator.
var _ agent.Coordinator = (*mockCoordinator)(nil)

// TestAppAguiServerInitialization tests that the AG-UI server is NOT initialized
// when agent is not configured (AGUI requires AgentCoordinator).
func TestAppAguiServerInitialization(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	enabled := true
	cfg := &config.Config{
		Options: &config.Options{
			AguiServer: &config.AguiServerOptions{
				Enabled:  &enabled,
				Port:     0,
				BasePath: "/agui",
			},
		},
		Providers: &csync.Map[string, config.ProviderConfig]{},
		Models:    make(map[config.SelectedModelType]config.SelectedModel),
	}
	cfg.SetupAgents()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app, err := New(ctx, db, cfg)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer app.Shutdown()

	// When agent is not configured, AGUI server is not started (requires AgentCoordinator)
	if app.AguiServer != nil {
		t.Fatal("Expected AguiServer to be nil when agent is not configured")
	}
}

// TestAppAguiServerDisabled tests that the AG-UI server is not initialized
// when disabled in the configuration.
func TestAppAguiServerDisabled(t *testing.T) {
	// Create a temporary database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create config with AG-UI server disabled
	disabled := false
	cfg := &config.Config{
		Options: &config.Options{
			AguiServer: &config.AguiServerOptions{
				Enabled: &disabled,
			},
		},
		Providers: &csync.Map[string, config.ProviderConfig]{},
		Models:    make(map[config.SelectedModelType]config.SelectedModel),
	}
	cfg.SetupAgents()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app, err := New(ctx, db, cfg)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer app.Shutdown()

	if app.AguiServer != nil {
		t.Fatal("Expected AguiServer to be nil when disabled")
	}
}

// TestAppAguiServerNilConfig tests that the app handles nil AG-UI server
// configuration gracefully.
func TestAppAguiServerNilConfig(t *testing.T) {
	// Create a temporary database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create config without AG-UI server options
	cfg := &config.Config{
		Options:   &config.Options{},
		Providers: &csync.Map[string, config.ProviderConfig]{},
		Models:    make(map[config.SelectedModelType]config.SelectedModel),
	}
	cfg.SetupAgents()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app, err := New(ctx, db, cfg)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer app.Shutdown()

	if app.AguiServer != nil {
		t.Fatal("Expected AguiServer to be nil when not configured")
	}
}

// TestStartOrRestartAguiServer tests StartOrRestartAguiServer when enabled and
// AgentCoordinator is available.
func TestStartOrRestartAguiServer(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	enabled := true
	port := 55551
	cfg := &config.Config{
		Options: &config.Options{
			AguiServer: &config.AguiServerOptions{
				Enabled:     &enabled,
				Port:        port,
				BasePath:    "/agui",
				CORSOrigins: []string{"*"},
			},
		},
		Providers: &csync.Map[string, config.ProviderConfig]{},
		Models:    make(map[config.SelectedModelType]config.SelectedModel),
	}
	cfg.SetupAgents()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app, err := New(ctx, db, cfg)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer app.Shutdown()

	// Inject mock coordinator so StartOrRestartAguiServer can create AGUI server
	app.AgentCoordinator = &mockCoordinator{}

	if err := app.StartOrRestartAguiServer(ctx); err != nil {
		t.Fatalf("StartOrRestartAguiServer: %v", err)
	}
	if app.AguiServer == nil {
		t.Fatal("Expected AguiServer to be set after StartOrRestartAguiServer")
	}
}

// TestStartOrRestartAguiServerDisabled tests StartOrRestartAguiServer when disabled (no-op).
func TestStartOrRestartAguiServerDisabled(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	disabled := false
	cfg := &config.Config{
		Options: &config.Options{
			AguiServer: &config.AguiServerOptions{
				Enabled: &disabled,
			},
		},
		Providers: &csync.Map[string, config.ProviderConfig]{},
		Models:    make(map[config.SelectedModelType]config.SelectedModel),
	}
	cfg.SetupAgents()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app, err := New(ctx, db, cfg)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	defer app.Shutdown()

	if err := app.StartOrRestartAguiServer(ctx); err != nil {
		t.Fatalf("StartOrRestartAguiServer: %v", err)
	}
	if app.AguiServer != nil {
		t.Fatal("Expected AguiServer to remain nil when disabled")
	}
}

// TestAguiServerStop tests that the AG-UI server can be stopped gracefully.
func TestAguiServerStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverCfg := aguiserver.ServerConfig{
		Port:        0, // Use random port
		BasePath:    "/agui",
		CORSOrigins: []string{"*"},
	}
	server := aguiserver.NewServer(serverCfg)

	// Start server in background
	go func() {
		if err := server.Start(ctx); err != nil && err != context.Canceled {
			t.Logf("Server error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop server
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
}
