package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/sethgrid/helloworld/logger"
	"github.com/sethgrid/kverr"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type errorResp struct {
	Message string `json:"message"`
}

// errorJSON is the standard JSON error response. Logs first so entries survive client disconnects.
<<<<<<< HEAD
// When err is non-nil it logs at ERROR; when nil it logs at INFO. Records span status when tracing is active.
=======
// When err is non-nil it logs at ERROR; when nil it logs at INFO.
>>>>>>> main
func errorJSON(w http.ResponseWriter, r *http.Request, statusCode int, userMsg string, err error) {
	log := logger.FromRequest(r).With("status_code", statusCode).With(kverr.Args(err)...)
	if err != nil {
		log.Error(userMsg, "error", err.Error())
	} else {
		log.Info(userMsg)
	}
<<<<<<< HEAD

	recordErrorSpan(r, statusCode, userMsg, err)
=======
>>>>>>> main

	w.Header().Set("content-type", "application/json")
	w.WriteHeader(statusCode)
	if encErr := json.NewEncoder(w).Encode(errorResp{Message: userMsg}); encErr != nil {
		log := logger.FromRequest(r).With("status_code", statusCode).With(kverr.Args(err)...)
		log.Error("failed to write error response", "encode_error", encErr.Error())
	}
}

<<<<<<< HEAD
func recordErrorSpan(r *http.Request, statusCode int, userMsg string, err error) {
	span := trace.SpanFromContext(r.Context())
	if !span.IsRecording() {
		return
	}
	if statusCode >= 400 {
		span.SetStatus(codes.Error, userMsg)
		if err != nil {
			span.RecordError(err)
		}
	}
}

// panicRecoverMiddleware recovers panics, records them on the active span, and responds with JSON via errorJSON.
=======
// panicRecoverMiddleware recovers panics and responds with JSON via errorJSON.
>>>>>>> main
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
<<<<<<< HEAD
				span := trace.SpanFromContext(r.Context())
				if span.IsRecording() {
					span.RecordError(wrapped)
					span.SetStatus(codes.Error, "panic")
				}
=======
>>>>>>> main
				errorJSON(w, r, http.StatusInternalServerError, "internal server error", wrapped)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
