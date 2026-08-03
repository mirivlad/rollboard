package testdb

import (
	"context"
	"os"
	"sync"
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

	// Deferred calls run last-in-first-out, and the order matters. Cancelling
	// the contender unblocks its pg_advisory_lock so the connection returns to
	// the pool; waiting for the goroutine guarantees that release has happened;
	// only then can pool.Close() finish. Closing a pool that still has a
	// connection blocked inside a query hangs forever, which is exactly how
	// this test used to wedge the whole suite when it failed.
	defer pool.Close()

	var contender sync.WaitGroup
	defer contender.Wait()

	contenderCtx, cancelContender := context.WithCancel(ctx)
	defer cancelContender()

	releaseFirst, err := AcquireExclusive(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	firstReleased := false
	defer func() {
		if !firstReleased {
			releaseFirst()
		}
	}()

	acquiredSecond := make(chan func(), 1)
	errs := make(chan error, 1)
	contender.Add(1)
	go func() {
		defer contender.Done()
		release, err := AcquireExclusive(contenderCtx, pool)
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
	firstReleased = true

	// Generous, because the shared test database may still be serving another
	// package's exclusive section when the suite runs packages in parallel.
	select {
	case release := <-acquiredSecond:
		release()
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(30 * time.Second):
		t.Fatal("second caller did not acquire lock after release")
	}
}
