// Package configure implements the `bootstrap-ai configure` command, which
// writes and edits user configuration overrides. Implemented in Phase 9.
package configure

import (
	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/spf13/cobra"
)

// New builds the configure command.
func New(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "configure",
		Short: "Create or edit configuration overrides",
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.NotImplemented("Configuration management", 9)
		},
	}
}
