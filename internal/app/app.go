// Package app wires up the application's dependencies (config, storage, use cases, HTTP server) and
// runs it. It's the composition root, kept separate from cmd/raccounting/main.go so main can stay a
// thin entry point.
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"raccounting/internal/config"
)

// Run loads configuration, opens storage, wires up the use cases and HTTP server, and blocks serving
// requests until the server stops.
func Run() error {
	time.Local = time.UTC

	config.LoadEnv()

	logCfg, err := config.LoadLog()
	if err != nil {
		return fmt.Errorf("invalid log configuration: %w", err)
	}

	closeLog, err := initLogger(logCfg)
	if err != nil {
		return fmt.Errorf("init log error: %w", err)
	}

	defer closeLog()

	appCfg, err := config.LoadApp()
	if err != nil {
		return fmt.Errorf("invalid app configuration: %w", err)
	}

	if !appCfg.CookieSecure {
		slog.Warn("COOKIE_SECURE is false: the session cookie is sent without the Secure flag, " +
			"so it can be intercepted over an unencrypted connection — set COOKIE_SECURE=true once served over HTTPS")
	}

	dbCfg, err := config.LoadDB()
	if err != nil {
		return fmt.Errorf("invalid DB configuration: %w", err)
	}

	// ctx is canceled the moment the process is asked to stop (Ctrl+C, or SIGTERM from e.g. Docker/
	// systemd), which is what lets runServer below drain in-flight requests instead of dropping them
	// — and, incidentally, is also what stops the session-cleanup goroutine started further down.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := openStorage(ctx, dbCfg)
	if err != nil {
		return err
	}

	defer store.Close()

	repos := newRepositories(store)
	svcs := newServices()

	startSessionCleanup(ctx, repos.Sessions, sessionCleanupInterval)

	if err = seedDemoUser(ctx, seedDemoUserRequest{
		Users:    repos.Users,
		Hasher:   svcs.Hasher,
		Username: appCfg.DemoUsername,
		Password: appCfg.DemoPassword,
	}); err != nil {
		return fmt.Errorf("failed to seed the demo user: %w", err)
	}

	srv := newServer(repos, svcs, appCfg.CookieSecure)

	handler, err := newHandler(srv)
	if err != nil {
		return err
	}

	addr := ":" + appCfg.Port
	httpServer := newHTTPServer(addr, handler)

	slog.InfoContext(ctx, fmt.Sprintf("listening on %s", addr))

	return runServer(ctx, httpServer)
}
