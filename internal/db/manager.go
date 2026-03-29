package db

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/sethgrid/helloworld/metrics"
)

// Manager manages database connections with support for read/write separation
// and centralized metrics collection.
type Manager struct {
	// Writer is the primary database connection for writes
	Writer *sql.DB
	// Reader is the database connection for reads (can be same as Writer for single-instance setups)
	Reader *sql.DB
	logger *slog.Logger
	closer chan struct{}
}

// NewManager creates a new database connection manager with separate writer and reader connections.
//
// sqlDriver is the registered sql driver name. Pass "" to use the default "mysql" driver.
// When OpenTelemetry tracing is enabled, pass the name returned by otelsql.Register() instead as
// that call wraps the "mysql" driver and returns a dynamically generated name that captures
// DB spans. Without this indirection, otelsql cannot intercept sql.Open.
//
//	 sqlDriver options:
//		""                          no tracing; NewManager defaults to "mysql"
//		otelsql.Register("mysql")   tracing enabled; dynamically registered driver wrapping "mysql"
func NewManager(sqlDriver, writerDSN, readerDSN string, logger *slog.Logger) (*Manager, error) {
	if sqlDriver == "" {
		sqlDriver = "mysql"
	}
	writer, err := sql.Open(sqlDriver, writerDSN)
	if err != nil {
		return nil, err
	}

	var reader *sql.DB
	if readerDSN != "" && readerDSN != writerDSN {
		reader, err = sql.Open(sqlDriver, readerDSN)
		if err != nil {
			writer.Close()
			return nil, err
		}
	} else {
		reader = writer
	}

	m := &Manager{
		Writer: writer,
		Reader: reader,
		logger: logger,
		closer: make(chan struct{}),
	}

	// Start centralized metrics collection
	go m.collectMetrics()

	return m, nil
}

// ConfigurePool sets connection pool settings for both reader and writer.
func (m *Manager) ConfigurePool(maxOpen, maxIdle int, maxLifetime, maxIdleTime time.Duration) {
	m.Writer.SetConnMaxLifetime(maxLifetime)
	m.Writer.SetMaxOpenConns(maxOpen)
	m.Writer.SetMaxIdleConns(maxIdle)
	m.Writer.SetConnMaxIdleTime(maxIdleTime)

	// Only configure reader separately if it's a different connection
	if m.Reader != m.Writer {
		m.Reader.SetConnMaxLifetime(maxLifetime)
		m.Reader.SetMaxOpenConns(maxOpen)
		m.Reader.SetMaxIdleConns(maxIdle)
		m.Reader.SetConnMaxIdleTime(maxIdleTime)
	}
}

// Ping checks connectivity for both reader and writer connections.
func (m *Manager) Ping(ctx context.Context) error {
	if err := m.Writer.PingContext(ctx); err != nil {
		return err
	}
	if m.Reader != m.Writer {
		if err := m.Reader.PingContext(ctx); err != nil {
			return err
		}
	}
	return nil
}

// collectMetrics periodically updates Prometheus metrics from database connection pool stats.
// This is the single source of truth for database metrics.
func (m *Manager) collectMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.closer:
			return
		case <-ticker.C:
			// Collect metrics for writer connection
			writerStats := m.Writer.Stats()
			metrics.DBConnectionsOpen.WithLabelValues("writer").Set(float64(writerStats.OpenConnections))
			metrics.DBConnectionsIdle.WithLabelValues("writer").Set(float64(writerStats.Idle))
			metrics.DBConnectionsInUse.WithLabelValues("writer").Set(float64(writerStats.InUse))
			metrics.DBConnectionsWaitCount.WithLabelValues("writer").Add(float64(writerStats.WaitCount))
			metrics.DBConnectionsWaitDuration.WithLabelValues("writer").Add(writerStats.WaitDuration.Seconds())
			metrics.DBConnectionsMaxIdleClosed.WithLabelValues("writer").Add(float64(writerStats.MaxIdleClosed))
			metrics.DBConnectionsMaxLifetimeClosed.WithLabelValues("writer").Add(float64(writerStats.MaxLifetimeClosed))

			// Collect metrics for reader connection if it's different
			if m.Reader != m.Writer {
				readerStats := m.Reader.Stats()
				metrics.DBConnectionsOpen.WithLabelValues("reader").Set(float64(readerStats.OpenConnections))
				metrics.DBConnectionsIdle.WithLabelValues("reader").Set(float64(readerStats.Idle))
				metrics.DBConnectionsInUse.WithLabelValues("reader").Set(float64(readerStats.InUse))
				metrics.DBConnectionsWaitCount.WithLabelValues("reader").Add(float64(readerStats.WaitCount))
				metrics.DBConnectionsWaitDuration.WithLabelValues("reader").Add(readerStats.WaitDuration.Seconds())
				metrics.DBConnectionsMaxIdleClosed.WithLabelValues("reader").Add(float64(readerStats.MaxIdleClosed))
				metrics.DBConnectionsMaxLifetimeClosed.WithLabelValues("reader").Add(float64(readerStats.MaxLifetimeClosed))
			}
		}
	}
}

// Close closes all database connections and stops metrics collection.
func (m *Manager) Close() error {
	close(m.closer)

	var errs []error
	if err := m.Writer.Close(); err != nil {
		errs = append(errs, err)
	}
	if m.Reader != m.Writer {
		if err := m.Reader.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		// Use errors.Join to combine multiple errors (Go 1.20+)
		return errors.Join(errs...)
	}
	return nil
}
