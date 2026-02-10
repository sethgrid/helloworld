package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"github.com/sethgrid/helloworld/logger"

	"github.com/sethgrid/kverr"
)

type helloworldResp struct {
	Hello               string `json:"hello"`
	EventStoreAvailable bool   `json:"event_store_available"`
	EventStoreMessage   string `json:"event_store_message"`
}

// handleHelloworld is a standalone handler function that receives dependencies via closure.
// Following modern Go patterns, handlers are not methods on the Server struct.
// The logger is injected via middleware and accessed through the request context.
func handleHelloworld(eventStore eventWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// delay param can be any unit of time, e.g. 1s, 500ms, 1.5s
		// if delay is provided, don't simulate a random failure
		// else, simulate a random failure and pass additional info into the logger via kverr
		if delay := r.URL.Query().Get("delay"); delay != "" {
			duration, err := time.ParseDuration(delay)
			if err != nil {
				errorJSON(w, r, http.StatusBadRequest, "invalid delay", kverr.New(err, "delay", delay))
				return
			}
			if duration > 90*time.Second {
				logger.FromRequest(r).Error("delay too long", "duration", duration.String())
				duration = 1 * time.Millisecond
			} else if duration < 1*time.Millisecond {
				duration = 1 * time.Millisecond
			}
			time.Sleep(duration)

			err = someWorkThatChecksContextDeadline(r.Context())
			if err != nil {
				errorJSON(w, r, http.StatusRequestTimeout, "context deadline exceeded", err)
				return
			}

		} else if err := RandomFailure(); err != nil {
			// NOTE: we don't have to tell other services that a kverr is being passed in
			errorJSON(w, r, http.StatusInternalServerError, "random failure", err)
			return
		}

		// Check event store availability
		eventStoreAvailable := eventStore.IsAvailable()
		eventStoreMessage := "Event store is available"
		if !eventStoreAvailable {
			eventStoreMessage = "Event store is not available"
		}

		resp := helloworldResp{
			Hello:               "World!",
			EventStoreAvailable: eventStoreAvailable,
			EventStoreMessage:   eventStoreMessage,
		}

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			logger.FromRequest(r).Error("unable to encode json", "error", err.Error())
		}
	}
}

func RandomFailure() error {
	val := rand.Intn(3)
	if rand.Intn(2) == 0 { // Generate a random integer: 0 or 1
		// NOTE: the key value pair "val":val will be available to the server error logs
		return kverr.New(errors.New("operation failed"), "val", val)
	}
	return nil
}

// someWorkThatChecksContextDeadline is a demo showing how to check if the context is canceled
func someWorkThatChecksContextDeadline(ctx context.Context) error {
	// Check if the context is canceled
	select {
	case <-ctx.Done():
		// Context is canceled
		return kverr.New(fmt.Errorf("context canceled"), "context_err", ctx.Err())
	default:
		// Context is still active
	}

	// Do whatever work you need to do

	return nil
}

// DoSomethingWithEvents is for illustrative purposes of faking during tests,
// showing how faked dependencies bubble up in test assertions.
// This is a standalone function that receives dependencies as parameters.
func DoSomethingWithEvents(eventStore eventWriter, logger *slog.Logger) error {
	err := eventStore.Write(180, "a message in a bottle")
	if err != nil {
		logger.Error(err.Error())
		return fmt.Errorf("unable to DoSomethingWithEvents: %w", err)
	}
	return nil
}
