package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riandyrn/otelchi"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"golang.org/x/sync/errgroup"

	"github.com/sethgrid/helloworld/internal/events"
	"github.com/sethgrid/helloworld/internal/taskqueue"
	"github.com/sethgrid/helloworld/internal/tracing"
	"github.com/sethgrid/helloworld/logger"
)

type contextKey string

var ctxUser contextKey = "user"

// maskDSN redacts sensitive information from a database connection string
func maskDSN(dsn string) string {
	// Simple masking: replace password with "***"
	// Format: user:password@tcp(host:port)/...
	if idx := strings.Index(dsn, "@"); idx > 0 {
		if colonIdx := strings.LastIndex(dsn[:idx], ":"); colonIdx > 0 {
			return dsn[:colonIdx+1] + "***" + dsn[idx:]
		}
	}
	return "***"
}

type eventWriter interface {
	Write(userID int64, message string) error
	Close() error
	IsAvailable() bool
}

type Server struct {
	config     Config
	taskq      taskqueue.Tasker
	eventStore eventWriter
	addr       string
	protocol   string

	mu                 sync.Mutex
	started            bool
	port               int
	internalPort       int
	srvErr             error
	inDebug            bool
	secureCookies      bool // Whether to use secure cookies (moved from global variable)
	internalHTTPServer *http.Server
	publicHTTPServer   *http.Server

	parentLogger *slog.Logger
	taskRunner   *taskqueue.Runner // Task queue runner for graceful shutdown

	tracerShutdown func(context.Context) error
	tracingEnabled bool
}

func New(conf Config) (*Server, error) {
	rootLogger := logger.New().With("version", conf.Version)

	var tracerShutdown func(context.Context) error
	tracingEnabled := conf.OtelExporterOTLPEndpoint != ""
	if tracingEnabled {
		shutdown, err := tracing.Install(context.Background(), tracing.Config{
			ServiceName:    conf.OtelServiceName,
			ServiceVersion: conf.Version,
			OTLPEndpoint:   conf.OtelExporterOTLPEndpoint,
			Insecure:       conf.OtelExporterOTLPInsecure,
			SampleRatio:    conf.OtelSampleRatio,
		})
		if err != nil {
			return nil, fmt.Errorf("tracing: %w", err)
		}
		tracerShutdown = shutdown
	}

	protocol := "http://"
	customTLS := ""
	if conf.ShouldSecure {
		protocol = "https://"
		// if we are securing cookies, we must be in a production environment
		// and we want to ensure we are connecting to mysql with a ca certificate
		if conf.DBCACertPath == "" {
			return nil, fmt.Errorf("db_ca_cert_path must be set when should_secure is true")
		}
		caCert, err := os.ReadFile(conf.DBCACertPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read CA cert: %w", err)
		}

		rootCertPool := x509.NewCertPool()
		if ok := rootCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to append CA cert to pool")
		}

		err = mysql.RegisterTLSConfig("custom", &tls.Config{
			RootCAs: rootCertPool,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to register TLS config: %w", err)
		}
		rootLogger.Info("totally setting custom tls")
		customTLS = "&tls=custom"
	} else {
		rootLogger.Error("secure cookies and tls to the db are turned off")
	}
	// Include database name in DSN - database should be bootstrapped before server starts
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?timeout=5s&parseTime=true%s",
		conf.DBUser,
		conf.DBPass,
		conf.DBHost,
		strings.TrimPrefix(conf.DBPort, ":"),
		conf.DBName,
		customTLS,
	)

	// DSN logging removed for security - never log database connection strings with passwords
	// If debugging is needed, use EnableDebug flag and log a masked version
	if conf.EnableDebug {
		maskedDSN := maskDSN(dsn)
		rootLogger.Debug("database connection", "dsn", maskedDSN)
	}

	var db *sql.DB
	var err error
	if tracingEnabled {
		driverName, regErr := otelsql.Register("mysql",
			otelsql.WithTracerProvider(otel.GetTracerProvider()),
			otelsql.WithAttributes(semconv.DBSystemMySQL),
		)
		if regErr != nil {
			return nil, fmt.Errorf("otelsql register: %w", regErr)
		}
		db, err = sql.Open(driverName, dsn)
	} else {
		db, err = sql.Open("mysql", dsn)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to connect to db: %w", err)
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5) // Allow idle connections for better performance
	db.SetConnMaxIdleTime(time.Minute * 3)

	if conf.RequireDBUp {
		if err := pingDB(db, 10, conf, rootLogger); err != nil {
			return nil, fmt.Errorf("unable to ping db: %w", err)
		}
	} else {
		go pingDB(db, 10, conf, rootLogger)
	}

	addr := fmt.Sprintf("%s:%d", conf.Hostname, conf.Port)
	taskq := taskqueue.NewMySQLTaskQueue(db, rootLogger, 3, 30*time.Second)
	eventStore := events.NewUserEvent(db, 2, rootLogger)

	return &Server{config: conf,
		port:           conf.Port,
		parentLogger:   rootLogger,
		addr:           addr,
		protocol:       protocol,
		inDebug:        conf.EnableDebug,
		secureCookies:  conf.ShouldSecure,
		taskq:          taskq,
		eventStore:     eventStore,
		tracerShutdown: tracerShutdown,
		tracingEnabled: tracingEnabled,
	}, nil
}

func pingDB(db *sql.DB, retryCount int, conf Config, logger *slog.Logger) error {
	var err error
	for i := 0; i < retryCount; i++ {
		logger.Info("attempting db; if hanging for local dev, restart docker",
			"repro", fmt.Sprintf("mysql --host=%s --port=%s -u%s -p --connect-timeout=10", conf.DBHost, conf.DBPort, conf.DBUser),
		)
		if err = db.Ping(); err != nil {
			time.Sleep(time.Duration(i) * time.Second)
			continue
		}
		break
	}
	if err != nil {
		return fmt.Errorf("unable to ping db after several attempts: %w", err)
	}
	return nil
}

func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var g errgroup.Group

	timeout := s.config.ShutdownTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if s.publicHTTPServer != nil {
		// Launch a goroutine to close the public HTTP server
		g.Go(func() error {
			err := s.publicHTTPServer.Shutdown(ctx)
			if err != nil {
				s.parentLogger.Error("unable to close public http server", "error", err.Error())
			}
			return err
		})
	}

	if s.internalHTTPServer != nil {
		// Launch a goroutine to close the internal HTTP server
		g.Go(func() error {
			err := s.internalHTTPServer.Shutdown(ctx)
			if err != nil {
				s.parentLogger.Error("unable to close internal http server", "error", err.Error())
			}
			return err
		})
	}

	if s.eventStore != nil {
		// Launch a goroutine to close the event store
		g.Go(func() error {
			err := s.eventStore.Close()
			if err != nil {
				s.parentLogger.Error("unable to close event store", "error", err.Error())
			}
			return err
		})
	}

	if s.taskRunner != nil {
		// Launch a goroutine to close the task queue runner
		g.Go(func() error {
			err := s.taskRunner.Close()
			if err != nil {
				s.parentLogger.Error("unable to close task queue runner", "error", err.Error())
			}
			return err
		})
	}

	// Wait for all goroutines to complete
	// errgroup.Wait() returns the first non-nil error, but we want to collect all errors
	// for better observability. However, errgroup doesn't support collecting all errors,
	// so we log each error as it occurs and return the first error encountered.
	err := g.Wait()
	if s.tracerShutdown != nil {
		ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if shutdownErr := s.tracerShutdown(ctx2); shutdownErr != nil {
			s.parentLogger.Error("tracing shutdown", "error", shutdownErr.Error())
		}
		cancel()
	}
	if err != nil {
		return fmt.Errorf("shutdown completed with errors: %w", err)
	}
	return nil
}

func (s *Server) newRouter() *chi.Mux {
	// CORS configuration - use explicit origins even in dev for better security
	var origins []string
	if s.config.ShouldSecure {
		origins = []string{"https://helloworld.com", "http://localhost:*"}
	} else {
		// Even in dev, use explicit origins instead of wildcard for better security
		// Wildcard "*" allows any origin, which is a security risk
		s.parentLogger.Warn("CORS configured for development - using explicit localhost origins")
		origins = []string{"http://localhost:*", "http://127.0.0.1:*"}
	}

	router := chi.NewRouter()
	router.Use(customCORSMiddleware(origins))

	router.Use(middleware.RealIP)
	// Public app only: Chi route patterns (e.g. /items/{id}) become span names via otelchi.WithChiRoutes.
	// Internal /metrics listener has no tracing middleware.
	if s.tracingEnabled {
		router.Use(otelchi.Middleware(s.config.OtelServiceName,
			otelchi.WithChiRoutes(router),
			otelchi.WithTracerProvider(otel.GetTracerProvider()),
		))
	}
	router.Use(timeoutMiddleware(s.config.RequestTimeout))
	router.Use(logger.Middleware(s.parentLogger, s.inDebug))
	router.Use(panicRecoverMiddleware)

	return router
}

func customCORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", strings.Join(allowedOrigins, ","))
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", strings.Join([]string{
				"Accept", "Authorization", "Content-Type", "X-CSRF-Token",
				"Accept-Language", "Referer", "User-Agent",
			}, ","))
			w.Header().Set("Access-Control-Expose-Headers", "Link")
			w.Header().Set("Access-Control-Max-Age", "300")

			// Handle preflight requests (OPTIONS)
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) InDebug() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inDebug
}

func (s *Server) Serve() error {
	// privateRouter is for internal only endpoints
	// this mux server will be spun up at the end of Serve() along side the standard router
	privateRouter := chi.NewRouter()
	privateRouter.Handle("/metrics", promhttp.Handler())
	// Health check uses eventStore to check DB connectivity
	// Both taskq and eventStore use the same DB, so either works
	privateRouter.Get("/healthcheck", handleHealthcheck(s.eventStore))
	privateRouter.Get("/status", handleStatus(s.eventStore, s.config.Version))

	// all application routes should be defined below
	router := s.newRouter()

	// if routes require authentication, use a new With or add it above as a separate middleware
	// router.Get("/", s.uiIndex)
	// Handlers receive dependencies at route definition time, following modern Go patterns.
	// Logger is injected via middleware and accessed through request context.
	// Rate limiting is applied only to the hello world endpoint
	router.With(rateLimitMiddleware(s.config.RateLimitRPS)).Get("/", handleHelloworld(s.eventStore))

	// normally we use a defer for unlocking
	// we are not doing that here because http.Serve below is a blocking call
	// if we don't explicitly release the lock, then the lock will stay in place the entire
	// life of the server
	s.mu.Lock()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		s.mu.Unlock()
		// capturing server error for easier debugging during testing
		s.setLastError(err)
		return fmt.Errorf("unable to start server listener: %w", err)
	}

	s.started = true
	s.port = listener.Addr().(*net.TCPAddr).Port
	s.parentLogger.Info("starting http listener", "port", s.port)

	s.mu.Unlock()

	go func() {
		internalHTTP := http.Server{
			ReadTimeout:       s.config.RequestTimeout,
			WriteTimeout:      s.config.RequestTimeout,
			IdleTimeout:       s.config.RequestTimeout,
			ReadHeaderTimeout: s.config.RequestTimeout,
			Handler:           privateRouter,
		}

		maxRetries := 5
		retryCount := 0
		baseDelay := 1 * time.Second

		for retryCount < maxRetries {
			l, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.InternalPort))
			if err != nil {
				// Exponential backoff: 1s, 2s, 4s, 8s, 16s
				delay := baseDelay * time.Duration(1<<uint(retryCount))
				s.parentLogger.Error("unable to start internal listener, retrying",
					"error", err.Error(),
					"retry", retryCount+1,
					"max_retries", maxRetries,
					"delay_seconds", delay.Seconds(),
				)
				s.setLastError(err)
				time.Sleep(delay)
				retryCount++
				continue
			}

			s.mu.Lock()
			s.internalHTTPServer = &internalHTTP
			s.internalPort = l.Addr().(*net.TCPAddr).Port
			s.parentLogger.Info("starting http internal listener", "port", s.internalPort)
			s.mu.Unlock()

			if err := internalHTTP.Serve(l); err != nil {
				// Server stopped, retry with exponential backoff
				delay := baseDelay * time.Duration(1<<uint(retryCount))
				s.parentLogger.Error("internal server stopped, retrying",
					"error", err.Error(),
					"retry", retryCount+1,
					"max_retries", maxRetries,
					"delay_seconds", delay.Seconds(),
				)
				s.setLastError(err)
				time.Sleep(delay)
				retryCount++
				continue
			}
			// If Serve returns without error, reset retry count
			retryCount = 0
		}

		// If we've exhausted retries, log final error
		s.parentLogger.Error("internal server failed after max retries, giving up",
			"max_retries", maxRetries,
		)
	}()

	runner := taskqueue.NewRunner(s.taskq, 1, s.parentLogger, 15*time.Second)
	s.taskRunner = runner
	go runner.Start()

	publicHTTP := http.Server{
		ReadTimeout:       s.config.RequestTimeout,
		WriteTimeout:      s.config.RequestTimeout,
		IdleTimeout:       s.config.RequestTimeout,
		ReadHeaderTimeout: s.config.RequestTimeout,
		Handler:           router,
	}

	s.mu.Lock()
	s.publicHTTPServer = &publicHTTP
	s.mu.Unlock()

	// Graceful shutdown on signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		s.parentLogger.Info("shutdown signal received, shutting down gracefully...")
		if err := s.Close(); err != nil {
			s.parentLogger.Error("error during shutdown", "error", err.Error())
		}
	}()

	// blocking
	if err := publicHTTP.Serve(listener); err != nil {
		// capturing server error for easier debugging during testing
		s.setLastError(err)
		return err
	}

	return nil
}

// WithLogWriter is a test helper for server configuration to override logger's writer.
// Typically used with the lockbuffer package for testing, allowing concurrent reads and writes,
// preventing races in the test suite.
func WithLogWriter(w io.Writer) func(*Server) {
	return func(s *Server) {
		logger := slog.New(slog.NewJSONHandler(w, nil))
		s.parentLogger = logger
		s.taskq = taskqueue.NewInMemoryTaskQueue(1, 15*time.Second, logger)
	}
}

// WithLogger is a test helper for server configuration to override logger
func WithLogger(logger *slog.Logger) func(*Server) {
	return func(s *Server) {
		s.parentLogger = logger
		s.taskq = taskqueue.NewInMemoryTaskQueue(1, 15*time.Second, logger)
	}
}

// WithConfig is a test helper for overwriting server configuration
func WithConfig(config Config) func(*Server) {
	return func(s *Server) {
		s.config = config
	}
}

func (s *Server) setLastError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.srvErr = err
}

// Port polls the port for a time until a non-zero port is set.
// If no port is set after a time, the port will return 0.
// This is an artificat of spinning up dynamic servers that can bind to any port,
// which we leverage for testing
func (s *Server) Port() int {
	for i := 0; i < 10; i++ {
		s.mu.Lock()
		if s.port != 0 {
			s.mu.Unlock()
			return s.port
		}
		s.mu.Unlock()
		time.Sleep(100 * time.Duration(i) * time.Millisecond)
	}

	return 0
}

// InternalPort polls the port for a time until a non-zero port is set.
// If no port is set after a time, the port will return 0.
// This is an artificat of spinning up dynamic servers that can bind to any port,
// which we leverage for testing
func (s *Server) InternalPort() int {
	for i := 0; i < 10; i++ {
		s.mu.Lock()
		if s.internalPort != 0 {
			s.mu.Unlock()
			return s.internalPort
		}
		s.mu.Unlock()
		time.Sleep(100 * time.Duration(i) * time.Millisecond)
	}

	return 0
}

func (s *Server) IsStarted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

func (s *Server) LastError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.srvErr
}

func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			// Pass the new context to the next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
