package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// RequestCount Counter (includes HTTP status for SLO / 5xx monitoring)
var RequestCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Number of HTTP requests handled by the server",
	},
	[]string{"method", "endpoint", "status"},
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

// Database connection pool metrics with store label to distinguish between different data stores
var (
	DBConnectionsOpen = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connections_open",
			Help: "Current number of open database connections",
		},
		[]string{"store"},
	)

	DBConnectionsIdle = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Current number of idle database connections",
		},
		[]string{"store"},
	)

	DBConnectionsInUse = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connections_in_use",
			Help: "Current number of in-use database connections",
		},
		[]string{"store"},
	)

	DBConnectionsWaitCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_connections_wait_total",
			Help: "Total number of connections waited for",
		},
		[]string{"store"},
	)

	DBConnectionsWaitDuration = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_connections_wait_duration_seconds_total",
			Help: "Total time spent waiting for connections",
		},
		[]string{"store"},
	)

	DBConnectionsMaxIdleClosed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_connections_max_idle_closed_total",
			Help: "Total number of connections closed due to SetMaxIdleConns",
		},
		[]string{"store"},
	)

	DBConnectionsMaxLifetimeClosed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_connections_max_lifetime_closed_total",
			Help: "Total number of connections closed due to SetConnMaxLifetime",
		},
		[]string{"store"},
	)

	// Database query timing metrics
	DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Histogram of database query duration",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"store", "operation"},
	)

	// External API call timing metrics
	APICallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_call_duration_seconds",
			Help:    "Histogram of external API call duration",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"service", "endpoint", "method"},
	)

	APICallCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_calls_total",
			Help: "Total number of external API calls",
		},
		[]string{"service", "endpoint", "method", "status"},
	)
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
	// Database query timing
	prometheus.MustRegister(DBQueryDuration)
	// External API call metrics
	prometheus.MustRegister(APICallDuration)
	prometheus.MustRegister(APICallCount)
}
