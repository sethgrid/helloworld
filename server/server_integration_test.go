//go:build unitintegration

// the build tag allows integration style tests with go test ./... -tags=unitintegration
package server

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// launchOrGetTestServer will try to run in a faked test server or will connect to the configured
// HOST_ADDR and PORT
func launchOrGetTestServer(t *testing.T) (theURL string, logs bytes.Buffer, closefn func() error) {
	if os.Getenv("USE_LOCAL_helloworld") != "" {
		host := os.Getenv("HOST_ADDR")
		if strings.Contains(host, "helloworld.com") {
			t.Fatalf("do not run this on production; this is for local testing")
		}
		return fmt.Sprintf("http://%s:%s", host, os.Getenv("PORT")), logs, func() error { return nil }
	}

	srv, err := newTestServer(&logs)
	require.NoError(t, err)
	return fmt.Sprintf("http://localhost:%d", srv.Port()), logs, srv.Close
}

// TestLoginAndAddMatcher will not run by default when running go test
// because of the build tag at the top of the file, you have to run tests with matching tags.
// go test ./... -tags=unitintegration
func TestSomething(t *testing.T) {
	theURL, logs, closefn := launchOrGetTestServer(t)
	defer closefn()
	defer dumpLogsOnFailure(t, logs)

	// call the server at theURL. Inspect logs.
	fmt.Sprintf(theURL)
	fmt.Sprintf(logs.String())

}

func dumpLogsOnFailure(t *testing.T, logBuf bytes.Buffer) {
	if t.Failed() {
		fmt.Printf("\nServer Log Dump:\n%s\n", logBuf.String())
	}
}
