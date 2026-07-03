// Package mac builds the installation plan for the macOS coding client:
// Homebrew, Git, OpenCode, and the WireGuard client. It consumes a join token
// from the Windows server, enrolls locally (generating its own keys), brings up
// the tunnel, and writes an OpenCode config pointing at the tunneled endpoint.
//
// No model is installed on the client. Steps depend on small injected hooks
// (Installed, Tunnel, Elevated) so the plan is exercisable with fakes.
package mac

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rezonmain/loca-agent/internal/config"
	"github.com/rezonmain/loca-agent/internal/installer"
	"github.com/rezonmain/loca-agent/internal/sys"
	"github.com/rezonmain/loca-agent/internal/ui"
)

// ifaceName is the WireGuard interface / config name on the client.
const ifaceName = "wg0"

// tunnel brings the WireGuard interface up/down and reports its state.
type tunnel interface {
	Up(ctx context.Context, iface, conf string) error
	Down(ctx context.Context, iface, conf string) error
	Active(ctx context.Context, iface string) bool
}

// Deps are the collaborators the macOS plan needs.
type Deps struct {
	Cfg       *config.Config
	Log       *slog.Logger
	UI        *ui.UI
	Runner    sys.Runner
	Elevated  func() bool           // true if running as root (not allowed for brew)
	Installed func(bin string) bool // PATH presence check
	Tunnel    tunnel

	Token              string // join token from the Windows server (required)
	ModelID            string // "" selects the registry default
	ClientName         string // label for this client; defaults to "mac-client"
	WithPSK            bool   // include a preshared key in the tunnel
	OpenCodeConfigPath string // override; defaults to ~/.config/opencode/opencode.json
}

// Plan returns the ordered installation steps for the macOS client.
func Plan(d Deps) []installer.Step {
	return []installer.Step{
		preflightStep(d),
		installHomebrewStep(d),
		installGitStep(d),
		installOpenCodeStep(d),
		installWireGuardStep(d),
		enrollStep(d),
		bringTunnelUpStep(d),
		writeOpenCodeConfigStep(d),
		reportStep(d),
	}
}

type layout struct {
	config     string
	clientConf string
	openCode   string
}

func (d Deps) layout() layout {
	cd := d.Cfg.Installer.Paths.ConfigDir
	oc := d.OpenCodeConfigPath
	if oc == "" {
		oc = defaultOpenCodePath()
	}
	return layout{
		config:     cd,
		clientConf: filepath.Join(cd, ifaceName+".conf"),
		openCode:   oc,
	}
}

func (d Deps) isInstalled(bin string) bool {
	if d.Installed != nil {
		return d.Installed(bin)
	}
	_, err := exec.LookPath(bin)
	return err == nil
}

func (d Deps) logf(msg string, args ...any) {
	if d.Log != nil {
		d.Log.Info(msg, args...)
	}
}

func (d Deps) clientName() string {
	if d.ClientName != "" {
		return d.ClientName
	}
	return "mac-client"
}

// defaultOpenCodePath returns the XDG-style OpenCode config location.
func defaultOpenCodePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "opencode.json"
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}
