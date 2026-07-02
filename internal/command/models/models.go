// Package models implements the `bootstrap-ai models` command, which lists the
// installable models defined in the registry (configs/models.yaml).
package models

import (
	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/spf13/cobra"
)

// New builds the models command.
func New(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List available models from the registry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			reg := a.Cfg.Models
			a.UI.Headingf("Available models (default: %s)", reg.Default)
			for _, m := range reg.Models {
				marker := "  "
				if m.ID == reg.Default {
					marker = "* "
				}
				a.UI.Printf("%s%-22s %s", marker, m.ID, m.Name)
				a.UI.Dimf("    repo=%s quant=%s ctx=%d min_vram=%dGB", m.Repo, m.Quant, m.ContextLength, m.MinVRAMGB)
			}
			return nil
		},
	}
}
