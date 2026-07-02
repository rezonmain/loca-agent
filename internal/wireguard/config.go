package wireguard

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	assets "github.com/rezonmain/loca-agent"
)

// Config is a renderable WireGuard configuration: one [Interface] and zero or
// more [Peer] sections. The same structure represents both server and client
// configs; they differ only in which fields are populated.
type Config struct {
	Interface Interface
	Peers     []Peer
}

// Interface is the local side of a tunnel.
type Interface struct {
	PrivateKey string // this machine's private key (never a peer's)
	Address    string // tunnel address with CIDR, e.g. "10.50.0.2/24"
	ListenPort int    // set for the server; 0 omits the line (clients)
	DNS        string // optional
}

// Peer is a remote side of a tunnel, identified only by its public key.
type Peer struct {
	Name         string   // comment label
	PublicKey    string   // the peer's public key
	PresharedKey string   // optional
	AllowedIPs   []string // CIDRs routed to this peer
	Endpoint     string   // host:port to dial; empty for listen-only peers
	Keepalive    int      // PersistentKeepalive seconds; 0 omits the line
}

const templatePath = "templates/wireguard/wg.conf.tmpl"

// Render produces the textual .conf for a Config.
func Render(cfg Config) (string, error) {
	tmpl, err := template.New("wg.conf.tmpl").
		Funcs(template.FuncMap{"join": strings.Join}).
		ParseFS(assets.Templates, templatePath)
	if err != nil {
		return "", fmt.Errorf("parse wireguard template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", fmt.Errorf("render wireguard config: %w", err)
	}
	return buf.String(), nil
}
