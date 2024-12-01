package taskqueue

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/sethgrid/kverr"
)

type MySQLTaskQueue struct {
	Logger          *slog.Logger
	DB              *sql.DB
	RetryLimit      int
	ReCheckoutAfter time.Duration
}

func NewMySQLTaskQueue(db *sql.DB, logger *slog.Logger, retryLimit int, itemExpiration time.Duration) *MySQLTaskQueue {
	return &MySQLTaskQueue{
		DB:              db,
		Logger:          logger,
		RetryLimit:      retryLimit,
		ReCheckoutAfter: itemExpiration,
	}
}

func (m *MySQLTaskQueue) AddTask(userID int, taskType string, payload string) (int, error) {
	result, err := m.DB.Exec(`
        INSERT INTO tasks (user_id, task_type, payload, status, created_at, updated_at)
        VALUES (?, ?, ?, 'open', NOW(), NOW())
    `, userID, taskType, payload)
	if err != nil {
		return 0, err
	}
	r, err := result.LastInsertId()
	// on int64 systems, there is no issue here. If we go to a 32 bit system (whyy????) then we can put protections here by forcing int64
	return int(r), err
}

func (m *MySQLTaskQueue) FetchOpenTask() (*Task, error) {
	// Begin a transaction
	tx, err := m.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback() // Ensure rollback in case of failure

	// Find an open task or an expired checked_out task
	expirationTime := time.Now().Add(-m.ReCheckoutAfter)
	task := &Task{}

	// Select a task in the transaction
	row := tx.QueryRow(`
        SELECT id, user_id, status, task_type, payload, created_at, updated_at, attempts 
        FROM tasks 
        WHERE (status = 'open' OR (status = 'checked_out' AND updated_at < ?))
        ORDER BY created_at ASC 
        LIMIT 1 FOR UPDATE
    `, expirationTime)

	err = row.Scan(&task.ID, &task.UserID, &task.Status, &task.TaskType, &task.Payload, &task.CreatedAt, &task.UpdatedAt, &task.Attempts)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No tasks available
		}
		return nil, fmt.Errorf("failed to fetch task: %w", err)
	}

	// Update task status and attempts in the same transaction
	_, err = tx.Exec(`
        UPDATE tasks 
        SET status = 'checked_out', updated_at = NOW(), attempts = attempts + 1
        WHERE id = ?
    `, task.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Return the task that was checked out
	task.Attempts++ // Reflect the incremented attempts
	task.Status = "checked_out"
	return task, nil

}

func (m *MySQLTaskQueue) CancelWhere(postWhereStatement string, args ...any) (int, error) {
	res, err := m.DB.Exec("delete from tasks where "+postWhereStatement, args...)
	if err != nil {
		return 0, fmt.Errorf("unable to cancel tasks: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("unable to get cow count when cancelling tasks")
	}

	// shouldn't be an issue on 64bit machines
	return int(count), nil
}

func (m *MySQLTaskQueue) MarkTaskComplete(taskID int) error {
	// Start a transaction
	tx, err := m.DB.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Step 1: Retrieve the task details before marking it complete
	var task Task // Assuming Task is a struct with the necessary fields
	err = tx.QueryRow(`
		SELECT id, user_id, status, task_type, attempts
		FROM tasks
		WHERE id = ?
		FOR UPDATE
	`, taskID).Scan(&task.ID, &task.UserID, &task.Status, &task.TaskType, &task.Attempts)
	if err != nil {
		return kverr.New(err, "task_id", taskID)
	}

	// Step 2: Log the task details
	m.Logger.Info("task complete", "task_id", task.ID, "user_id", task.UserID, "attempts", task.Attempts, "task_type", task.TaskType)

	// Step 3: Mark the task as complete
	_, err = tx.Exec(`
		UPDATE tasks
		SET status = 'complete', updated_at = NOW()
		WHERE id = ?
	`, taskID)
	if err != nil {
		return kverr.New(err, "task_id", taskID, "user_id", task.UserID)
	}

	return nil
}

func (m *MySQLTaskQueue) CheckAndMarkDeadTasks() error {
	// Start a transaction
	tx, err := m.DB.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Step 1: Select tasks that meet the criteria
	rows, err := tx.Query(`
			SELECT id, user_id, attempts, task_type
			FROM tasks
			WHERE status = 'checked_out' AND attempts >= ?
			FOR UPDATE
		`, m.RetryLimit)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Step 2: Iterate through the tasks and log details
	var tasks []Task // Assuming Task is a struct with the necessary fields
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.UserID, &task.Attempts, &task.TaskType); err != nil {
			return err
		}

		// Step 3: Log the task details
		m.Logger.Error("dead task", "task_id", task.ID, "user_id", task.UserID, "attempt", task.Attempts, "task_type", task.TaskType)

		// Add task to a list for batch update
		tasks = append(tasks, task)
	}

	// Step 4: Update all selected tasks to 'dead'
	for _, task := range tasks {
		_, err := tx.Exec(`
				UPDATE tasks
				SET status = 'dead', updated_at = NOW()
				WHERE id = ?
			`, task.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *MySQLTaskQueue) Close() error {
	return nil
}
