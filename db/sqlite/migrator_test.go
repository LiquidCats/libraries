package sqlite_test

import (
	"database/sql"
	"embed"
	"io/fs"
	"testing"

	"github.com/LiquidCats/libraries/db/v2/sqlite"
)

//go:embed testdata/migrations/*.sql
var embedded embed.FS

// migrationsFS returns the embedded migrations rooted at the SQL files
// themselves, which is what MigrateUp's iofs.New(migrations, ".") expects.
func migrationsFS(t *testing.T) fs.FS {
	t.Helper()

	sub, err := fs.Sub(embedded, "testdata/migrations")
	if err != nil {
		t.Fatalf("sub fs: %v", err)
	}

	return sub
}

func TestMigrateUp(t *testing.T) {
	ctx := t.Context()
	conn := connect(t)

	if err := sqlite.MigrateUp(ctx, conn, migrationsFS(t)); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	if _, err := conn.ExecContext(ctx, "INSERT INTO things (id, name) VALUES (1, 'a')"); err != nil {
		t.Fatalf("insert into migrated table: %v", err)
	}

	var version int
	if err := conn.QueryRowContext(ctx, "SELECT version FROM schema_migrations").Scan(&version); err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}

	// Second run hits migrate.ErrNoChange and must be swallowed.
	if err := sqlite.MigrateUp(ctx, conn, migrationsFS(t)); err != nil {
		t.Fatalf("second migrate up: %v", err)
	}

	var rows int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM things").Scan(&rows); err != nil {
		t.Fatalf("count things: %v", err)
	}
	if rows != 1 {
		t.Errorf("rows = %d, want 1 (re-run should be a no-op)", rows)
	}
}

func TestMigrateUpNoMigrations(t *testing.T) {
	conn := connect(t)

	err := sqlite.MigrateUp(t.Context(), conn, fs.FS(embedded))
	if err == nil {
		t.Fatal("want error for an FS with no migrations at its root, got nil")
	}
}

// connect opens an in-memory database. :memory: is per connection, so this
// relies on Connect's maxOpenConns default of 1 — do not override it.
func connect(t *testing.T) *sql.DB {
	t.Helper()

	conn, err := sqlite.Connect(t.Context(), sqlite.WithDatabase(":memory:"))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		if err = conn.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	return conn
}
