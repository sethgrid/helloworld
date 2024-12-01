package kverr

import (
	"errors"
	"fmt"
	"sync"
)

// Error stores a causal error and a lockable map of kv pairs
type Error struct {
	Err error

	mu sync.RWMutex
	kv map[string]any
}

// Error() implements the error interface. This will NOT dump the kv contents by design.
// For structured logging, you want to decrease the cardinality of fields:
//
//	dynamicStr = "unable to delete user 180: mysql has gone away"
//	structured = `{"message": "unable to delete user", "error": "mysql has gone away", "user_id": 180}`
//
// With the dynamicStr, the cardinality of that error in logs is the number of affected users times the number of error conditions.
// With structured, there is a cardility of 1 for the types of errors and their causes no matter how many users are affected.
// This makes monitoring much less resource intensive.
func (e *Error) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("%v", e.kv)
}

// New function to create a new Error with a variadic list of key-value pairs that can later be extracted for logging and monitoring.
// If an underlying kverr Error is found, the keyValues from that error are lifted into the new Error ensuring no kv pair loss when
// wrapping multiple layers of errors. It is up to the implementer to ensure that all keys have a matching pair.
func New(err error, keyValues ...any) *Error {
	m := make(map[string]any)

	e := &Error{}
	if errors.As(err, &e) {
		e.mu.Lock()
		for k, v := range e.kv {
			m[k] = v
		}
		e.mu.Unlock()
	}

	for i := 0; i < len(keyValues)-1; i += 2 {
		key, ok := keyValues[i].(string)
		if !ok {
			continue // Skip if the key is not a string
		}
		m[key] = keyValues[i+1]
	}
	return &Error{
		Err: err,
		kv:  m,
	}
}

// YoinkArgs pulls the KV pairs from an underlying Error and returns a slice, with optional additional kv arg pairs
func YoinkArgs(err error, kvArgs ...any) []any {
	e := &Error{}
	if errors.As(err, &e) {
		return append(e.mapToSlice(), kvArgs...)
	}
	return kvArgs
}

func Map(e *Error) map[string]any {
	m := make(map[string]any)
	e.mu.RLock()
	defer e.mu.RUnlock()
	for k, v := range e.kv {
		m[k] = v
	}
	return m
}

func (e *Error) mapToSlice() []any {
	var slice []any
	e.mu.RLock()
	defer e.mu.RUnlock()
	for k, v := range e.kv {
		slice = append(slice, k, v)
	}
	return slice
}
