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

// AppUsageBetween returns app totals for sessions that overlap the given range.
func (s *Store) AppUsageBetween(ctx context.Context, start time.Time, end time.Time) ([]AppUsage, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT app, COALESCE(SUM(duration), 0), COUNT(*)
		 FROM sessions
		 WHERE started_at < ? AND ended_at > ?
		 GROUP BY app
		 ORDER BY SUM(duration) DESC, app ASC`,
		end,
		start,
	)
	if err != nil {
		return nil, fmt.Errorf("query app usage: %w", err)
	}
	defer rows.Close()

	var usage []AppUsage
	for rows.Next() {
		var row AppUsage
		if err := rows.Scan(&row.App, &row.Seconds, &row.Sessions); err != nil {
			return nil, fmt.Errorf("scan app usage: %w", err)
		}

		usage = append(usage, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate app usage: %w", err)
	}

	return usage, nil
}

// DailyUsageBetween returns daily totals for sessions that overlap the given range.
func (s *Store) DailyUsageBetween(ctx context.Context, start time.Time, end time.Time) ([]DailyUsage, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, app, title, started_at, ended_at, duration
		 FROM sessions
		 WHERE started_at < ? AND ended_at > ?
		 ORDER BY started_at ASC, id ASC`,
		end,
		start,
	)
	if err != nil {
		return nil, fmt.Errorf("query daily usage: %w", err)
	}
	defer rows.Close()

	byDay := make(map[time.Time]*DailyUsage)
	var order []time.Time
	location := start.Location()

	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan daily session: %w", err)
		}

		day := startOfDay(session.StartedAt.In(location))
		row, ok := byDay[day]
		if !ok {
			usage := DailyUsage{Day: day}
			row = &usage
			byDay[day] = row
			order = append(order, day)
		}
		row.Seconds += session.Duration
		row.Sessions++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily usage: %w", err)
	}

	var usage []DailyUsage
	for _, day := range order {
		usage = append(usage, *byDay[day])
	}

	return usage, nil
}

// AppUsageByName returns totals and sessions for one application, matching case-insensitively.
func (s *Store) AppUsageByName(ctx context.Context, app string) (AppUsage, []Session, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, app, title, started_at, ended_at, duration
		 FROM sessions
		 WHERE app = ? COLLATE NOCASE
		 ORDER BY started_at DESC, id DESC`,
		app,
	)
	if err != nil {
		return AppUsage{}, nil, fmt.Errorf("query app sessions: %w", err)
	}
	defer rows.Close()

	var usage AppUsage
	var sessions []Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return AppUsage{}, nil, fmt.Errorf("scan app session: %w", err)
		}

		if usage.App == "" {
			usage.App = session.App
		}
		usage.Seconds += session.Duration
		usage.Sessions++
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return AppUsage{}, nil, fmt.Errorf("iterate app sessions: %w", err)
	}
	if usage.App == "" {
		usage.App = app
	}

	return usage, sessions, nil
}

// TotalUsage returns total recorded usage for all sessions.
func (s *Store) TotalUsage(ctx context.Context) (int64, int64, error) {
	var seconds sql.NullInt64
	var sessions int64
	err := s.db.QueryRowContext(ctx, `SELECT SUM(duration), COUNT(*) FROM sessions`).Scan(&seconds, &sessions)
	if err != nil {
		return 0, 0, fmt.Errorf("query total usage: %w", err)
	}

	return seconds.Int64, sessions, nil
}

// MostActiveHour returns the hour with the highest recorded usage.
func (s *Store) MostActiveHour(ctx context.Context) (HourUsage, bool, error) {
	sessions, err := s.ListSessions(ctx)
	if err != nil {
		return HourUsage{}, false, err
	}
	if len(sessions) == 0 {
		return HourUsage{}, false, nil
	}

	byHour := make(map[int]HourUsage)
	for _, session := range sessions {
		hour := session.StartedAt.Hour()
		usage := byHour[hour]
		usage.Hour = hour
		usage.Seconds += session.Duration
		usage.Sessions++
		byHour[hour] = usage
	}

	best := HourUsage{Hour: -1}
	for _, usage := range byHour {
		if usage.Seconds > best.Seconds || (usage.Seconds == best.Seconds && usage.Sessions > best.Sessions) {
			best = usage
		}
	}

	return best, true, nil
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
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
