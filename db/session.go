package db

import "time"

// Session is one continuous span of time spent in an application window.
type Session struct {
	ID        int64
	App       string
	Title     string
	StartedAt time.Time
	EndedAt   time.Time
	Duration  int64
}
