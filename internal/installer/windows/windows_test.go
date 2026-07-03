package windows

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rezonmain/loca-agent/internal/config"
	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/installer"
	"github.com/rezonmain/loca-agent/internal/model"
	"github.com/rezonmain/loca-agent/internal/platform"
	"github.com/rezonmain/loca-agent/internal/service"
	"github.com/rezonmain/loca-agent/internal/wireguard"
)

type fakeFetcher struct{}

func (fakeFetcher) Fetch(_ context.Context, _, dst string, _ func(model.Progress)) error {
	return os.WriteFile(dst, []byte("installer"), 0o644)
}

type fakeLlama struct{ path string }

func (f fakeLlama) Acquire(_ context.Context, _ config.Versions, _ string) (string, error) {
	return f.path, nil
}

type fakeModels struct{}

func (fakeModels) Download(_ context.Context, _ config.ModelSource, m config.Model, destDir string) ([]model.Result, error) {
	p := filepath.Join(destDir, m.Files[0].Name)
	_ = os.MkdirAll(destDir, 0o755)
	_ = os.WriteFile(p, []byte("gguf"), 0o644)
	return []model.Result{{File: m.Files[0], Path: p, Verified: true}}, nil
}

type fakeService struct{ installed, started bool }

func (f *fakeService) Install(context.Context, service.Spec) error { f.installed = true; return nil }
func (f *fakeService) Start(context.Context, string) error         { f.started = true; return nil }
func (f *fakeService) Stop(context.Context, string) error          { return nil }
func (f *fakeService) Remove(context.Context, string) error        { return nil }
func (f *fakeService) Status(context.Context, string) (service.State, error) {
	if f.started {
		return service.StateRunning, nil
	}
	return service.StateNotInstalled, nil
}

type fakeTunnel struct {
	active bool
	ups    int
}

func (f *fakeTunnel) Up(context.Context, string, string) error   { f.active = true; f.ups++; return nil }
func (f *fakeTunnel) Down(context.Context, string, string) error { f.active = false; return nil }
func (f *fakeTunnel) Active(context.Context, string) bool        { return f.active }

type fakePeers struct{}

func (fakePeers) Load() ([]wireguard.ServerPeer, error) { return nil, nil }

type okRunner struct{}

func (okRunner) Run(context.Context, string, ...string) (string, error) { return "", nil }

func testDeps(t *testing.T) (Deps, *fakeService, *fakeTunnel) {
	t.Helper()
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	tmp := t.TempDir()
	cfg.Installer.Paths.InstallDir = filepath.Join(tmp, "install")
	cfg.Installer.Paths.ModelDir = filepath.Join(tmp, "models")
	cfg.Installer.Paths.ConfigDir = filepath.Join(tmp, "config")
	cfg.Installer.Paths.LogDir = filepath.Join(tmp, "logs")

	svc := &fakeService{}
	tun := &fakeTunnel{}
	d := Deps{
		Cfg:      cfg,
		Runner:   okRunner{},
		Elevated: func() bool { return true },
		Detect: func(context.Context) (platform.SystemInfo, error) {
			return platform.SystemInfo{OS: "windows"}, nil
		},
		Fetcher:  fakeFetcher{},
		Llama:    fakeLlama{path: filepath.Join(cfg.Installer.Paths.InstallDir, "llama.cpp", "llama-server.exe")},
		Models:   fakeModels{},
		Service:  svc,
		Tunnel:   tun,
		Peers:    fakePeers{},
		Endpoint: "vpn.example.com:51820",
	}
	return d, svc, tun
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
		"preflight", "install-wireguard", "acquire-llama", "download-model",
		"wireguard-server-config", "wireguard-tunnel-up", "install-llama-service",
		"configure-firewall", "report",
	}
	if got := stepNames(Plan(d)); !reflect.DeepEqual(got, want) {
		t.Errorf("plan order = %v\nwant %v", got, want)
	}
}

func TestPreflightRequiresAdmin(t *testing.T) {
	d, _, _ := testDeps(t)
	d.Elevated = func() bool { return false }

	err := preflightStep(d).Run(context.Background(), installer.NewState(installer.WindowsServer, nil))
	if err == nil {
		t.Fatalf("expected preflight to fail without admin")
	}
	if ue, ok := errs.As(err); !ok || ue.Code != "not_elevated" {
		t.Errorf("expected not_elevated UserError, got %v", err)
	}
}

func TestFirewallAddRuleArgs(t *testing.T) {
	got := addRuleArgs("R", 51820)
	want := []string{
		"advfirewall", "firewall", "add", "rule",
		"name=R", "dir=in", "action=allow", "protocol=UDP", "localport=51820",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("addRuleArgs = %v\nwant %v", got, want)
	}
}

func TestFullPlanWithFakes(t *testing.T) {
	d, svc, tun := testDeps(t)
	st := installer.NewState(installer.WindowsServer, nil)

	if err := (&installer.Engine{}).Run(context.Background(), st, Plan(d)); err != nil {
		t.Fatalf("plan failed: %v", err)
	}

	token := st.String(installer.KeyJoinToken)
	if token == "" {
		t.Fatalf("expected a join token in state")
	}
	if _, err := wireguard.DecodeToken(token); err != nil {
		t.Errorf("minted token does not decode: %v", err)
	}
	if !svc.installed || !svc.started {
		t.Errorf("expected service installed and started: %+v", svc)
	}
	if !tun.active {
		t.Errorf("expected tunnel to be active")
	}

	// The server config was written to disk.
	if !fileExists(st.String(installer.KeyServerConfigPath)) {
		t.Errorf("expected server config to be written")
	}
}
