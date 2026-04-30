package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const createSessionsTable = `
CREATE TABLE IF NOT EXISTS sessions (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    app       TEXT NOT NULL,
    title     TEXT,
    started_at DATETIME NOT NULL,
    ended_at  DATETIME NOT NULL,
    duration  INTEGER NOT NULL
);`

// Store owns the SQLite connection and session persistence methods.
type Store struct {
	db *sql.DB
}

// Open creates a SQLite-backed store and runs migrations before returning it.
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	store := &Store{db: conn}
	if err := store.Migrate(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return store, nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}

	return s.db.Close()
}

// Migrate applies the database schema required by the application.
func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("database store is not initialized")
	}

	if _, err := s.db.ExecContext(ctx, createSessionsTable); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	return nil
}

// CreateSession inserts a new session and updates session.ID with the new row ID.
func (s *Store) CreateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return errors.New("session is required")
	}

	result, err := s.db.ExecContext(
		ctx,
		`INSERT INTO sessions (app, title, started_at, ended_at, duration) VALUES (?, ?, ?, ?, ?)`,
		session.App,
		session.Title,
		session.StartedAt,
		session.EndedAt,
		session.Duration,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read inserted session id: %w", err)
	}

	session.ID = id
	return nil
}

// GetSession returns one session by ID.
func (s *Store) GetSession(ctx context.Context, id int64) (Session, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, app, title, started_at, ended_at, duration FROM sessions WHERE id = ?`,
		id,
	)

	session, err := scanSession(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, err
		}

		return Session{}, fmt.Errorf("get session: %w", err)
	}

	return session, nil
}

// ListSessions returns all sessions ordered by start time.
func (s *Store) ListSessions(ctx context.Context) ([]Session, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, app, title, started_at, ended_at, duration FROM sessions ORDER BY started_at ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}

		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return sessions, nil
}

// UpdateSession replaces the editable fields for an existing session.
func (s *Store) UpdateSession(ctx context.Context, session Session) error {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE sessions SET app = ?, title = ?, started_at = ?, ended_at = ?, duration = ? WHERE id = ?`,
		session.App,
		session.Title,
		session.StartedAt,
		session.EndedAt,
		session.Duration,
		session.ID,
	)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated session count: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteSession removes one session by ID.
func (s *Store) DeleteSession(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted session count: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(scanner sessionScanner) (Session, error) {
	var session Session
	var title sql.NullString
	var startedAt time.Time
	var endedAt time.Time

	err := scanner.Scan(
		&session.ID,
		&session.App,
		&title,
		&startedAt,
		&endedAt,
		&session.Duration,
	)
	if err != nil {
		return Session{}, err
	}

	session.Title = title.String
	session.StartedAt = startedAt
	session.EndedAt = endedAt
	return session, nil
}
