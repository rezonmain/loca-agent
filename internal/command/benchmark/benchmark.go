// Package benchmark implements the `bootstrap-ai benchmark` command, which
// measures endpoint latency and generation throughput. Implemented in Phase 10.
package benchmark

import (
	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/spf13/cobra"
)

// New builds the benchmark command.
func New(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "benchmark",
		Short: "Measure endpoint latency and tokens/sec",
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.NotImplemented("Benchmarking", 10)
		},
	}
}
