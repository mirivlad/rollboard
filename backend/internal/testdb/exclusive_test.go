package testdb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAcquireExclusiveSerializesTestDatabaseAccess(t *testing.T) {
	dsn := os.Getenv("ROLLBOARD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ROLLBOARD_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	releaseFirst, err := AcquireExclusive(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFirst()

	acquiredSecond := make(chan func(), 1)
	errs := make(chan error, 1)
	go func() {
		release, err := AcquireExclusive(ctx, pool)
		if err != nil {
			errs <- err
			return
		}
		acquiredSecond <- release
	}()
	select {
	case release := <-acquiredSecond:
		release()
		t.Fatal("second caller acquired lock before first caller released it")
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(100 * time.Millisecond):
	}
	releaseFirst()
	releaseFirst = func() {}
	select {
	case release := <-acquiredSecond:
		release()
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("second caller did not acquire lock after release")
	}
}
