package identity

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"rollboard/internal/storage/postgres"
)

func TestRepositoryRegisterRejectsDuplicateEmail(t *testing.T) {
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
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	input := RegistrationInput{Email: "author@example.com", DisplayName: "Author", Password: "correct-horse-battery-staple"}
	if _, err := repo.Register(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Register(ctx, input); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("err=%v", err)
	}
}

func TestRepositoryCreatesAndResolvesGuestSession(t *testing.T) {
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
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool)
	if err != nil {
		t.Fatal(err)
	}

	guest, err := repo.CreateGuest(ctx, "  Guest player  ")
	if err != nil {
		t.Fatal(err)
	}
	if guest.ID == "" || guest.DisplayName != "Guest player" {
		t.Fatalf("guest = %#v, want persisted normalized guest", guest)
	}
	session, token, err := repo.CreateGuestSession(ctx, guest.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || session.TokenDigest == nil {
		t.Fatalf("session must return raw token once and persist only digest: %#v", session)
	}
	actor, err := repo.LookupSession(ctx, token, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if actor == nil || actor.Guest == nil || actor.Guest.ID != guest.ID || actor.User != nil {
		t.Fatalf("actor = %#v, want the guest actor", actor)
	}
	missing, err := repo.LookupSession(ctx, "not-a-real-token", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("unknown token resolved to %#v", missing)
	}
	if err := repo.DeleteSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	revoked, err := repo.LookupSession(ctx, token, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if revoked != nil {
		t.Fatalf("revoked token resolved to %#v", revoked)
	}
}

func TestRepositoryAuthenticatesAndResolvesAccountSession(t *testing.T) {
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
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := repo.Register(ctx, RegistrationInput{Email: "author@example.com", DisplayName: "Author", Password: "correct-horse-battery-staple"})
	if err != nil {
		t.Fatal(err)
	}
	authenticated, err := repo.Authenticate(ctx, "AUTHOR@example.com", "correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if authenticated.ID != registered.ID || authenticated.PasswordHash != "" {
		t.Fatalf("authenticated = %#v, want persisted user without password hash", authenticated)
	}
	if _, err := repo.Authenticate(ctx, "author@example.com", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v, want ErrInvalidCredentials", err)
	}
	_, token, err := repo.CreateUserSession(ctx, authenticated.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	actor, err := repo.LookupSession(ctx, token, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if actor == nil || actor.User == nil || actor.User.ID != registered.ID || actor.Guest != nil {
		t.Fatalf("actor = %#v, want the user actor", actor)
	}
}

func TestRepositoryClaimsGuestOnlyOnce(t *testing.T) {
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
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	guest, err := repo.CreateGuest(ctx, "Guest player")
	if err != nil {
		t.Fatal(err)
	}
	user, err := repo.Register(ctx, RegistrationInput{Email: "author@example.com", DisplayName: "Author", Password: "correct-horse-battery-staple"})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := repo.ClaimGuest(ctx, guest.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ClaimedByUserID == nil || *claimed.ClaimedByUserID != user.ID || claimed.ClaimedAt == nil {
		t.Fatalf("claimed guest = %#v, want claim metadata", claimed)
	}
	if _, err := repo.ClaimGuest(ctx, guest.ID, user.ID); !errors.Is(err, ErrGuestClaimed) {
		t.Fatalf("second claim error = %v, want ErrGuestClaimed", err)
	}
}
