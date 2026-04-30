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

// AppUsage is an aggregate usage total for one application.
type AppUsage struct {
	App      string
	Seconds  int64
	Sessions int64
}

// DailyUsage is an aggregate usage total for one calendar day.
type DailyUsage struct {
	Day      time.Time
	Seconds  int64
	Sessions int64
}

// HourUsage is an aggregate usage total for one hour of day.
type HourUsage struct {
	Hour     int
	Seconds  int64
	Sessions int64
}
