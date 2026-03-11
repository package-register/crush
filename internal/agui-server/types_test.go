package aguiserver

import (
	"context"
	"testing"
	"time"
)

func TestConnectionCreation(t *testing.T) {
	conn := NewConnection("test-id", "thread-123", "run-456")

	if conn.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", conn.ID)
	}
	if conn.ThreadID != "thread-123" {
		t.Errorf("Expected ThreadID 'thread-123', got '%s'", conn.ThreadID)
	}
	if conn.RunID != "run-456" {
		t.Errorf("Expected RunID 'run-456', got '%s'", conn.RunID)
	}
	if conn.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
	if conn.Events == nil {
		t.Error("Expected Events channel to be initialized")
	}
	if conn.Done == nil {
		t.Error("Expected Done channel to be initialized")
	}
}

func TestConnectionChannelBufferSize(t *testing.T) {
	conn := NewConnection("test-id", "thread-123", "run-456")

	// Test that the channel can buffer events
	for i := range 64 {
		select {
		case conn.Events <- Event{Type: "TEST"}:
		default:
			t.Errorf("Expected channel to buffer 64 events, blocked at %d", i)
		}
	}
}

func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	if config.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Port)
	}
	if config.BasePath != "/agui" {
		t.Errorf("Expected default base path '/agui', got '%s'", config.BasePath)
	}
	if len(config.CORSOrigins) != 1 || config.CORSOrigins[0] != "*" {
		t.Errorf("Expected default CORS origins ['*'], got %v", config.CORSOrigins)
	}
}

func TestServerCreation(t *testing.T) {
	config := ServerConfig{
		Port:        9090,
		BasePath:    "/api/agui",
		CORSOrigins: []string{"http://localhost:3000"},
	}

	srv := NewServer(config)
	if srv == nil {
		t.Fatal("Expected server to be created")
	}

	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	if server.config.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", server.config.Port)
	}
	if server.config.BasePath != "/api/agui" {
		t.Errorf("Expected base path '/api/agui', got '%s'", server.config.BasePath)
	}
	if len(server.config.CORSOrigins) != 1 {
		t.Errorf("Expected 1 CORS origin, got %d", len(server.config.CORSOrigins))
	}
}

func TestServerConnectionsMap(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)

	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	if server.connections == nil {
		t.Error("Expected connections map to be initialized")
	}

	if len(server.connections) != 0 {
		t.Errorf("Expected empty connections map, got %d entries", len(server.connections))
	}
}

func TestRunRequestValidation(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)

	server, ok := srv.(*server)
	if !ok {
		t.Fatal("Expected server to be of type *server")
	}

	tests := []struct {
		name    string
		req     RunRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: RunRequest{
				ThreadID: "thread-123",
				RunID:    "run-456",
				Messages: []Message{
					{Role: "user", Content: "Hello"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty messages",
			req: RunRequest{
				ThreadID: "thread-123",
				RunID:    "run-456",
				Messages: []Message{},
			},
			wantErr: true,
		},
		{
			name: "missing role",
			req: RunRequest{
				ThreadID: "thread-123",
				RunID:    "run-456",
				Messages: []Message{
					{Role: "", Content: "Hello"},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple messages",
			req: RunRequest{
				ThreadID: "thread-123",
				RunID:    "run-456",
				Messages: []Message{
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi there!"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.validateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConnectionManager(t *testing.T) {
	manager := NewConnectionManager()

	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections, got %d", manager.Count())
	}

	// Add connection
	conn := NewConnection("conn-1", "thread-1", "run-1")
	manager.Add(conn)

	if manager.Count() != 1 {
		t.Errorf("Expected 1 connection after add, got %d", manager.Count())
	}

	// Get connection
	retrieved, ok := manager.Get("conn-1")
	if !ok {
		t.Error("Expected to retrieve connection")
	}
	if retrieved.ID != "conn-1" {
		t.Errorf("Expected connection ID 'conn-1', got '%s'", retrieved.ID)
	}

	// Get non-existent connection
	_, ok = manager.Get("non-existent")
	if ok {
		t.Error("Expected not to find non-existent connection")
	}

	// List connections
	conns := manager.List()
	if len(conns) != 1 {
		t.Errorf("Expected 1 connection in list, got %d", len(conns))
	}

	// Remove connection
	manager.Remove("conn-1")
	if manager.Count() != 0 {
		t.Errorf("Expected 0 connections after remove, got %d", manager.Count())
	}
}

func TestConnectionManagerConcurrency(t *testing.T) {
	manager := NewConnectionManager()

	// Add multiple connections concurrently
	done := make(chan bool)
	for i := range 10 {
		go func(id int) {
			conn := NewConnection(
				"conn-"+string(rune(id)),
				"thread-"+string(rune(id)),
				"run-"+string(rune(id)),
			)
			manager.Add(conn)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for range 10 {
		<-done
	}

	if manager.Count() != 10 {
		t.Errorf("Expected 10 connections, got %d", manager.Count())
	}
}

func TestServerStopWithNoHTTPServer(t *testing.T) {
	config := DefaultServerConfig()
	srv := NewServer(config)

	// Stop should not panic when httpServer is nil
	ctx := context.Background()
	err := srv.Stop(ctx)
	if err != nil {
		t.Errorf("Expected no error when stopping server without httpServer, got %v", err)
	}
}

func TestConnectionClose(t *testing.T) {
	conn := NewConnection("test-id", "thread-123", "run-456")

	// Close the connection
	close(conn.Done)

	// Verify channel is closed
	select {
	case _, ok := <-conn.Done:
		if ok {
			t.Error("Expected Done channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Done channel should be closed immediately")
	}
}
