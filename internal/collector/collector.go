package collector

import (
	"context"
	"time"

	"github.com/RileighLuvsCatz/retrace/db"
	"github.com/RileighLuvsCatz/retrace/internal/window"
)

const DefaultPollInterval = 2 * time.Second

// Store persists completed usage sessions.
type Store interface {
	CreateSession(ctx context.Context, session *db.Session) error
}

// Config controls the active-window polling loop.
type Config struct {
	PollInterval time.Duration
	Now          func() time.Time
}

// Run polls the active window until ctx is canceled and writes completed sessions.
func Run(ctx context.Context, store Store, provider window.Provider, config Config) error {
	interval := config.PollInterval
	if interval <= 0 {
		interval = DefaultPollInterval
	}

	now := config.Now
	if now == nil {
		now = time.Now
	}

	aggregator := Aggregator{store: store}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := aggregator.Observe(ctx, pollWindow(ctx, provider), now()); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return aggregator.Flush(context.Background(), now())
		case tickedAt := <-ticker.C:
			if err := aggregator.Observe(ctx, pollWindow(ctx, provider), tickedAt); err != nil {
				return err
			}
		}
	}
}

func pollWindow(ctx context.Context, provider window.Provider) window.Info {
	info, err := provider.CurrentWindow(ctx)
	if err != nil {
		return window.Info{}
	}

	return info
}

// Aggregator coalesces continuous matching window observations into sessions.
type Aggregator struct {
	store  Store
	active *activeSession
}

// Observe records a focused window observation at observedAt.
func (a *Aggregator) Observe(ctx context.Context, info window.Info, observedAt time.Time) error {
	if info.App == "" {
		return a.Flush(ctx, observedAt)
	}

	if a.active == nil {
		a.active = &activeSession{info: info, startedAt: observedAt, lastSeenAt: observedAt}
		return nil
	}

	if sameWindow(a.active.info, info) {
		a.active.lastSeenAt = observedAt
		return nil
	}

	if err := a.Flush(ctx, observedAt); err != nil {
		return err
	}

	a.active = &activeSession{info: info, startedAt: observedAt, lastSeenAt: observedAt}
	return nil
}

// Flush writes the active session through endedAt.
func (a *Aggregator) Flush(ctx context.Context, endedAt time.Time) error {
	if a.active == nil {
		return nil
	}

	active := a.active
	a.active = nil

	if endedAt.Before(active.startedAt) || endedAt.Equal(active.startedAt) {
		return nil
	}

	session := db.Session{
		App:       active.info.App,
		Title:     active.info.Title,
		StartedAt: active.startedAt,
		EndedAt:   endedAt,
		Duration:  int64(endedAt.Sub(active.startedAt).Seconds()),
	}

	if session.Duration <= 0 {
		return nil
	}

	return a.store.CreateSession(ctx, &session)
}

type activeSession struct {
	info       window.Info
	startedAt  time.Time
	lastSeenAt time.Time
}

func sameWindow(left window.Info, right window.Info) bool {
	return left.App == right.App && left.Title == right.Title
}
