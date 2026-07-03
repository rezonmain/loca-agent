package mac

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rezonmain/loca-agent/internal/config"
	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/installer"
	"github.com/rezonmain/loca-agent/internal/wireguard"
)

type fakeTunnel struct{ active bool }

func (f *fakeTunnel) Up(context.Context, string, string) error   { f.active = true; return nil }
func (f *fakeTunnel) Down(context.Context, string, string) error { f.active = false; return nil }
func (f *fakeTunnel) Active(context.Context, string) bool        { return f.active }

type recRunner struct{ calls [][]string }

func (r *recRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return "", nil
}

func (r *recRunner) sawPrefix(prefix ...string) bool {
	for _, c := range r.calls {
		if len(c) >= len(prefix) && reflect.DeepEqual(c[:len(prefix)], prefix) {
			return true
		}
	}
	return false
}

// validToken mints a real join token from a throwaway server identity.
func validToken(t *testing.T) string {
	t.Helper()
	kp, _ := wireguard.GenerateKeypair()
	srv := wireguard.ServerIdentity{
		Keys: kp, Address: "10.50.0.1", Subnet: "10.50.0.0/24",
		ListenPort: 51820, Endpoint: "vpn.example.com:51820", Keepalive: 25,
	}
	tok, err := srv.MintToken("10.50.0.2").Encode()
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return tok
}

func testDeps(t *testing.T) (Deps, *fakeTunnel, *recRunner) {
	t.Helper()
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	tmp := t.TempDir()
	cfg.Installer.Paths.ConfigDir = filepath.Join(tmp, "config")

	tun := &fakeTunnel{}
	run := &recRunner{}
	d := Deps{
		Cfg:                cfg,
		Runner:             run,
		Elevated:           func() bool { return false },
		Installed:          func(string) bool { return false }, // force all install steps to run
		Tunnel:             tun,
		Token:              validToken(t),
		WithPSK:            true,
		OpenCodeConfigPath: filepath.Join(tmp, "opencode", "opencode.json"),
	}
	return d, tun, run
}

func stepNames(steps []installer.Step) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name()
	}
	return names
}

func TestPlanStepOrder(t *testing.T) {
	d, _, _ := testDeps(t)
	want := []string{
		"preflight", "install-homebrew", "install-git", "install-opencode",
		"install-wireguard", "wireguard-enroll", "wireguard-tunnel-up",
		"opencode-config", "report",
	}
	if got := stepNames(Plan(d)); !reflect.DeepEqual(got, want) {
		t.Errorf("plan order = %v\nwant %v", got, want)
	}
}

func TestPreflightRejectsRoot(t *testing.T) {
	d, _, _ := testDeps(t)
	d.Elevated = func() bool { return true }
	err := preflightStep(d).Run(context.Background(), installer.NewState(installer.MacClient, nil))
	if ue, ok := errs.As(err); !ok || ue.Code != "root_not_allowed" {
		t.Errorf("expected root_not_allowed, got %v", err)
	}
}

func TestPreflightRequiresToken(t *testing.T) {
	d, _, _ := testDeps(t)
	d.Token = ""
	err := preflightStep(d).Run(context.Background(), installer.NewState(installer.MacClient, nil))
	if ue, ok := errs.As(err); !ok || ue.Code != "token_required" {
		t.Errorf("expected token_required, got %v", err)
	}
}

func TestOpenCodeInstalledViaNpm(t *testing.T) {
	d, _, run := testDeps(t)
	d.Cfg.Versions.OpenCode.InstallVia = "npm"
	d.Cfg.Versions.OpenCode.Package = "opencode-ai"

	if err := installOpenCodeStep(d).Run(context.Background(), installer.NewState(installer.MacClient, nil)); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !run.sawPrefix("npm", "install", "-g", "opencode-ai") {
		t.Errorf("expected npm install; calls: %v", run.calls)
	}
}

func TestFullPlanWithFakes(t *testing.T) {
	d, tun, _ := testDeps(t)
	st := installer.NewState(installer.MacClient, nil)

	if err := (&installer.Engine{}).Run(context.Background(), st, Plan(d)); err != nil {
		t.Fatalf("plan failed: %v", err)
	}

	// Enrollment reply is produced and decodes.
	reply := st.String(installer.KeyEnrollReply)
	if reply == "" {
		t.Fatalf("expected enrollment reply in state")
	}
	if _, err := wireguard.DecodeReply(reply); err != nil {
		t.Errorf("reply does not decode: %v", err)
	}

	// Client config written; tunnel up.
	conf := st.String(installer.KeyClientConfigPath)
	if _, err := os.Stat(conf); err != nil {
		t.Errorf("expected client config written: %v", err)
	}
	if !tun.active {
		t.Errorf("expected tunnel active")
	}

	// OpenCode config written and valid JSON.
	data, err := os.ReadFile(d.OpenCodeConfigPath)
	if err != nil {
		t.Fatalf("read opencode config: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Errorf("opencode config not valid JSON: %v", err)
	}
}
