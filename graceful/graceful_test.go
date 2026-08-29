package graceful_test

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/LiquidCats/libraries/graceful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignalsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := graceful.Signals(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Expected context.Canceled, got %v", err)
	}
}

func TestSignalsReceived(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- graceful.Signals(ctx)
	}()

	// Allow some time for the goroutine to start and listen for signals.
	time.Sleep(100 * time.Millisecond)

	// Send a SIGINT signal to the current process.
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("Failed to send SIGINT: %v", err)
	}

	select {
	case err := <-done:
		assert.ErrorIs(t, err, graceful.ErrShutdownBySignal)
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for signal to be handled")
	}
}

func TestWaitContextAllRunnersSucceed(t *testing.T) {
	ctx := context.Background()

	runner1 := func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	runner2 := func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	err := graceful.WaitContext(ctx, runner1, runner2)
	require.NoError(t, err)
}

func TestWaitContextRunnerError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("runner error")

	runner1 := func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return expectedErr
	}

	runner2 := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	err := graceful.WaitContext(ctx, runner1, runner2)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestWaitContextContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	runner := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	err := graceful.WaitContext(ctx, runner)

	assert.Error(t, err)

	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("Expected a context cancellation error, got %v", err)
	}
}

func TestWaitContextShutdownSignalIgnored(t *testing.T) {
	ctx := context.Background()

	runner1 := func(ctx context.Context) error {
		return graceful.ErrShutdownBySignal
	}

	runner2 := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}

	err := graceful.WaitContext(ctx, runner1, runner2)
	assert.NoError(t, err)
}
