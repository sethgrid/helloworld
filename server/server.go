package server

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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

	mu           sync.Mutex
	started      bool
	port         int
	internalPort int
	srvErr       error
	inDebug      bool

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

	for i := 0; i < 10; i++ {
		rootLogger.Info("attempting db; if hanging for local dev, restart docker",
			"repro", fmt.Sprintf("mysql --host=%s --port=%s -u%s -p --connect-timeout=10", conf.DBHost, conf.DBPort, conf.DBUser),
		)
		if err = db.Ping(); err != nil {
			time.Sleep(time.Duration(i) * time.Second)
			continue
		}
		break
	}
	if err != nil {
		return nil, fmt.Errorf("unable to ping db after several attempts: %w", err)
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

func (s *Server) Close() error {
	// TODO - capture error group

	return nil
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
	router.Use(logger.Middleware(s.parentLogger, s.inDebug))
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(60 * time.Second))

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
				"Accept-Language", "Hx-Current-Url", "Hx-Request", "Hx-Target",
				"Hx-Trigger", "Referer", "User-Agent",
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
	router := s.newRouter().With(s.loggerMiddleware)

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
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       30 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
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
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		Handler:           router,
	}

	// blocking
	if err := publicHTTP.Serve(listener); err != nil {
		// capturing server error for easier debugging during testing
		s.setLastError(err)
		return err
	}

	return nil
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

func (s *Server) loggerMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// create an instance of the logger bound to this single request
		// if we change rid, also change rid's backup init in GetLoggerFromRequest
		_, _, r = logger.NewRequestLogger(ctx, r, s.parentLogger)

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
