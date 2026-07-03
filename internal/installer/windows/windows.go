// Package windows builds the installation plan for the Windows AI inference
// server: WireGuard, the llama.cpp server (as a service), the model download,
// the firewall rule, and the WireGuard join token for enrolling the macOS
// client. It becomes a pure inference server — no Git, no repos, no agent.
//
// Steps depend on small consumer-side interfaces (llamaAcquirer, modelDownloader,
// tunnel, peerLoader, service.Manager), so the whole plan is exercisable with
// fakes and no real installation.
package windows

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/rezonmain/loca-agent/internal/config"
	"github.com/rezonmain/loca-agent/internal/installer"
	"github.com/rezonmain/loca-agent/internal/model"
	"github.com/rezonmain/loca-agent/internal/platform"
	"github.com/rezonmain/loca-agent/internal/service"
	"github.com/rezonmain/loca-agent/internal/sys"
	"github.com/rezonmain/loca-agent/internal/ui"
	"github.com/rezonmain/loca-agent/internal/wireguard"
)

// ifaceName is the WireGuard interface / tunnel-service name.
const ifaceName = "wg0"

// llamaAcquirer downloads and unpacks llama.cpp, returning the server binary.
type llamaAcquirer interface {
	Acquire(ctx context.Context, v config.Versions, destDir string) (string, error)
}

// modelDownloader fetches and verifies a model's files.
type modelDownloader interface {
	Download(ctx context.Context, src config.ModelSource, m config.Model, destDir string) ([]model.Result, error)
}

// tunnel brings the WireGuard interface up/down and reports its state.
type tunnel interface {
	Up(ctx context.Context, iface, conf string) error
	Down(ctx context.Context, iface, conf string) error
	Active(ctx context.Context, iface string) bool
}

// peerLoader loads the server's registered clients.
type peerLoader interface {
	Load() ([]wireguard.ServerPeer, error)
}

// Deps are the collaborators the Windows plan needs. The install command wires
// concrete implementations; tests wire fakes.
type Deps struct {
	Cfg      *config.Config
	Log      *slog.Logger
	UI       *ui.UI
	Runner   sys.Runner
	Elevated func() bool
	Detect   func(ctx context.Context) (platform.SystemInfo, error)
	Fetcher  model.Source // downloads the WireGuard installer
	Llama    llamaAcquirer
	Models   modelDownloader
	Service  service.Manager
	Tunnel   tunnel
	Peers    peerLoader

	Endpoint string // public host:port the client dials (user-provided)
	ModelID  string // "" selects the registry default
}

// Plan returns the ordered installation steps for the Windows AI server.
func Plan(d Deps) []installer.Step {
	return []installer.Step{
		preflightStep(d),
		installWireGuardStep(d),
		acquireLlamaStep(d),
		downloadModelStep(d),
		writeServerConfigStep(d),
		bringTunnelUpStep(d),
		installServiceStep(d),
		firewallStep(d),
		reportStep(d),
	}
}

// layout resolves the install paths from config.
type layout struct {
	install    string
	model      string
	config     string
	log        string
	serverConf string
	serverKey  string
	llamaDir   string
}

func (d Deps) layout() layout {
	p := d.Cfg.Installer.Paths
	return layout{
		install:    p.InstallDir,
		model:      p.ModelDir,
		config:     p.ConfigDir,
		log:        p.LogDir,
		serverConf: filepath.Join(p.ConfigDir, ifaceName+".conf"),
		serverKey:  filepath.Join(p.ConfigDir, "server.key"),
		llamaDir:   filepath.Join(p.InstallDir, "llama.cpp"),
	}
}

func (d Deps) logf(msg string, args ...any) {
	if d.Log != nil {
		d.Log.Info(msg, args...)
	}
}
