package taskqueue

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sethgrid/kverr"
)

type Task struct {
	ID        int
	UserID    int
	Status    string
	TaskType  string
	Payload   string
	Attempts  int
	CreatedAt time.Time
	UpdatedAt time.Time

	mu sync.Mutex
}

// Tasker defines the interface for task queue operations.
type Tasker interface {
	AddTask(userID int, taskType string, payload string) (int, error)
	FetchOpenTask() (*Task, error)
	MarkTaskComplete(taskID int) error
	CheckAndMarkDeadTasks() error
	CancelWhere(postWhereStatement string, args ...any) (int, error)
	Close() error
}

type Runner struct {
	// could add other dependencies, like the user store
	TaskStore Tasker

	workers      int
	taskCh       chan *Task
	logger       *slog.Logger
	pollInterval time.Duration

	mu      sync.Mutex
	wg      sync.WaitGroup
	closeCh chan struct{}
}

// NewRunner initializes the task queue with any implementation of Tasker
func NewRunner(taskStore Tasker, workers int, logger *slog.Logger, pollInterval time.Duration) *Runner {
	return &Runner{
		TaskStore:    taskStore,
		workers:      workers,
		taskCh:       make(chan *Task),
		pollInterval: pollInterval,
		logger:       logger,
		mu:           sync.Mutex{},
		wg:           sync.WaitGroup{},
		closeCh:      make(chan struct{}),
	}
}

func (tq *Runner) Start() {
	for i := 0; i < tq.workers; i++ {
		go tq.worker(i)
	}
	go func() {
		for {
			select {
			case <-tq.closeCh:
				return
			default:
			}
			tq.wg.Add(1)
			err := tq.TaskStore.CheckAndMarkDeadTasks()
			if err != nil {
				tq.logger.Error("unable to check for dead tasks", "error", err.Error())
			}
			tq.wg.Done()
			time.Sleep(tq.pollInterval)
		}
	}()
	go tq.pollTasks()

	<-tq.closeCh // block until closed
}

func (tq *Runner) Close() error {
	close(tq.closeCh)
	tq.wg.Wait()
	return tq.TaskStore.Close()
}

func (tq *Runner) worker(id int) {
	tq.logger.Debug(fmt.Sprintf("Worker %d started", id))
	for task := range tq.taskCh {
		tq.logger.Debug(fmt.Sprintf("Worker %d processing task %d", id, task.ID))
		// can we just copy *task or will wg mess us up?
		task.mu.Lock()
		cpy := Task{
			ID:       task.ID,
			UserID:   task.UserID,
			Status:   task.Status,
			TaskType: task.TaskType,
			Payload:  task.Payload,
			Attempts: task.Attempts,
		}
		task.mu.Unlock()

		tq.wg.Add(1)
		tq.processTask(cpy)
		tq.wg.Done()

	}
}

var ErrClosed = fmt.Errorf("runner closed")

func (tq *Runner) pollTasks() {
	defer func() {
		// between checking for the closed channel and pushing a task, we could have closed the task channel.
		// this should only have a chance to happen in testing during / server shutdown
		if r := recover(); r != nil {
			tq.logger.Error(fmt.Sprintf("Recovered from panic in poller: %v", r))
		}
	}()

	for {
		select {
		case <-tq.closeCh:
			return
		default:

		}
		task, err := tq.TaskStore.FetchOpenTask()
		if err != nil {
			if err == ErrClosed {
				tq.logger.Info("runner closed, exit poller")
				return
			}
			tq.logger.Error("error fetching task", "error", err.Error())
			time.Sleep(tq.pollInterval)
			continue
		}

		if task != nil {
			tq.taskCh <- task
		} else {
			time.Sleep(tq.pollInterval)
		}
	}
}

func (tq *Runner) processTask(task Task) {
	logger := tq.logger.With("user_id", task.UserID, "task_type", task.TaskType, "task_id", task.ID, "attempts", task.Attempts)
	switch task.TaskType {
	// TODO add some task types
	case "some predefined task":
		// pass
	default:
		logger.Error("unknown task", "task_type", task.TaskType)
		return
	}

	err := tq.TaskStore.MarkTaskComplete(task.ID)
	if err != nil {
		logger.Error("unable to mark task complete", kverr.YoinkArgs(err)...)

	}
}
