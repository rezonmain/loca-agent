package windows

import (
	"context"
	"os"
	"path/filepath"

	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/installer"
	"github.com/rezonmain/loca-agent/internal/llama"
	"github.com/rezonmain/loca-agent/internal/model"
	"github.com/rezonmain/loca-agent/internal/service"
	"github.com/rezonmain/loca-agent/internal/wgtunnel"
	"github.com/rezonmain/loca-agent/internal/wireguard"
)

// preflightStep verifies Administrator rights and records the detected system.
func preflightStep(d Deps) installer.Step {
	return installer.FuncStep{
		Label: "preflight",
		RunFunc: func(ctx context.Context, st *installer.State) error {
			if d.Elevated != nil && !d.Elevated() {
				return errs.New("not_elevated",
					"Administrator privileges are required to install the AI server",
					"Re-run bootstrap-ai from an elevated (Administrator) terminal.")
			}
			if d.Detect != nil {
				if info, err := d.Detect(ctx); err == nil {
					st.Set(installer.KeySystemInfo, info)
					if _, ok := info.PrimaryGPU(); !ok {
						d.logf("no GPU detected; llama.cpp will run on CPU (slow)")
					}
				}
			}
			return nil
		},
	}
}

// installWireGuardStep downloads and runs the WireGuard installer if absent.
func installWireGuardStep(d Deps) installer.Step {
	exe := wireguardExePath()
	return installer.FuncStep{
		Label: "install-wireguard",
		VerifyFunc: func(_ context.Context, _ *installer.State) (bool, error) {
			return fileExists(exe), nil
		},
		RunFunc: func(ctx context.Context, _ *installer.State) error {
			dst := filepath.Join(os.TempDir(), "wireguard-installer.exe")
			if err := d.Fetcher.Fetch(ctx, d.Cfg.Versions.WireGuard.WindowsInstallerURL, dst, nil); err != nil {
				return errs.Wrap(err, "wireguard_download",
					"Failed to download the WireGuard installer", "Check your internet connection.")
			}
			// /S requests a silent NSIS install; verify installs the exe presence.
			if _, err := d.Runner.Run(ctx, dst, "/S"); err != nil {
				return errs.Wrap(err, "wireguard_install",
					"WireGuard installation failed",
					"Run the downloaded installer manually: "+dst)
			}
			return nil
		},
	}
}

// acquireLlamaStep downloads and unpacks the llama.cpp server binary.
func acquireLlamaStep(d Deps) installer.Step {
	l := d.layout()
	return installer.FuncStep{
		Label: "acquire-llama",
		VerifyFunc: func(_ context.Context, st *installer.State) (bool, error) {
			if bin, err := llama.FindServerBinary(l.llamaDir); err == nil {
				st.Set(installer.KeyServerBinary, bin)
				return true, nil
			}
			return false, nil
		},
		RunFunc: func(ctx context.Context, st *installer.State) error {
			bin, err := d.Llama.Acquire(ctx, d.Cfg.Versions, l.install)
			if err != nil {
				return err
			}
			st.Set(installer.KeyServerBinary, bin)
			return nil
		},
	}
}

// downloadModelStep resolves and downloads the selected model.
func downloadModelStep(d Deps) installer.Step {
	l := d.layout()
	return installer.FuncStep{
		Label: "download-model",
		VerifyFunc: func(_ context.Context, st *installer.State) (bool, error) {
			m, err := model.Resolve(d.Cfg.Models, d.ModelID)
			if err != nil {
				return false, err
			}
			path := filepath.Join(l.model, m.Files[0].Name)
			if fileExists(path) {
				st.Set(installer.KeyModelPath, path)
				return true, nil
			}
			return false, nil
		},
		RunFunc: func(ctx context.Context, st *installer.State) error {
			m, err := model.Resolve(d.Cfg.Models, d.ModelID)
			if err != nil {
				return err
			}
			if _, err := d.Models.Download(ctx, d.Cfg.Models.Source, m, l.model); err != nil {
				return err
			}
			st.Set(installer.KeyModelPath, filepath.Join(l.model, m.Files[0].Name))
			return nil
		},
	}
}

// writeServerConfigStep renders the server WireGuard config and mints a join
// token for the next client. It always runs so the config stays in sync with
// the registered peers.
func writeServerConfigStep(d Deps) installer.Step {
	l := d.layout()
	return installer.FuncStep{
		Label: "wireguard-server-config",
		RunFunc: func(_ context.Context, st *installer.State) error {
			keys, err := wgtunnel.LoadOrCreateKey(l.serverKey)
			if err != nil {
				return err
			}
			id := wireguard.ServerIdentity{
				Name:       "windows-server",
				Keys:       keys,
				Address:    d.Cfg.Network.WireGuard.ServerAddress,
				Subnet:     d.Cfg.Network.WireGuard.Subnet,
				ListenPort: d.Cfg.Network.WireGuard.ListenPort,
				Endpoint:   d.Endpoint,
				Keepalive:  d.Cfg.Network.WireGuard.KeepaliveSeconds,
			}
			peers, err := d.Peers.Load()
			if err != nil {
				return err
			}
			cfg, err := id.Config(peers)
			if err != nil {
				return err
			}
			rendered, err := wireguard.Render(cfg)
			if err != nil {
				return err
			}
			if err := writeConfig(l.serverConf, rendered); err != nil {
				return err
			}
			st.Set(installer.KeyServerConfigPath, l.serverConf)

			addr, err := id.NextClientAddress(peers)
			if err != nil {
				return err
			}
			token, err := id.MintToken(addr).Encode()
			if err != nil {
				return err
			}
			st.Set(installer.KeyJoinToken, token)
			return nil
		},
	}
}

// bringTunnelUpStep starts the WireGuard tunnel if it is not already active.
func bringTunnelUpStep(d Deps) installer.Step {
	l := d.layout()
	return installer.FuncStep{
		Label: "wireguard-tunnel-up",
		VerifyFunc: func(ctx context.Context, _ *installer.State) (bool, error) {
			return d.Tunnel.Active(ctx, ifaceName), nil
		},
		RunFunc: func(ctx context.Context, _ *installer.State) error {
			return d.Tunnel.Up(ctx, ifaceName, l.serverConf)
		},
		RollbackFunc: func(ctx context.Context, _ *installer.State) error {
			return d.Tunnel.Down(ctx, ifaceName, l.serverConf)
		},
	}
}

// installServiceStep installs and starts the llama.cpp inference service.
func installServiceStep(d Deps) installer.Step {
	l := d.layout()
	name := d.Cfg.Installer.Service.Name
	return installer.FuncStep{
		Label: "install-llama-service",
		VerifyFunc: func(ctx context.Context, _ *installer.State) (bool, error) {
			state, err := d.Service.Status(ctx, name)
			if err != nil {
				return false, err
			}
			return state == service.StateRunning, nil
		},
		RunFunc: func(ctx context.Context, st *installer.State) error {
			m, err := model.Resolve(d.Cfg.Models, d.ModelID)
			if err != nil {
				return err
			}
			binary := st.String(installer.KeyServerBinary)
			modelPath := st.String(installer.KeyModelPath)
			args := llama.Args(llama.OptionsFromConfig(d.Cfg.Network, m, modelPath))

			_ = os.MkdirAll(l.log, 0o755)
			spec := service.Spec{
				Name:        name,
				DisplayName: d.Cfg.Installer.Service.DisplayName,
				Executable:  binary,
				Args:        args,
				WorkingDir:  filepath.Dir(binary),
				Stdout:      filepath.Join(l.log, "llama-out.log"),
				Stderr:      filepath.Join(l.log, "llama-err.log"),
				AutoStart:   true,
			}
			if err := d.Service.Install(ctx, spec); err != nil {
				return err
			}
			return d.Service.Start(ctx, name)
		},
		RollbackFunc: func(ctx context.Context, _ *installer.State) error {
			return d.Service.Remove(ctx, name)
		},
	}
}

// firewallStep allows inbound WireGuard UDP traffic.
func firewallStep(d Deps) installer.Step {
	port := d.Cfg.Network.WireGuard.ListenPort
	return installer.FuncStep{
		Label: "configure-firewall",
		VerifyFunc: func(ctx context.Context, _ *installer.State) (bool, error) {
			return firewallRuleExists(ctx, d.Runner, firewallRuleName), nil
		},
		RunFunc: func(ctx context.Context, _ *installer.State) error {
			if _, err := d.Runner.Run(ctx, "netsh", addRuleArgs(firewallRuleName, port)...); err != nil {
				return errs.Wrap(err, "firewall_rule",
					"Failed to add the WireGuard firewall rule",
					"Add an inbound UDP rule for port "+itoa(port)+" manually.")
			}
			return nil
		},
		RollbackFunc: func(ctx context.Context, _ *installer.State) error {
			_, err := d.Runner.Run(ctx, "netsh", deleteRuleArgs(firewallRuleName)...)
			return err
		},
	}
}

// reportStep prints the join token and next steps for enrolling the client.
func reportStep(d Deps) installer.Step {
	return installer.FuncStep{
		Label: "report",
		RunFunc: func(_ context.Context, st *installer.State) error {
			token := st.String(installer.KeyJoinToken)
			if d.UI == nil {
				return nil
			}
			d.UI.Headingf("Windows AI server ready")
			d.UI.Notef("Share this join token with the macOS client:")
			d.UI.Printf("%s", token)
			d.UI.Notef("On the Mac:  bootstrap-ai install --role client --token <token>")
			d.UI.Notef("Then register the reply here:  bootstrap-ai wireguard add-peer <reply>")
			return nil
		},
	}
}
