package server

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sethgrid/helloworld/logger/lockbuffer"
	"github.com/sethgrid/helloworld/taskqueue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthcheck(t *testing.T) {
	srv, err := newTestServer()
	require.NoError(t, err)
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthcheck", srv.InternalPort()))
	require.NoError(t, err)

	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestEventStoreErr(t *testing.T) {
	logbuf := lockbuffer.NewLockBuffer()

	srv, err := newTestServer(WithLogWriter(logbuf))
	require.NoError(t, err)
	defer srv.Close()

	// replace the event store
	srv.eventStore = &fakeEventStore{err: fmt.Errorf("oh noes, mysql err")}

	err = srv.DoSomethingWithEvents()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "oh noes, mysql err")

	require.Contains(t, logbuf.String(), "oh noes, mysql err")
}

func TestGracefulShutdownOK(t *testing.T) {
	srv, err := newTestServer()
	require.NoError(t, err)

	concurrentRequests := 10

	wg := sync.WaitGroup{}
	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			source := fmt.Sprintf("http://localhost:%d/?delay=1s", srv.Port())
			resp, err := http.Get(source)
			require.NoError(t, err)
			assert.Equal(t, resp.StatusCode, http.StatusOK, "source: %s", source)
			wg.Done()
		}()
	}

	// give time for all requests to go out
	// note: avoid sleeping in tests. To keep tests fast, poll for the expected state or event over a channel
	// AVOID: time.Sleep(1000 * time.Millisecond)
	// ATTEMPT: poll for the expected state
	assertMetric(t, srv, "http_in_flight_requests", float64(concurrentRequests), 2*time.Second)

	err = srv.Close()
	require.NoError(t, err)

	_, err = http.Get(fmt.Sprintf("http://localhost:%d/?delay=1s", srv.Port()))
	require.Error(t, err)

	assert.Contains(t, err.Error(), "connection refused")
	// make sure that all requests have successfully completed
	wg.Wait()
}

func TestGracefulShutdownErr(t *testing.T) {
	logbuf := lockbuffer.NewLockBuffer()

	srv, err := newTestServer(WithLogWriter(logbuf))
	require.NoError(t, err)

	srv.eventStore = &fakeEventStore{err: fmt.Errorf("oh noes, close err")}

	err = srv.Close()
	require.Error(t, err)
	assert.Contains(t, logbuf.String(), `"error":"oh noes, close err"`)
	assert.Contains(t, logbuf.String(), `"msg":"unable to close event store"`)
}

func TestContextTimeoutAndRequestTimeout(t *testing.T) {
	logbuf := lockbuffer.NewLockBuffer()
	customConfig := Config{
		// server kills any request that takes longer than this
		RequestTimeout: 100 * time.Millisecond,
	}

	srv, err := newTestServer(WithConfig(customConfig), WithLogWriter(logbuf))
	require.NoError(t, err)

	source := fmt.Sprintf("http://localhost:%d/?delay=101ms", srv.Port())
	_, err = http.Get(source)
	require.Error(t, err)

	assert.Contains(t, logbuf.String(), `"error":"context canceled"`)

}

func assertMetric(t *testing.T, srv *Server, metric string, target float64, timeout time.Duration) {
	start := time.Now()
	for {
		value, err := getMetric(srv, metric)
		if err == nil && value == target {
			return
		} else if err != nil {
			t.Errorf("error fetching metric: %v", err)
			return
		}

		fmt.Printf("waiting for metric %s to reach %f, currently at %f\n", metric, target, value)

		// Check if timeout has been reached
		if time.Since(start) >= timeout {
			t.Errorf("timeout reached before target metric value was reached: %s=%f, got %s=%f", metric, target, metric, value)
			return
		}

		// Wait for the next interval
		time.Sleep(100 * time.Millisecond)
	}
}

func getMetric(srv *Server, metric string) (float64, error) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", srv.InternalPort()))
	if err != nil {
		return 0, err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	resp.Body.Close()

	return findMetricValue(buf, metric)
}

// findMetricValue parses the metrics data and retrieves the number from the first line with the given prefix.
func findMetricValue(metrics *bytes.Buffer, prefix string) (float64, error) {
	scanner := bufio.NewScanner(metrics)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip comment lines
		if strings.HasPrefix(line, "#") {
			continue
		}
		// Check if the line starts with the desired prefix
		if strings.HasPrefix(line, prefix) {
			// Split the line into the metric name and value
			parts := strings.Fields(line)
			if len(parts) < 2 {
				return 0, fmt.Errorf("malformed metric line: %s", line)
			}
			// Convert the value to a float
			value, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				return 0, fmt.Errorf("invalid metric value: %v", err)
			}
			return value, nil
		}
	}
	// Return an error if no matching prefix was found
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading metrics: %v", err)
	}
	return 0, fmt.Errorf("metric with prefix '%s' not found", prefix)
}

// newTestServer is generally called with no parameter, but can be called with optional logger or config overrides
/*
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	customConfig := Config{
		ShutdownTimeout: 5 * time.Second,
		RequestTimeout:  10 * time.Second,
	}
	srv, err := newTestServer(WithLogger(logger), WithConfig(customConfig))
	if err != nil {
		log.Fatal(err)
	}
*/
func newTestServer(opts ...func(*Server)) (*Server, error) {
	// Default writer for the logger
	writer := io.Discard
	log := slog.New(slog.NewJSONHandler(writer, nil))

	// Default configuration
	defaultConfig := Config{
		ShutdownTimeout: 3 * time.Second,
		RequestTimeout:  3 * time.Second,
	}

	// Initialize task queue
	q := taskqueue.NewInMemoryTaskQueue(1, 15*time.Second, log)

	// Create server with default values
	srv := &Server{
		port:         0, // OS will bind a random available port
		config:       defaultConfig,
		internalPort: 0,
		protocol:     "http://",
		taskq:        q,
		parentLogger: log,
		eventStore:   &fakeEventStore{},
		mu:           sync.Mutex{},
	}

	// Apply optional arguments
	for _, opt := range opts {
		opt(srv)
	}

	// Start server
	go srv.Serve()

	port := srv.Port()
	internalPort := srv.InternalPort()

	if err := srv.LastError(); err != nil {
		return nil, err
	}
	if port == 0 {
		return nil, fmt.Errorf("server did not bind to a new port")
	}
	if internalPort == 0 {
		return nil, fmt.Errorf("server did not bind to a new internal port")
	}

	srv.addr = fmt.Sprintf("localhost:%d", port)

	taskqueue.NewRunner(srv.taskq, 1, srv.parentLogger, 75*time.Millisecond)

	return srv, nil
}

type fakeEventStore struct {
	err error
}

func (f *fakeEventStore) Write(userID int64, message string) error {
	return f.err
}

func (f *fakeEventStore) Close() error {
	return f.err
}
