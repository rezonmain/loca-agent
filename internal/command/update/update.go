// Package update implements the `bootstrap-ai update` command, which upgrades
// installed components to the versions pinned in configs/versions.yaml.
package update

import (
	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/spf13/cobra"
)

// New builds the update command.
func New(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update installed components to pinned versions",
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.NotImplemented("Updating", 8)
		},
	}
}
