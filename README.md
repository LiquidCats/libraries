# libraries

Shared Go libraries used across LiquidCats services. Each directory is an
independent Go module with its own `go.mod` — import them individually, there is
no umbrella module.

| Module | Import path | What it does |
|---|---|---|
| [`graceful`](graceful) | `github.com/LiquidCats/libraries/graceful` | Run HTTP/gRPC servers, cron, tickers and channel workers under one context, shut them all down on signal. |
| [`workers`](workers) | `github.com/LiquidCats/libraries/workers` | Generic autoscaling worker pool. |
| [`observer`](observer) | `github.com/LiquidCats/libraries/observer` | Event subject/observer fan-out with a fixed worker count. |
| [`evm-lib`](evm-lib) | `github.com/LiquidCats/libraries/evm-lib` | Decode EVM transaction calldata into value transfers, no ABI or node required. |
| [`utxo-lib`](utxo-lib) | `github.com/LiquidCats/libraries/utxo-lib` | Bitcoin network parameters (magic bytes, address prefixes). |

```sh
go get github.com/LiquidCats/libraries/graceful
```

## graceful

`WaitContext` runs every `Runner` in an errgroup and returns once one fails or a
signal arrives. `Signals` is itself a Runner, so shutdown is just another
participant.

```go
err := graceful.WaitContext(ctx,
    graceful.Signals,
    graceful.Server(router, graceful.WithPort("8080")),
    graceful.GRPCRunner(attacher, graceful.WithGRPCPort("9090")),
    graceful.ScheduleRunner(tasks...),
    graceful.Ticker(time.Minute, refresh),
    graceful.Worker(eventsCh, handleEvent),
)
if err != nil && !eris.Is(err, graceful.ErrShutdownBySignal) {
    log.Fatal().Err(err).Msg("shutdown")
}
```

Runners:

- `Server(http.Handler, ...ServerOpt)` — `WithPort`, `WithReadTimeout`, `WithWriteTimeout`.
- `GRPCRunner(GRPCAttacher, ...GRPCOpt)` — `WithGRPCPort`, `WithConnectionTimeout`. The attacher registers your services on the `*grpc.Server`.
- `ScheduleRunner(...Task)` — cron entries, backed by `robfig/cron/v3`.
- `Ticker(interval, Runner, ...TickerOpt)` — `WithTickerLogger`.
- `Worker[T](<-chan T, WorkerHandler[T], ...WorkerOpt)` — `WithWorkerLogger`.

A ticker or worker whose handler returns an error wraps it in `ErrTickerFailure`
/ `ErrWorkerFailure`, which fails the group and unwinds everything else.

## workers

Generic pool that grows and shrinks between `min` and `max` workers based on
queue load, with a cooldown between scaling decisions.

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

go pool.Start(ctx)          // blocks until ctx is done
pool.Submit(ctx, job)       // ErrPoolClosed after Close
```

Defaults: min 3, max 5, polling 300ms, scale up above 0.7 load, down below 0.3,
5s cooldown. `New` validates the options and returns `ErrInvalidMinWorkerCount`
and friends rather than panicking. `ActiveWorkers`, `BusyWorkers` and
`QueueSize` expose the current state for metrics.

## observer

```go
subject := observer.NewSubject(4)
subject.Register("block.new", indexer)
go subject.Run(ctx)

subject.Notify(&observer.Event{Name: "block.new", Data: block})
```

`Register` is safe to call while running. `Notify` blocks until a worker picks
the event up — the channel is unbuffered, so a slow observer applies
backpressure to the producer.

## evm-lib

Decodes raw calldata into `[]Transfer` without touching a node or loading ABIs.
Each codec claims a set of 4-byte selectors; the parser walks them in order and
uses the first that matches.

```go
p := parser.New(
    codecs.NewWETHDecoder(),
    codecs.NewUniswapV2RouterDecoder(),
    codecs.NewUniswapV3PaymentsDecoder(),
    codecs.NewAaveWETHGatewayDecoder(),
    codecs.NewDisperseDecoder(),
    codecs.NewGnosisSafeDecoder(),
    codecs.NewGnosisMultiSendDecoder(),
    codecs.NewHeuristicDecoder(),   // last: guesses from calldata shape
)

out, err := p.Parse(types.RawInputData("a9059cbb..."))  // hex, no 0x prefix
```

Every `Transfer` carries a `Confidence`, since a heuristic match is weaker than a
known selector. `From` is zero when calldata alone cannot identify the source —
`Parse` has no caller/callee context.

`NewMulticallDecoder` and `NewMulticall3ValueDecoder` take a `SubParser` so
batched inner calls get decoded recursively; pass the `*Parser` itself.
`codecs.NewChain` composes decoders into one, and `codecs/abi.go` holds the
ABI word readers (`ReadAddress`, `ReadUint256`, `ReadDynamicBytes`, …) if you
are writing your own codec.

## utxo-lib

```go
params := netparams.BitcoinMainNet
params.IsBech32SegwitPrefix("bc1")
```

`BitcoinMainNet`, `BitcoinTestNet3`, `BitcoinTestNet4`, `BitcoinRegNet`.

## Development

Modules are released independently: a tag must be prefixed with the module
directory (`graceful/v1.2.0`) for `go get` to resolve it. They also pin
different Go versions — check the module's `go.mod` before using a new language
feature.

The root `Makefile` runs across every module. Docker is the only requirement —
the Go toolchain and linter come from pinned images, so nothing has to be
installed on the host and nothing depends on the host's Go version.

```sh
make            # test + lint, all modules
make test
make lint
make lint-fix   # or: make fmt
make bench
make tidy       # rewrites go.mod/go.sum
make shell      # interactive container in the same sandbox

make test MODULES=graceful   # narrow to one module
```

Containers run unprivileged as the calling user, with a read-only root
filesystem, all capabilities dropped, and `--network=none`. Only `make deps`
and `make tidy` get network access; `make deps` warms the module cache
(`~/.cache/liquidcats-libraries`) so everything else can stay offline, and runs
automatically when a `go.mod` or `go.sum` changes. `make clean` drops the cache.

Linting uses the shared `.golangci.yaml` at the repo root. CI runs test and
lint for every module on push to `main` and on pull requests, in a matrix job
per module.
