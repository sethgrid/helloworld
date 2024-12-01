package events

import (
	"database/sql"
	"log/slog"
	"time"
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
