package server

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/sethgrid/helloworld/logger"
)

// StatusResponse represents the status page response
type StatusResponse struct {
	Status      string                 `json:"status"`
	Version     string                 `json:"version"`
	Uptime      string                 `json:"uptime"`
	Timestamp   time.Time              `json:"timestamp"`
	Components  map[string]ComponentStatus `json:"components"`
	System      SystemInfo             `json:"system"`
}

// ComponentStatus represents the status of a component
type ComponentStatus struct {
	Status      string    `json:"status"`
	LastChecked time.Time `json:"last_checked,omitempty"`
	Message     string    `json:"message,omitempty"`
}

// SystemInfo represents system-level information
type SystemInfo struct {
	GoVersion   string `json:"go_version"`
	NumGoroutine int    `json:"num_goroutines"`
	NumCPU      int    `json:"num_cpu"`
}

var serverStartTime = time.Now()

// handleStatus returns a comprehensive status page with component health checks
func handleStatus(eventStore eventWriter, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromRequest(r)
		
		components := make(map[string]ComponentStatus)
		
		// Check event store
		eventStoreStatus := ComponentStatus{
			Status:      "healthy",
			LastChecked: time.Now(),
		}
		if eventStore == nil {
			eventStoreStatus.Status = "unavailable"
			eventStoreStatus.Message = "Event store not configured"
		} else if !eventStore.IsAvailable() {
			eventStoreStatus.Status = "unhealthy"
			eventStoreStatus.Message = "Database unreachable"
		}
		components["event_store"] = eventStoreStatus
		
		// Determine overall status
		overallStatus := "healthy"
		for _, comp := range components {
			if comp.Status != "healthy" {
				overallStatus = "degraded"
				if comp.Status == "unhealthy" {
					overallStatus = "unhealthy"
					break
				}
			}
		}
		
		// Get system info
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		status := StatusResponse{
			Status:    overallStatus,
			Version:   version,
			Uptime:    time.Since(serverStartTime).String(),
			Timestamp: time.Now(),
			Components: components,
			System: SystemInfo{
				GoVersion:    runtime.Version(),
				NumGoroutine: runtime.NumGoroutine(),
				NumCPU:       runtime.NumCPU(),
			},
		}
		
		// Set appropriate status code
		statusCode := http.StatusOK
		if overallStatus == "unhealthy" {
			statusCode = http.StatusServiceUnavailable
		} else if overallStatus == "degraded" {
			statusCode = http.StatusOK // Still return 200 but indicate degraded status
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		
		if err := json.NewEncoder(w).Encode(status); err != nil {
			log.Error("failed to encode status response", "error", err)
		}
	}
}
