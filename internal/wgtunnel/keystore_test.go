package wgtunnel

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrCreateKeyCreatesThenLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "server.key")

	created, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.PrivateKey == "" || created.PublicKey == "" {
		t.Fatalf("empty keypair: %+v", created)
	}

	// Second call loads the same key (stable identity).
	loaded, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.PrivateKey != created.PrivateKey || loaded.PublicKey != created.PublicKey {
		t.Errorf("key not stable across calls")
	}

	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("key perm = %o, want 600", perm)
		}
	}
}

func TestLoadOrCreateKeyRejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.key")
	if err := os.WriteFile(path, []byte("not-a-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateKey(path); err == nil {
		t.Errorf("expected error for invalid stored key")
	}
}
