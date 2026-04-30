package collector

import (
	"context"
	"testing"
	"time"

	"github.com/RileighLuvsCatz/retrace/db"
	"github.com/RileighLuvsCatz/retrace/internal/window"
)

func TestAggregatorCoalescesContinuousObservations(t *testing.T) {
	t.Parallel()

	store := &memoryStore{}
	aggregator := Aggregator{store: store}
	startedAt := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)

	if err := aggregator.Observe(context.Background(), window.Info{App: "Editor", Title: "ReTrace"}, startedAt); err != nil {
		t.Fatalf("observe first window: %v", err)
	}
	if err := aggregator.Observe(context.Background(), window.Info{App: "Editor", Title: "ReTrace"}, startedAt.Add(2*time.Second)); err != nil {
		t.Fatalf("observe matching window: %v", err)
	}
	if err := aggregator.Observe(context.Background(), window.Info{App: "Browser", Title: "Docs"}, startedAt.Add(5*time.Second)); err != nil {
		t.Fatalf("observe changed window: %v", err)
	}
	if err := aggregator.Flush(context.Background(), startedAt.Add(9*time.Second)); err != nil {
		t.Fatalf("flush final window: %v", err)
	}

	if len(store.sessions) != 2 {
		t.Fatalf("expected 2 completed sessions, got %d", len(store.sessions))
	}

	assertSession(t, store.sessions[0], "Editor", "ReTrace", startedAt, startedAt.Add(5*time.Second), 5)
	assertSession(t, store.sessions[1], "Browser", "Docs", startedAt.Add(5*time.Second), startedAt.Add(9*time.Second), 4)
}

func TestAggregatorFlushesOnMissingWindow(t *testing.T) {
	t.Parallel()

	store := &memoryStore{}
	aggregator := Aggregator{store: store}
	startedAt := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)

	if err := aggregator.Observe(context.Background(), window.Info{App: "Terminal", Title: "Build"}, startedAt); err != nil {
		t.Fatalf("observe window: %v", err)
	}
	if err := aggregator.Observe(context.Background(), window.Info{}, startedAt.Add(3*time.Second)); err != nil {
		t.Fatalf("observe missing window: %v", err)
	}

	if len(store.sessions) != 1 {
		t.Fatalf("expected 1 completed session, got %d", len(store.sessions))
	}

	assertSession(t, store.sessions[0], "Terminal", "Build", startedAt, startedAt.Add(3*time.Second), 3)
}

type memoryStore struct {
	sessions []db.Session
}

func (s *memoryStore) CreateSession(_ context.Context, session *db.Session) error {
	s.sessions = append(s.sessions, *session)
	return nil
}

func assertSession(t *testing.T, got db.Session, app string, title string, startedAt time.Time, endedAt time.Time, duration int64) {
	t.Helper()

	if got.App != app {
		t.Fatalf("app mismatch: got %q, want %q", got.App, app)
	}
	if got.Title != title {
		t.Fatalf("title mismatch: got %q, want %q", got.Title, title)
	}
	if !got.StartedAt.Equal(startedAt) {
		t.Fatalf("started_at mismatch: got %v, want %v", got.StartedAt, startedAt)
	}
	if !got.EndedAt.Equal(endedAt) {
		t.Fatalf("ended_at mismatch: got %v, want %v", got.EndedAt, endedAt)
	}
	if got.Duration != duration {
		t.Fatalf("duration mismatch: got %d, want %d", got.Duration, duration)
	}
}
