package taskqueue

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql" // used if we are manually testing the queue against the db via unit tests
	"github.com/sethgrid/helloworld/logger"
	"github.com/stretchr/testify/assert"
)

// queueInSQL will swap out the test friendly in-memory queue to use local mysql.
// i didn't want to spend time setting this up as an integration test yet. Can
// run tests manually with this set to true if any changes are made
var queueInSQL = false

func TestTaskQueue(t *testing.T) {
	var err error
	var buf bytes.Buffer
	var q Tasker

	log := logger.New(&buf)
	retries := 3
	workersCount := 3

	itemExpiration := 500 * time.Millisecond
	pollInterval := 100 * time.Millisecond

	if queueInSQL {
		dsn := "testuser:testuser@tcp(127.0.0.1:3306)/helloworld?parseTime=true"
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			t.Fatal(err)
		}
		if err = db.Ping(); err != nil {
			t.Fatal(err)
		}

		// give sql more time
		itemExpiration *= 3
		pollInterval *= 3

		q = NewMySQLTaskQueue(db, log, retries, itemExpiration)
	} else {
		q = NewInMemoryTaskQueue(retries, itemExpiration, log)
	}

	runner := NewRunner(q, workersCount, log, pollInterval)
	go runner.Start()

	userA := 1
	_, err = q.AddTask(userA, "use some pre-defined task type", "some payload")
	assert.NoError(t, err)

	// give the task time to do its thing. gross to sleep in tests.
	// todo: use signalling
	time.Sleep(100 * time.Millisecond)

	err = runner.Close()
	assert.NoError(t, err)

	fmt.Printf("logs:\n%s\n", buf.String())

	assertLogged(t, buf.String(), `"msg":"unknown task"`, `"task_type":"use some pre-defined task type"`)
}

func assertLogged(t *testing.T, logLines string, tokens ...string) {
	lines := strings.Split(logLines, "\n")

	// Check if any line contains all the tokens
	for _, line := range lines {
		if containsAllTokens(line, tokens) {
			return
		}
	}

	// If no line contained all tokens, log the error
	t.Errorf("got logs:\n%s\nmissing tokens on any single line: %v", logLines, tokens)
}

// Helper function to check if a line contains all tokens
func containsAllTokens(line string, tokens []string) bool {
	for _, token := range tokens {
		if !strings.Contains(line, token) {
			return false
		}
	}
	return true
}
