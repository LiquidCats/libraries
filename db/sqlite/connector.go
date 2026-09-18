package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrDatabaseRequired = errors.New("database required")
)

type config struct {
	database        string
	maxOpenConns    int
	maxIdleConns    int
	connMaxIdleTime time.Duration
	connMaxLifetime time.Duration
}

type ConnectionOption func(*config)

func WithDatabase(database string) ConnectionOption {
	return func(cfg *config) {
		cfg.database = database
	}
}

func WithMaxOpenConns(maxOpenConns int) ConnectionOption {
	return func(cfg *config) {
		cfg.maxOpenConns = maxOpenConns
	}
}

func WithMaxIdleConns(maxIdleConns int) ConnectionOption {
	return func(cfg *config) {
		cfg.maxIdleConns = maxIdleConns
	}
}

func WithConnMaxIdleTime(connMaxIdleTime time.Duration) ConnectionOption {
	return func(cfg *config) {
		cfg.connMaxIdleTime = connMaxIdleTime
	}
}

func WithConnMaxLifetime(connMaxLifetime time.Duration) ConnectionOption {
	return func(cfg *config) {
		cfg.connMaxLifetime = connMaxLifetime
	}
}

const (
	DefaultMaxOpenConns    = 1
	DefaultMaxIdleConns    = 1
	DefaultConnMaxIdleTime = 5 * time.Minute
	DefaultConnMaxLifetime = 15 * time.Minute
)

func Connect(ctx context.Context, opts ...ConnectionOption) (*sql.DB, error) {
	cfg := &config{
		database:        "db.sqlite",
		maxOpenConns:    DefaultMaxOpenConns,
		maxIdleConns:    DefaultMaxIdleConns,
		connMaxIdleTime: DefaultConnMaxIdleTime,
		connMaxLifetime: DefaultConnMaxLifetime,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.database == "" {
		return nil, ErrDatabaseRequired
	}

	conn, err := sql.Open("sqlite", cfg.database)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	conn.SetMaxOpenConns(cfg.maxOpenConns)
	conn.SetMaxIdleConns(cfg.maxIdleConns)
	conn.SetConnMaxIdleTime(cfg.connMaxIdleTime)
	conn.SetConnMaxLifetime(cfg.connMaxLifetime)

	// sql.Open never touches the file; without this a bad path or permission
	// error only surfaces at some unrelated later query.
	if err = conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return conn, nil
}
