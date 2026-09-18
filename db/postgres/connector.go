package postgres

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUnparsableConfiguration = errors.New("unparsable configuration")
	ErrDatabaseRequired        = errors.New("database required")
	ErrUserRequired            = errors.New("user required")
	ErrPasswordRequired        = errors.New("password required")
	ErrHostRequired            = errors.New("host required")
	ErrPortRequired            = errors.New("port required")
)

type config struct {
	host     string
	port     string
	database string
	user     string
	password string

	disableSSL bool
}

func (c *config) ToDSN() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.user, c.password),
		Host:   net.JoinHostPort(c.host, c.port),
		Path:   "/" + c.database,
	}
	if c.disableSSL {
		u.RawQuery = "sslmode=disable"
	}
	return u.String()
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
		return nil, fmt.Errorf("%w: %w", ErrUnparsableConfiguration, err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return pool, nil
}
