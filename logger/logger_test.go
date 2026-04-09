package logger

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMiddlewareCardinalityProtection verifies that unmatched routes — whether they
// produce an explicit 404 or no WriteHeader call at all (chi returns status 0 for
// unmatched routes) — have their paths redacted in logs and metrics to prevent
// scanner traffic from causing a cardinality explosion.
func TestMiddlewareCardinalityProtection(t *testing.T) {
	tests := []struct {
		name         string
		handler      http.HandlerFunc
		path         string
		wantRedacted bool
	}{
		{
			name: "known path passes through unchanged",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			path:         "/api/users",
			wantRedacted: false,
		},
		{
			name: "explicit 404 triggers redaction",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			path:         "/wp-admin/login.php",
			wantRedacted: true,
		},
		{
			name: "status 0 (unmatched chi route, WriteHeader never called) triggers redaction",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// intentionally write nothing — chi returns 0 for unmatched routes
			},
			path:         "/.env",
			wantRedacted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))
			mw := Middleware(log, true)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			mw(http.HandlerFunc(tt.handler)).ServeHTTP(rr, req)

			output := buf.String()
			if tt.wantRedacted {
				if !strings.Contains(output, "path_high_cardinality") {
					t.Errorf("expected path_high_cardinality in log output, got: %s", output)
				}
				if !strings.Contains(output, "redacted for cardinality protection") {
					t.Errorf("expected redacted path in log output, got: %s", output)
				}
			} else {
				if strings.Contains(output, "path_high_cardinality") {
					t.Errorf("did not expect path_high_cardinality in log output, got: %s", output)
				}
			}
		})
	}
}

// TestPrintable verifies that the logger instance can be coerced over to a different
// interface. slog doesn't provide Print, but the chi logger middleware needs something
// that matches that interface. ergo, make sure we can coerce the logger over
func TestPrintable(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	printable := Printer((*slog.Logger)(logger))

	// Test cases
	tests := []struct {
		name     string
		input    []any
		expected string
	}{
		{
			name:     "Single argument",
			input:    []any{"hello"},
			expected: "hello",
		},
		{
			name:     "Multiple arguments",
			input:    []any{"hello", 123, true},
			expected: "hello 123 true",
		},
		{
			name:     "Empty arguments",
			input:    []any{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear the buffer before each test
			buf.Reset()

			// Call the Print method
			printable.Print(tt.input...)

			// Compare the actual output with the expected result
			if got := buf.String(); !strings.Contains(got, tt.expected) {
				t.Errorf("Print() = %v, want %v", got, tt.expected)
			}
		})
	}
}
