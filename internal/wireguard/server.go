package wireguard

// ServerIdentity is the Windows inference server's own tunnel identity. It holds
// the server's private key (which never leaves the machine) and the parameters
// needed to render its config and mint enrollment tokens for clients.
type ServerIdentity struct {
	Name       string
	Keys       Keypair
	Address    string // bare server tunnel IP, e.g. "10.50.0.1"
	Subnet     string // e.g. "10.50.0.0/24"
	ListenPort int
	Endpoint   string // public host:port clients dial, e.g. "vpn.example.com:51820"
	Keepalive  int
}

// ServerPeer is a registered client as seen from the server side, identified by
// its public key. The server never learns a client's private key.
type ServerPeer struct {
	Name         string
	PublicKey    string
	Address      string // bare client tunnel IP
	PresharedKey string // optional; must match the client's [Peer] block
}

// Config renders the server's WireGuard configuration: its own [Interface] plus
// one [Peer] per registered client, each locked to that client's /32.
func (s ServerIdentity) Config(peers []ServerPeer) (Config, error) {
	addr, err := WithPrefix(s.Address, s.Subnet)
	if err != nil {
		return Config{}, err
	}
	iface := Interface{
		PrivateKey: s.Keys.PrivateKey,
		Address:    addr,
		ListenPort: s.ListenPort,
	}
	rendered := make([]Peer, 0, len(peers))
	for _, p := range peers {
		rendered = append(rendered, Peer{
			Name:         p.Name,
			PublicKey:    p.PublicKey,
			PresharedKey: p.PresharedKey,
			AllowedIPs:   []string{p.Address + "/32"},
		})
	}
	return Config{Interface: iface, Peers: rendered}, nil
}

// NextClientAddress allocates the lowest free tunnel address, reserving the
// server's own address and every already-registered peer.
func (s ServerIdentity) NextClientAddress(existing []ServerPeer) (string, error) {
	taken := make([]string, 0, len(existing)+1)
	taken = append(taken, s.Address)
	for _, p := range existing {
		taken = append(taken, p.Address)
	}
	return NextClientAddress(s.Subnet, taken)
}

// MintToken builds a JoinToken for a client that will occupy clientAddress. The
// token contains only public information and is safe to move over any channel.
func (s ServerIdentity) MintToken(clientAddress string) JoinToken {
	return JoinToken{
		Version:         tokenVersion,
		ServerPublicKey: s.Keys.PublicKey,
		ServerAddress:   s.Address,
		Endpoint:        s.Endpoint,
		Subnet:          s.Subnet,
		ClientAddress:   clientAddress,
		ListenPort:      s.ListenPort,
		Keepalive:       s.Keepalive,
	}
}

// PeerFromReply converts a client's enrollment reply into a ServerPeer ready to
// append with add-peer.
func PeerFromReply(r EnrollmentReply) ServerPeer {
	return ServerPeer{
		Name:         r.Name,
		PublicKey:    r.ClientPublicKey,
		Address:      r.ClientAddress,
		PresharedKey: r.PresharedKey,
	}
}
