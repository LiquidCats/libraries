package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type managerCtxKey string

const (
	txCtxValue      managerCtxKey = "tx-manager:transaction"
	queriesCtxValue managerCtxKey = "tx-manager:queries"
)

// ErrNoTransaction is returned when a context did not come from Begin.
var ErrNoTransaction = errors.New("no transaction in context")

type TxOption func(*sql.TxOptions)

// WithReadOnly begins the transaction in read-only mode. There is no isolation
// level option: the modernc driver ignores sql.TxOptions.Isolation entirely.
func WithReadOnly() TxOption {
	return func(txOpts *sql.TxOptions) {
		txOpts.ReadOnly = true
	}
}

type TxManager struct {
	conn *sql.DB
}

type Queries[T any] interface {
	WithTx(tx *sql.Tx) *T
}

func NewTxManager(conn *sql.DB) *TxManager {
	return &TxManager{
		conn: conn,
	}
}

func (m *TxManager) Begin(ctx context.Context, opts ...TxOption) (context.Context, error) {
	// Isolation is left unset: the modernc driver reads only ReadOnly.
	cfg := sql.TxOptions{
		ReadOnly: false,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	tx, err := m.conn.BeginTx(ctx, &cfg)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}

	txCtx := context.WithValue(ctx, txCtxValue, tx)

	return txCtx, nil
}

func (m *TxManager) Commit(ctx context.Context) error {
	tx, ok := ctx.Value(txCtxValue).(*sql.Tx)
	if !ok {
		return ErrNoTransaction
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (m *TxManager) Rollback(ctx context.Context) error {
	tx, ok := ctx.Value(txCtxValue).(*sql.Tx)
	if !ok {
		return ErrNoTransaction
	}

	if err := tx.Rollback(); err != nil {
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

	tx, ok := txCtx.Value(txCtxValue).(*sql.Tx)
	if !ok {
		return ErrNoTransaction
	}

	txQueries := m.queries.WithTx(tx)

	txCtx = context.WithValue(txCtx, queriesCtxValue, txQueries)

	if cbErr := callback(txCtx); cbErr != nil {
		if rbErr := m.manager.Rollback(txCtx); rbErr != nil {
			return fmt.Errorf("rollback query tx: %w (callback: %w)", rbErr, cbErr)
		}
		return fmt.Errorf("query tx callback: %w", cbErr)
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
