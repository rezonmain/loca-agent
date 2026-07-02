package probe

import (
	"context"
	"errors"
	"testing"
)

// fakeLocator resolves only the executables in its set.
type fakeLocator struct {
	found map[string]string
}

func (f fakeLocator) LookPath(file string) (string, error) {
	if p, ok := f.found[file]; ok {
		return p, nil
	}
	return "", errors.New("not found")
}

// fakeRunner returns canned output per resolved path.
type fakeRunner struct {
	outputs map[string]string
}

func (f fakeRunner) Run(_ context.Context, name string, _ ...string) (string, error) {
	if out, ok := f.outputs[name]; ok {
		return out, nil
	}
	return "", errors.New("no output configured")
}

func TestCommandProbeInstalledWithVersion(t *testing.T) {
	loc := fakeLocator{found: map[string]string{"git": "/usr/bin/git"}}
	run := fakeRunner{outputs: map[string]string{"/usr/bin/git": "git version 2.39.3"}}

	p := &commandProbe{
		name: "Git", candidates: []string{"git"},
		versionArgs: []string{"--version"}, parse: firstVersion,
		loc: loc, run: run,
	}
	st := p.Probe(context.Background())
	if !st.Installed {
		t.Fatalf("expected installed")
	}
	if st.Version != "2.39.3" {
		t.Errorf("version = %q, want 2.39.3", st.Version)
	}
	if st.Path != "/usr/bin/git" {
		t.Errorf("path = %q", st.Path)
	}
}

func TestCommandProbeNotInstalled(t *testing.T) {
	p := &commandProbe{
		name: "OpenCode", candidates: []string{"opencode"},
		loc: fakeLocator{found: map[string]string{}}, run: fakeRunner{},
	}
	st := p.Probe(context.Background())
	if st.Installed {
		t.Errorf("expected not installed")
	}
	if st.Detail == "" {
		t.Errorf("expected a detail message explaining absence")
	}
}

func TestCommandProbeCandidateFallback(t *testing.T) {
	// First candidate missing, second present.
	loc := fakeLocator{found: map[string]string{"server": "/opt/llama/server"}}
	run := fakeRunner{outputs: map[string]string{"/opt/llama/server": "version: 4585 (abc)"}}
	p := &commandProbe{
		name: "llama.cpp server", candidates: []string{"llama-server", "server"},
		versionArgs: []string{"--version"}, parse: firstVersion,
		loc: loc, run: run,
	}
	st := p.Probe(context.Background())
	if !st.Installed || st.Path != "/opt/llama/server" {
		t.Fatalf("expected fallback candidate to resolve, got %+v", st)
	}
	if st.Version != "4585" {
		t.Errorf("version = %q, want 4585", st.Version)
	}
}

func TestFirstVersion(t *testing.T) {
	cases := map[string]string{
		"git version 2.39.3":            "2.39.3",
		"wireguard-tools v1.0.20210914": "1.0.20210914",
		"version: 4585 (abc123)":        "4585",
		"no numbers here":               "",
	}
	for in, want := range cases {
		if got := firstVersion(in); got != want {
			t.Errorf("firstVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProbeAllOrder(t *testing.T) {
	loc := fakeLocator{found: map[string]string{"git": "/usr/bin/git"}}
	run := fakeRunner{outputs: map[string]string{"/usr/bin/git": "git version 2.39.3"}}
	statuses := ProbeAll(context.Background(), Default(run, loc))
	if len(statuses) != 4 {
		t.Fatalf("expected 4 statuses, got %d", len(statuses))
	}
	if statuses[0].Name != "WireGuard" || statuses[1].Name != "Git" {
		t.Errorf("probe order changed: %q, %q", statuses[0].Name, statuses[1].Name)
	}
}
