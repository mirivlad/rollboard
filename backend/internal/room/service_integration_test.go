package room

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"rollboard/internal/catalog"
	"rollboard/internal/game"
	"rollboard/internal/identity"
	"rollboard/internal/storage/postgres"
	"rollboard/internal/testdb"
)

func TestRoomCreateJoinAndModerationAuthorization(t *testing.T) {
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
	hostUser := registerRoomUser(t, ctx, identities, "host@example.com", "Host")
	memberUser := registerRoomUser(t, ctx, identities, "member@example.com", "Member")
	secondMember := registerRoomUser(t, ctx, identities, "second@example.com", "Second")

	catalogService, err := catalog.NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	createdGame, err := catalogService.CreateGame(ctx, hostUser.ID, roomDefinition("Room game"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalogService.Publish(ctx, hostUser.ID, createdGame.ID)
	if err != nil {
		t.Fatal(err)
	}

	rooms, err := NewService(pool, catalogService)
	if err != nil {
		t.Fatal(err)
	}
	host := identity.Actor{User: &hostUser}
	member := identity.Actor{User: &memberUser}
	room, err := rooms.Create(ctx, host, version.ID, CreateInput{Title: "Friday game", MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if room.GameVersionID != version.ID || room.Status != StatusLobby {
		t.Fatalf("Create() = %#v, want lobby pinned to version %q", room, version.ID)
	}
	if _, err := rooms.Create(ctx, host, createdGame.ID, CreateInput{Title: "Draft reference", MaxPlayers: 2}); !errors.Is(err, ErrGameVersionNotFound) {
		t.Fatalf("Create() with game ID error = %v, want ErrGameVersionNotFound", err)
	}

	joined, err := rooms.Join(ctx, member, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if joined.RoomID != room.ID || joined.ActorID != memberUser.ID {
		t.Fatalf("Join() = %#v, want member in room", joined)
	}
	if _, err := rooms.Join(ctx, member, room.ID); !errors.Is(err, ErrAlreadyMember) {
		t.Fatalf("second Join() error = %v, want ErrAlreadyMember", err)
	}
	if _, err := rooms.Join(ctx, identity.Actor{User: &secondMember}, room.ID); !errors.Is(err, ErrRoomFull) {
		t.Fatalf("Join() full room error = %v, want ErrRoomFull", err)
	}
	if err := rooms.Mute(ctx, member, room.ID, room.HostMemberID, true); !errors.Is(err, ErrNotHost) {
		t.Fatalf("member Mute() error = %v, want ErrNotHost", err)
	}
	if err := rooms.Mute(ctx, host, room.ID, joined.ID, true); err != nil {
		t.Fatalf("host Mute() error = %v", err)
	}
	if _, err := rooms.SendMessage(ctx, member, room.ID, "Muted message"); !errors.Is(err, ErrMemberMuted) {
		t.Fatalf("muted SendMessage() error = %v, want ErrMemberMuted", err)
	}
	if err := rooms.Mute(ctx, host, room.ID, joined.ID, false); err != nil {
		t.Fatalf("host unmute() error = %v", err)
	}
	message, err := rooms.SendMessage(ctx, member, room.ID, "  Hello room  ")
	if err != nil {
		t.Fatal(err)
	}
	if message.Body != "Hello room" || message.MemberID != joined.ID {
		t.Fatalf("SendMessage() = %#v, want persisted normalized message", message)
	}
	messages, err := rooms.ListMessages(ctx, member, room.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].ID != message.ID {
		t.Fatalf("ListMessages() = %#v, want persisted room message", messages)
	}
	stored, err := rooms.Get(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.HostMemberID != room.HostMemberID || len(stored.Members) != 2 || stored.Members[1].MutedAt != nil {
		t.Fatalf("Get() = %#v, want persisted members and unmuted state", stored)
	}
	if err := rooms.Remove(ctx, member, room.ID, room.HostMemberID); !errors.Is(err, ErrNotHost) {
		t.Fatalf("member Remove() error = %v, want ErrNotHost", err)
	}
}

func TestRoomStartPinsPlayersAndRejectsOutOfTurnRoll(t *testing.T) {
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
	hostUser := registerRoomUser(t, ctx, identities, "start-host@example.com", "Host")
	guestIdentity, err := identities.CreateGuest(ctx, "Guest")
	if err != nil {
		t.Fatal(err)
	}
	catalogService, err := catalog.NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	createdGame, err := catalogService.CreateGame(ctx, hostUser.ID, roomDefinition("Realtime game"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalogService.Publish(ctx, hostUser.ID, createdGame.ID)
	if err != nil {
		t.Fatal(err)
	}
	rooms, err := NewService(pool, catalogService)
	if err != nil {
		t.Fatal(err)
	}
	host := identity.Actor{User: &hostUser}
	guest := identity.Actor{Guest: &guestIdentity}
	created, err := rooms.Create(ctx, host, version.ID, CreateInput{Title: "Realtime room", MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.Join(ctx, guest, created.ID); err != nil {
		t.Fatal(err)
	}

	started, err := rooms.Start(ctx, host, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Status != StatusActive || started.Session == nil || started.Session.Mode != "multiplayer" || len(started.Members) != 2 {
		t.Fatalf("Start() = %#v, want active multiplayer session with two members", started)
	}
	if started.Members[0].PlayerID == "" || started.Members[1].PlayerID == "" {
		t.Fatalf("Start() members = %#v, want durable player slots", started.Members)
	}
	if _, err := rooms.Roll(ctx, guest, created.ID); !errors.Is(err, ErrNotYourTurn) {
		t.Fatalf("guest Roll() error = %v, want ErrNotYourTurn", err)
	}
	transition, err := rooms.Roll(ctx, host, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if transition.Sequence != started.Sequence+1 || transition.Session == nil || len(transition.Events) == 0 {
		t.Fatalf("Roll() = %#v, want persisted authoritative transition", transition)
	}
}

func registerRoomUser(t *testing.T, ctx context.Context, identities *identity.Repository, email, displayName string) identity.User {
	t.Helper()
	user, err := identities.Register(ctx, identity.RegistrationInput{Email: email, DisplayName: displayName, Password: "correct-horse-battery-staple"})
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func roomDefinition(title string) game.GameDefinition {
	return game.GameDefinition{
		Title: title,
		Board: game.Board{
			Width: 96, Height: 96, CellSize: 96,
			Cells: []game.CellDefinition{{ID: "start", Title: "Start", Type: "start", Fields: map[string]any{}}},
		},
		Rules: game.RuleSet{
			Dice:      game.DiceRule{Count: 1, Sides: 6},
			Resources: map[string]game.ResourceRule{},
			CellTypes: map[string]game.CellTypeDef{"start": {Title: "Start", Fields: map[string]game.FieldDef{}}},
		},
	}
}
