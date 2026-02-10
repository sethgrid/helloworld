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

// Database connection pool metrics
var (
	DBConnectionsOpen = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_open",
		Help: "Current number of open database connections",
	})

	DBConnectionsIdle = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_idle",
		Help: "Current number of idle database connections",
	})

	DBConnectionsInUse = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_in_use",
		Help: "Current number of in-use database connections",
	})

	DBConnectionsWaitCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "db_connections_wait_total",
		Help: "Total number of connections waited for",
	})

	DBConnectionsWaitDuration = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "db_connections_wait_duration_seconds_total",
		Help: "Total time spent waiting for connections",
	})

	DBConnectionsMaxIdleClosed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "db_connections_max_idle_closed_total",
		Help: "Total number of connections closed due to SetMaxIdleConns",
	})

	DBConnectionsMaxLifetimeClosed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "db_connections_max_lifetime_closed_total",
		Help: "Total number of connections closed due to SetConnMaxLifetime",
	})
)

func init() {
	// Register the metrics
	// Counters
	prometheus.MustRegister(RequestCount)
	// Gauges
	prometheus.MustRegister(InFlightGauge)
	prometheus.MustRegister(DBConnectionsOpen)
	prometheus.MustRegister(DBConnectionsIdle)
	prometheus.MustRegister(DBConnectionsInUse)
	// Histograms
	prometheus.MustRegister(RequestDuration)
	// Database connection pool counters
	prometheus.MustRegister(DBConnectionsWaitCount)
	prometheus.MustRegister(DBConnectionsWaitDuration)
	prometheus.MustRegister(DBConnectionsMaxIdleClosed)
	prometheus.MustRegister(DBConnectionsMaxLifetimeClosed)
}
