package server

import (
	"encoding/json"
	"net/http"

	"github.com/sethgrid/helloworld/logger"
)

type errorResp struct {
	Message string `json:"message"`
}

// ErrorJSON prepares the user message for json format. In the event an err is present, an ERROR level log will be emitted, else INFO
func (s *Server) ErrorJSON(w http.ResponseWriter, r *http.Request, statusCode int, userMsg string, err error) {
	logger := logger.FromRequest(r).With("status_code", statusCode)
	if err != nil {
		defer logger.Error(userMsg, "error", err.Error())
	} else {
		// the main use case for this is to suppress login or other user input errors being reported as errors
		defer logger.Info(userMsg)
	}

	w.Header().Set("content-type", "application/json")
	w.WriteHeader(statusCode)

	json.NewEncoder(w).Encode(errorResp{Message: userMsg})
}
