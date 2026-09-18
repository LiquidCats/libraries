package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/LiquidCats/libraries/db/v2/sqlite"
)

// queries stands in for a generated sqlc Queries type: the zero-tx value talks
// to the pool, and WithTx returns a copy bound to the transaction.
type queries struct {
	db *sql.DB
	tx *sql.Tx
}

func (q *queries) WithTx(tx *sql.Tx) *queries {
	return &queries{db: q.db, tx: tx}
}

func (q *queries) insert(ctx context.Context, name string) error {
	const stmt = "INSERT INTO things (name) VALUES (?)"

	var err error
	if q.tx != nil {
		_, err = q.tx.ExecContext(ctx, stmt, name)
	} else {
		_, err = q.db.ExecContext(ctx, stmt, name)
	}
	if err != nil {
		return err
	}

	return nil
}

// setup opens an in-memory database with the things table already created.
func setup(t *testing.T) (*sql.DB, *sqlite.QueriesTxManager[queries]) {
	t.Helper()

	conn := connect(t)
	if _, err := conn.ExecContext(t.Context(),
		"CREATE TABLE things (id INTEGER PRIMARY KEY, name TEXT NOT NULL)",
	); err != nil {
		t.Fatalf("create table: %v", err)
	}

	mgr := sqlite.NewQueriesTxManager[queries](
		sqlite.NewTxManager(conn),
		&queries{db: conn},
	)

	return conn, mgr
}

func countThings(t *testing.T, conn *sql.DB) int {
	t.Helper()

	var n int
	if err := conn.QueryRowContext(t.Context(), "SELECT count(*) FROM things").Scan(&n); err != nil {
		t.Fatalf("count things: %v", err)
	}

	return n
}

func TestTransactional(t *testing.T) {
	errCallback := errors.New("callback failed")

	tests := []struct {
		name        string
		callbackErr error
		wantRows    int
	}{
		{name: "commit persists the row", callbackErr: nil, wantRows: 1},
		{name: "callback error rolls back", callbackErr: errCallback, wantRows: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, mgr := setup(t)

			err := mgr.Transactional(t.Context(), func(txCtx context.Context) error {
				if err := mgr.GetQueries(txCtx).insert(txCtx, "thing"); err != nil {
					return err
				}
				return tt.callbackErr
			})

			if !errors.Is(err, tt.callbackErr) {
				t.Fatalf("Transactional err = %v, want %v", err, tt.callbackErr)
			}

			if got := countThings(t, conn); got != tt.wantRows {
				t.Errorf("rows = %d, want %d", got, tt.wantRows)
			}
		})
	}
}

func TestGetQueries(t *testing.T) {
	_, mgr := setup(t)

	fallback := mgr.GetQueries(t.Context())
	if fallback == nil {
		t.Fatal("GetQueries outside a transaction = nil, want the manager's own queries")
	}
	if fallback.tx != nil {
		t.Error("GetQueries outside a transaction returned a tx-bound value")
	}

	err := mgr.Transactional(t.Context(), func(txCtx context.Context) error {
		scoped := mgr.GetQueries(txCtx)
		if scoped == nil {
			t.Error("GetQueries inside a transaction = nil")
			return nil
		}
		if scoped.tx == nil {
			t.Error("GetQueries inside a transaction returned the fallback, want tx-bound")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Transactional: %v", err)
	}
}

func TestNoTransactionInContext(t *testing.T) {
	mgr := sqlite.NewTxManager(connect(t))

	tests := []struct {
		name string
		op   func(context.Context) error
	}{
		{name: "commit", op: mgr.Commit},
		{name: "rollback", op: mgr.Rollback},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.op(t.Context()); !errors.Is(err, sqlite.ErrNoTransaction) {
				t.Errorf("err = %v, want ErrNoTransaction", err)
			}
		})
	}
}

// TestWithReadOnlyOption only checks that the option is accepted end to end:
// the modernc driver honors sql.TxOptions.ReadOnly by picking a non-locking
// BEGIN, not by rejecting writes, so a rejected-write assertion would be
// testing behavior the driver doesn't provide.
func TestWithReadOnlyOption(t *testing.T) {
	conn, mgr := setup(t)

	err := mgr.Transactional(t.Context(), func(txCtx context.Context) error {
		return mgr.GetQueries(txCtx).insert(txCtx, "thing")
	}, sqlite.WithReadOnly())
	if err != nil {
		t.Fatalf("Transactional with WithReadOnly: %v", err)
	}

	if got := countThings(t, conn); got != 1 {
		t.Errorf("rows = %d, want 1", got)
	}
}
