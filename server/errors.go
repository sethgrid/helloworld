package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/sethgrid/helloworld/logger"
	"github.com/sethgrid/kverr"
)

type errorResp struct {
	Message string `json:"message"`
}

// errorHandler is the standard way to send JSON error responses. It logs first so entries are kept
// even if the client disconnects. When err is non-nil it logs at ERROR; when err is nil it logs at INFO.
// kverr attributes on err are merged into the log line.
func errorHandler(w http.ResponseWriter, r *http.Request, statusCode int, userMsg string, err error) {
	log := logger.FromRequest(r).With("status_code", statusCode).With(kverr.Args(err)...)
	if err != nil {
		log.Error(userMsg, "error", err.Error())
	} else {
		log.Info(userMsg)
	}

	w.Header().Set("content-type", "application/json")
	w.WriteHeader(statusCode)
	if encErr := json.NewEncoder(w).Encode(errorResp{Message: userMsg}); encErr != nil {
		log := logger.FromRequest(r).With("status_code", statusCode).With(kverr.Args(err)...)
		log.Error("failed to write error response", "encode_error", encErr.Error())
	}
}

// panicRecoverMiddleware catches panics, logs a stack trace via kverr, and responds with JSON through errorHandler.
func panicRecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				var err error
				switch v := rec.(type) {
				case error:
					err = v
				default:
					err = fmt.Errorf("%v", v)
				}
				wrapped := kverr.New(err, "stack", stack)
				errorHandler(w, r, http.StatusInternalServerError, "internal server error", wrapped)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
