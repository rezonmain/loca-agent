// Package app defines the shared application context that CLI commands depend
// on. It is constructed once during CLI startup and injected into each command
// package, keeping commands decoupled from wiring and easy to test.
package app

import (
	"log/slog"

	"github.com/rezonmain/loca-agent/internal/config"
	"github.com/rezonmain/loca-agent/internal/platform"
	"github.com/rezonmain/loca-agent/internal/ui"
)

// App carries the dependencies every command needs.
type App struct {
	Log      *slog.Logger
	Cfg      *config.Config
	Platform platform.Provider
	UI       *ui.UI
}

// NotImplemented reports that a feature is planned for a later phase. It writes
// a friendly note and returns nil so the CLI exits cleanly.
func (a *App) NotImplemented(feature string, phase int) error {
	a.UI.Notef("%s is planned for Phase %d and is not implemented in this build.", feature, phase)
	return nil
}
