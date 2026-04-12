package logger

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestMiddlewareCardinalityProtection verifies that requests matched to a registered
// chi route use the route pattern as the metric label, while unregistered routes
// (scanner traffic, typos, catch-alls) are bucketed as "other" with the real path
// logged separately to prevent cardinality explosion.
func TestMiddlewareCardinalityProtection(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantRedacted bool
	}{
		{
			name:         "registered route uses pattern, no redaction",
			path:         "/api/users",
			wantRedacted: false,
		},
		{
			name:         "scanner path not in router triggers redaction",
			path:         "/wp-admin/login.php",
			wantRedacted: true,
		},
		{
			name:         "dot-env probe triggers redaction",
			path:         "/.env",
			wantRedacted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))
			mw := Middleware(log, true)

			router := chi.NewRouter()
			router.Use(mw)
			router.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			output := buf.String()
			if tt.wantRedacted {
				if !strings.Contains(output, "path_high_cardinality") {
					t.Errorf("expected path_high_cardinality in log output, got: %s", output)
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
