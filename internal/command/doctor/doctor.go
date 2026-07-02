// Package doctor implements the `bootstrap-ai doctor` command. In this phase it
// verifies configuration and hardware; the full health-check suite (llama.cpp,
// endpoint reachability, WireGuard, OpenCode, latency) is added in Phase 10.
package doctor

import (
	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/rezonmain/loca-agent/internal/probe"
	"github.com/rezonmain/loca-agent/internal/sys"
	"github.com/spf13/cobra"
)

// New builds the doctor command.
func New(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the local machine and inference path",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			a.UI.Headingf("bootstrap-ai doctor")
			a.UI.Successf("Configuration loaded and valid")

			si, _ := a.Platform.Detect(ctx)
			if gpu, ok := si.PrimaryGPU(); ok {
				a.UI.Successf("GPU detected: %s (backend hint: %s)", gpu.Model, si.BackendHint())
			} else {
				a.UI.Warnf("No GPU detected — inference would fall back to CPU")
			}

			a.UI.Headingf("Dependencies")
			runner := sys.NewExecRunner(a.Log)
			for _, st := range probe.ProbeAll(ctx, probe.New(runner)) {
				switch {
				case st.Installed && st.Version != "":
					a.UI.Successf("%s installed (%s)", st.Name, st.Version)
				case st.Installed:
					a.UI.Successf("%s installed", st.Name)
				default:
					a.UI.Warnf("%s not found", st.Name)
				}
			}

			a.UI.Notef("Endpoint, WireGuard-link, and latency checks arrive in Phase 10.")
			return nil
		},
	}
}
