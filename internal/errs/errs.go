// Package errs defines the user-facing error type used throughout bootstrap-ai.
//
// A UserError carries a stable machine-readable Code, a human Message, an
// actionable Fix suggestion, and an optional wrapped Cause. The CLI's top
// layer renders Message and Fix to the user and logs Cause at debug level.
// Programmer bugs should still use panics or plain errors; UserError is for
// conditions the user can understand and act on.
package errs

import (
	"errors"
	"fmt"
	"strings"
)

// UserError is an actionable, user-facing error.
type UserError struct {
	// Code is a stable identifier, e.g. "wireguard_missing".
	Code string
	// Message states what went wrong, in plain language.
	Message string
	// Fix suggests how to resolve it. May be empty.
	Fix string
	// Cause is the underlying error, if any.
	Cause error
}

// Error implements the error interface.
func (e *UserError) Error() string {
	var b strings.Builder
	b.WriteString(e.Message)
	if e.Cause != nil {
		fmt.Fprintf(&b, ": %v", e.Cause)
	}
	return b.String()
}

// Unwrap exposes the wrapped cause for errors.Is/As.
func (e *UserError) Unwrap() error { return e.Cause }

// New builds a UserError without a cause.
func New(code, message, fix string) *UserError {
	return &UserError{Code: code, Message: message, Fix: fix}
}

// Wrap builds a UserError around an existing cause.
func Wrap(cause error, code, message, fix string) *UserError {
	return &UserError{Code: code, Message: message, Fix: fix, Cause: cause}
}

// As reports whether err is (or wraps) a *UserError and returns it.
func As(err error) (*UserError, bool) {
	var ue *UserError
	if errors.As(err, &ue) {
		return ue, true
	}
	return nil, false
}
