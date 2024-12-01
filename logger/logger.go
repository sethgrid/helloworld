package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sethgrid/helloworld/metrics"
)

type contextKey string

var CtxLogger contextKey = "logger"

func New(logWriter ...io.Writer) *slog.Logger {
	var writer io.Writer = os.Stdout
	if len(logWriter) == 1 {
		writer = logWriter[0]
	}
	return slog.New(slog.NewJSONHandler(writer, nil))
}

func AddToCtx(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, CtxLogger, l)
}

// FromRequest takes the server logger instance as a backup logger if none are found.
// notice that the backup is variadic; that allows you to _exclude_ the final parameter.
// this is a hack to allow FromRequest(ctx) and FromRequest(ctx, backupLogger).
// This allows for an abundance of paranoia
func FromRequest(r *http.Request, backupLogger ...*slog.Logger) *slog.Logger {
	return FromCtx(r.Context(), backupLogger...)
}

// FromCtx takes the server logger instance as a backup logger if none are found.
// a backup logger of nil will create a standard json handler logging to stdout.
// notice that the backup is variadic; that allows you to _exclude_ the final parameter.
// this is a hack to allow FromCtx(ctx) and FromCtx(ctx, backupLogger)
func FromCtx(ctx context.Context, backupLogger ...*slog.Logger) *slog.Logger {
	l := ctx.Value(CtxLogger)
	switch v := l.(type) {
	case *slog.Logger:
		return v
	default:
		if len(backupLogger) != 1 {
			backupLogger = append(backupLogger, slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("unexpected_request_log_init_nil", true))
		}
		// if we change rid, also change rid init in NewRequestLogger
		return backupLogger[0].With("rid", uuid.NewString(), "unexpected_request_log_init_backup_logger", true)
	}
}

// NewRequestLogger will attempt to pull the logger from the request first as to not overwrite it,
// then it will pull from the context. In the event that no logger exists, a new logger is added to
// a context and an http.Request and returned. Pass in the server logger as a backup.
func NewRequestLogger(ctx context.Context, r *http.Request, log *slog.Logger) (*slog.Logger, context.Context, *http.Request) {
	// Check if the logger is present in the request context
	requestLogger := r.Context().Value(CtxLogger)
	if existingLog, ok := requestLogger.(*slog.Logger); ok {
		// If logger exists in the request context, use it
		return existingLog, ctx, r
	}

	// Check if the logger is present in the general context
	if existingLog, ok := ctx.Value(CtxLogger).(*slog.Logger); ok {
		// If logger exists in the general context, use it
		return existingLog, ctx, r
	}

	// No logger exists, create a new one
	newLogger := log.With("rid", uuid.NewString())
	ctx = context.WithValue(ctx, CtxLogger, newLogger)

	return newLogger, ctx, r.WithContext(ctx)
}

type Printable struct {
	slog.Logger
}

// Print matches the middleware in Chi
func (p Printable) Print(v ...any) {
	if len(v) > 0 {
		format := strings.Repeat("%v ", len(v))
		p.Error(fmt.Sprintf(format[:len(format)-1], v...))
	}
}

func Printer(l *slog.Logger) Printable {
	return Printable{*l}
}

// RequestLogger returns a logger handler using a custom LogFormatter.
func Middleware(logger *slog.Logger, shouldPrint bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			logger, _, r := NewRequestLogger(r.Context(), r, logger)
			middleware.WithLogEntry(r, nil)
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			start := time.Now()
			metrics.InFlightGauge.Inc()

			defer func() {
				metrics.InFlightGauge.Dec()

				path := r.URL.Path
				if ww.Status() == http.StatusNotFound {
					// prevent path scanners from polluting logs and messing up path / endpoint cardinality.
					// use a separate, dedicated key that we are not aggregating against. Keeps memory down.
					logger = logger.With("path_high_cardinality", path)
					path = "redacted for cardinality protection"
				}
				duration := time.Since(start)

				metrics.RequestCount.With(prometheus.Labels{"method": r.Method, "endpoint": path}).Inc()
				metrics.RequestDuration.With(prometheus.Labels{"method": r.Method, "endpoint": path}).Observe(duration.Seconds())
				logger.Info("route",
					"path", path,
					"verb", r.Method,
					"status", http.StatusText(ww.Status()),
					"code", ww.Status(),
					"bytes", ww.BytesWritten(),
					"duration_ms", duration.Milliseconds(),
				)
			}()

			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}
