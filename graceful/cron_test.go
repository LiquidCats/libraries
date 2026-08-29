package graceful_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LiquidCats/libraries/graceful"
)

type mockTask struct {
	runCount int32
}

func (m *mockTask) Spec() string {
	// Run a task very frequently so the test can observe it quickly.
	return "@every 1s"
}

func (m *mockTask) Run() {
	atomic.AddInt32(&m.runCount, 1)
}

func TestScheduleRunnerRunsTask(t *testing.T) {
	task := &mockTask{}
	runner := graceful.ScheduleRunner(task)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error)

	go func() { errCh <- runner(ctx) }()

	// Allow some time for the task to run a few times.
	time.Sleep(2 * time.Second)
	cancel()

	// Wait for the runner to finish and check its return value.
	err := <-errCh
	require.Equal(t, context.Canceled, err)

	// The task should have run at least once.
	assert.Greater(t, atomic.LoadInt32(&task.runCount), int32(0), "task was not executed")
}

func TestScheduleRunnerMultipleTasks(t *testing.T) {
	task1 := &mockTask{}
	task2 := &mockTask{}
	runner := graceful.ScheduleRunner(task1, task2)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error)

	wg := &sync.WaitGroup{}
	wg.Go(func() {
		errCh <- runner(ctx)
	})

	time.Sleep(2 * time.Second)
	cancel()

	err := <-errCh
	require.Equal(t, context.Canceled, err)

	// Both tasks should have run at least once.
	assert.Greater(t, atomic.LoadInt32(&task1.runCount), int32(0), "task1 was not executed")
	assert.Greater(t, atomic.LoadInt32(&task2.runCount), int32(0), "task2 was not executed")
}

func TestScheduleRunnerContextCancel(t *testing.T) {
	task := &mockTask{}
	runner := graceful.ScheduleRunner(task)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error)

	go func() { errCh <- runner(ctx) }()

	// Cancel immediately; the scheduler should exit quickly.
	cancel()

	err := <-errCh
	assert.Equal(t, context.Canceled, err)
}
