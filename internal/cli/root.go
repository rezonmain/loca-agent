// Package cli wires the bootstrap-ai command tree together. It owns global
// flags, constructs the shared app.App during startup, and delegates each
// subcommand to its own package under internal/command.
package cli

import (
	"log/slog"

	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/rezonmain/loca-agent/internal/command/benchmark"
	"github.com/rezonmain/loca-agent/internal/command/configure"
	"github.com/rezonmain/loca-agent/internal/command/doctor"
	"github.com/rezonmain/loca-agent/internal/command/install"
	"github.com/rezonmain/loca-agent/internal/command/logs"
	"github.com/rezonmain/loca-agent/internal/command/models"
	"github.com/rezonmain/loca-agent/internal/command/status"
	"github.com/rezonmain/loca-agent/internal/command/uninstall"
	"github.com/rezonmain/loca-agent/internal/command/update"
	wgcmd "github.com/rezonmain/loca-agent/internal/command/wireguard"
	"github.com/rezonmain/loca-agent/internal/config"
	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/logging"
	"github.com/rezonmain/loca-agent/internal/platform"
	"github.com/rezonmain/loca-agent/internal/sys"
	"github.com/rezonmain/loca-agent/internal/ui"
	"github.com/spf13/cobra"
)

// Execute builds and runs the root command, returning a process exit code.
func Execute(version string) int {
	a := &app.App{}

	var (
		verbose   int
		configDir string
		logFile   string
	)

	root := &cobra.Command{
		Use:           "bootstrap-ai",
		Short:         "Install and manage self-hosted AI coding infrastructure",
		Long:          "bootstrap-ai installs and manages a WireGuard-linked macOS coding client\n(OpenCode) and a Windows llama.cpp inference server.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			level := slog.LevelInfo
			switch {
			case verbose >= 2:
				level = logging.LevelVerbose
			case verbose == 1:
				level = slog.LevelDebug
			}

			log, err := logging.New(logging.Options{Level: level, File: logFile, Color: ui.ColorEnabled()})
			if err != nil {
				return err
			}
			a.Log = log
			a.UI = ui.New()
			a.Platform = platform.New(sys.NewExecRunner(log))

			if configDir == "" {
				configDir = config.DefaultUserDir()
			}
			cfg, err := config.Load(configDir)
			if err != nil {
				return err
			}
			a.Cfg = cfg
			return nil
		},
	}

	root.PersistentFlags().CountVarP(&verbose, "verbose", "v", "increase verbosity (-v debug, -vv trace)")
	root.PersistentFlags().StringVar(&configDir, "config", "", "config override directory (default: user config dir)")
	root.PersistentFlags().StringVar(&logFile, "log-file", "", "also write logs to this file")

	root.AddCommand(
		install.New(a),
		doctor.New(a),
		benchmark.New(a),
		update.New(a),
		uninstall.New(a),
		configure.New(a),
		models.New(a),
		status.New(a),
		logs.New(a),
		wgcmd.New(a),
	)

	if err := root.Execute(); err != nil {
		printError(a, err)
		return 1
	}
	return 0
}

// printError renders an error to the user, expanding UserError diagnostics.
func printError(a *app.App, err error) {
	if a.UI == nil {
		// PreRun failed before UI init; fall back to a plain logger-less path.
		a.UI = ui.New()
	}
	if ue, ok := errs.As(err); ok {
		a.UI.Failf("%s", ue.Message)
		if ue.Fix != "" {
			a.UI.Notef("Suggested fix: %s", ue.Fix)
		}
		if a.Log != nil && ue.Cause != nil {
			a.Log.Debug("error cause", "code", ue.Code, "cause", ue.Cause)
		}
		return
	}
	a.UI.Failf("%s", err.Error())
}
