package identity

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"rollboard/internal/storage/postgres"
)

func TestRepositoryRegisterRejectsDuplicateEmail(t *testing.T) {
	dsn := os.Getenv("ROLLBOARD_TEST_DATABASE_URL")
	if dsn == "" { t.Skip("ROLLBOARD_TEST_DATABASE_URL is required") }
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil { t.Fatal(err) }
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool); err != nil { t.Fatal(err) }
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil { t.Fatal(err) }
	repo, err := NewRepository(pool)
	if err != nil { t.Fatal(err) }
	input := RegistrationInput{Email:"author@example.com", DisplayName:"Author", Password:"correct-horse-battery-staple"}
	if _, err := repo.Register(ctx, input); err != nil { t.Fatal(err) }
	if _, err := repo.Register(ctx, input); !errors.Is(err, ErrEmailTaken) { t.Fatalf("err=%v", err) }
}
