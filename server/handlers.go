package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/sethgrid/helloworld/logger"

	"github.com/sethgrid/kverr"
)

type helloworldResp struct {
	Hello string `json:"hello"`
}

func (s *Server) helloworldHandler(w http.ResponseWriter, r *http.Request) {
	// delay param can be any unit of time, e.g. 1s, 500ms, 1.5s
	// if delay is provided, don't simulate a random failure
	// else, simulate a random failure and pass additional info into the logger via kverr
	if delay := r.URL.Query().Get("delay"); delay != "" {
		duration, err := time.ParseDuration(delay)
		if err != nil {
			s.ErrorJSON(w, r, http.StatusBadRequest, "invalid delay", kverr.New(err, "delay", delay))
			return
		}
		if duration > 10*time.Second {
			duration = 10 * time.Second
		} else if duration < 1*time.Millisecond {
			duration = 1 * time.Millisecond
		}
		time.Sleep(duration)
	} else if err := RandomFailure(); err != nil {
		// NOTE: we don't have to tell other services that a kverr is being passed in
		s.ErrorJSON(w, r, http.StatusInternalServerError, "random failure", err)
		return
	}

	resp := helloworldResp{Hello: "World!"}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.FromRequest(r).Error("unable to encode json", "error", err.Error())
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

// DoSomethingWithEvents is for illustrative purposes of faking during tests,
// showing how faked dependencies bubble up in test assertions
func (s *Server) DoSomethingWithEvents() error {
	err := s.eventStore.Write(180, "a message in a bottle")
	if err != nil {
		s.parentLogger.Error(err.Error())
		return fmt.Errorf("unable to DoSomethingWithEvents: %w", err)
	}
	return nil
}
