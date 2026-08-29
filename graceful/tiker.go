package graceful

import (
	"context"
	"time"

	"github.com/rotisserie/eris"
	"github.com/rs/zerolog"
)

var ErrTickerFailure = eris.New("ticker failure")

type ticker struct {
	logger *zerolog.Logger
}

type TickerOpt func(*ticker)

func WithTickerLogger(logger *zerolog.Logger) TickerOpt {
	return func(t *ticker) {
		if logger != nil {
			t.logger = logger
		}
	}
}

func Ticker(interval time.Duration, runner Runner, opts ...TickerOpt) Runner {
	if interval <= 0 {
		panic("interval must be greater than zero")
	}

	noop := zerolog.Nop()
	cfg := &ticker{
		logger: &noop,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context) error {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		cfg.logger.Info().Msg("starting ticker")
		defer cfg.logger.Info().Msg("stopped ticker")

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				if err := runner(ctx); err != nil {
					if eris.Is(err, ErrTickerFailure) {
						return err
					}
					cfg.logger.
						Error().
						Any("error", eris.ToJSON(err, true)).
						Msg("runner failed")
				}
			}
		}
	}
}
