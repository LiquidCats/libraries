# workers

Generic worker pool that grows and shrinks between a minimum and maximum worker
count based on queue load, with a cooldown between scaling decisions.

```sh
go get github.com/LiquidCats/libraries/workers
```

```go
pool, err := workers.New(handle,
	workers.WithMinWorkerCount[Job](4),
	workers.WithMaxWorkerCount[Job](32),
	workers.WithMaxLoad[Job](0.8),
)
if err != nil {
	return err
}
defer pool.Close()

go pool.Start(ctx)     // blocks until ctx is done
pool.Submit(ctx, job)  // ErrPoolClosed after Close
```

`handle` is `func(ctx context.Context, v T) error`.

## Options

| Option | Default |
|--------|---------|
| `WithMinWorkerCount` | 3 |
| `WithMaxWorkerCount` | 5 |
| `WithPollingFrequency` | 300ms |
| `WithMaxLoad` (scale up above) | 0.7 |
| `WithMinLoad` (scale down below) | 0.3 |
| `WithScalingCooldown` | 5s |

`New` validates the options and returns `ErrInvalidMinWorkerCount`,
`ErrInvalidMaxWorkerCount`, `ErrInvalidMinMaxWorkerCount`,
`ErrInvalidPollingFrequency`, `ErrInvalidMaxLoad` or `ErrInvalidMinLoad` instead
of panicking.

## Metrics

`ActiveWorkers()`, `BusyWorkers()` and `QueueSize()` expose the current state.
