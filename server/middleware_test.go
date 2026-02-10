package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCORSMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		method         string
		origin         string
		expectedStatus int
		expectedOrigin string
	}{
		{
			name:           "OPTIONS preflight with allowed origin",
			allowedOrigins: []string{"http://localhost:3000", "https://example.com"},
			method:         "OPTIONS",
			origin:         "http://localhost:3000",
			expectedStatus: http.StatusNoContent,
			expectedOrigin: "http://localhost:3000,https://example.com", // CORS middleware joins all origins
		},
		{
			name:           "GET request with allowed origin",
			allowedOrigins: []string{"http://localhost:3000"},
			method:         "GET",
			origin:         "http://localhost:3000",
			expectedStatus: http.StatusOK,
			expectedOrigin: "http://localhost:3000",
		},
		{
			name:           "POST request with allowed origin",
			allowedOrigins: []string{"https://example.com"},
			method:         "POST",
			origin:         "https://example.com",
			expectedStatus: http.StatusOK,
			expectedOrigin: "https://example.com",
		},
		{
			name:           "Multiple allowed origins",
			allowedOrigins: []string{"http://localhost:*", "http://127.0.0.1:*"},
			method:         "GET",
			origin:         "http://localhost:3000",
			expectedStatus: http.StatusOK,
			expectedOrigin: "http://localhost:*,http://127.0.0.1:*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := customCORSMiddleware(tt.allowedOrigins)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("OK"))
				}),
			)

			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedOrigin != "" {
				assert.Equal(t, tt.expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"))
			}
			assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
			assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
		})
	}
}

func TestTimeoutMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		timeout        time.Duration
		handlerDelay   time.Duration
		expectedStatus int
		shouldTimeout  bool
	}{
		{
			name:           "Handler completes within timeout",
			timeout:        100 * time.Millisecond,
			handlerDelay:   50 * time.Millisecond,
			expectedStatus: http.StatusOK,
			shouldTimeout:  false,
		},
		{
			name:           "Handler exceeds timeout",
			timeout:        50 * time.Millisecond,
			handlerDelay:   100 * time.Millisecond,
			expectedStatus: http.StatusOK, // Handler still writes OK, but context is canceled
			shouldTimeout:  true,
		},
		{
			name:           "Very short timeout",
			timeout:        10 * time.Millisecond,
			handlerDelay:   50 * time.Millisecond,
			expectedStatus: http.StatusOK,
			shouldTimeout:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := timeoutMiddleware(tt.timeout)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Simulate work that checks context
					select {
					case <-r.Context().Done():
						// Context was canceled (timeout)
						return
					case <-time.After(tt.handlerDelay):
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("OK"))
					}
				}),
			)

			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()

			start := time.Now()
			handler.ServeHTTP(w, req)
			duration := time.Since(start)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.shouldTimeout {
				// Handler should return quickly due to timeout
				assert.Less(t, duration, tt.handlerDelay, "Handler should have timed out before delay completes")
			} else {
				// Handler should complete normally
				assert.GreaterOrEqual(t, duration, tt.handlerDelay-time.Millisecond*10, "Handler should have run for approximately the delay duration")
			}
		})
	}
}

func TestTimeoutMiddlewareContextPropagation(t *testing.T) {
	handler := timeoutMiddleware(50 * time.Millisecond)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check that context has timeout set
			deadline, ok := r.Context().Deadline()
			assert.True(t, ok, "Context should have a deadline")
			assert.WithinDuration(t, time.Now().Add(50*time.Millisecond), deadline, 10*time.Millisecond)

			// Verify context is not already done
			select {
			case <-r.Context().Done():
				t.Error("Context should not be done at handler start")
			default:
				// Good, context is still active
			}

			w.WriteHeader(http.StatusOK)
		}),
	)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
