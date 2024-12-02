package server

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

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
	var logbuf bytes.Buffer

	srv, err := newTestServer(&logbuf)
	require.NoError(t, err)
	defer srv.Close()

	// replace the event store
	srv.eventStore = &fakeEventStore{err: fmt.Errorf("oh noes, mysql err")}

	err = srv.DoSomethingWithEvents()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "oh noes, mysql err")

	require.Contains(t, logbuf.String(), "oh noes, mysql err")
}

func TestGracefulShutdown(t *testing.T) {
	srv, err := newTestServer()
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	for i := 0; i < 3; i++ {
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
	// todo: instead of sleeping in tests, use the metrics endpoint to show inflight request
	time.Sleep(100 * time.Millisecond)

	err = srv.Close()
	require.NoError(t, err)

	_, err = http.Get(fmt.Sprintf("http://localhost:%d/?delay=1s", srv.Port()))
	require.Error(t, err)

	assert.Contains(t, err.Error(), "connection refused")
	// make sure that all requests have successfully completed
	wg.Wait()
}

// newTestServer is generally called with no parameter. A bit of a hack on variadics, but if you want to pass in a buffer, pass one in.
// var buf bytes.Buffer
// newTestServer(buf)
// and later: buf.String()
// NOTE: currently the task runner uses a default user store. if the server is using a custom user store it wont be related.
// TODO: consider using functional options for logger, stores, and mailer err
func newTestServer(logWriter ...io.Writer) (*Server, error) {
	writer := io.Discard
	if len(logWriter) == 1 {
		writer = logWriter[0]
	}

	log := slog.New(slog.NewJSONHandler(writer, nil))
	q := taskqueue.NewInMemoryTaskQueue(1, 15*time.Second, log)

	srv := &Server{
		// port explicitly set to zero.
		// this means that the OS will bind a random available port.
		// During testing, this means we can spin up multiple servers with no port collision.
		port:         0,
		internalPort: 0,
		protocol:     "http://",
		taskq:        q,
		parentLogger: log,
		eventStore:   &fakeEventStore{},

		mu: sync.Mutex{},
	}

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

	taskqueue.NewRunner(srv.taskq, 1, log, 75*time.Millisecond)

	return srv, nil

}

type fakeEventStore struct {
	err error
}

func (f *fakeEventStore) Write(userID int64, message string) error {
	return f.err
}

func (f *fakeEventStore) Close() error {
	return nil
}
