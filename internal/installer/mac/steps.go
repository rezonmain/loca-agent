package mac

import (
	"context"
	"fmt"
	"strings"

	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/installer"
	"github.com/rezonmain/loca-agent/internal/model"
	"github.com/rezonmain/loca-agent/internal/opencode"
	"github.com/rezonmain/loca-agent/internal/wgtunnel"
	"github.com/rezonmain/loca-agent/internal/wireguard"
)

// preflightStep ensures the install is not run as root and a token is present.
func preflightStep(d Deps) installer.Step {
	return installer.FuncStep{
		Label: "preflight",
		RunFunc: func(_ context.Context, _ *installer.State) error {
			if d.Elevated != nil && d.Elevated() {
				return errs.New("root_not_allowed",
					"Do not run the macOS client install as root",
					"Re-run as your normal user — Homebrew refuses to run as root. The tunnel step uses sudo when needed.")
			}
			if strings.TrimSpace(d.Token) == "" {
				return errs.New("token_required",
					"A WireGuard join token is required",
					"Pass --token with the token printed by the Windows server install.")
			}
			return nil
		},
	}
}

// installHomebrewStep installs Homebrew if brew is not on PATH.
func installHomebrewStep(d Deps) installer.Step {
	return installer.FuncStep{
		Label: "install-homebrew",
		VerifyFunc: func(_ context.Context, _ *installer.State) (bool, error) {
			return d.isInstalled("brew"), nil
		},
		RunFunc: func(ctx context.Context, _ *installer.State) error {
			script := fmt.Sprintf(`NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL %s)"`, d.Cfg.Versions.Homebrew.InstallURL)
			if _, err := d.Runner.Run(ctx, "/bin/bash", "-c", script); err != nil {
				return errs.Wrap(err, "homebrew_install",
					"Homebrew installation failed",
					"Install Homebrew manually from https://brew.sh, then re-run install.")
			}
			return nil
		},
	}
}

// installGitStep installs Git via Homebrew if it is missing.
func installGitStep(d Deps) installer.Step {
	return installer.FuncStep{
		Label: "install-git",
		VerifyFunc: func(_ context.Context, _ *installer.State) (bool, error) {
			return d.isInstalled("git"), nil
		},
		RunFunc: func(ctx context.Context, _ *installer.State) error {
			return brewInstall(ctx, d, "git")
		},
	}
}

// installOpenCodeStep installs OpenCode via npm or brew per config.
func installOpenCodeStep(d Deps) installer.Step {
	return installer.FuncStep{
		Label: "install-opencode",
		VerifyFunc: func(_ context.Context, _ *installer.State) (bool, error) {
			return d.isInstalled("opencode"), nil
		},
		RunFunc: func(ctx context.Context, _ *installer.State) error {
			oc := d.Cfg.Versions.OpenCode
			var err error
			switch oc.InstallVia {
			case "brew":
				err = brewInstall(ctx, d, oc.Package)
			default: // npm
				_, err = d.Runner.Run(ctx, "npm", "install", "-g", oc.Package)
			}
			if err != nil {
				return errs.Wrap(err, "opencode_install",
					"OpenCode installation failed",
					"Install OpenCode manually ("+oc.InstallVia+" "+oc.Package+"), then re-run install.")
			}
			return nil
		},
	}
}

// installWireGuardStep installs the wireguard-tools (wg, wg-quick) via Homebrew.
func installWireGuardStep(d Deps) installer.Step {
	return installer.FuncStep{
		Label: "install-wireguard",
		VerifyFunc: func(_ context.Context, _ *installer.State) (bool, error) {
			return d.isInstalled("wg"), nil
		},
		RunFunc: func(ctx context.Context, _ *installer.State) error {
			return brewInstall(ctx, d, "wireguard-tools")
		},
	}
}

// enrollStep decodes the token, generates local keys, writes the client config,
// and records the enrollment reply for the server to register.
func enrollStep(d Deps) installer.Step {
	l := d.layout()
	return installer.FuncStep{
		Label: "wireguard-enroll",
		RunFunc: func(_ context.Context, st *installer.State) error {
			token, err := wireguard.DecodeToken(d.Token)
			if err != nil {
				return errs.Wrap(err, "invalid_token",
					"The provided join token is invalid",
					"Get a fresh token from the Windows server install.")
			}
			enr, err := wireguard.Enroll(token, d.clientName(), d.WithPSK)
			if err != nil {
				return err
			}
			rendered, err := wireguard.Render(enr.Config)
			if err != nil {
				return err
			}
			if err := wgtunnel.WriteConfig(l.clientConf, rendered); err != nil {
				return err
			}
			reply, err := enr.Reply.Encode()
			if err != nil {
				return err
			}
			st.Set(installer.KeyClientConfigPath, l.clientConf)
			st.Set(installer.KeyEnrollReply, reply)
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
			return d.Tunnel.Up(ctx, ifaceName, l.clientConf)
		},
		RollbackFunc: func(ctx context.Context, _ *installer.State) error {
			return d.Tunnel.Down(ctx, ifaceName, l.clientConf)
		},
	}
}

// writeOpenCodeConfigStep writes the OpenCode config pointing at the endpoint.
func writeOpenCodeConfigStep(d Deps) installer.Step {
	l := d.layout()
	return installer.FuncStep{
		Label: "opencode-config",
		RunFunc: func(_ context.Context, _ *installer.State) error {
			m, err := model.Resolve(d.Cfg.Models, d.ModelID)
			if err != nil {
				return err
			}
			return opencode.Write(l.openCode, opencode.FromConfig(d.Cfg, m))
		},
	}
}

// reportStep prints the enrollment reply and where the config was written.
func reportStep(d Deps) installer.Step {
	l := d.layout()
	return installer.FuncStep{
		Label: "report",
		RunFunc: func(_ context.Context, st *installer.State) error {
			if d.UI == nil {
				return nil
			}
			d.UI.Headingf("macOS client configured")
			d.UI.Notef("Register this client on the Windows server by running there:")
			d.UI.Printf("bootstrap-ai wireguard add-peer %s", st.String(installer.KeyEnrollReply))
			d.UI.Notef("OpenCode config written to: %s", l.openCode)
			d.UI.Notef("Endpoint: %s", d.Cfg.Network.EndpointURL())
			return nil
		},
	}
}

func brewInstall(ctx context.Context, d Deps, pkg string) error {
	if _, err := d.Runner.Run(ctx, "brew", "install", pkg); err != nil {
		return errs.Wrap(err, "brew_install",
			"Failed to install "+pkg+" via Homebrew",
			"Run: brew install "+pkg)
	}
	return nil
}
