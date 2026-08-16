package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"raccounting/internal/config"
)

// logFilePerm/logDirPerm are what a freshly created log file and its parent directory get: readable
// by anyone who can reach them, writable only by the user running the process.
const (
	logFilePerm = 0o644
	logDirPerm  = 0o755
)

// initLogger installs the process-wide structured logger (via slog.SetDefault) that use cases call
// through the package-level slog.ErrorContext etc. to record operational errors.
//
// Records fan out to up to two destinations, each with its own threshold and its own format. The
// console goes to stdout as plain text; the file gets JSON, for a log aggregator to ingest. The
// returned function closes the log file and must be called before the process exits; it is safe to
// call even when no file was opened.
func initLogger(cfg config.LogConfig) (closeLog func(), err error) {
	handlers := []slog.Handler{
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.ConsoleLevel}),
	}

	closeLog = func() {}

	if cfg.FilePath != "" {
		file, ferr := openLogFile(cfg.FilePath)
		if ferr != nil {
			return nil, ferr
		}

		closeLog = func() { _ = file.Close() }

		handlers = append(handlers, slog.NewJSONHandler(file, &slog.HandlerOptions{Level: cfg.FileLevel}))
	}

	slog.SetDefault(slog.New(multiHandler{handlers: handlers}))

	return closeLog, nil
}

// openLogFile opens the log file for appending, creating it and any missing parent directories.
func openLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), logDirPerm); err != nil {
		return nil, fmt.Errorf("failed to create the log directory for %s: %w", path, err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, logFilePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open the log file %s: %w", path, err)
	}

	return file, nil
}

// multiHandler fans one record out to several handlers, which is what lets the console and the file
// hold different levels: each destination keeps its own slog.Handler with its own threshold, and
// this handler asks each one whether it wants the record.
type multiHandler struct {
	handlers []slog.Handler
}

// Enabled reports whether any destination wants records at this level.
func (h multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, sub := range h.handlers {
		if sub.Enabled(ctx, level) {
			return true
		}
	}

	return false
}

// Handle passes the record to every destination whose own threshold admits it. Each gets a clone:
// slog.Record shares its backing array between copies, so handing the same one to two handlers risks
// them clobbering each other's attributes.
func (h multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs []error

	for _, sub := range h.handlers {
		if !sub.Enabled(ctx, record.Level) {
			continue
		}

		if err := sub.Handle(ctx, record.Clone()); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// WithAttrs returns a handler whose destinations have all been given the attributes.
func (h multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.derive(func(sub slog.Handler) slog.Handler { return sub.WithAttrs(attrs) })
}

// WithGroup returns a handler whose destinations have all opened the group.
func (h multiHandler) WithGroup(name string) slog.Handler {
	return h.derive(func(sub slog.Handler) slog.Handler { return sub.WithGroup(name) })
}

// derive builds a new multiHandler by applying with to each destination, leaving the receiver —
// which slog may still be using — untouched.
func (h multiHandler) derive(with func(slog.Handler) slog.Handler) slog.Handler {
	subs := make([]slog.Handler, len(h.handlers))
	for i, sub := range h.handlers {
		subs[i] = with(sub)
	}

	return multiHandler{handlers: subs}
}
