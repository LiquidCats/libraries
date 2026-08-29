package graceful

import (
	"context"

	"github.com/rotisserie/eris"
	"github.com/rs/zerolog"
)

var ErrWorkerFailure = eris.New("worker failure")

type worker struct {
	logger *zerolog.Logger
}

type WorkerOpt func(*worker)

func WithWorkerLogger(logger *zerolog.Logger) WorkerOpt {
	return func(w *worker) {
		if logger != nil {
			w.logger = logger
		}
	}
}

type WorkerHandler[T any] func(context.Context, T) error

func Worker[T any](ch <-chan T, runner WorkerHandler[T], opts ...WorkerOpt) Runner {
	noop := zerolog.Nop()
	cfg := &worker{
		logger: &noop,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return func(ctx context.Context) error {
		for value := range ch {
			if err := runner(ctx, value); err != nil {
				if eris.Is(err, ErrWorkerFailure) {
					return err
				}
				cfg.logger.
					Error().
					Any("error", eris.ToJSON(err, true)).
					Msg("runner failed")
			}
		}

		return nil
	}
}
