package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"rollboard/internal/testdb"
)

func TestMigrateIsIdempotent(t *testing.T) {
	dsn := os.Getenv("ROLLBOARD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ROLLBOARD_TEST_DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	t.Cleanup(pool.Close)
	release, err := testdb.AcquireExclusive(ctx, pool)
	if err != nil {
		t.Fatalf("lock test database: %v", err)
	}
	t.Cleanup(release)

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = 1`).Scan(&count); err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("migration count = %d, want 1", count)
	}
}
