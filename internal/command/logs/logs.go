// Package logs implements the `bootstrap-ai logs` command, which shows recent
// bootstrap-ai and inference-server logs. Implemented in a later phase.
package logs

import (
	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/spf13/cobra"
)

// New builds the logs command.
func New(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "logs",
		Short: "Show recent logs",
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.NotImplemented("Log viewing", 10)
		},
	}
}
