// Package platform detects the host system: OS, architecture, CPU, GPU(s), and
// physical memory. Detection is exposed through the Provider interface so it
// can be faked in tests, and OS-specific probing lives in build-tagged files
// (detect_darwin.go, detect_windows.go, detect_linux.go).
package platform

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/rezonmain/loca-agent/internal/sys"
)

// Provider detects host system information.
type Provider interface {
	Detect(ctx context.Context) (SystemInfo, error)
}

// SystemInfo is the detected hardware and OS profile.
type SystemInfo struct {
	OS       string // runtime.GOOS
	Arch     string // runtime.GOARCH
	CPU      CPUInfo
	GPU      []GPUInfo
	RAMBytes uint64
}

// CPUInfo describes the processor.
type CPUInfo struct {
	Model   string
	Logical int
}

// GPUInfo describes a graphics adapter. VRAMBytes is 0 when unknown.
type GPUInfo struct {
	Vendor    string
	Model     string
	VRAMBytes uint64
}

// Vendor identifiers used for backend selection.
const (
	VendorAMD    = "amd"
	VendorNVIDIA = "nvidia"
	VendorApple  = "apple"
	VendorIntel  = "intel"
	VendorOther  = "other"
)

// New returns the default Provider. A nil runner uses a real command runner.
func New(run sys.Runner) Provider {
	if run == nil {
		run = sys.NewExecRunner(nil)
	}
	return &provider{run: run}
}

type provider struct {
	run sys.Runner
}

func (p *provider) Detect(ctx context.Context) (SystemInfo, error) {
	si := SystemInfo{OS: runtime.GOOS, Arch: runtime.GOARCH}
	si.CPU.Logical = runtime.NumCPU()
	// Per-OS probing is best-effort: failures degrade fields rather than abort.
	detectSpecifics(ctx, p.run, &si)
	return si, nil
}

// PrimaryGPU returns the first detected GPU, if any.
func (si SystemInfo) PrimaryGPU() (GPUInfo, bool) {
	if len(si.GPU) == 0 {
		return GPUInfo{}, false
	}
	return si.GPU[0], true
}

// BackendHint suggests a llama.cpp backend based on the primary GPU vendor.
func (si SystemInfo) BackendHint() string {
	gpu, ok := si.PrimaryGPU()
	if !ok {
		return "cpu"
	}
	switch gpu.Vendor {
	case VendorAMD:
		return "vulkan"
	case VendorNVIDIA:
		return "cuda"
	case VendorApple:
		return "metal"
	default:
		return "cpu"
	}
}

// RAMHuman formats RAM as a human-readable string.
func (si SystemInfo) RAMHuman() string {
	return humanBytes(si.RAMBytes)
}

// classifyVendor maps a raw adapter description to a known vendor id.
func classifyVendor(desc string) string {
	d := strings.ToLower(desc)
	switch {
	case strings.Contains(d, "amd"), strings.Contains(d, "radeon"), strings.Contains(d, "advanced micro"):
		return VendorAMD
	case strings.Contains(d, "nvidia"), strings.Contains(d, "geforce"), strings.Contains(d, "rtx"):
		return VendorNVIDIA
	case strings.Contains(d, "apple"):
		return VendorApple
	case strings.Contains(d, "intel"):
		return VendorIntel
	default:
		return VendorOther
	}
}

func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
