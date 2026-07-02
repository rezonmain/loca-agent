// Package sys centralizes execution of external commands.
//
// All shelling-out in bootstrap-ai goes through a Runner so that calls are
// consistently logged and can be faked in tests. Higher-level packages depend
// on the Runner interface, never on os/exec directly.
package sys

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// Runner executes an external command and returns its combined stdout.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, err error)
}

// ExecRunner runs commands via os/exec, logging each invocation at debug level.
type ExecRunner struct {
	Log *slog.Logger
}

// NewExecRunner constructs an ExecRunner. A nil logger disables logging.
func NewExecRunner(log *slog.Logger) *ExecRunner {
	return &ExecRunner{Log: log}
}

// Run executes name with args and returns stdout. On failure the returned error
// includes stderr for actionable diagnostics.
func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	if r.Log != nil {
		r.Log.Debug("exec", "cmd", name, "args", strings.Join(args, " "))
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
