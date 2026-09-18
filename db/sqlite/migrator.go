package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	sqlitemigrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func MigrateUp(ctx context.Context, conn *sql.DB, migrations fs.FS) error {
	sourceDriver, err := iofs.New(migrations, ".")
	if err != nil {
		return fmt.Errorf("new migration source driver: %w", err)
	}

	// Create a new sqlite migration driver instance.
	dbDriver, err := sqlitemigrate.WithInstance(conn, &sqlitemigrate.Config{})
	if err != nil {
		return fmt.Errorf("create migration db driver: %w", err)
	}

	// Create the migrate instance using the source and database drivers.
	m, err := migrate.NewWithInstance(
		"iofs", sourceDriver,
		"sqlite", dbDriver,
	)
	if err != nil {
		return fmt.Errorf("create migration instance: %w", err)
	}

	// Run the up migrations.
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migration up: %w", err)
	}

	return nil
}
