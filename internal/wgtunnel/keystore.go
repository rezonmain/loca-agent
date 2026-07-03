package wgtunnel

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/wireguard"
)

// LoadOrCreateKey returns the key pair stored at path, generating and saving a
// new one on first use. The private key is written owner-only. Persisting the
// key keeps a machine's identity (and any issued tokens) stable across runs.
func LoadOrCreateKey(path string) (wireguard.Keypair, error) {
	if b, err := os.ReadFile(path); err == nil {
		priv := strings.TrimSpace(string(b))
		pub, err := wireguard.PublicFromPrivate(priv)
		if err != nil {
			return wireguard.Keypair{}, errs.Wrap(err, "wg_key_invalid",
				"The stored WireGuard key is invalid",
				"Delete "+path+" and re-run to regenerate it.")
		}
		return wireguard.Keypair{PrivateKey: priv, PublicKey: pub}, nil
	}

	keys, err := wireguard.GenerateKeypair()
	if err != nil {
		return wireguard.Keypair{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return wireguard.Keypair{}, errs.Wrap(err, "wg_key_dir", "Could not create the key directory", "")
	}
	if err := os.WriteFile(path, []byte(keys.PrivateKey), 0o600); err != nil {
		return wireguard.Keypair{}, errs.Wrap(err, "wg_key_write", "Could not save the WireGuard key", "")
	}
	return keys, nil
}
