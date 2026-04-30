package cmd

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RileighLuvsCatz/retrace/db"
)

func TestTodayCommandPrintsAppUsage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "trace.db")
	store, err := db.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})

	now := time.Now()
	session := db.Session{
		App:       "Editor",
		Title:     "ReTrace",
		StartedAt: now.Add(-30 * time.Minute),
		EndedAt:   now.Add(-20 * time.Minute),
		Duration:  600,
	}
	if err := store.CreateSession(ctx, &session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command := NewRootCommand(stdout, stderr)
	command.SetArgs([]string{"--db", dbPath, "today"})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute today: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Today") {
		t.Fatalf("expected today header, got %q", output)
	}
	if !strings.Contains(output, "Editor") {
		t.Fatalf("expected editor usage, got %q", output)
	}
	if !strings.Contains(output, "10m 00s") {
		t.Fatalf("expected formatted duration, got %q", output)
	}
}

func TestUnknownCommandReturnsError(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command := NewRootCommand(stdout, stderr)
	command.SetArgs([]string{"nope"})

	if err := command.Execute(); err == nil {
		t.Fatal("expected unknown command to fail")
	}
}
