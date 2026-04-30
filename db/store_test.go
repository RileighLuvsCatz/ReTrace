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

func TestUsageAggregates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestStore(t)
	base := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)

	sessions := []Session{
		testSession("Editor", "ReTrace", base),
		testSession("Browser", "Docs", base.Add(2*time.Hour)),
		testSession("Editor", "Tests", base.AddDate(0, 0, 1)),
	}
	for i := range sessions {
		if err := store.CreateSession(ctx, &sessions[i]); err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
	}

	apps, err := store.AppUsageBetween(ctx, base, base.AddDate(0, 0, 7))
	if err != nil {
		t.Fatalf("app usage: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 app totals, got %d", len(apps))
	}
	if apps[0].App != "Editor" || apps[0].Seconds != 1200 || apps[0].Sessions != 2 {
		t.Fatalf("unexpected top app usage: %+v", apps[0])
	}

	days, err := store.DailyUsageBetween(ctx, base, base.AddDate(0, 0, 7))
	if err != nil {
		t.Fatalf("daily usage: %v", err)
	}
	if len(days) != 2 {
		t.Fatalf("expected 2 daily totals, got %d", len(days))
	}
	if days[0].Seconds != 1200 || days[0].Sessions != 2 {
		t.Fatalf("unexpected first day usage: %+v", days[0])
	}

	total, count, err := store.TotalUsage(ctx)
	if err != nil {
		t.Fatalf("total usage: %v", err)
	}
	if total != 1800 || count != 3 {
		t.Fatalf("unexpected total usage: total=%d count=%d", total, count)
	}

	hour, ok, err := store.MostActiveHour(ctx)
	if err != nil {
		t.Fatalf("most active hour: %v", err)
	}
	if !ok || hour.Hour != 9 || hour.Seconds != 1200 {
		t.Fatalf("unexpected most active hour: ok=%t hour=%+v", ok, hour)
	}
}

func TestAppUsageByName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := openTestStore(t)
	session := testSession("Editor", "ReTrace", time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC))

	if err := store.CreateSession(ctx, &session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	usage, sessions, err := store.AppUsageByName(ctx, "editor")
	if err != nil {
		t.Fatalf("app usage by name: %v", err)
	}
	if usage.App != "Editor" || usage.Seconds != 600 || usage.Sessions != 1 {
		t.Fatalf("unexpected app usage: %+v", usage)
	}
	if len(sessions) != 1 || sessions[0].Title != "ReTrace" {
		t.Fatalf("unexpected sessions: %+v", sessions)
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
