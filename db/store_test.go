package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenMigratesSessionsTable(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)

	var tableName string
	err := store.db.QueryRowContext(
		context.Background(),
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'sessions'`,
	).Scan(&tableName)
	if err != nil {
		t.Fatalf("sessions table was not created: %v", err)
	}
	if tableName != "sessions" {
		t.Fatalf("expected sessions table, got %q", tableName)
	}
}

func TestSessionCRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestStore(t)
	first := testSession("Terminal", "Build", time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC))
	second := testSession("Editor", "ReTrace", time.Date(2026, 4, 29, 8, 0, 0, 0, time.UTC))

	if err := store.CreateSession(ctx, &first); err != nil {
		t.Fatalf("create first session: %v", err)
	}
	if first.ID == 0 {
		t.Fatal("expected inserted session ID")
	}
	if err := store.CreateSession(ctx, &second); err != nil {
		t.Fatalf("create second session: %v", err)
	}

	got, err := store.GetSession(ctx, first.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	assertSession(t, got, first)

	listed, err := store.ListSessions(ctx)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(listed))
	}
	if listed[0].ID != second.ID || listed[1].ID != first.ID {
		t.Fatalf("sessions were not ordered by started_at: got IDs %d, %d", listed[0].ID, listed[1].ID)
	}

	first.App = "Shell"
	first.Title = "Tests"
	first.Duration = 900
	if err := store.UpdateSession(ctx, first); err != nil {
		t.Fatalf("update session: %v", err)
	}

	updated, err := store.GetSession(ctx, first.ID)
	if err != nil {
		t.Fatalf("get updated session: %v", err)
	}
	assertSession(t, updated, first)

	if err := store.DeleteSession(ctx, first.ID); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, err := store.GetSession(ctx, first.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected deleted session to be missing, got %v", err)
	}
}

func TestSessionRequiredFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestStore(t)

	_, err := store.db.ExecContext(
		ctx,
		`INSERT INTO sessions (app, title, started_at, ended_at, duration) VALUES (?, ?, ?, ?, ?)`,
		nil,
		"Missing app",
		time.Now().UTC(),
		time.Now().UTC(),
		60,
	)
	if err == nil {
		t.Fatal("expected missing app to fail")
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "trace.db"))
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close test store: %v", err)
		}
	})

	return store
}

func testSession(app string, title string, startedAt time.Time) Session {
	return Session{
		App:       app,
		Title:     title,
		StartedAt: startedAt,
		EndedAt:   startedAt.Add(10 * time.Minute),
		Duration:  600,
	}
}

func assertSession(t *testing.T, got Session, want Session) {
	t.Helper()

	if got.ID != want.ID {
		t.Fatalf("ID mismatch: got %d, want %d", got.ID, want.ID)
	}
	if got.App != want.App {
		t.Fatalf("app mismatch: got %q, want %q", got.App, want.App)
	}
	if got.Title != want.Title {
		t.Fatalf("title mismatch: got %q, want %q", got.Title, want.Title)
	}
	if !got.StartedAt.Equal(want.StartedAt) {
		t.Fatalf("started_at mismatch: got %v, want %v", got.StartedAt, want.StartedAt)
	}
	if !got.EndedAt.Equal(want.EndedAt) {
		t.Fatalf("ended_at mismatch: got %v, want %v", got.EndedAt, want.EndedAt)
	}
	if got.Duration != want.Duration {
		t.Fatalf("duration mismatch: got %d, want %d", got.Duration, want.Duration)
	}
}
