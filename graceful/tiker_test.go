package graceful_test

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LiquidCats/libraries/graceful"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// Test that the ticker invokes the runner at the specified interval.
func TestTickerRunsAtInterval(t *testing.T) {
	interval := 5 * time.Millisecond
	var cnt int32
	runner := func(ctx context.Context) error {
		atomic.AddInt32(&cnt, 1)
		return nil
	}
	ticker := graceful.Ticker(interval, runner)

	ctx, cancel := context.WithCancel(context.Background())

	wg := &sync.WaitGroup{}

	wg.Go(func() {
		_ = ticker(ctx)
	})

	// Let the ticker run a few times
	time.Sleep(30 * time.Millisecond)
	cancel()
	wg.Wait()
	// Expect at least 5 ticks (interval 5ms * 5 = 25ms)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&cnt), int32(5))
}

// Test that a runner returning ErrTickerFailure causes the ticker to stop with that error.
func TestTickerErrTickerFailure(t *testing.T) {
	interval := 10 * time.Millisecond
	runner := func(ctx context.Context) error { return graceful.ErrTickerFailure }
	ticker := graceful.Ticker(interval, runner)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	var runErr error
	go func() {
		defer wg.Done()
		runErr = ticker(ctx)
	}()

	// Allow at least one tick
	time.Sleep(20 * time.Millisecond)
	cancel()
	wg.Wait()
	assert.Equal(t, graceful.ErrTickerFailure, runErr)
}

// Test that non-failure errors are logged and the ticker continues.
func TestTickerNonFailureErrorLogged(t *testing.T) {
	interval := 5 * time.Millisecond
	var tickCnt int32
	// First tick errors, subsequent ticks succeed
	runner := func(ctx context.Context) error {
		atomic.AddInt32(&tickCnt, 1)
		if atomic.LoadInt32(&tickCnt) == 1 {
			return fmt.Errorf("test error")
		}
		return nil
	}
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	ticker := graceful.Ticker(interval, runner, graceful.WithTickerLogger(&logger))

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = ticker(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	wg.Wait()

	// The ticker should have run at least 4 times
	assert.GreaterOrEqual(t, atomic.LoadInt32(&tickCnt), int32(4))
	logs := buf.String()
	assert.Contains(t, logs, "runner failed")
	assert.Contains(t, logs, "test error")
}

// Test that the ticker stops when the context is cancelled.
func TestTickerContextCancellation(t *testing.T) {
	interval := 5 * time.Millisecond
	runner := func(ctx context.Context) error { return nil }
	ticker := graceful.Ticker(interval, runner)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	err := ticker(ctx)
	assert.EqualError(t, err, context.Canceled.Error())
}

// Test that WithTickerLogger sets the logger used by the ticker.
func TestTickerWithLogger(t *testing.T) {
	interval := 5 * time.Millisecond
	runner := func(ctx context.Context) error { return nil }
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	ticker := graceful.Ticker(interval, runner, graceful.WithTickerLogger(&logger))

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = ticker(ctx)
	}()
	time.Sleep(15 * time.Millisecond)
	cancel()
	wg.Wait()

	logs := buf.String()
	assert.Contains(t, logs, "starting ticker")
	assert.Contains(t, logs, "stopped ticker")
}
