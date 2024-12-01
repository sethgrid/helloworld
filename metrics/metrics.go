package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// RequestCount Counter
var RequestCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Number of HTTP requests handled by the server",
	},
	[]string{"method", "endpoint"},
)

// InFlightGauge
var InFlightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "http_in_flight_requests",
	Help: "Current number of in-flight requests",
})

// RequestDuration Histogram
var RequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Histogram of request duration for HTTP requests",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, .75, 1, 1.5, 2, 2.5, 3, 5, 10, 30, 60, 120},
	},
	[]string{"method", "endpoint"},
)

func init() {
	// Register the metrics
	// Counters
	prometheus.MustRegister(RequestCount)
	// Gauges
	prometheus.MustRegister(InFlightGauge)
	// Histograms
	prometheus.MustRegister(RequestDuration)
}
