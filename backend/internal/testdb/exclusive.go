// Package testdb contains helpers for integration tests sharing one database.
package testdb

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

const exclusiveLockKey int64 = 7_291_274_183

// AcquireExclusive serializes destructive integration-test setup against the
// shared ROLLBOARD_TEST_DATABASE_URL database. The returned function releases
// both the advisory lock and its dedicated connection.
func AcquireExclusive(ctx context.Context, pool *pgxpool.Pool) (func(), error) {
	if pool == nil {
		return nil, fmt.Errorf("test database pool is required")
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire test database connection: %w", err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, exclusiveLockKey); err != nil {
		conn.Release()
		return nil, fmt.Errorf("lock shared test database: %w", err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, exclusiveLockKey)
			conn.Release()
		})
	}, nil
}
