package wgtunnel

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/wireguard"
)

// PeerStore persists the server's registered clients so the server config can
// be re-rendered when a peer is added. The file may contain preshared keys, so
// it is written owner-only.
type PeerStore struct {
	path string
}

// NewPeerStore returns a store backed by the given file path.
func NewPeerStore(path string) *PeerStore { return &PeerStore{path: path} }

// Load reads the registered peers, returning an empty slice if none exist yet.
func (s *PeerStore) Load() ([]wireguard.ServerPeer, error) {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errs.Wrap(err, "peerstore_read", "Could not read the WireGuard peer store", "")
	}
	var peers []wireguard.ServerPeer
	if err := json.Unmarshal(b, &peers); err != nil {
		return nil, errs.Wrap(err, "peerstore_parse", "The WireGuard peer store is corrupt", "Delete the peers file and re-register clients.")
	}
	return peers, nil
}

// Save writes the peer list with owner-only permissions.
func (s *PeerStore) Save(peers []wireguard.ServerPeer) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return errs.Wrap(err, "peerstore_dir", "Could not create the peer store directory", "")
	}
	b, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, b, 0o600); err != nil {
		return errs.Wrap(err, "peerstore_write", "Could not write the WireGuard peer store", "")
	}
	return nil
}

// Add registers a peer and returns the full updated list. A peer with the same
// public key replaces the existing entry (re-enrollment is idempotent).
func (s *PeerStore) Add(peer wireguard.ServerPeer) ([]wireguard.ServerPeer, error) {
	peers, err := s.Load()
	if err != nil {
		return nil, err
	}
	replaced := false
	for i := range peers {
		if peers[i].PublicKey == peer.PublicKey {
			peers[i] = peer
			replaced = true
			break
		}
	}
	if !replaced {
		peers = append(peers, peer)
	}
	if err := s.Save(peers); err != nil {
		return nil, err
	}
	return peers, nil
}
