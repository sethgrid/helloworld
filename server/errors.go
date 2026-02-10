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

// errorJSON prepares the user message for json format. In the event an err is present, an ERROR level log will be emitted, else INFO
// This is a standalone helper function that doesn't require the Server struct, following modern Go handler patterns.
// The log is written first to ensure it's captured even if the response write fails.
func errorJSON(w http.ResponseWriter, r *http.Request, statusCode int, userMsg string, err error) {
	// NOTE: if this was a kverr, those key:value pairs will be pulled out and attached to our error log here
	// Log first to ensure it's captured even if response write fails due to timeout
	logger := logger.FromRequest(r).With("status_code", statusCode).With(kverr.YoinkArgs(err)...)
	logger.Error(userMsg, "error", err.Error())

	// Try to write response, but don't fail if it's already too late
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResp{Message: userMsg})
}
