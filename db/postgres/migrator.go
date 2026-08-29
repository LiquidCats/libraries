package postgres

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rotisserie/eris"
)

func MigrateUp(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection from pool: %w", err)
	}
	// Only the connection's config is needed below; without this the
	// connection never returns to the pool.
	defer conn.Release()

	sourceDriver, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("new migration source driver: %w", err)
	}

	dbConn := stdlib.OpenDB(*conn.Conn().Config())
	defer func() {
		_ = dbConn.Close()
	}()

	// Create a new pgx migration driver instance.
	dbDriver, err := pgxmigrate.WithInstance(dbConn, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("create migration db driver: %w", err)
	}

	// Create the migrate instance using the source and database drivers.
	m, err := migrate.NewWithInstance(
		"iofs", sourceDriver,
		"pgx", dbDriver,
	)
	if err != nil {
		return fmt.Errorf("create migration instance: %w", err)
	}

	// Run the up migrations.
	if err = m.Up(); err != nil && !eris.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migration up: %w", err)
	}

	return nil
}
