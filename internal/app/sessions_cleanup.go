package app

import (
	"context"
	"log/slog"
	"time"

	"raccounting/internal/port"
)

// sessionCleanupInterval is how often expired sessions are swept from storage.
const sessionCleanupInterval = time.Hour

// startSessionCleanup runs an immediate sweep of expired sessions, then repeats every interval, in
// the background, until ctx is canceled. Sessions are otherwise only ever removed lazily — the one
// time an expired token happens to be presented again (see the authenticate use case) — so without
// this sweep the sessions table grows forever with rows nobody ever came back to use.
func startSessionCleanup(ctx context.Context, sessions port.SessionRepository, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			if n, err := sessions.DeleteExpired(ctx, time.Now().UTC()); err != nil {
				slog.ErrorContext(ctx, "session cleanup error", "error", err)
			} else if n > 0 {
				slog.InfoContext(ctx, "session cleanup done", "expired_sessions", n)
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
