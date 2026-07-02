// Package probe detects whether external dependencies (WireGuard, Git,
// OpenCode, llama.cpp) are installed and, when possible, their versions.
//
// Detection is expressed through the Prober interface and a data-driven
// commandProbe so adding a new dependency is a one-line registry entry rather
// than new logic. Filesystem/exec access goes through injected interfaces so
// probers are unit-testable with fakes.
package probe

import (
	"context"
	"regexp"

	"github.com/rezonmain/loca-agent/internal/sys"
)

// Status is the result of probing one dependency.
type Status struct {
	Name      string // display name, e.g. "WireGuard"
	Installed bool
	Version   string // best-effort; empty if not determinable
	Path      string // resolved binary path when installed
	Detail    string // extra context (e.g. why it was not found)
}

// Prober detects a single dependency.
type Prober interface {
	Name() string
	Probe(ctx context.Context) Status
}

// Locator resolves an executable name to a path, like exec.LookPath.
type Locator interface {
	LookPath(file string) (string, error)
}

// commandProbe detects a dependency by locating one of several candidate
// binaries and parsing the output of a version command.
type commandProbe struct {
	name        string
	candidates  []string
	versionArgs []string
	parse       func(string) string
	loc         Locator
	run         sys.Runner
}

func (p *commandProbe) Name() string { return p.name }

func (p *commandProbe) Probe(ctx context.Context) Status {
	for _, cand := range p.candidates {
		path, err := p.loc.LookPath(cand)
		if err != nil {
			continue
		}
		st := Status{Name: p.name, Installed: true, Path: path}
		if out, err := p.run.Run(ctx, path, p.versionArgs...); err == nil && p.parse != nil {
			st.Version = p.parse(out)
		}
		return st
	}
	return Status{Name: p.name, Installed: false, Detail: "not found on PATH"}
}

// Default returns the standard set of dependency probers in a stable order.
func Default(run sys.Runner, loc Locator) []Prober {
	return []Prober{
		&commandProbe{name: "WireGuard", candidates: []string{"wg"}, versionArgs: []string{"--version"}, parse: firstVersion, loc: loc, run: run},
		&commandProbe{name: "Git", candidates: []string{"git"}, versionArgs: []string{"--version"}, parse: firstVersion, loc: loc, run: run},
		&commandProbe{name: "OpenCode", candidates: []string{"opencode"}, versionArgs: []string{"--version"}, parse: firstVersion, loc: loc, run: run},
		&commandProbe{name: "llama.cpp server", candidates: []string{"llama-server", "server"}, versionArgs: []string{"--version"}, parse: firstVersion, loc: loc, run: run},
	}
}

// ProbeAll runs every prober and returns their statuses in order.
func ProbeAll(ctx context.Context, probers []Prober) []Status {
	out := make([]Status, 0, len(probers))
	for _, p := range probers {
		out = append(out, p.Probe(ctx))
	}
	return out
}

var (
	// dotted matches semver-like versions (git, wireguard-tools, opencode).
	dottedRe = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)
	// build matches a bare integer, used by llama.cpp (e.g. "version: 4585").
	buildRe = regexp.MustCompile(`\d+`)
)

// firstVersion extracts a version token from version-command output, preferring
// a dotted version and falling back to a bare build number.
func firstVersion(s string) string {
	if v := dottedRe.FindString(s); v != "" {
		return v
	}
	return buildRe.FindString(s)
}
