package utils

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// TimerService executes a specific task periodically.
type TimerService struct {
	name     string
	interval time.Duration
	task     func()
	running  int32 // Atomic flag: 1 if running, 0 if stopped
}

// NewTimerService creates a new periodic task service.
func NewTimerService(name string, interval time.Duration, task func()) *TimerService {
	return &TimerService{
		name:     name,
		interval: interval,
		task:     task,
	}
}

func (t *TimerService) Name() string {
	return t.name
}

func (t *TimerService) Start(ctx context.Context) error {
	atomic.StoreInt32(&t.running, 1)
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Execute the task in a separate goroutine so it doesn't block the ticker
			go t.task()
		case <-ctx.Done():
			atomic.StoreInt32(&t.running, 0)
			return nil
		}
	}
}

func (t *TimerService) Stop() error {
	atomic.StoreInt32(&t.running, 0)
	return nil
}

func (t *TimerService) Status() string {
	if atomic.LoadInt32(&t.running) == 1 {
		return fmt.Sprintf("Running (Interval: %s)", t.interval.String())
	}
	return "Stopped"
}
