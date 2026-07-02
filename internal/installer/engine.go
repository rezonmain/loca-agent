package installer

import (
	"context"
	"fmt"
	"log/slog"
)

// Engine executes a plan of Steps. The zero value is usable; set Log for debug
// tracing and Progress to observe per-step status (e.g. for a UI).
type Engine struct {
	Log      *slog.Logger
	Progress func(name string, status StepStatus)
}

// Run executes steps in order. For each step it verifies first (skipping when
// already satisfied), then runs. If a step's Run fails — or its Verify errors,
// or the context is cancelled — the engine rolls back the successfully
// completed steps in reverse order and returns the original cause.
//
// Because Verify skips satisfied steps, re-running a partially completed install
// resumes from where it left off.
func (e *Engine) Run(ctx context.Context, st *State, steps []Step) error {
	var completed []Step

	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return e.rollback(ctx, st, completed, err)
		}

		satisfied, err := step.Verify(ctx, st)
		if err != nil {
			e.report(step.Name(), StatusFailed)
			return e.rollback(ctx, st, completed, fmt.Errorf("verify %q: %w", step.Name(), err))
		}
		if satisfied {
			e.report(step.Name(), StatusSkipped)
			continue
		}

		e.report(step.Name(), StatusRunning)
		if err := step.Run(ctx, st); err != nil {
			e.report(step.Name(), StatusFailed)
			// Return the cause unwrapped so an actionable UserError surfaces.
			return e.rollback(ctx, st, completed, err)
		}
		e.report(step.Name(), StatusDone)
		completed = append(completed, step)
	}

	return nil
}

// rollback undoes completed steps newest-first. Rollback errors are logged but
// do not mask the original cause, which is returned to the caller.
func (e *Engine) rollback(ctx context.Context, st *State, completed []Step, cause error) error {
	for i := len(completed) - 1; i >= 0; i-- {
		s := completed[i]
		if err := s.Rollback(ctx, st); err != nil {
			if e.Log != nil {
				e.Log.Warn("rollback failed", "step", s.Name(), "err", err)
			}
			continue
		}
		e.report(s.Name(), StatusRolledBack)
	}
	return cause
}

func (e *Engine) report(name string, status StepStatus) {
	if e.Progress != nil {
		e.Progress(name, status)
	}
	if e.Log != nil {
		e.Log.Debug("installer step", "step", name, "status", status.String())
	}
}
