package config

import (
	"fmt"
	"log/slog"
)

// Logging defaults, used whenever the corresponding env var isn't set. The console default is
// deliberately quiet — an operator watching a terminal wants problems, not a running commentary —
// while the file, when one is configured, keeps the fuller INFO record you actually want when
// reconstructing what happened.
const (
	defaultConsoleLogLevel = slog.LevelWarn
	defaultFileLogLevel    = slog.LevelInfo
)

// LogConfig describes where log records go and how much of them each destination gets. The two
// destinations are independent: the console can stay at warn while a file records everything from
// info down.
type LogConfig struct {
	ConsoleLevel slog.Level
	FilePath     string
	FileLevel    slog.Level
}

// LoadLog assembles the logging configuration: LOG_CONSOLE_LEVEL for stdout, LOG_FILE_PATH plus
// LOG_FILE_LEVEL for the file.
func LoadLog() (LogConfig, error) {
	consoleLevel, err := levelEnv("LOG_CONSOLE_LEVEL", defaultConsoleLogLevel)
	if err != nil {
		return LogConfig{}, err
	}

	fileLevel, err := levelEnv("LOG_FILE_LEVEL", defaultFileLogLevel)
	if err != nil {
		return LogConfig{}, err
	}

	return LogConfig{
		ConsoleLevel: consoleLevel,
		FilePath:     stringEnv("LOG_FILE_PATH", ""),
		FileLevel:    fileLevel,
	}, nil
}

// levelEnv reads name as a slog level — "debug", "info", "warn" or "error", case insensitive —
// falling back to def when it's unset.
func levelEnv(name string, def slog.Level) (slog.Level, error) {
	v := stringEnv(name, "")
	if v == "" {
		return def, nil
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(v)); err != nil {
		return 0, fmt.Errorf("invalid %s %q: must be debug, info, warn or error", name, v)
	}

	return level, nil
}
