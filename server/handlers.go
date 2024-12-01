package server

import (
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"

	"github.com/sethgrid/helloworld/logger"

	"github.com/sethgrid/kverr"
)

type helloworldResp struct {
	Hello string `json:"hello"`
}

func (s *Server) helloworldHandler(w http.ResponseWriter, r *http.Request) {

	// simulate some work
	if err := RandomFailure(); err != nil {
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
