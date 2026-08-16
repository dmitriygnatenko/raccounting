package main

import (
	"log/slog"
	"os"

	"raccounting/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
