// Package uninstall implements the `bootstrap-ai uninstall` command, which
// reverses installation by running each step's rollback in reverse order.
package uninstall

import (
	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/spf13/cobra"
)

// New builds the uninstall command.
func New(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove installed components and configuration",
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.NotImplemented("Uninstallation", 8)
		},
	}
}
