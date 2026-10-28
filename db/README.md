# db

Postgres and SQLite transaction managers plus `io/fs` migration runners.

```sh
go get github.com/LiquidCats/libraries/db/v2
```

Two independent driver packages, `postgres` (on `pgx`) and `sqlite` (on
`database/sql` + `modernc.org/sqlite`, pure Go, no cgo), each shipping a
`TxManager` and a `MigrateUp`. They don't share code: `pgx.Tx.Commit(ctx)` and
`sql.Tx.Commit()` have different signatures.

## Connecting

`Connect` builds the pool/handle from functional options and validates them.

- postgres: `WithHost`, `WithPort`, `WithUser`, `WithPassword`, `WithDatabase`,
  `WithDisableSSL`. Missing values return `ErrHostRequired`, `ErrPortRequired`,
  `ErrUserRequired`, `ErrPasswordRequired` or `ErrDatabaseRequired`.
- sqlite: `WithDatabase`, `WithMaxOpenConns`, `WithMaxIdleConns`,
  `WithConnMaxIdleTime`, `WithConnMaxLifetime`.

## Transactions

`TxManager` carries a transaction on the context; `QueriesTxManager[T]` wraps a
generated `Queries` type so a callback runs against the transaction and is
rolled back on any error.

```go
pool, err := postgres.Connect(ctx,
	postgres.WithHost("localhost"),
	postgres.WithPort("5432"),
	postgres.WithDatabase("app"),
	postgres.WithUser("app"),
	postgres.WithPassword("secret"),
)

mgr := postgres.NewQueriesTxManager(postgres.NewTxManager(pool), sqlcQueries)

err = mgr.Transactional(ctx, func(txCtx context.Context) error {
	q := mgr.GetQueries(txCtx) // transaction-scoped
	return q.InsertThing(txCtx, thing)
}, postgres.WithIsoLevel(pgx.Serializable))
```

`Commit`, `Rollback` and `Transactional` return `ErrNoTransaction` if the
context did not come from `Begin`.

## Migrations

`MigrateUp` runs from an `fs.FS` rooted at the migration files themselves —
`fs.Sub` a `migrations/` directory first, `MigrateUp` does not do that for you:

```go
//go:embed migrations/*.sql
var raw embed.FS

migrations, err := fs.Sub(raw, "migrations")
if err != nil {
	// ...
}

err = postgres.MigrateUp(ctx, pool, migrations)
```

## sqlite notes

Same shape as postgres. There is no `WithIsoLevel`: the driver ignores
`sql.TxOptions.Isolation`, so the option would silently do nothing.
`WithReadOnly` is honored — it picks a non-locking `BEGIN`, though SQLite still
won't reject a write inside it.

Pragmas and other DSN knobs ride through `WithDatabase`:

```go
conn, err := sqlite.Connect(ctx, sqlite.WithDatabase(
	"file:app.db?_txlock=immediate&_busy_timeout=5000&_pragma=journal_mode(WAL)&_fk=1",
))
```

`:memory:` databases are per connection, so concurrent access needs
`WithMaxOpenConns(1)` (the default) or one connection's changes won't be visible
to the others.
