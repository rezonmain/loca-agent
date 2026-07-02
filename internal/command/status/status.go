// Package status implements the `bootstrap-ai status` command, which prints the
// detected system profile alongside the configured inference endpoint.
package status

import (
	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/spf13/cobra"
)

// New builds the status command.
func New(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the detected system and configured endpoint",
		RunE: func(cmd *cobra.Command, _ []string) error {
			si, _ := a.Platform.Detect(cmd.Context())

			a.UI.Headingf("System")
			a.UI.Printf("  OS/Arch : %s/%s", si.OS, si.Arch)
			a.UI.Printf("  CPU     : %s (%d logical)", orUnknown(si.CPU.Model), si.CPU.Logical)
			a.UI.Printf("  RAM     : %s", si.RAMHuman())
			if gpu, ok := si.PrimaryGPU(); ok {
				a.UI.Printf("  GPU     : %s (%s)", gpu.Model, gpu.Vendor)
			} else {
				a.UI.Printf("  GPU     : (none detected)")
			}
			a.UI.Printf("  Backend : %s (hint)", si.BackendHint())

			a.UI.Headingf("Configuration")
			m, _ := a.Cfg.Models.DefaultModel()
			a.UI.Printf("  Model    : %s", m.Name)
			a.UI.Printf("  Endpoint : %s", a.Cfg.Network.EndpointURL())
			a.UI.Printf("  Tunnel   : server=%s clients=%v",
				a.Cfg.Network.WireGuard.ServerAddress, a.Cfg.Network.WireGuard.ClientAddresses)
			a.UI.Printf("  Install  : %s", a.Cfg.Installer.Paths.InstallDir)
			return nil
		},
	}
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
