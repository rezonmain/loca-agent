// Package install implements the `bootstrap-ai install` command. It resolves the
// machine role, wires concrete collaborators into the role's installer plan, and
// runs the plan through the installer engine.
package install

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/installer"
	"github.com/rezonmain/loca-agent/internal/installer/mac"
	"github.com/rezonmain/loca-agent/internal/installer/windows"
	"github.com/rezonmain/loca-agent/internal/llama"
	"github.com/rezonmain/loca-agent/internal/model"
	"github.com/rezonmain/loca-agent/internal/service"
	"github.com/rezonmain/loca-agent/internal/sys"
	"github.com/rezonmain/loca-agent/internal/ui"
	"github.com/rezonmain/loca-agent/internal/wgtunnel"
	"github.com/spf13/cobra"
)

// New builds the install command.
func New(a *app.App) *cobra.Command {
	var (
		roleFlag string
		token    string
		endpoint string
		modelID  string
		noPSK    bool
		yes      bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install and configure this machine (client or server)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			role, err := resolveRole(a.UI, roleFlag, yes)
			if err != nil {
				return err
			}

			engine := &installer.Engine{Log: a.Log, Progress: progressReporter(a.UI)}
			st := installer.NewState(role, a.Log)

			switch role {
			case installer.WindowsServer:
				deps, err := buildWindowsDeps(a, endpoint, modelID, yes)
				if err != nil {
					return err
				}
				return engine.Run(ctx, st, windows.Plan(deps))
			case installer.MacClient:
				deps, err := buildMacDeps(a, token, modelID, !noPSK, yes)
				if err != nil {
					return err
				}
				return engine.Run(ctx, st, mac.Plan(deps))
			default:
				return errs.New("unknown_role", "Unknown machine role", "Use --role client or --role server.")
			}
		},
	}

	cmd.Flags().StringVar(&roleFlag, "role", "", "machine role: client (macOS) or server (Windows)")
	cmd.Flags().StringVar(&token, "token", "", "join token from the server (client only)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "public host:port clients dial (server only)")
	cmd.Flags().StringVar(&modelID, "model", "", "model id to install (default: registry default)")
	cmd.Flags().BoolVar(&noPSK, "no-psk", false, "disable the WireGuard preshared key")
	cmd.Flags().BoolVar(&yes, "yes", false, "assume defaults; do not prompt")
	return cmd
}

// resolveRole determines the machine role from the flag, or the host OS, and
// confirms interactively unless --yes was given.
func resolveRole(u *ui.UI, flag string, yes bool) (installer.MachineKind, error) {
	if flag != "" {
		return installer.ParseMachineKind(flag)
	}
	def := installer.MacClient
	if runtime.GOOS == "windows" {
		def = installer.WindowsServer
	}
	if yes || u == nil {
		return def, nil
	}
	if u.Confirm(fmt.Sprintf("Install this machine as the %s?", def), true) {
		return def, nil
	}
	if def == installer.WindowsServer {
		return installer.MacClient, nil
	}
	return installer.WindowsServer, nil
}

func buildWindowsDeps(a *app.App, endpoint, modelID string, yes bool) (windows.Deps, error) {
	runner := sys.NewExecRunner(a.Log)
	src := model.NewHTTPSource(nil)

	if endpoint == "" {
		if yes {
			return windows.Deps{}, errs.New("endpoint_required",
				"--endpoint is required (the public host:port clients will dial)",
				"Pass --endpoint your-host:51820, or omit --yes to be prompted.")
		}
		endpoint = a.UI.Prompt("Public endpoint clients will dial (host:port)",
			fmt.Sprintf("your-host:%d", a.Cfg.Network.WireGuard.ListenPort))
	}

	peers := wgtunnel.NewPeerStore(filepath.Join(a.Cfg.Installer.Paths.ConfigDir, "peers.json"))
	return windows.Deps{
		Cfg:      a.Cfg,
		Log:      a.Log,
		UI:       a.UI,
		Runner:   runner,
		Elevated: sys.IsElevated,
		Detect:   a.Platform.Detect,
		Fetcher:  src,
		Llama:    llama.NewAcquirer(src, a.Log),
		Models:   &model.Downloader{Source: src, Log: a.Log, OnProgress: downloadProgress(a.UI)},
		Service:  service.New("nssm", runner),
		Tunnel:   wgtunnel.NewController(runner),
		Peers:    peers,
		Endpoint: endpoint,
		ModelID:  modelID,
	}, nil
}

func buildMacDeps(a *app.App, token, modelID string, withPSK, yes bool) (mac.Deps, error) {
	runner := sys.NewExecRunner(a.Log)

	if token == "" {
		if yes {
			return mac.Deps{}, errs.New("token_required",
				"--token is required on the client",
				"Pass --token <token> printed by the Windows server install.")
		}
		token = a.UI.Prompt("Paste the join token from the Windows server", "")
	}

	// wg-quick needs root; Homebrew must not. Only the tunnel runner gets sudo.
	tunnelRunner := sys.WithSudo(runner)
	return mac.Deps{
		Cfg:       a.Cfg,
		Log:       a.Log,
		UI:        a.UI,
		Runner:    runner,
		Elevated:  sys.IsElevated,
		Installed: func(bin string) bool { _, err := exec.LookPath(bin); return err == nil },
		Tunnel:    wgtunnel.NewController(tunnelRunner),
		Token:     token,
		ModelID:   modelID,
		WithPSK:   withPSK,
	}, nil
}

// progressReporter renders per-step status to the UI.
func progressReporter(u *ui.UI) func(name string, status installer.StepStatus) {
	return func(name string, status installer.StepStatus) {
		if u == nil {
			return
		}
		switch status {
		case installer.StatusRunning:
			u.Notef("%s ...", name)
		case installer.StatusDone:
			u.Successf("%s", name)
		case installer.StatusSkipped:
			u.Dimf("%s (already satisfied)", name)
		case installer.StatusFailed:
			u.Failf("%s", name)
		case installer.StatusRolledBack:
			u.Warnf("%s (rolled back)", name)
		}
	}
}

// downloadProgress prints model download progress at 10% increments.
func downloadProgress(u *ui.UI) func(model.Progress) {
	last := -1
	return func(p model.Progress) {
		if u == nil || p.Total <= 0 {
			return
		}
		pct := int(p.Downloaded * 100 / p.Total)
		if pct/10 > last/10 || pct == 100 {
			last = pct
			u.Dimf("  downloading %s: %d%%", p.File, pct)
		}
	}
}
