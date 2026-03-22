package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/sethgrid/helloworld/internal/db"
)

// UserEvent represents something the user will want to know about
type UserEvent struct {
	dbManager *db.Manager
	closer    chan struct{}

	UserID    int64     `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Message   string    `json:"message,omitempty"`

	logger *slog.Logger
}

func NewUserEvent(dbManager *db.Manager, maxEventsPerUser int, logger *slog.Logger) *UserEvent {
	closeCh := make(chan struct{})
	ue := &UserEvent{dbManager: dbManager, closer: closeCh, logger: logger}

	// Start scheduled work goroutine
	go func() {
		t := time.NewTicker(1 * time.Hour)
		defer t.Stop()

		for {
			select {
			case <-closeCh:
				return
			case <-t.C:
				ue.logger.Info("scheduled work: call some function")
				// add key value pairs for structured logs
				recordsUpdated := 3
				ue.logger.Info("scheduled work complete", "update_count", recordsUpdated)
			}
		}
	}()

	return ue
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
	if evt.dbManager == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := evt.dbManager.Ping(ctx); err != nil {
		evt.logger.Debug("event store unavailable", "error", err.Error())
		return false
	}
	return true
}
