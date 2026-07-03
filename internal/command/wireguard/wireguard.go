// Package wireguard implements the `bootstrap-ai wireguard` command group. Its
// add-peer subcommand registers a macOS client on the server from the client's
// enrollment reply, re-rendering the server config and reloading the tunnel.
package wireguard

import (
	"path/filepath"

	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/sys"
	"github.com/rezonmain/loca-agent/internal/wgtunnel"
	wg "github.com/rezonmain/loca-agent/internal/wireguard"
	"github.com/spf13/cobra"
)

const ifaceName = "wg0"

// New builds the wireguard command group.
func New(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wireguard",
		Short: "Manage the WireGuard tunnel and peers",
	}
	cmd.AddCommand(newAddPeerCmd(a))
	return cmd
}

func newAddPeerCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "add-peer <enrollment-reply>",
		Short: "Register a client from its enrollment reply (server side)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			reply, err := wg.DecodeReply(args[0])
			if err != nil {
				return errs.Wrap(err, "invalid_reply",
					"The enrollment reply is invalid",
					"Paste the exact reply printed by the macOS client install.")
			}

			cfgDir := a.Cfg.Installer.Paths.ConfigDir
			store := wgtunnel.NewPeerStore(filepath.Join(cfgDir, "peers.json"))
			peers, err := store.Add(wg.PeerFromReply(reply))
			if err != nil {
				return err
			}

			keys, err := wgtunnel.LoadOrCreateKey(filepath.Join(cfgDir, "server.key"))
			if err != nil {
				return err
			}
			id := wg.ServerIdentity{
				Name:       "windows-server",
				Keys:       keys,
				Address:    a.Cfg.Network.WireGuard.ServerAddress,
				Subnet:     a.Cfg.Network.WireGuard.Subnet,
				ListenPort: a.Cfg.Network.WireGuard.ListenPort,
				Keepalive:  a.Cfg.Network.WireGuard.KeepaliveSeconds,
			}
			serverCfg, err := id.Config(peers)
			if err != nil {
				return err
			}
			rendered, err := wg.Render(serverCfg)
			if err != nil {
				return err
			}
			confPath := filepath.Join(cfgDir, ifaceName+".conf")
			if err := wgtunnel.WriteConfig(confPath, rendered); err != nil {
				return err
			}

			// Reload the tunnel so the new peer takes effect.
			ctrl := wgtunnel.NewController(sys.NewExecRunner(a.Log))
			_ = ctrl.Down(ctx, ifaceName, confPath)
			if err := ctrl.Up(ctx, ifaceName, confPath); err != nil {
				return err
			}

			a.UI.Successf("Registered %s (%s) — %d peer(s) total", reply.Name, reply.ClientAddress, len(peers))
			return nil
		},
	}
}
