package taskqueue

import (
	"time"

	"github.com/sethgrid/helloworld/metrics"
)

const storeLabel = "taskqueue"

// timeDBOperation times a database operation and records it in Prometheus metrics
func timeDBOperation(operation string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	
	metrics.DBQueryDuration.WithLabelValues(storeLabel, operation).Observe(duration.Seconds())
	
	return err
}
