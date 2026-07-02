// Package logging provides structured, leveled logging built on log/slog.
//
// It exposes a console handler with human-readable, optionally colorized output
// and can additionally tee JSON logs to a file. Verbosity maps to slog levels:
// Info (default), Debug (-v), and Verbose (-vv, a custom sub-debug level).
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LevelVerbose is a custom level below Debug for very fine-grained tracing.
const LevelVerbose = slog.Level(-8)

// Options configures a logger.
type Options struct {
	Level slog.Level // minimum level to emit on the console
	File  string     // optional path for JSON file logs ("" disables)
	Color bool       // colorize console output
	Out   io.Writer  // console destination (defaults to os.Stderr)
}

// New constructs a *slog.Logger from Options.
func New(opts Options) (*slog.Logger, error) {
	if opts.Out == nil {
		opts.Out = os.Stderr
	}
	handlers := []slog.Handler{newConsoleHandler(opts.Out, opts.Level, opts.Color)}

	if opts.File != "" {
		if err := os.MkdirAll(filepath.Dir(opts.File), 0o755); err != nil {
			return nil, fmt.Errorf("create log directory: %w", err)
		}
		f, err := os.OpenFile(opts.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		handlers = append(handlers, slog.NewJSONHandler(f, &slog.HandlerOptions{Level: LevelVerbose}))
	}

	return slog.New(&multiHandler{handlers: handlers}), nil
}

// multiHandler fans a record out to several handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: next}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: next}
}

// consoleHandler renders records as "LEVEL message key=value ...".
type consoleHandler struct {
	w     io.Writer
	level slog.Level
	color bool
	attrs []slog.Attr
}

func newConsoleHandler(w io.Writer, level slog.Level, color bool) *consoleHandler {
	return &consoleHandler{w: w, level: level, color: color}
}

func (h *consoleHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	label, colorCode := levelLabel(r.Level)

	var b strings.Builder
	if h.color {
		b.WriteString(colorCode)
		b.WriteString(label)
		b.WriteString("\033[0m")
	} else {
		b.WriteString(label)
	}
	b.WriteByte(' ')
	b.WriteString(r.Message)

	for _, a := range h.attrs {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value.Any())
	}
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value.Any())
		return true
	})
	b.WriteByte('\n')

	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

// WithGroup is a no-op for console output; groups are preserved in file logs.
func (h *consoleHandler) WithGroup(_ string) slog.Handler { return h }

func levelLabel(l slog.Level) (label, colorCode string) {
	switch {
	case l >= slog.LevelError:
		return "ERROR", "\033[31m" // red
	case l >= slog.LevelWarn:
		return "WARN ", "\033[33m" // yellow
	case l >= slog.LevelInfo:
		return "INFO ", "\033[36m" // cyan
	case l >= slog.LevelDebug:
		return "DEBUG", "\033[35m" // magenta
	default:
		return "TRACE", "\033[90m" // grey
	}
}
