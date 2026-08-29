package graceful

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rotisserie/eris"
	"golang.org/x/sync/errgroup"
)

var ErrShutdownBySignal = eris.New("shutdown by signal")

type Runner func(ctx context.Context) error

func Signals(ctx context.Context) error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigs)

	for {
		select {
		case <-sigs:
			return ErrShutdownBySignal
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func WaitContext(ctx context.Context, runners ...Runner) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	group, ctx := errgroup.WithContext(ctx)

	// Start a goroutine for each runner
	for _, r := range runners {
		runner := r
		group.Go(func() error {
			return runner(ctx)
		})
	}

	err := group.Wait()
	if eris.Is(err, ErrShutdownBySignal) {
		return nil
	}

	return eris.Wrap(err, "shutting down with error")
}
