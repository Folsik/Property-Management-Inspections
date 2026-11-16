package main

import (
	"log/slog"
	"os"

	"github.com/Folsik/Property-Management-Inspections/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}
