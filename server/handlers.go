package server

import (
	"encoding/json"
	"net/http"

	"github.com/sethgrid/helloworld/logger"
)

type helloworldResp struct {
	Hello string `json:"hello"`
}

func (s *Server) helloworldHandler(w http.ResponseWriter, r *http.Request) {
	resp := helloworldResp{Hello: "World!"}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.FromRequest(r).Error("unable to encode json", "error", err.Error())
	}
}
