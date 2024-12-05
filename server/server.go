package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"

	"github.com/sethgrid/helloworld/events"
	"github.com/sethgrid/helloworld/logger"
	"github.com/sethgrid/helloworld/taskqueue"
)

// secureCookies is a yucky global because it cleans up the cookie helper functions a lot
// and it is _always_ the same value in the same environment, so only ever set once. This
// would never change even in a test except as a test to verify secure vs nonsecure cookie
// generation which I'm not concerned about at this time
var secureCookies bool

type contextKey string

var ctxUser contextKey = "user"

type eventWriter interface {
	Write(userID int64, message string) error
	Close() error
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
	internalHTTPServer *http.Server
	publicHTTPServer   *http.Server

	parentLogger *slog.Logger
}

func New(conf Config) (*Server, error) {
	rootLogger := logger.New().With("version", conf.Version)

	protocol := "http://"
	customTLS := ""
	if conf.ShouldSecure {
		protocol = "https://"
		// if we are securing cookies, we must be in a production environment
		// and we want to ensure we are connecting to mysql with a ca certificate
		caCert, err := os.ReadFile(conf.DBCACertPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read CA cert: %v", err)
		}

		rootCertPool := x509.NewCertPool()
		if ok := rootCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to append CA cert to pool")
		}

		err = mysql.RegisterTLSConfig("custom", &tls.Config{
			RootCAs: rootCertPool,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to register TLS config: %v", err)
		}
		rootLogger.Info("totally setting custom tls")
		customTLS = "&tls=custom"
	} else {
		rootLogger.Error("secure cookies and tls to the db are turned off")
	}
	// TODO: after db bootstrap, include db name before timeout and other options
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?timeout=5s&parseTime=true%s",
		conf.DBUser,
		conf.DBPass,
		conf.DBHost,
		strings.TrimPrefix(conf.DBPort, ":"),

		customTLS,
	)

	// helpful if you are having trouble reaching the db; warning: prints password
	log.Println("dsn: ", dsn)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to db: %w", err)
	}

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(0)
	db.SetConnMaxIdleTime(time.Minute * 3)

	if conf.RequireDBUp {
		if err := pingDB(db, 10, conf, rootLogger); err != nil {
			return nil, fmt.Errorf("unable to ping db: %w", err)
		}
	} else {
		go pingDB(db, 10, conf, rootLogger)
	}

	if conf.ShouldSecure {
		secureCookies = true
	}

	addr := fmt.Sprintf("%s:%d", conf.Hostname, conf.Port)
	return &Server{config: conf,
		port:         conf.Port,
		parentLogger: rootLogger,
		addr:         addr,
		protocol:     protocol,
		inDebug:      conf.EnableDebug,
		taskq:        taskqueue.NewMySQLTaskQueue(db, rootLogger, 3, 30*time.Second),
		eventStore:   events.NewUserEvent(db, 2, rootLogger),
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

	// Wait for all goroutines to complete and return any error
	return g.Wait()
}

func (s *Server) newRouter() *chi.Mux {
	// if you need to support CORS, pass in orgins here
	var origins []string
	if s.config.ShouldSecure {
		origins = []string{"https://helloworld.com", "http://localhost:*"}
	} else {
		s.parentLogger.Warn("opening CORS to allow all")
		origins = []string{"*"}
	}

	router := chi.NewRouter()
	router.Use(customCORSMiddleware(origins))

	router.Use(middleware.RealIP)
	router.Use(timeoutMiddleware(s.config.RequestTimeout))
	router.Use(logger.Middleware(s.parentLogger, s.inDebug))
	router.Use(middleware.Recoverer)

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
	privateRouter.Get("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("200 OK"))
	})

	// all application routes should be defined below
	router := s.newRouter()

	// if routes require authentication, use a new With or add it above as a separate middleware
	// router.Get("/", s.uiIndex)
	router.Get("/", s.helloworldHandler)

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

		for {
			l, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.InternalPort))
			if err != nil {
				// capturing server error for easier debugging during testing
				s.setLastError(err)
				time.Sleep(30 * time.Second)
				continue
			}

			s.mu.Lock()
			s.internalHTTPServer = &internalHTTP
			s.internalPort = l.Addr().(*net.TCPAddr).Port
			s.parentLogger.Info("starting http internal listener", "port", s.internalPort)
			s.mu.Unlock()

			if err := internalHTTP.Serve(l); err != nil {
				// capturing server error for easier debugging during testing
				s.setLastError(err)
				time.Sleep(30 * time.Second)
				continue
			}
		}
	}()

	runner := taskqueue.NewRunner(s.taskq, 1, s.parentLogger, 15*time.Second)
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
