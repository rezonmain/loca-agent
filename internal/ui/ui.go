// Package ui renders human-facing terminal output and prompts.
//
// Output is colorized when writing to a TTY and degrades to plain text
// otherwise (pipes, CI, redirected files). The UI layer is intentionally thin
// and free of business logic so commands can be tested by injecting buffers.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// ANSI color codes; empty when color is disabled.
const (
	reset  = "\033[0m"
	green  = "\033[32m"
	red    = "\033[31m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	dim    = "\033[90m"
	bold   = "\033[1m"
)

// UI writes messages and reads interactive input.
type UI struct {
	out   io.Writer
	err   io.Writer
	in    *bufio.Reader
	color bool
}

// New returns a UI bound to the process stdio with color auto-detected.
func New() *UI {
	return &UI{
		out:   os.Stdout,
		err:   os.Stderr,
		in:    bufio.NewReader(os.Stdin),
		color: ColorEnabled(),
	}
}

// NewWith returns a UI bound to explicit streams (useful for tests).
func NewWith(out, errOut io.Writer, in io.Reader, color bool) *UI {
	return &UI{out: out, err: errOut, in: bufio.NewReader(in), color: color}
}

// ColorEnabled reports whether stdout is an interactive terminal and color is
// not disabled via NO_COLOR.
func ColorEnabled() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (u *UI) paint(color, s string) string {
	if !u.color {
		return s
	}
	return color + s + reset
}

// Printf writes an uncolored line to stdout.
func (u *UI) Printf(format string, a ...any) {
	fmt.Fprintf(u.out, format+"\n", a...)
}

// Successf writes a green check line.
func (u *UI) Successf(format string, a ...any) {
	fmt.Fprintln(u.out, u.paint(green, "✓ ")+fmt.Sprintf(format, a...))
}

// Failf writes a red cross line.
func (u *UI) Failf(format string, a ...any) {
	fmt.Fprintln(u.out, u.paint(red, "✗ ")+fmt.Sprintf(format, a...))
}

// Warnf writes a yellow warning line.
func (u *UI) Warnf(format string, a ...any) {
	fmt.Fprintln(u.out, u.paint(yellow, "! ")+fmt.Sprintf(format, a...))
}

// Notef writes an informational line.
func (u *UI) Notef(format string, a ...any) {
	fmt.Fprintln(u.out, u.paint(cyan, "› ")+fmt.Sprintf(format, a...))
}

// Headingf writes a bold section heading.
func (u *UI) Headingf(format string, a ...any) {
	fmt.Fprintln(u.out, u.paint(bold, fmt.Sprintf(format, a...)))
}

// Dimf writes a de-emphasized line.
func (u *UI) Dimf(format string, a ...any) {
	fmt.Fprintln(u.out, u.paint(dim, fmt.Sprintf(format, a...)))
}

// Prompt asks a free-text question and returns the trimmed answer. The default
// is returned on empty input or a non-interactive stream.
func (u *UI) Prompt(question, def string) string {
	if def != "" {
		fmt.Fprintf(u.out, "%s [%s]: ", question, def)
	} else {
		fmt.Fprintf(u.out, "%s: ", question)
	}
	line, err := u.in.ReadString('\n')
	if err != nil {
		return def
	}
	if line = strings.TrimSpace(line); line != "" {
		return line
	}
	return def
}

// Confirm asks a yes/no question and returns the answer. def is returned on
// empty input or a non-interactive stream.
func (u *UI) Confirm(prompt string, def bool) bool {
	suffix := " [y/N] "
	if def {
		suffix = " [Y/n] "
	}
	fmt.Fprint(u.out, prompt+suffix)
	line, err := u.in.ReadString('\n')
	if err != nil {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return def
	}
}
