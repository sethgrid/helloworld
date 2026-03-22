// Package kverr provides structured key-value error context that accumulates
// through wrapping layers and can be extracted at the logging site.
//
// The central idea is to separate error message cardinality from context:
//
//	// high-cardinality — bad for alerting
//	"unable to delete user 180: mysql has gone away"
//
//	// structured — cardinality of 1 per error type, context in fields
//	msg="unable to delete user" error="mysql has gone away" user_id=180
//
// Errors created with [New] satisfy [errors.Is] and [errors.As] for any
// sentinel or typed error wrapped anywhere in the chain.
package kverr

import (
	"errors"
	"fmt"
	"log/slog"
)

// Error carries a causal error and a set of structured attributes that
// accumulate as the error bubbles up through the call stack.
type Error struct {
	// Err is the wrapped causal error.
	Err   error
	attrs []slog.Attr
}

// Error implements the error interface. It returns the causal error's message
// and intentionally does not include KV attributes — keeping error strings
// low-cardinality for monitoring and alerting.
func (e *Error) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("%v", e.attrs)
}

// Unwrap returns the wrapped error, enabling [errors.Is] and [errors.As] to
// traverse through a kverr boundary.
func (e *Error) Unwrap() error {
	return e.Err
}

// LogValue implements [slog.LogValuer]. A *Error passed directly as a slog
// argument emits its KV attributes as a structured group automatically:
//
//	slog.Error("something went wrong", "err", err)
func (e *Error) LogValue() slog.Value {
	return slog.GroupValue(e.attrs...)
}

// New wraps err with the provided args. args may be [slog.Attr] values
// (typed), alternating string/any pairs (convenient), or a mix of both:
//
//	kverr.New(err, "user_id", uid)
//	kverr.New(err, slog.String("user_id", uid))
//	kverr.New(err, slog.String("user_id", uid), "req_id", rid)
//
// If err already carries a *Error anywhere in its chain, those attributes are
// merged in first — no KV context is lost through any number of wrapping
// layers, including [fmt.Errorf] layers in between.
//
// In the string/any form, a non-string key or an unpaired trailing key is
// silently skipped.
func New(err error, args ...any) *Error {
	var attrs []slog.Attr

	// Merge attributes from any ancestor *Error in the chain.
	var ancestor *Error
	if errors.As(err, &ancestor) {
		attrs = append(attrs, ancestor.attrs...)
	}

	// Parse args: slog.Attr | string+any | skip.
	for i := 0; i < len(args); i++ {
		switch a := args[i].(type) {
		case slog.Attr:
			attrs = append(attrs, a)
		case string:
			if i+1 < len(args) {
				attrs = append(attrs, slog.Any(a, args[i+1]))
				i++
			}
		}
	}

	return &Error{Err: err, attrs: attrs}
}

// Args extracts KV pairs from any *Error in the chain and returns them as a
// flat []any slice (alternating key, value), suitable for slog's variadic API:
//
//	slog.Error("msg", kverr.Args(err)...)
//
// extra follows the same mixed format as [New] and is appended after the
// extracted pairs. Returns an empty (non-nil) slice if err carries no kverr
// context.
func Args(err error, extra ...any) []any {
	out := make([]any, 0)
	var e *Error
	if errors.As(err, &e) {
		for _, a := range e.attrs {
			out = append(out, a.Key, a.Value.Any())
		}
	}
	for i := 0; i < len(extra); i++ {
		switch a := extra[i].(type) {
		case slog.Attr:
			out = append(out, a.Key, a.Value.Any())
		case string:
			if i+1 < len(extra) {
				out = append(out, a, extra[i+1])
				i++
			}
		}
	}
	return out
}

// Map returns a shallow copy of all KV pairs from any *Error in the chain as
// map[string]any, suitable for zap, zerolog, or direct key access. Returns an
// empty (non-nil) map if err carries no kverr context.
func Map(err error) map[string]any {
	m := make(map[string]any)
	var e *Error
	if errors.As(err, &e) {
		for _, a := range e.attrs {
			m[a.Key] = a.Value.Any()
		}
	}
	return m
}
