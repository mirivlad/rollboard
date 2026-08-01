package catalog

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"rollboard/internal/game"
	"rollboard/internal/identity"
	"rollboard/internal/storage/postgres"
	"rollboard/internal/testdb"
)

func TestPublishKeepsEarlierVersionImmutable(t *testing.T) {
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
	release, err := testdb.AcquireExclusive(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}
	identities, err := identity.NewRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := identities.Register(ctx, identity.RegistrationInput{
		Email: "author@example.com", DisplayName: "Author", Password: "correct-horse-battery-staple",
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	draft := validDefinition("First title")
	created, err := service.CreateGame(ctx, owner.ID, draft)
	if err != nil {
		t.Fatal(err)
	}
	published, err := service.Publish(ctx, owner.ID, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if published.VersionNumber != 1 {
		t.Fatalf("version = %d, want 1", published.VersionNumber)
	}
	draft.Title = "Changed draft title"
	if err := service.SaveDraft(ctx, owner.ID, created.ID, draft); err != nil {
		t.Fatal(err)
	}
	versionOne, err := service.GetVersion(ctx, created.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if versionOne == nil || versionOne.Definition.Title != "First title" {
		t.Fatalf("version one = %#v, want immutable first title", versionOne)
	}
}

func TestGetVersionByIDReturnsPublishedImmutableVersion(t *testing.T) {
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
	release, err := testdb.AcquireExclusive(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}
	identities, err := identity.NewRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := identities.Register(ctx, identity.RegistrationInput{Email: "version-id@example.com", DisplayName: "Version owner", Password: "correct-horse-battery-staple"})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateGame(ctx, owner.ID, validDefinition("Pinned version"))
	if err != nil {
		t.Fatal(err)
	}
	published, err := service.Publish(ctx, owner.ID, created.ID)
	if err != nil {
		t.Fatal(err)
	}

	version, err := service.GetVersionByID(ctx, published.ID)
	if err != nil {
		t.Fatal(err)
	}
	if version == nil || version.ID != published.ID || version.Definition.Title != "Pinned version" {
		t.Fatalf("GetVersionByID() = %#v, want published immutable version", version)
	}
	missing, err := service.GetVersionByID(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil || missing != nil {
		t.Fatalf("missing GetVersionByID() = %#v, %v; want nil, nil", missing, err)
	}
}

func TestDraftAccessIsOwnerOnlyAndInvalidDraftCannotPublish(t *testing.T) {
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
	release, err := testdb.AcquireExclusive(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}
	identities, err := identity.NewRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := identities.Register(ctx, identity.RegistrationInput{Email: "owner@example.com", DisplayName: "Owner", Password: "correct-horse-battery-staple"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := identities.Register(ctx, identity.RegistrationInput{Email: "other@example.com", DisplayName: "Other", Password: "correct-horse-battery-staple"})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateGame(ctx, owner.ID, validDefinition("Private draft"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetDraft(ctx, other.ID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other owner GetDraft() error = %v, want ErrNotFound", err)
	}
	draft, err := service.GetDraft(ctx, owner.ID, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	definition := draft.Definition
	definition.Board.CellSize = 0
	if err := service.SaveDraft(ctx, owner.ID, created.ID, definition); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(ctx, owner.ID, created.ID); !errors.Is(err, ErrInvalidDefinition) {
		t.Fatalf("Publish() error = %v, want ErrInvalidDefinition", err)
	}
	if version, err := service.GetVersion(ctx, created.ID, 1); err != nil || version != nil {
		t.Fatalf("GetVersion() = %#v, %v; want no published version", version, err)
	}
}

func TestListOwnedGamesAndPublishedVersions(t *testing.T) {
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
	release, err := testdb.AcquireExclusive(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}
	identities, err := identity.NewRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := identities.Register(ctx, identity.RegistrationInput{Email: "catalog-owner@example.com", DisplayName: "Owner", Password: "correct-horse-battery-staple"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := identities.Register(ctx, identity.RegistrationInput{Email: "catalog-other@example.com", DisplayName: "Other", Password: "correct-horse-battery-staple"})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.CreateGame(ctx, owner.ID, validDefinition("First game"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateGame(ctx, owner.ID, validDefinition("Second game"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(ctx, owner.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	otherGame, err := service.CreateGame(ctx, other.ID, validDefinition("Other game"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(ctx, other.ID, otherGame.ID); err != nil {
		t.Fatal(err)
	}

	games, err := service.ListGames(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 2 || games[0].ID != second.ID || games[1].ID != first.ID {
		t.Fatalf("ListGames() = %#v, want only owner games newest first", games)
	}
	versions, err := service.ListVersions(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0].GameID != second.ID || versions[0].Definition.Title != "Second game" {
		t.Fatalf("ListVersions() = %#v, want only owner's published version", versions)
	}
}

func validDefinition(title string) game.GameDefinition {
	return game.GameDefinition{
		Title:   title,
		Version: 1,
		Board: game.Board{
			Width: 96, Height: 96, CellSize: 96,
			Cells: []game.CellDefinition{{
				ID: "start", Title: "Start", Type: "start", Visual: game.CellVisual{}, Fields: map[string]any{},
			}},
		},
		Rules: game.RuleSet{
			Dice:      game.DiceRule{Count: 1, Sides: 6},
			Resources: map[string]game.ResourceRule{},
			CellTypes: map[string]game.CellTypeDef{"start": {Title: "Start", Fields: map[string]game.FieldDef{}}},
		},
	}
}
