// Package service manages the OS service that supervises the llama.cpp
// inference server. The current backend targets Windows via NSSM (the
// Non-Sucking Service Manager), which wraps the non-service-aware
// llama-server.exe as a proper Windows service.
//
// All behavior goes through the sys.Runner, so the manager is pure command
// construction and unit-testable on any platform. The Manager interface keeps
// callers decoupled so launchd/systemd backends can be added later.
package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rezonmain/loca-agent/internal/sys"
)

// State is a service's lifecycle state.
type State string

const (
	StateRunning      State = "running"
	StateStopped      State = "stopped"
	StateNotInstalled State = "not-installed"
	StateUnknown      State = "unknown"
)

// Spec describes a service to install.
type Spec struct {
	Name        string            // service identifier
	DisplayName string            // human-friendly name
	Executable  string            // path to the program to run
	Args        []string          // program arguments
	WorkingDir  string            // working directory
	Env         map[string]string // extra environment variables
	Stdout      string            // optional stdout log file
	Stderr      string            // optional stderr log file
	AutoStart   bool              // start automatically at boot
}

// Manager installs and controls a service.
type Manager interface {
	Install(ctx context.Context, spec Spec) error
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	Status(ctx context.Context, name string) (State, error)
}

// NSSMManager drives the nssm.exe CLI.
type NSSMManager struct {
	nssm string
	run  sys.Runner
}

// New returns the default Manager for the host (NSSM-backed).
func New(nssmPath string, run sys.Runner) Manager { return NewNSSM(nssmPath, run) }

// NewNSSM constructs an NSSM-backed manager. An empty path defaults to "nssm"
// (resolved on PATH).
func NewNSSM(nssmPath string, run sys.Runner) *NSSMManager {
	if nssmPath == "" {
		nssmPath = "nssm"
	}
	return &NSSMManager{nssm: nssmPath, run: run}
}

// Install registers the service and applies its configuration.
func (m *NSSMManager) Install(ctx context.Context, spec Spec) error {
	install := append([]string{"install", spec.Name, spec.Executable}, spec.Args...)
	if _, err := m.exec(ctx, install...); err != nil {
		return fmt.Errorf("nssm install %s: %w", spec.Name, err)
	}
	for _, cmd := range m.settings(spec) {
		if _, err := m.exec(ctx, cmd...); err != nil {
			return fmt.Errorf("nssm configure %s: %w", spec.Name, err)
		}
	}
	return nil
}

// settings builds the ordered `nssm set` commands for a spec. Env keys are
// sorted so the output is deterministic.
func (m *NSSMManager) settings(spec Spec) [][]string {
	var cmds [][]string
	set := func(key string, vals ...string) {
		cmds = append(cmds, append([]string{"set", spec.Name, key}, vals...))
	}
	if spec.DisplayName != "" {
		set("DisplayName", spec.DisplayName)
	}
	if spec.WorkingDir != "" {
		set("AppDirectory", spec.WorkingDir)
	}
	if spec.Stdout != "" {
		set("AppStdout", spec.Stdout)
	}
	if spec.Stderr != "" {
		set("AppStderr", spec.Stderr)
	}
	if len(spec.Env) > 0 {
		keys := make([]string, 0, len(spec.Env))
		for k := range spec.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		pairs := make([]string, 0, len(keys))
		for _, k := range keys {
			pairs = append(pairs, k+"="+spec.Env[k])
		}
		set("AppEnvironmentExtra", pairs...)
	}
	start := "SERVICE_DEMAND_START"
	if spec.AutoStart {
		start = "SERVICE_AUTO_START"
	}
	set("Start", start)
	return cmds
}

// Start starts the service.
func (m *NSSMManager) Start(ctx context.Context, name string) error {
	if _, err := m.exec(ctx, "start", name); err != nil {
		return fmt.Errorf("nssm start %s: %w", name, err)
	}
	return nil
}

// Stop stops the service.
func (m *NSSMManager) Stop(ctx context.Context, name string) error {
	if _, err := m.exec(ctx, "stop", name); err != nil {
		return fmt.Errorf("nssm stop %s: %w", name, err)
	}
	return nil
}

// Remove stops (best-effort) and unregisters the service.
func (m *NSSMManager) Remove(ctx context.Context, name string) error {
	_, _ = m.exec(ctx, "stop", name)
	if _, err := m.exec(ctx, "remove", name, "confirm"); err != nil {
		return fmt.Errorf("nssm remove %s: %w", name, err)
	}
	return nil
}

// Status reports the service state. A service that does not exist yields
// StateNotInstalled (not an error) so callers can use it in Verify checks.
func (m *NSSMManager) Status(ctx context.Context, name string) (State, error) {
	out, err := m.exec(ctx, "status", name)
	if err != nil {
		if notInstalled(out, err) {
			return StateNotInstalled, nil
		}
		return StateUnknown, fmt.Errorf("nssm status %s: %w", name, err)
	}
	s := strings.ToUpper(strings.TrimSpace(out))
	switch {
	case s == "":
		return StateNotInstalled, nil
	case strings.Contains(s, "SERVICE_RUNNING"):
		return StateRunning, nil
	case strings.Contains(s, "SERVICE_STOPPED"), strings.Contains(s, "SERVICE_PAUSED"):
		return StateStopped, nil
	default:
		return StateUnknown, nil
	}
}

func (m *NSSMManager) exec(ctx context.Context, args ...string) (string, error) {
	return m.run.Run(ctx, m.nssm, args...)
}

// notInstalled recognizes nssm's "service does not exist" responses across
// stdout and the error (which carries stderr).
func notInstalled(out string, err error) bool {
	hay := strings.ToLower(out)
	if err != nil {
		hay += " " + strings.ToLower(err.Error())
	}
	return strings.Contains(hay, "can't open service") ||
		strings.Contains(hay, "does not exist") ||
		strings.Contains(hay, "service not found")
}
