package server

import (
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
