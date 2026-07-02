//go:build darwin

package platform

import (
	"context"
	"strconv"
	"strings"

	"github.com/rezonmain/loca-agent/internal/sys"
)

func detectSpecifics(ctx context.Context, run sys.Runner, si *SystemInfo) {
	if out, err := run.Run(ctx, "sysctl", "-n", "hw.memsize"); err == nil {
		if n, err := strconv.ParseUint(strings.TrimSpace(out), 10, 64); err == nil {
			si.RAMBytes = n
		}
	}
	if out, err := run.Run(ctx, "sysctl", "-n", "machdep.cpu.brand_string"); err == nil {
		si.CPU.Model = strings.TrimSpace(out)
	}
	detectGPUDarwin(ctx, run, si)
}

// detectGPUDarwin parses `system_profiler SPDisplaysDataType` for chipset names.
func detectGPUDarwin(ctx context.Context, run sys.Runner, si *SystemInfo) {
	out, err := run.Run(ctx, "system_profiler", "SPDisplaysDataType")
	if err != nil {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Chipset Model:") {
			model := strings.TrimSpace(strings.TrimPrefix(line, "Chipset Model:"))
			if model != "" {
				si.GPU = append(si.GPU, GPUInfo{Vendor: classifyVendor(model), Model: model})
			}
		}
	}
}
