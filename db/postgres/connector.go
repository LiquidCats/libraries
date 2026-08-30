package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUnparsableConfigration = errors.New("unparsable configuration")
	ErrDatabaseRequired        = errors.New("database required")
	ErrUserRequired            = errors.New("user required")
	ErrPasswordRequired        = errors.New("password required")
	ErrHostRequired            = errors.New("host required")
	ErrPortRequired            = errors.New("port required")
)

type config struct {
	driver   string
	host     string
	port     string
	database string
	user     string
	password string

	disableSSL bool
}

func (c *config) ToDSN() string {
	dsn := "postgres://" + c.user + ":" + c.password + "@" + c.host + ":" + c.port + "/" + c.database
	if c.disableSSL {
		dsn += "?sslmode=disable"
	}
	return dsn
}

type ConnectionOption func(*config)

func WithDisableSSL() ConnectionOption {
	return func(cfg *config) {
		cfg.disableSSL = true
	}
}

func WithUser(user string) ConnectionOption {
	return func(cfg *config) {
		cfg.user = user
	}
}

func WithPassword(password string) ConnectionOption {
	return func(cfg *config) {
		cfg.password = password
	}
}

func WithHost(host string) ConnectionOption {
	return func(cfg *config) {
		cfg.host = host
	}
}

func WithPort(port string) ConnectionOption {
	return func(cfg *config) {
		cfg.port = port
	}
}

func WithDatabase(database string) ConnectionOption {
	return func(cfg *config) {
		cfg.database = database
	}
}

func Connect(ctx context.Context, opts ...ConnectionOption) (*pgxpool.Pool, error) {
	cfg := &config{
		driver:   "postgres",
		host:     "localhost",
		port:     "5432",
		database: "",
		user:     "",
		password: "",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.database == "" {
		return nil, ErrDatabaseRequired
	}

	if cfg.user == "" {
		return nil, ErrUserRequired
	}

	if cfg.password == "" {
		return nil, ErrPasswordRequired
	}

	if cfg.host == "" {
		return nil, ErrHostRequired
	}

	if cfg.port == "" {
		return nil, ErrPortRequired
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.ToDSN())
	if err != nil {
		return nil, ErrUnparsableConfigration
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return pool, nil
}
