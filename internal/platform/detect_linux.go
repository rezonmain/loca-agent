//go:build linux

package platform

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/rezonmain/loca-agent/internal/sys"
)

func detectSpecifics(ctx context.Context, run sys.Runner, si *SystemInfo) {
	si.RAMBytes = readMemTotal()
	si.CPU.Model = readCPUModel()
	detectGPULinux(ctx, run, si)
}

func readMemTotal() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "MemTotal:") {
			fields := strings.Fields(sc.Text())
			if len(fields) >= 2 {
				if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
					return kb * 1024 // meminfo reports kB
				}
			}
		}
	}
	return 0
}

func readCPUModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "model name") {
			if _, val, ok := strings.Cut(sc.Text(), ":"); ok {
				return strings.TrimSpace(val)
			}
		}
	}
	return ""
}

// detectGPULinux uses lspci when available.
func detectGPULinux(ctx context.Context, run sys.Runner, si *SystemInfo) {
	out, err := run.Run(ctx, "lspci")
	if err != nil {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		l := strings.ToLower(line)
		if strings.Contains(l, "vga compatible controller") || strings.Contains(l, "3d controller") {
			desc := line
			if _, after, ok := strings.Cut(line, ": "); ok {
				desc = after
			}
			si.GPU = append(si.GPU, GPUInfo{Vendor: classifyVendor(desc), Model: strings.TrimSpace(desc)})
		}
	}
}
