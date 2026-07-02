// Package install implements the `bootstrap-ai install` command, which runs the
// machine-role installation workflow (macOS client or Windows AI server).
// The installation engine and steps are implemented in Phases 5–8.
package install

import (
	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/spf13/cobra"
)

// New builds the install command.
func New(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install and configure this machine (client or server)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.NotImplemented("Installation", 5)
		},
	}
}
