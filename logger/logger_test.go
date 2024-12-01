package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

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
