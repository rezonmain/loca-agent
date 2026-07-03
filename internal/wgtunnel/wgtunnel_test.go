package wgtunnel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/rezonmain/loca-agent/internal/wireguard"
)

func TestWriteConfigPermsAndContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wg", "wg0.conf")
	if err := WriteConfig(path, "[Interface]\n"); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "[Interface]\n" {
		t.Fatalf("content = %q err = %v", data, err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("perm = %o, want 600 (config holds a private key)", perm)
		}
	}
}

func TestWindowsCommands(t *testing.T) {
	w := windowsPlatform{exe: "wireguard"}
	if got := w.up("wg0", `C:\wg\wg0.conf`); !reflect.DeepEqual(got, command{"wireguard", []string{"/installtunnelservice", `C:\wg\wg0.conf`}}) {
		t.Errorf("up = %+v", got)
	}
	if got := w.down("wg0", "x"); !reflect.DeepEqual(got, command{"wireguard", []string{"/uninstalltunnelservice", "wg0"}}) {
		t.Errorf("down = %+v", got)
	}
	if !w.isActive("STATE : 4 RUNNING", nil) {
		t.Errorf("expected RUNNING to be active")
	}
	if w.isActive("STATE : 1 STOPPED", nil) {
		t.Errorf("expected STOPPED to be inactive")
	}
	if w.isActive("RUNNING", errors.New("sc failed")) {
		t.Errorf("error should mean inactive")
	}
}

func TestUnixCommands(t *testing.T) {
	u := unixPlatform{wgQuick: "wg-quick", wg: "wg"}
	if got := u.up("wg0", "/etc/wg/wg0.conf"); !reflect.DeepEqual(got, command{"wg-quick", []string{"up", "/etc/wg/wg0.conf"}}) {
		t.Errorf("up = %+v", got)
	}
	if got := u.down("wg0", "/etc/wg/wg0.conf"); !reflect.DeepEqual(got, command{"wg-quick", []string{"down", "/etc/wg/wg0.conf"}}) {
		t.Errorf("down = %+v", got)
	}
	if got := u.status("wg0"); !reflect.DeepEqual(got, command{"wg", []string{"show", "wg0"}}) {
		t.Errorf("status = %+v", got)
	}
	if !u.isActive("", nil) || u.isActive("", errors.New("down")) {
		t.Errorf("unix isActive should track error presence")
	}
}

// fakeRunner records invocations and returns a programmed response.
type fakeRunner struct {
	calls [][]string
	out   string
	err   error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.out, f.err
}

func TestControllerUpInvokesPlatform(t *testing.T) {
	r := &fakeRunner{}
	c := &Controller{run: r, p: unixPlatform{wgQuick: "wg-quick", wg: "wg"}}
	if err := c.Up(context.Background(), "wg0", "/etc/wg/wg0.conf"); err != nil {
		t.Fatalf("Up: %v", err)
	}
	want := []string{"wg-quick", "up", "/etc/wg/wg0.conf"}
	if len(r.calls) != 1 || !reflect.DeepEqual(r.calls[0], want) {
		t.Errorf("calls = %v, want [%v]", r.calls, want)
	}
}

func TestControllerUpWrapsError(t *testing.T) {
	r := &fakeRunner{err: errors.New("boom")}
	c := &Controller{run: r, p: unixPlatform{wgQuick: "wg-quick"}}
	err := c.Up(context.Background(), "wg0", "/x.conf")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestControllerActive(t *testing.T) {
	up := &Controller{run: &fakeRunner{out: "interface: wg0", err: nil}, p: unixPlatform{wg: "wg"}}
	if !up.Active(context.Background(), "wg0") {
		t.Errorf("expected active when wg show succeeds")
	}
	down := &Controller{run: &fakeRunner{err: errors.New("no such device")}, p: unixPlatform{wg: "wg"}}
	if down.Active(context.Background(), "wg0") {
		t.Errorf("expected inactive when wg show fails")
	}
}

func TestPeerStoreRoundTrip(t *testing.T) {
	store := NewPeerStore(filepath.Join(t.TempDir(), "peers.json"))

	// Empty before anything is saved.
	peers, err := store.Load()
	if err != nil || len(peers) != 0 {
		t.Fatalf("empty load: %v %v", peers, err)
	}

	all, err := store.Add(wireguard.ServerPeer{Name: "mac", PublicKey: "PK1", Address: "10.50.0.2", PresharedKey: "psk"})
	if err != nil || len(all) != 1 {
		t.Fatalf("add 1: %v %v", all, err)
	}
	all, _ = store.Add(wireguard.ServerPeer{Name: "mac2", PublicKey: "PK2", Address: "10.50.0.3"})
	if len(all) != 2 {
		t.Fatalf("add 2: got %d", len(all))
	}

	// Re-adding the same public key replaces, not appends.
	all, _ = store.Add(wireguard.ServerPeer{Name: "mac-renamed", PublicKey: "PK1", Address: "10.50.0.2"})
	if len(all) != 2 {
		t.Errorf("re-add should replace; got %d peers", len(all))
	}

	reloaded, _ := store.Load()
	var found bool
	for _, p := range reloaded {
		if p.PublicKey == "PK1" && p.Name == "mac-renamed" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected replaced peer to persist, got %+v", reloaded)
	}
}

func TestPeerStorePerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits differ on Windows")
	}
	path := filepath.Join(t.TempDir(), "peers.json")
	store := NewPeerStore(path)
	if _, err := store.Add(wireguard.ServerPeer{PublicKey: "PK", PresharedKey: "secret"}); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("peers file perm = %o, want 600 (may contain PSKs)", perm)
	}
}
