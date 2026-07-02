// Package installer runs an ordered list of idempotent, reversible steps that
// install and configure a machine for its role (macOS client or Windows AI
// server).
//
// The engine is OS-agnostic: concrete steps live in sibling packages and are
// injected as a plan ([]Step). Each Step verifies before it runs — so
// re-running install is safe and naturally resumable — and can roll back its
// effect if a later step fails. Steps hold their own dependencies (config,
// runners, clients); State carries only role information and values handed from
// one step to the next.
package installer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// MachineKind is the role a machine is being installed as.
type MachineKind int

const (
	// MacClient runs the coding agent (OpenCode) and the WireGuard client.
	MacClient MachineKind = iota
	// WindowsServer runs the llama.cpp inference server and WireGuard peer.
	WindowsServer
)

func (k MachineKind) String() string {
	switch k {
	case MacClient:
		return "macos-client"
	case WindowsServer:
		return "windows-server"
	default:
		return "unknown"
	}
}

// ParseMachineKind resolves a user-supplied role string.
func ParseMachineKind(s string) (MachineKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "mac", "macos", "client", "mac-client", "macos-client":
		return MacClient, nil
	case "windows", "win", "server", "windows-server":
		return WindowsServer, nil
	default:
		return 0, fmt.Errorf("unknown machine type %q (use \"client\" or \"server\")", s)
	}
}

// State is shared across the steps of a single install run. Data carries values
// produced by one step for consumption by later steps (e.g. a generated join
// token or a downloaded model path).
type State struct {
	Kind MachineKind
	Log  *slog.Logger
	data map[string]any
}

// NewState creates an empty State for a role.
func NewState(kind MachineKind, log *slog.Logger) *State {
	return &State{Kind: kind, Log: log, data: make(map[string]any)}
}

// Set stores a value for later steps.
func (s *State) Set(key string, v any) { s.data[key] = v }

// Get retrieves a previously stored value.
func (s *State) Get(key string) (any, bool) {
	v, ok := s.data[key]
	return v, ok
}

// String retrieves a string value, or "" if absent or not a string.
func (s *State) String(key string) string {
	if v, ok := s.data[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

// Step is one unit of installation work. Implementations should make Run
// idempotent where feasible and must tolerate Rollback being called only after
// a successful Run.
type Step interface {
	// Name is a short, human-readable label.
	Name() string
	// Verify reports whether the desired state already holds. When satisfied is
	// true the engine skips Run. A non-nil err aborts the install.
	Verify(ctx context.Context, st *State) (satisfied bool, err error)
	// Run performs the step's action.
	Run(ctx context.Context, st *State) error
	// Rollback best-effort undoes a successful Run. May be a no-op.
	Rollback(ctx context.Context, st *State) error
}

// FuncStep adapts plain functions into a Step, reducing boilerplate for simple
// steps. Nil VerifyFunc means "always run"; nil RunFunc/RollbackFunc are no-ops.
type FuncStep struct {
	Label        string
	VerifyFunc   func(ctx context.Context, st *State) (bool, error)
	RunFunc      func(ctx context.Context, st *State) error
	RollbackFunc func(ctx context.Context, st *State) error
}

func (f FuncStep) Name() string { return f.Label }

func (f FuncStep) Verify(ctx context.Context, st *State) (bool, error) {
	if f.VerifyFunc != nil {
		return f.VerifyFunc(ctx, st)
	}
	return false, nil
}

func (f FuncStep) Run(ctx context.Context, st *State) error {
	if f.RunFunc != nil {
		return f.RunFunc(ctx, st)
	}
	return nil
}

func (f FuncStep) Rollback(ctx context.Context, st *State) error {
	if f.RollbackFunc != nil {
		return f.RollbackFunc(ctx, st)
	}
	return nil
}

// StepStatus describes what happened to a step during a run.
type StepStatus int

const (
	StatusSkipped StepStatus = iota
	StatusRunning
	StatusDone
	StatusFailed
	StatusRolledBack
)

func (s StepStatus) String() string {
	switch s {
	case StatusSkipped:
		return "skipped"
	case StatusRunning:
		return "running"
	case StatusDone:
		return "done"
	case StatusFailed:
		return "failed"
	case StatusRolledBack:
		return "rolled-back"
	default:
		return "unknown"
	}
}
