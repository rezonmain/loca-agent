package sys

import "context"

// SudoRunner wraps a Runner so every command is executed via sudo. It is used
// selectively — e.g. for macOS wg-quick, which needs root — while leaving other
// commands (like Homebrew, which refuses root) on the unwrapped runner.
type SudoRunner struct {
	Inner Runner
}

// WithSudo returns a Runner that prefixes every command with sudo.
func WithSudo(r Runner) Runner {
	return SudoRunner{Inner: r}
}

// Run executes "sudo <name> <args...>".
func (s SudoRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	return s.Inner.Run(ctx, "sudo", append([]string{name}, args...)...)
}
