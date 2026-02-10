package events

import (
	"database/sql"
	"log/slog"
	"time"

	"github.com/sethgrid/helloworld/metrics"
)

// UserEvent represents something the user will want to know about
type UserEvent struct {
	db     *sql.DB
	closer chan struct{}

	UserID    int64     `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Message   string    `json:"message,omitempty"`

	logger *slog.Logger
}

func NewUserEvent(db *sql.DB, maxEventsPerUser int, logger *slog.Logger) *UserEvent {
	closeCh := make(chan struct{})
	ue := &UserEvent{db: db, closer: closeCh, logger: logger}
	
	// Start scheduled work goroutine
	go func() {
		t := time.NewTicker(1 * time.Hour)
		defer t.Stop()

		for {
			select {
			case <-closeCh:
				return
			case <-t.C:
				ue.logger.Info("scheduled workd: call some function")
				// add key value pairs for structured logs
				recordsUpdated := 3
				ue.logger.Info("scheduled work complete", "update_count", recordsUpdated)
			}
		}
	}()

	// Start metrics collection for this event store's database connection
	go ue.collectMetrics(closeCh)

	return ue
}

// collectMetrics periodically updates Prometheus metrics from database connection pool stats
func (evt *UserEvent) collectMetrics(closeCh chan struct{}) {
	if evt.db == nil {
		return
	}
	const storeLabel = "events"
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-closeCh:
			return
		case <-ticker.C:
			stats := evt.db.Stats()
			metrics.DBConnectionsOpen.WithLabelValues(storeLabel).Set(float64(stats.OpenConnections))
			metrics.DBConnectionsIdle.WithLabelValues(storeLabel).Set(float64(stats.Idle))
			metrics.DBConnectionsInUse.WithLabelValues(storeLabel).Set(float64(stats.InUse))
			metrics.DBConnectionsWaitCount.WithLabelValues(storeLabel).Add(float64(stats.WaitCount))
			metrics.DBConnectionsWaitDuration.WithLabelValues(storeLabel).Add(stats.WaitDuration.Seconds())
			metrics.DBConnectionsMaxIdleClosed.WithLabelValues(storeLabel).Add(float64(stats.MaxIdleClosed))
			metrics.DBConnectionsMaxLifetimeClosed.WithLabelValues(storeLabel).Add(float64(stats.MaxLifetimeClosed))
		}
	}
}

// Close cleans up the work loop that is created when a new UserEvent is created
func (evt *UserEvent) Close() error {
	close(evt.closer)
	return nil
}

// Write would write to a hypotetical stream or db
func (evt *UserEvent) Write(userID int64, message string) error {
	evt.logger.Info("writing event message", "user_id", userID, "msg", message)
	return nil
}

// IsAvailable pings the database to check if the event store is available.
// Returns true if the ping succeeds, false otherwise.
func (evt *UserEvent) IsAvailable() bool {
	if evt.db == nil {
		return false
	}
	if err := evt.db.Ping(); err != nil {
		evt.logger.Debug("event store unavailable", "error", err.Error())
		return false
	}
	return true
}
