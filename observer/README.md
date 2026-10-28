# observer

Event subject/observer fan-out with a fixed number of workers.

```sh
go get github.com/LiquidCats/libraries/observer
```

```go
subject := observer.NewSubject(4) // 4 workers

subject.Register("block.new", indexer) // indexer implements observer.Observer
go subject.Run(ctx)

subject.Notify(&observer.Event{Name: "block.new", Data: block})
```

`Observer` is `Update(ctx context.Context, data any) error`.

- `Register` is safe to call while the subject is running. Several observers may
  listen to one event; a worker runs them concurrently.
- `Notify` blocks until a worker picks the event up. The channel is unbuffered,
  so a slow observer applies backpressure to the producer.
- If any observer returns an error, `Run` stops and returns it.
- Events with no registered observer are dropped.
