package wireguard

// Enrollment is the result of a client consuming a JoinToken: freshly generated
// local keys, the rendered client Config, and the reply to hand back to the
// server for registration.
type Enrollment struct {
	Keys   Keypair
	Config Config
	Reply  EnrollmentReply
}

// Enroll generates the client's keys locally (never leaving this machine) and
// builds its WireGuard config from a validated token. When withPSK is true, a
// preshared key is generated and placed in both the client's peer block and the
// reply. AllowedIPs is the server's /32 so only inference traffic is tunneled.
func Enroll(token JoinToken, clientName string, withPSK bool) (Enrollment, error) {
	if err := token.validate(); err != nil {
		return Enrollment{}, err
	}

	keys, err := GenerateKeypair()
	if err != nil {
		return Enrollment{}, err
	}

	var psk string
	if withPSK {
		if psk, err = GeneratePresharedKey(); err != nil {
			return Enrollment{}, err
		}
	}

	addr, err := WithPrefix(token.ClientAddress, token.Subnet)
	if err != nil {
		return Enrollment{}, err
	}

	cfg := Config{
		Interface: Interface{
			PrivateKey: keys.PrivateKey,
			Address:    addr,
		},
		Peers: []Peer{{
			Name:         "inference-server",
			PublicKey:    token.ServerPublicKey,
			PresharedKey: psk,
			AllowedIPs:   []string{token.ServerAddress + "/32"},
			Endpoint:     token.Endpoint,
			Keepalive:    token.Keepalive,
		}},
	}

	reply := EnrollmentReply{
		Version:         tokenVersion,
		Name:            clientName,
		ClientPublicKey: keys.PublicKey,
		ClientAddress:   token.ClientAddress,
		PresharedKey:    psk,
	}

	return Enrollment{Keys: keys, Config: cfg, Reply: reply}, nil
}
