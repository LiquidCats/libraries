package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type managerCtxKey string

const (
	txCtxValue      managerCtxKey = "tx-manager:transaction"
	queriesCtxValue managerCtxKey = "tx-manager:queries"
)

// ErrNoTransaction is returned when a context did not come from Begin.
var ErrNoTransaction = errors.New("no transaction in context")

// TxIsoLevel is the transaction isolation level (serializable, repeatable read,
// read committed or read uncommitted).
type TxIsoLevel string

// Transaction isolation levels.
const (
	Serializable    TxIsoLevel = "serializable"
	RepeatableRead  TxIsoLevel = "repeatable read"
	ReadCommitted   TxIsoLevel = "read committed"
	ReadUncommitted TxIsoLevel = "read uncommitted"
)

type TxOption func(*pgx.TxOptions)

func WithIsoLevel(lvl TxIsoLevel) TxOption {
	return func(txOpts *pgx.TxOptions) {
		txOpts.IsoLevel = pgx.TxIsoLevel(lvl)
	}
}

type TxManager struct {
	conn *pgxpool.Pool
}

type Queries[T any] interface {
	WithTx(tx pgx.Tx) *T
}

func NewTxManager(conn *pgxpool.Pool) *TxManager {
	return &TxManager{
		conn: conn,
	}
}

func (m *TxManager) Begin(ctx context.Context, opts ...TxOption) (context.Context, error) {
	cfg := pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadWrite,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	tx, err := m.conn.BeginTx(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}

	txCtx := context.WithValue(ctx, txCtxValue, tx)

	return txCtx, nil
}

func (m *TxManager) Commit(ctx context.Context) error {
	tx, ok := ctx.Value(txCtxValue).(pgx.Tx)
	if !ok {
		return ErrNoTransaction
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (m *TxManager) Rollback(ctx context.Context) error {
	tx, ok := ctx.Value(txCtxValue).(pgx.Tx)
	if !ok {
		return ErrNoTransaction
	}

	if err := tx.Rollback(ctx); err != nil {
		return fmt.Errorf("rollback tx: %w", err)
	}

	return nil
}

type QueriesTxManager[T any] struct {
	queries Queries[T]
	manager *TxManager
}

func NewQueriesTxManager[T any](manager *TxManager, queries Queries[T]) *QueriesTxManager[T] {
	return &QueriesTxManager[T]{
		manager: manager,
		queries: queries,
	}
}

type TxCallback func(ctx context.Context) error

func (m *QueriesTxManager[T]) Transactional(ctx context.Context, callback TxCallback, opts ...TxOption) error {
	txCtx, err := m.manager.Begin(ctx, opts...)
	if err != nil {
		return fmt.Errorf("begin query tx: %w", err)
	}

	tx, ok := txCtx.Value(txCtxValue).(pgx.Tx)
	if !ok {
		return ErrNoTransaction
	}

	txQueries := m.queries.WithTx(tx)

	txCtx = context.WithValue(txCtx, queriesCtxValue, txQueries)

	if err = callback(txCtx); err != nil {
		if err = m.manager.Rollback(txCtx); err != nil {
			return fmt.Errorf("rollback query tx: %w", err)
		}
		return fmt.Errorf("query tx callback: %w", err)
	}

	if err = m.manager.Commit(txCtx); err != nil {
		return fmt.Errorf("commit query tx: %w", err)
	}

	return nil
}

// GetQueries returns the transaction-scoped queries stored by Transactional,
// falling back to the manager's own queries outside a transaction. It returns
// nil when neither is a *T.
func (m *QueriesTxManager[T]) GetQueries(ctx context.Context) *T {
	if queries, ok := ctx.Value(queriesCtxValue).(*T); ok && queries != nil {
		return queries
	}

	if queries, ok := any(m.queries).(*T); ok {
		return queries
	}

	return nil
}
