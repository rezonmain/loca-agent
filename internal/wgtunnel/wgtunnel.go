// Package wgtunnel is the OS-touching side of WireGuard: it writes rendered
// configs to disk with safe permissions and brings the tunnel up and down using
// the platform's native tooling (wireguard.exe tunnel services on Windows,
// wg-quick on macOS/Linux).
//
// The pure crypto and config rendering live in internal/wireguard; this package
// only performs side effects. Command construction is data-driven and selected
// by OS, so it is unit-testable with a fake runner on any platform.
package wgtunnel

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/sys"
)

// WriteConfig writes a rendered WireGuard config with owner-only permissions
// (it contains a private key). Parent directories are created 0700.
func WriteConfig(path, contents string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return errs.Wrap(err, "wg_config_dir", "Could not create the WireGuard config directory", "")
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return errs.Wrap(err, "wg_config_write", "Could not write the WireGuard config", "")
	}
	return nil
}

// command is a resolved executable plus arguments.
type command struct {
	bin  string
	args []string
}

// platform builds the OS-specific tunnel commands.
type platform interface {
	up(iface, conf string) command
	down(iface, conf string) command
	status(iface string) command
	isActive(output string, err error) bool
}

// Controller brings the tunnel up/down and reports its state.
type Controller struct {
	run sys.Runner
	p   platform
}

// NewController returns a Controller wired to the host platform's tooling.
func NewController(run sys.Runner) *Controller {
	return &Controller{run: run, p: newPlatform()}
}

func newPlatform() platform {
	switch runtime.GOOS {
	case "windows":
		return windowsPlatform{exe: "wireguard"}
	default:
		return unixPlatform{wgQuick: "wg-quick", wg: "wg"}
	}
}

// Up brings the tunnel described by conf online.
func (c *Controller) Up(ctx context.Context, iface, conf string) error {
	cmd := c.p.up(iface, conf)
	if _, err := c.run.Run(ctx, cmd.bin, cmd.args...); err != nil {
		return errs.Wrap(err, "wg_tunnel_up",
			"Failed to bring up the WireGuard tunnel",
			"Ensure WireGuard is installed and you are running with administrator/root privileges.")
	}
	return nil
}

// Down takes the tunnel offline.
func (c *Controller) Down(ctx context.Context, iface, conf string) error {
	cmd := c.p.down(iface, conf)
	if _, err := c.run.Run(ctx, cmd.bin, cmd.args...); err != nil {
		return errs.Wrap(err, "wg_tunnel_down",
			"Failed to bring down the WireGuard tunnel",
			"Ensure WireGuard is installed and you are running with administrator/root privileges.")
	}
	return nil
}

// Active reports whether the tunnel is currently up. It is best-effort: any
// failure to determine state is reported as inactive.
func (c *Controller) Active(ctx context.Context, iface string) bool {
	cmd := c.p.status(iface)
	out, err := c.run.Run(ctx, cmd.bin, cmd.args...)
	return c.p.isActive(out, err)
}

// windowsPlatform uses the WireGuard tunnel-service mechanism.
type windowsPlatform struct{ exe string }

func (w windowsPlatform) up(_, conf string) command {
	return command{w.exe, []string{"/installtunnelservice", conf}}
}
func (w windowsPlatform) down(iface, _ string) command {
	return command{w.exe, []string{"/uninstalltunnelservice", iface}}
}
func (w windowsPlatform) status(iface string) command {
	return command{"sc", []string{"query", "WireGuardTunnel$" + iface}}
}
func (w windowsPlatform) isActive(output string, err error) bool {
	return err == nil && strings.Contains(strings.ToUpper(output), "RUNNING")
}

// unixPlatform uses wg-quick (macOS/Linux). These commands require root, which
// the installer arranges via sudo.
type unixPlatform struct {
	wgQuick string
	wg      string
}

func (u unixPlatform) up(_, conf string) command {
	return command{u.wgQuick, []string{"up", conf}}
}
func (u unixPlatform) down(_, conf string) command {
	return command{u.wgQuick, []string{"down", conf}}
}
func (u unixPlatform) status(iface string) command {
	return command{u.wg, []string{"show", iface}}
}
func (u unixPlatform) isActive(_ string, err error) bool {
	return err == nil
}
