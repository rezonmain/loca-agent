package windows

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/wireguard"
)

// loadOrCreateServerKeys returns the server's persistent key pair, generating
// and saving it on first run. The private key is stored owner-only. Persisting
// the identity keeps the server's public key (and thus issued tokens) stable
// across re-runs.
func loadOrCreateServerKeys(path string) (wireguard.Keypair, error) {
	if b, err := os.ReadFile(path); err == nil {
		priv := strings.TrimSpace(string(b))
		pub, err := wireguard.PublicFromPrivate(priv)
		if err != nil {
			return wireguard.Keypair{}, errs.Wrap(err, "server_key_invalid",
				"The stored WireGuard server key is invalid",
				"Delete "+path+" and re-run install to regenerate it.")
		}
		return wireguard.Keypair{PrivateKey: priv, PublicKey: pub}, nil
	}

	keys, err := wireguard.GenerateKeypair()
	if err != nil {
		return wireguard.Keypair{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return wireguard.Keypair{}, errs.Wrap(err, "server_key_dir", "Could not create the key directory", "")
	}
	if err := os.WriteFile(path, []byte(keys.PrivateKey), 0o600); err != nil {
		return wireguard.Keypair{}, errs.Wrap(err, "server_key_write", "Could not save the WireGuard server key", "")
	}
	return keys, nil
}
