// Package config loads and validates the application's configuration from the environment. The
// configuration is split into three independent groups, one per file: app.go (how the server runs),
// db.go (how it reaches storage) and log.go (where it writes records). Each has its own Load
// function. This file holds what they share: reading the .env file, and the typed env readers each
// group parses its own variables with.
package config

import (
	"cmp"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// LoadEnv populates process env vars from a local .env file, if present — convenient for local
// development so you don't have to export DB_HOST etc. by hand. Real environment variables always
// win: godotenv.Load never overwrites a variable that's already set, so this is a no-op in
// production/Docker where config comes from the real environment.
func LoadEnv() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: failed to read .env: %v", err)
	}
}

// stringEnv reads name, falling back to def when it's unset or holds nothing but whitespace.
func stringEnv(name, def string) string {
	return cmp.Or(strings.TrimSpace(os.Getenv(name)), def)
}

// portEnv reads name as a TCP port a listener can actually bind, falling back to def when it's
// unset.
func portEnv(name, def string) (string, error) {
	v := stringEnv(name, def)

	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("invalid %s %q: must be a number between 1 and 65535", name, v)
	}

	return v, nil
}

// boolEnv reads name as a boolean, falling back to def when it's unset.
func boolEnv(name string, def bool) (bool, error) {
	v := stringEnv(name, "")
	if v == "" {
		return def, nil
	}

	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q: must be true or false", name, v)
	}

	return b, nil
}

// intEnv reads name as an integer, falling back to def when it's unset.
func intEnv(name string, def int) (int, error) {
	v := stringEnv(name, "")
	if v == "" {
		return def, nil
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: must be an integer", name, v)
	}

	return n, nil
}

// durationEnv reads name as a Go duration string (e.g. "5m", "30s"), falling back to def when it's
// unset.
func durationEnv(name string, def time.Duration) (time.Duration, error) {
	v := stringEnv(name, "")
	if v == "" {
		return def, nil
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, v, err)
	}

	return d, nil
}
