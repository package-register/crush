package app

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agui-server"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	_ "modernc.org/sqlite"
)

// TestAppAguiServerInitialization tests that the AG-UI server is properly
// initialized when enabled in the configuration.
func TestAppAguiServerInitialization(t *testing.T) {
	// Create a temporary database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create config with AG-UI server enabled
	enabled := true
	cfg := &config.Config{
		Options: &config.Options{
			AguiServer: &config.AguiServerOptions{
				Enabled:  &enabled,
				Port:     0, // Use random port
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

	if app.AguiServer == nil {
		t.Fatal("Expected AguiServer to be initialized")
	}

	// Verify cleanup function was added
	if len(app.cleanupFuncs) == 0 {
		t.Fatal("Expected cleanup functions to be added")
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
