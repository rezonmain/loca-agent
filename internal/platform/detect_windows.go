//go:build windows

package platform

import (
	"context"
	"strconv"
	"strings"

	"github.com/rezonmain/loca-agent/internal/sys"
)

func detectSpecifics(ctx context.Context, run sys.Runner, si *SystemInfo) {
	if out, err := psQuery(ctx, run, "(Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory"); err == nil {
		if n, err := strconv.ParseUint(strings.TrimSpace(out), 10, 64); err == nil {
			si.RAMBytes = n
		}
	}
	if out, err := psQuery(ctx, run, "(Get-CimInstance Win32_Processor | Select-Object -First 1).Name"); err == nil {
		si.CPU.Model = strings.TrimSpace(out)
	}
	detectGPUWindows(ctx, run, si)
}

// detectGPUWindows enumerates video controllers with their reported VRAM.
func detectGPUWindows(ctx context.Context, run sys.Runner, si *SystemInfo) {
	// AdapterRAM is a uint32 and undercounts on GPUs with >4 GB, but is a
	// reasonable hint; robust VRAM detection is refined in a later phase.
	out, err := psQuery(ctx, run,
		"Get-CimInstance Win32_VideoController | ForEach-Object { \"$($_.Name)|$($_.AdapterRAM)\" }")
	if err != nil {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, ramStr, _ := strings.Cut(line, "|")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		gpu := GPUInfo{Vendor: classifyVendor(name), Model: name}
		if n, err := strconv.ParseUint(strings.TrimSpace(ramStr), 10, 64); err == nil {
			gpu.VRAMBytes = n
		}
		si.GPU = append(si.GPU, gpu)
	}
}

func psQuery(ctx context.Context, run sys.Runner, expr string) (string, error) {
	return run.Run(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", expr)
}
