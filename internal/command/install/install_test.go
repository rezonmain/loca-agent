package install

import (
	"testing"

	"github.com/rezonmain/loca-agent/internal/app"
	"github.com/rezonmain/loca-agent/internal/config"
	"github.com/rezonmain/loca-agent/internal/errs"
	"github.com/rezonmain/loca-agent/internal/installer"
	"github.com/rezonmain/loca-agent/internal/platform"
	"github.com/rezonmain/loca-agent/internal/ui"
)

func TestResolveRoleFromFlag(t *testing.T) {
	got, err := resolveRole(nil, "server", false)
	if err != nil || got != installer.WindowsServer {
		t.Errorf("role = %v err = %v, want WindowsServer", got, err)
	}
	got, err = resolveRole(nil, "client", false)
	if err != nil || got != installer.MacClient {
		t.Errorf("role = %v err = %v, want MacClient", got, err)
	}
}

func TestResolveRoleInvalidFlag(t *testing.T) {
	if _, err := resolveRole(nil, "banana", false); err == nil {
		t.Errorf("expected error for invalid role")
	}
}

func TestResolveRoleYesUsesDefault(t *testing.T) {
	// With --yes and no flag, the host-OS default is used without prompting (nil UI).
	if _, err := resolveRole(nil, "", true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func testApp(t *testing.T) *app.App {
	t.Helper()
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return &app.App{Cfg: cfg, UI: ui.New(), Platform: platform.New(nil)}
}

func TestBuildWindowsDepsRequiresEndpoint(t *testing.T) {
	_, err := buildWindowsDeps(testApp(t), "", "", true)
	if ue, ok := errs.As(err); !ok || ue.Code != "endpoint_required" {
		t.Errorf("expected endpoint_required, got %v", err)
	}
}

func TestBuildMacDepsRequiresToken(t *testing.T) {
	_, err := buildMacDeps(testApp(t), "", "", true, true)
	if ue, ok := errs.As(err); !ok || ue.Code != "token_required" {
		t.Errorf("expected token_required, got %v", err)
	}
}

func TestBuildWindowsDepsWired(t *testing.T) {
	d, err := buildWindowsDeps(testApp(t), "vpn.example.com:51820", "", true)
	if err != nil {
		t.Fatalf("buildWindowsDeps: %v", err)
	}
	if d.Endpoint != "vpn.example.com:51820" || d.Llama == nil || d.Models == nil ||
		d.Service == nil || d.Tunnel == nil || d.Peers == nil {
		t.Errorf("windows deps not fully wired: %+v", d)
	}
}
