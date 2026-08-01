package room

import (
	"context"
	"encoding/json"
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
	afterJoin, err := rooms.Get(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterJoin.Sequence != room.Sequence+1 || len(afterJoin.Members) != 2 {
		t.Fatalf("room after Join() = %#v, want sequenced membership change", afterJoin)
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

	startCommand := Command{ID: "8f4650d1-82c2-4ff2-9b5e-3f90ba2e2c03", Type: "start"}
	started, err := rooms.StartWithCommand(ctx, host, created.ID, startCommand)
	if err != nil {
		t.Fatal(err)
	}
	if started.Status != StatusActive || started.Session == nil || started.Session.Mode != "multiplayer" || len(started.Members) != 2 {
		t.Fatalf("Start() = %#v, want active multiplayer session with two members", started)
	}
	if started.Members[0].PlayerID == "" || started.Members[1].PlayerID == "" {
		t.Fatalf("Start() members = %#v, want durable player slots", started.Members)
	}
	duplicateStart, err := rooms.StartWithCommand(ctx, host, created.ID, startCommand)
	if err != nil || !duplicateStart.Duplicate || duplicateStart.Sequence != started.Sequence || duplicateStart.StoredEvent == nil {
		t.Fatalf("duplicate StartWithCommand() = %#v, err=%v", duplicateStart, err)
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

func TestRoomRollStoresReplayableEventAndDeduplicatesCommand(t *testing.T) {
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
	hostUser := registerRoomUser(t, ctx, identities, "journal-host@example.com", "Host")
	guestIdentity, err := identities.CreateGuest(ctx, "Guest")
	if err != nil {
		t.Fatal(err)
	}
	catalogService, err := catalog.NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	createdGame, err := catalogService.CreateGame(ctx, hostUser.ID, roomDefinition("Journal game"))
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
	created, err := rooms.Create(ctx, host, version.ID, CreateInput{Title: "Journal room", MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.Join(ctx, guest, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.Start(ctx, host, created.ID); err != nil {
		t.Fatal(err)
	}

	command := Command{ID: "0b66d950-6f5f-4f8d-b16e-49aa54d56d6d", Type: "roll"}
	first, err := rooms.RollWithCommand(ctx, host, created.ID, command)
	if err != nil {
		t.Fatal(err)
	}
	if first.StoredEvent == nil || first.StoredEvent.Type != "room_event" {
		t.Fatalf("first RollWithCommand() = %#v, want stored room event", first)
	}
	events, contiguous, err := rooms.EventsSince(ctx, host, created.ID, first.Sequence-1)
	if err != nil || !contiguous || len(events) != 1 || events[0].Sequence != first.Sequence {
		t.Fatalf("EventsSince() = %#v, contiguous=%v, err=%v", events, contiguous, err)
	}
	second, err := rooms.RollWithCommand(ctx, host, created.ID, command)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Duplicate || second.Sequence != first.Sequence || second.StoredEvent == nil || second.StoredEvent.Sequence != first.StoredEvent.Sequence {
		t.Fatalf("duplicate RollWithCommand() = %#v, want original receipt %#v", second, first)
	}
}

func TestRoomDuplicateChatStoresOneMessage(t *testing.T) {
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
	hostUser := registerRoomUser(t, ctx, identities, "chat-host@example.com", "Host")
	guestIdentity, err := identities.CreateGuest(ctx, "Guest")
	if err != nil {
		t.Fatal(err)
	}
	catalogService, err := catalog.NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	createdGame, err := catalogService.CreateGame(ctx, hostUser.ID, roomDefinition("Chat game"))
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
	created, err := rooms.Create(ctx, host, version.ID, CreateInput{Title: "Chat room", MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.Join(ctx, guest, created.ID); err != nil {
		t.Fatal(err)
	}

	command := Command{ID: "286e7e24-e3da-4f89-8a63-e831f5e9bd81", Type: "chat"}
	first, err := rooms.SendMessageWithCommand(ctx, guest, created.ID, "hello", command)
	if err != nil {
		t.Fatal(err)
	}
	second, err := rooms.SendMessageWithCommand(ctx, guest, created.ID, "hello", command)
	if err != nil {
		t.Fatal(err)
	}
	if first.StoredEvent == nil || !second.Duplicate || second.StoredEvent == nil || second.StoredEvent.Sequence != first.StoredEvent.Sequence {
		t.Fatalf("chat receipts = %#v and %#v", first, second)
	}
	messages, err := rooms.ListMessages(ctx, guest, created.ID, 10)
	if err != nil || len(messages) != 1 || messages[0].ID != first.ID {
		t.Fatalf("messages = %#v, err=%v", messages, err)
	}
}

func TestRoomDuplicateActionReturnsStoredTransition(t *testing.T) {
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
	hostUser := registerRoomUser(t, ctx, identities, "action-host@example.com", "Host")
	guestIdentity, err := identities.CreateGuest(ctx, "Guest")
	if err != nil {
		t.Fatal(err)
	}
	catalogService, err := catalog.NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	createdGame, err := catalogService.CreateGame(ctx, hostUser.ID, roomDefinition("Action game"))
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
	created, err := rooms.Create(ctx, host, version.ID, CreateInput{Title: "Action room", MaxPlayers: 2})
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
	started.Session.State.PendingAction = &game.PendingAction{
		Type: "choice", PlayerID: started.Session.CurrentPlayer().ID,
		Options: []game.ActionOption{{ID: "continue", Title: "Continue"}},
	}
	raw, err := json.Marshal(started.Session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rooms SET session_json = $2::jsonb WHERE id = $1`, created.ID, string(raw)); err != nil {
		t.Fatal(err)
	}

	command := Command{ID: "d97cc84e-12d5-45a1-8f96-7d377465bafb", Type: "action"}
	first, err := rooms.ResolveActionWithCommand(ctx, host, created.ID, "continue", command)
	if err != nil {
		t.Fatal(err)
	}
	second, err := rooms.ResolveActionWithCommand(ctx, host, created.ID, "continue", command)
	if err != nil {
		t.Fatal(err)
	}
	if first.StoredEvent == nil || !second.Duplicate || second.Sequence != first.Sequence || second.StoredEvent == nil {
		t.Fatalf("action receipts = %#v and %#v", first, second)
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
