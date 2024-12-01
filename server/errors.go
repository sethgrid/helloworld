package server

import (
	"encoding/json"
	"net/http"

	"github.com/sethgrid/helloworld/logger"
	"github.com/sethgrid/kverr"
)

type errorResp struct {
	Message string `json:"message"`
}

// ErrorJSON prepares the user message for json format. In the event an err is present, an ERROR level log will be emitted, else INFO
func (s *Server) ErrorJSON(w http.ResponseWriter, r *http.Request, statusCode int, userMsg string, err error) {
	// NOTE: if this was a kverr, those key:value pairs will be pulled out and attached to our error log here
	logger := logger.FromRequest(r).With("status_code", statusCode).With(kverr.YoinkArgs(err)...)
	defer logger.Error(userMsg, "error", err.Error())

	w.Header().Set("content-type", "application/json")
	w.WriteHeader(statusCode)

	json.NewEncoder(w).Encode(errorResp{Message: userMsg})
}
