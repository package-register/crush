package session

import (
	"context"
	"database/sql"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) (*sql.DB, *db.Queries) {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	goose.SetBaseFS(db.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		conn.Close()
		t.Fatalf("set dialect: %v", err)
	}
	if err := goose.Up(conn, "migrations"); err != nil {
		conn.Close()
		t.Fatalf("migrate: %v", err)
	}
	q := db.New(conn)
	return conn, q
}

func TestCreateWithID(t *testing.T) {
	conn, q := setupTestDB(t)
	defer conn.Close()

	svc := NewService(q, conn)
	ctx := context.Background()

	// Create session with custom ID
	s, err := svc.CreateWithID(ctx, "agui-thread-123", "AG-UI: demo")
	if err != nil {
		t.Fatalf("CreateWithID: %v", err)
	}
	if s.ID != "agui-thread-123" {
		t.Errorf("ID = %q, want agui-thread-123", s.ID)
	}
	if s.Title != "AG-UI: demo" {
		t.Errorf("Title = %q, want AG-UI: demo", s.Title)
	}

	// Retrieve and verify
	got, err := svc.Get(ctx, "agui-thread-123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != s.ID || got.Title != s.Title {
		t.Errorf("Get returned %+v, want %+v", got, s)
	}
}

func TestCreateWithID_DuplicateIDFails(t *testing.T) {
	conn, q := setupTestDB(t)
	defer conn.Close()

	svc := NewService(q, conn)
	ctx := context.Background()

	_, err := svc.CreateWithID(ctx, "dup-id", "First")
	if err != nil {
		t.Fatalf("first CreateWithID: %v", err)
	}

	_, err = svc.CreateWithID(ctx, "dup-id", "Second")
	if err == nil {
		t.Error("expected error on duplicate ID")
	}
}
