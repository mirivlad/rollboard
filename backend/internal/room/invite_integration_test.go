package room

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"rollboard/internal/catalog"
	"rollboard/internal/identity"
	"rollboard/internal/storage/postgres"
	"rollboard/internal/testdb"
)

// inviteFixture gives each test a host, a published version and a room.
func inviteFixture(t *testing.T) (*Service, *pgxpool.Pool, identity.Actor, identity.Actor, Room) {
	t.Helper()
	dsn := os.Getenv("ROLLBOARD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ROLLBOARD_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	release, err := testdb.AcquireExclusive(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
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
	hostUser := registerRoomUser(t, ctx, identities, "invite-host@example.com", "Host")
	guestUser := registerRoomUser(t, ctx, identities, "invite-guest@example.com", "Guest")
	host := identity.Actor{User: &hostUser}
	joiner := identity.Actor{User: &guestUser}

	catalogService, err := catalog.NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	created, err := catalogService.CreateGame(ctx, hostUser.ID, roomDefinition("Invite game"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalogService.Publish(ctx, hostUser.ID, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(pool, catalogService)
	if err != nil {
		t.Fatal(err)
	}
	room, err := service.Create(ctx, host, version.ID, CreateInput{Title: "Invite room", MaxPlayers: 4})
	if err != nil {
		t.Fatal(err)
	}
	return service, pool, host, joiner, room
}

func TestInviteLinkLetsSomebodyJoinWithoutTheRoomID(t *testing.T) {
	service, _, host, joiner, room := inviteFixture(t)
	ctx := context.Background()

	token, err := service.InviteToken(ctx, host, room.ID)
	if err != nil {
		t.Fatalf("InviteToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("room was created without an invite token")
	}

	invite, err := service.ResolveInvite(ctx, token)
	if err != nil {
		t.Fatalf("ResolveInvite() error = %v", err)
	}
	if invite.RoomID != room.ID || invite.Title != "Invite room" {
		t.Fatalf("invite = %#v, want the created room", invite)
	}
	if invite.GameTitle != "Invite game" {
		t.Fatalf("invite.GameTitle = %q, want the published game title", invite.GameTitle)
	}
	if !invite.Joinable {
		t.Fatal("a fresh lobby with room to spare reported itself unjoinable")
	}

	joinedRoomID, err := service.JoinByInvite(ctx, joiner, token)
	if err != nil {
		t.Fatalf("JoinByInvite() error = %v", err)
	}
	if joinedRoomID != room.ID {
		t.Fatalf("joined %q, want %q", joinedRoomID, room.ID)
	}

	loaded, err := service.Get(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Members) != 2 {
		t.Fatalf("members = %d, want host plus the invited player", len(loaded.Members))
	}
}

// Following your own link again should put you back in the room rather than
// failing, which is what a person clicking a bookmark expects.
func TestFollowingAnInviteTwiceIsNotAnError(t *testing.T) {
	service, _, host, joiner, room := inviteFixture(t)
	ctx := context.Background()
	token, err := service.InviteToken(ctx, host, room.ID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.JoinByInvite(ctx, joiner, token); err != nil {
		t.Fatalf("first join failed: %v", err)
	}
	if _, err := service.JoinByInvite(ctx, joiner, token); err != nil {
		t.Fatalf("second join failed: %v", err)
	}

	loaded, err := service.Get(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Members) != 2 {
		t.Fatalf("members = %d, want no duplicate membership", len(loaded.Members))
	}
}

func TestUnknownInviteTokenIsNotFound(t *testing.T) {
	service, _, _, joiner, _ := inviteFixture(t)
	ctx := context.Background()

	if _, err := service.ResolveInvite(ctx, "definitely-not-a-real-token"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ResolveInvite() error = %v, want ErrNotFound", err)
	}
	if _, err := service.JoinByInvite(ctx, joiner, "definitely-not-a-real-token"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("JoinByInvite() error = %v, want ErrNotFound", err)
	}
	if _, err := service.ResolveInvite(ctx, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ResolveInvite(\"\") error = %v, want ErrNotFound", err)
	}
}

// Rotating is the only way to withdraw a link that spread too far, so the old
// token must genuinely stop working.
func TestRotatingAnInviteInvalidatesTheOldLink(t *testing.T) {
	service, _, host, joiner, room := inviteFixture(t)
	ctx := context.Background()

	original, err := service.InviteToken(ctx, host, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := service.RotateInvite(ctx, host, room.ID)
	if err != nil {
		t.Fatalf("RotateInvite() error = %v", err)
	}
	if rotated == original {
		t.Fatal("rotation returned the same token")
	}

	if _, err := service.ResolveInvite(ctx, original); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the withdrawn token still resolves: %v", err)
	}
	if _, err := service.JoinByInvite(ctx, joiner, original); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the withdrawn token still lets somebody join: %v", err)
	}
	if _, err := service.JoinByInvite(ctx, joiner, rotated); err != nil {
		t.Fatalf("the new token does not work: %v", err)
	}
}

func TestOnlyTheHostCanReadOrRotateAnInvite(t *testing.T) {
	service, _, host, other, room := inviteFixture(t)
	ctx := context.Background()

	if _, err := service.InviteToken(ctx, other, room.ID); !errors.Is(err, ErrNotHost) {
		t.Fatalf("InviteToken() for a non-host = %v, want ErrNotHost", err)
	}
	// Rotation answers NotFound rather than NotHost so the endpoint cannot be
	// used to confirm that a room ID exists.
	if _, err := service.RotateInvite(ctx, other, room.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RotateInvite() for a non-host = %v, want ErrNotFound", err)
	}

	guestActor := identity.Actor{Guest: &identity.Guest{ID: "11111111-1111-1111-1111-111111111111", DisplayName: "Guest"}}
	if _, err := service.InviteToken(ctx, guestActor, room.ID); !errors.Is(err, ErrAccountRequired) {
		t.Fatalf("InviteToken() for a guest = %v, want ErrAccountRequired", err)
	}

	// The host is still able to do both.
	if _, err := service.InviteToken(ctx, host, room.ID); err != nil {
		t.Fatalf("host lost access to their own invite: %v", err)
	}
}

func TestInviteReportsAFullRoomAsNotJoinable(t *testing.T) {
	service, pool, host, joiner, _ := inviteFixture(t)
	ctx := context.Background()

	catalogService, err := catalog.NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	created, err := catalogService.CreateGame(ctx, host.User.ID, roomDefinition("Tiny game"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalogService.Publish(ctx, host.User.ID, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	small, err := service.Create(ctx, host, version.ID, CreateInput{Title: "Two seats", MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	token, err := service.InviteToken(ctx, host, small.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.JoinByInvite(ctx, joiner, token); err != nil {
		t.Fatal(err)
	}

	invite, err := service.ResolveInvite(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if invite.Joinable {
		t.Fatalf("invite = %#v, want joinable false once the room is full", invite)
	}
}
