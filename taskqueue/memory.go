package taskqueue

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type InMemoryTaskQueue struct {
	tasks          map[int]*Task
	mu             sync.Mutex
	nextID         int64
	RetryLimit     int
	ItemExpiration time.Duration
	logger         *slog.Logger
}

func NewInMemoryTaskQueue(retryLimit int, itemExpiration time.Duration, logger *slog.Logger) *InMemoryTaskQueue {
	return &InMemoryTaskQueue{
		tasks:          make(map[int]*Task, 3), // should set to the number of workers + 1
		nextID:         1,
		RetryLimit:     retryLimit,
		ItemExpiration: itemExpiration,
		logger:         logger,
	}
}

func (m *InMemoryTaskQueue) AddTask(userID int, taskType string, payload string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task := &Task{
		ID:        int(m.nextID),
		UserID:    userID,
		Status:    "open",
		TaskType:  taskType,
		Payload:   payload,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.logger.Info("add task", "user_id", userID, "task_type", taskType, "task_id", task.ID)
	m.tasks[task.ID] = task
	m.nextID++
	return task.ID, nil
}

func (m *InMemoryTaskQueue) FetchOpenTask() (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.tasks {
		m.logger.Debug("ranging task, task pulled")
		if task.Status == "open" || (task.Status == "checked_out" && time.Since(task.UpdatedAt) > m.ItemExpiration) {
			m.logger.Debug("check out", "task_id", task.ID, "user_id", task.UserID, "attempt", task.Attempts)
			task.mu.Lock()
			task.Status = "checked_out"
			task.Attempts++
			task.UpdatedAt = time.Now()
			task.mu.Unlock()
			return task, nil
		}
	}

	return nil, nil
}

func (m *InMemoryTaskQueue) MarkTaskComplete(taskID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found")
	}
	task.mu.Lock()
	defer task.mu.Unlock()

	m.logger.Info("task complete", "task_id", task.ID, "user_id", task.UserID, "attempt", task.Attempts, "task_type", task.TaskType)
	task.Status = "complete"
	return nil
}

func (m *InMemoryTaskQueue) CancelWhere(postWhereStatement string, args ...any) (int, error) {
	// to be implemented?
	return 0, nil
}

func (m *InMemoryTaskQueue) CheckAndMarkDeadTasks() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Debug("ranging tasks to find expired items")
	for _, task := range m.tasks {
		task.mu.Lock()
		if task.Status == "checked_out" && task.Attempts >= m.RetryLimit {
			task.Status = "dead"
			m.logger.Error("dead task", "task_id", task.ID, "user_id", task.UserID, "attempt", task.Attempts, "task_type", task.TaskType)
		}
		task.mu.Unlock()
	}
	return nil
}

func (m *InMemoryTaskQueue) Close() error {
	m.mu.Lock()

	for k := range m.tasks {
		delete(m.tasks, k)
	}

	m.mu.Unlock()
	return nil
}
