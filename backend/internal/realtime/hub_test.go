package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"rollboard/internal/game"
	"rollboard/internal/identity"
	"rollboard/internal/room"
)

func TestHubBroadcastsOneOrderedTransitionAndRejectsOutOfTurnRoll(t *testing.T) {
	host := identity.User{ID: "host-id", DisplayName: "Host"}
	guest := identity.Guest{ID: "guest-id", DisplayName: "Guest"}
	stored := &room.Room{
		ID: "room-id", Status: room.StatusActive, Sequence: 4,
		Members: []room.RoomMember{
			{ID: "host-member", ActorKind: "user", ActorID: host.ID, PlayerID: "player_1", DisplayName: host.DisplayName},
			{ID: "guest-member", ActorKind: "guest", ActorID: guest.ID, PlayerID: "player_2", DisplayName: guest.DisplayName},
		},
	}
	service := &fakeRoomService{room: stored, transition: room.Transition{
		RoomID: "room-id", Sequence: 5,
		Session: &game.GameSession{ID: "session-id", State: game.GameState{Status: "active"}},
		Events:  []game.GameEvent{{ID: "event-id", Type: "dice_roll", Message: "Host rolled 4"}},
	}}
	hub, err := NewHub(service)
	if err != nil {
		t.Fatal(err)
	}
	hostClient, err := hub.Connect(context.Background(), "room-id", identity.Actor{User: &host}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer hostClient.Close()
	guestClient, err := hub.Connect(context.Background(), "room-id", identity.Actor{Guest: &guest}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer guestClient.Close()
	if state := receiveEnvelope(t, hostClient); state.Type != EventRoomState || state.Sequence != 4 {
		t.Fatalf("host initial event = %#v, want room state sequence 4", state)
	}
	if state := receiveEnvelope(t, guestClient); state.Type != EventRoomState || state.Sequence != 4 {
		t.Fatalf("guest initial event = %#v, want room state sequence 4", state)
	}

	if _, err := hub.Submit(context.Background(), "room-id", identity.Actor{User: &host}, Intent{Type: IntentRoll, CommandID: "00000000-0000-4000-8000-000000000001"}); err != nil {
		t.Fatal(err)
	}
	hostEvent := receiveEnvelope(t, hostClient)
	guestEvent := receiveEnvelope(t, guestClient)
	if hostEvent.Type != EventRoomEvent || hostEvent.Sequence != 5 || !reflect.DeepEqual(hostEvent, guestEvent) {
		t.Fatalf("broadcasts = %#v and %#v, want equal ordered room event", hostEvent, guestEvent)
	}
	if _, err := hub.Submit(context.Background(), "room-id", identity.Actor{Guest: &guest}, Intent{Type: IntentRoll, CommandID: "00000000-0000-4000-8000-000000000002"}); !errors.Is(err, room.ErrNotYourTurn) {
		t.Fatalf("out-of-turn Submit() error = %v, want ErrNotYourTurn", err)
	}
	if service.rollCalls != 2 {
		t.Fatalf("roll calls = %d, want both intentions checked by service", service.rollCalls)
	}
	assertNoEnvelope(t, hostClient)
	assertNoEnvelope(t, guestClient)
}

func TestHubRejectsMutatingIntentWithoutCommandID(t *testing.T) {
	host := identity.User{ID: "host-id", DisplayName: "Host"}
	service := &fakeRoomService{room: &room.Room{ID: "room-id", Status: room.StatusActive, Members: []room.RoomMember{{ActorKind: "user", ActorID: host.ID}}}}
	hub, err := NewHub(service)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()

	if _, err := hub.Submit(context.Background(), "room-id", identity.Actor{User: &host}, Intent{Type: IntentRoll}); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("Submit() error = %v, want ErrInvalidCommand", err)
	}
}

func TestHubsBroadcastTransitionsAcrossReplicas(t *testing.T) {
	host := identity.User{ID: "host-id", DisplayName: "Host"}
	guest := identity.Guest{ID: "guest-id", DisplayName: "Guest"}
	stored := &room.Room{ID: "room-id", Status: room.StatusActive, Sequence: 4, Members: []room.RoomMember{
		{ID: "host-member", ActorKind: "user", ActorID: host.ID, PlayerID: "player_1", DisplayName: host.DisplayName},
		{ID: "guest-member", ActorKind: "guest", ActorID: guest.ID, PlayerID: "player_2", DisplayName: guest.DisplayName},
	}}
	transition := room.Transition{RoomID: "room-id", Sequence: 5, Session: &game.GameSession{ID: "session-id", State: game.GameState{Status: "active"}}}
	backplane := newMemoryBackplane()
	primary, err := NewHub(&fakeRoomService{room: stored, transition: transition}, WithBackplane(backplane))
	if err != nil {
		t.Fatal(err)
	}
	defer primary.Close()
	replica, err := NewHub(&fakeRoomService{room: stored, transition: transition}, WithBackplane(backplane))
	if err != nil {
		t.Fatal(err)
	}
	defer replica.Close()

	hostClient, err := primary.Connect(context.Background(), "room-id", identity.Actor{User: &host}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer hostClient.Close()
	guestClient, err := replica.Connect(context.Background(), "room-id", identity.Actor{Guest: &guest}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer guestClient.Close()
	receiveEnvelope(t, hostClient)
	receiveEnvelope(t, guestClient)

	if _, err := primary.Submit(context.Background(), "room-id", identity.Actor{User: &host}, Intent{Type: IntentRoll, CommandID: "00000000-0000-4000-8000-000000000003"}); err != nil {
		t.Fatal(err)
	}
	hostEvent := receiveEnvelope(t, hostClient)
	guestEvent := receiveEnvelope(t, guestClient)
	if hostEvent.Type != EventRoomEvent || hostEvent.Sequence != 5 || !reflect.DeepEqual(hostEvent, guestEvent) {
		t.Fatalf("replica events = %#v and %#v, want the same sequenced transition", hostEvent, guestEvent)
	}
	assertNoEnvelope(t, hostClient)
	assertNoEnvelope(t, guestClient)
}

func TestHubReconnectsWithContiguousJournalEvents(t *testing.T) {
	host := identity.User{ID: "host-id", DisplayName: "Host"}
	service := &fakeRoomService{
		room: &room.Room{ID: "room-id", Sequence: 7, Members: []room.RoomMember{{ActorKind: "user", ActorID: host.ID}}},
		replayEvents: []room.StoredEvent{
			{RoomID: "room-id", Sequence: 6, Type: EventRoomEvent, Payload: json.RawMessage(`{"roomId":"room-id","sequence":6}`)},
			{RoomID: "room-id", Sequence: 7, Type: EventRoomEvent, Payload: json.RawMessage(`{"roomId":"room-id","sequence":7}`)},
		},
		replayContiguous: true,
	}
	hub, err := NewHub(service)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	client, err := hub.Connect(context.Background(), "room-id", identity.Actor{User: &host}, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	first := receiveEnvelope(t, client)
	second := receiveEnvelope(t, client)
	if first.Type != EventRoomEvent || first.Sequence != 6 || second.Type != EventRoomEvent || second.Sequence != 7 {
		t.Fatalf("reconnect events = %#v, %#v; want sequences 6 then 7", first, second)
	}
}

func TestHubsBroadcastTransitionsThroughRedis(t *testing.T) {
	redisURL := os.Getenv("ROLLBOARD_TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("ROLLBOARD_TEST_REDIS_URL is required")
	}
	host := identity.User{ID: "host-id", DisplayName: "Host"}
	guest := identity.Guest{ID: "guest-id", DisplayName: "Guest"}
	stored := &room.Room{ID: "redis-room-id", Status: room.StatusActive, Sequence: 4, Members: []room.RoomMember{
		{ID: "host-member", ActorKind: "user", ActorID: host.ID, PlayerID: "player_1", DisplayName: host.DisplayName},
		{ID: "guest-member", ActorKind: "guest", ActorID: guest.ID, PlayerID: "player_2", DisplayName: guest.DisplayName},
	}}
	transition := room.Transition{RoomID: stored.ID, Sequence: 5, Session: &game.GameSession{ID: "session-id", State: game.GameState{Status: "active"}}}
	primaryBackplane, err := NewRedisBackplane(context.Background(), redisURL)
	if err != nil {
		t.Fatal(err)
	}
	primary, err := NewHub(&fakeRoomService{room: stored, transition: transition}, WithBackplane(primaryBackplane))
	if err != nil {
		_ = primaryBackplane.Close()
		t.Fatal(err)
	}
	defer primary.Close()
	replicaBackplane, err := NewRedisBackplane(context.Background(), redisURL)
	if err != nil {
		t.Fatal(err)
	}
	replica, err := NewHub(&fakeRoomService{room: stored, transition: transition}, WithBackplane(replicaBackplane))
	if err != nil {
		_ = replicaBackplane.Close()
		t.Fatal(err)
	}
	defer replica.Close()

	hostClient, err := primary.Connect(context.Background(), stored.ID, identity.Actor{User: &host}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer hostClient.Close()
	guestClient, err := replica.Connect(context.Background(), stored.ID, identity.Actor{Guest: &guest}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer guestClient.Close()
	receiveEnvelope(t, hostClient)
	receiveEnvelope(t, guestClient)

	if _, err := primary.Submit(context.Background(), stored.ID, identity.Actor{User: &host}, Intent{Type: IntentRoll, CommandID: "00000000-0000-4000-8000-000000000004"}); err != nil {
		t.Fatal(err)
	}
	hostEvent := receiveEnvelope(t, hostClient)
	guestEvent := receiveEnvelope(t, guestClient)
	if hostEvent.Type != EventRoomEvent || hostEvent.Sequence != 5 || !reflect.DeepEqual(hostEvent, guestEvent) {
		t.Fatalf("Redis replica events = %#v and %#v, want the same sequenced transition", hostEvent, guestEvent)
	}
	assertNoEnvelope(t, hostClient)
	assertNoEnvelope(t, guestClient)

	stored.Sequence = 6
	stored.Members = stored.Members[:1]
	if err := primary.Refresh(context.Background(), stored.ID); err != nil {
		t.Fatal(err)
	}
	if event := receiveEnvelope(t, hostClient); event.Type != EventRoomState || event.Sequence != 6 {
		t.Fatalf("host refresh event = %#v, want room state sequence 6", event)
	}
	assertClientClosed(t, guestClient)
}

func TestHubBroadcastsPersistedChatMessage(t *testing.T) {
	user := identity.User{ID: "host-id", DisplayName: "Host"}
	guest := identity.Guest{ID: "guest-id", DisplayName: "Guest"}
	service := &fakeRoomService{
		room: &room.Room{ID: "room-id", Sequence: 8, Members: []room.RoomMember{
			{ActorKind: "user", ActorID: user.ID}, {ActorKind: "guest", ActorID: guest.ID},
		}},
		chat: room.RoomMessage{ID: "message-id", RoomID: "room-id", Body: "Hello", DisplayName: guest.DisplayName, Sequence: 9},
	}
	hub, err := NewHub(service)
	if err != nil {
		t.Fatal(err)
	}
	hostClient, err := hub.Connect(context.Background(), "room-id", identity.Actor{User: &user}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer hostClient.Close()
	guestClient, err := hub.Connect(context.Background(), "room-id", identity.Actor{Guest: &guest}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer guestClient.Close()
	receiveEnvelope(t, hostClient)
	receiveEnvelope(t, guestClient)

	if _, err := hub.Submit(context.Background(), "room-id", identity.Actor{Guest: &guest}, Intent{Type: IntentChat, CommandID: "00000000-0000-4000-8000-000000000005", Body: "Hello"}); err != nil {
		t.Fatal(err)
	}
	if event := receiveEnvelope(t, hostClient); event.Type != EventChatMessage || event.Sequence != 9 {
		t.Fatalf("host chat event = %#v, want persisted chat broadcast", event)
	}
	if event := receiveEnvelope(t, guestClient); event.Type != EventChatMessage || event.Sequence != 9 {
		t.Fatalf("guest chat event = %#v, want persisted chat broadcast", event)
	}
}

func TestHubRefreshBroadcastsLatestRoomState(t *testing.T) {
	host := identity.User{ID: "host-id", DisplayName: "Host"}
	guest := identity.Guest{ID: "guest-id", DisplayName: "Guest"}
	service := &fakeRoomService{room: &room.Room{ID: "room-id", Sequence: 1, Members: []room.RoomMember{
		{ActorKind: "user", ActorID: host.ID},
		{ActorKind: "guest", ActorID: guest.ID},
	}}}
	hub, err := NewHub(service)
	if err != nil {
		t.Fatal(err)
	}
	hostClient, err := hub.Connect(context.Background(), "room-id", identity.Actor{User: &host}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer hostClient.Close()
	guestClient, err := hub.Connect(context.Background(), "room-id", identity.Actor{Guest: &guest}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer guestClient.Close()
	receiveEnvelope(t, hostClient)
	receiveEnvelope(t, guestClient)

	service.room.Sequence = 2
	if err := hub.Refresh(context.Background(), "room-id"); err != nil {
		t.Fatal(err)
	}
	if event := receiveEnvelope(t, hostClient); event.Type != EventRoomState || event.Sequence != 2 {
		t.Fatalf("host refresh event = %#v, want room state sequence 2", event)
	}
	if event := receiveEnvelope(t, guestClient); event.Type != EventRoomState || event.Sequence != 2 {
		t.Fatalf("guest refresh event = %#v, want room state sequence 2", event)
	}
}

func TestHubRefreshDisconnectsRemovedMember(t *testing.T) {
	host := identity.User{ID: "host-id", DisplayName: "Host"}
	guest := identity.Guest{ID: "guest-id", DisplayName: "Guest"}
	service := &fakeRoomService{room: &room.Room{ID: "room-id", Sequence: 1, Members: []room.RoomMember{
		{ActorKind: "user", ActorID: host.ID},
		{ActorKind: "guest", ActorID: guest.ID},
	}}}
	hub, err := NewHub(service)
	if err != nil {
		t.Fatal(err)
	}
	hostClient, err := hub.Connect(context.Background(), "room-id", identity.Actor{User: &host}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer hostClient.Close()
	guestClient, err := hub.Connect(context.Background(), "room-id", identity.Actor{Guest: &guest}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer guestClient.Close()
	receiveEnvelope(t, hostClient)
	receiveEnvelope(t, guestClient)

	service.room.Sequence = 2
	service.room.Members = service.room.Members[:1]
	if err := hub.Refresh(context.Background(), "room-id"); err != nil {
		t.Fatal(err)
	}
	if event := receiveEnvelope(t, hostClient); event.Type != EventRoomState || event.Sequence != 2 {
		t.Fatalf("host refresh event = %#v, want room state sequence 2", event)
	}
	assertClientClosed(t, guestClient)
}

func TestHubStartBroadcastsRoomStateWithAssignedPlayers(t *testing.T) {
	host := identity.User{ID: "host-id", DisplayName: "Host"}
	initial := &room.Room{ID: "room-id", Sequence: 1, Status: room.StatusLobby, Members: []room.RoomMember{{ActorKind: "user", ActorID: host.ID}}}
	started := &room.Room{ID: "room-id", Sequence: 2, Status: room.StatusActive, Session: &game.GameSession{State: game.GameState{Status: "active"}}, Members: []room.RoomMember{{ActorKind: "user", ActorID: host.ID, PlayerID: "player_1"}}}
	service := &fakeRoomService{room: initial, startRoom: started}
	hub, err := NewHub(service)
	if err != nil {
		t.Fatal(err)
	}
	client, err := hub.Connect(context.Background(), "room-id", identity.Actor{User: &host}, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	receiveEnvelope(t, client)

	if _, err := hub.Submit(context.Background(), "room-id", identity.Actor{User: &host}, Intent{Type: IntentStart, CommandID: "00000000-0000-4000-8000-000000000006"}); err != nil {
		t.Fatal(err)
	}
	event := receiveEnvelope(t, client)
	if event.Type != EventRoomState || event.Sequence != 2 {
		t.Fatalf("start event = %#v, want room state sequence 2", event)
	}
	var payload room.Room
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Members[0].PlayerID != "player_1" || payload.Session == nil {
		t.Fatalf("start payload = %#v, want assigned player IDs and session", payload)
	}
}

func receiveEnvelope(t *testing.T, client *Client) Envelope {
	t.Helper()
	select {
	case event := <-client.Events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for room event")
		return Envelope{}
	}
}

func assertNoEnvelope(t *testing.T, client *Client) {
	t.Helper()
	select {
	case event := <-client.Events:
		t.Fatalf("unexpected room event: %#v", event)
	case <-time.After(50 * time.Millisecond):
	}
}

func assertClientClosed(t *testing.T, client *Client) {
	t.Helper()
	select {
	case _, open := <-client.Events:
		if open {
			t.Fatal("client received an event after removal instead of being disconnected")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for removed client to disconnect")
	}
}

type fakeRoomService struct {
	room             *room.Room
	startRoom        *room.Room
	transition       room.Transition
	rollCalls        int
	chat             room.RoomMessage
	replayEvents     []room.StoredEvent
	replayContiguous bool
	lastInventory    [3]string
	lastTrade        *game.TradeOffer
}

type memoryBackplane struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[int]func(BackplaneEvent)
}

func newMemoryBackplane() *memoryBackplane {
	return &memoryBackplane{subscribers: make(map[int]func(BackplaneEvent))}
}

func (b *memoryBackplane) Publish(_ context.Context, event BackplaneEvent) error {
	b.mu.Lock()
	callbacks := make([]func(BackplaneEvent), 0, len(b.subscribers))
	for _, callback := range b.subscribers {
		callbacks = append(callbacks, callback)
	}
	b.mu.Unlock()
	for _, callback := range callbacks {
		go callback(event)
	}
	return nil
}

func (b *memoryBackplane) Subscribe(ctx context.Context, callback func(BackplaneEvent)) error {
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.subscribers[id] = callback
	b.mu.Unlock()
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subscribers, id)
		b.mu.Unlock()
	}()
	return nil
}

func (b *memoryBackplane) Close() error { return nil }

func (f *fakeRoomService) Get(context.Context, string) (*room.Room, error) { return f.room, nil }

func (f *fakeRoomService) EventsSince(context.Context, identity.Actor, string, uint64) ([]room.StoredEvent, bool, error) {
	return f.replayEvents, f.replayContiguous, nil
}

func (f *fakeRoomService) Start(context.Context, identity.Actor, string) (*room.Room, error) {
	if f.startRoom != nil {
		f.room = f.startRoom
	}
	return f.room, nil
}

func (f *fakeRoomService) StartWithCommand(ctx context.Context, actor identity.Actor, roomID string, _ room.Command) (*room.Room, error) {
	return f.Start(ctx, actor, roomID)
}

func (f *fakeRoomService) Roll(_ context.Context, actor identity.Actor, _ string) (room.Transition, error) {
	f.rollCalls++
	if actor.Guest != nil {
		return room.Transition{}, room.ErrNotYourTurn
	}
	return f.transition, nil
}

func (f *fakeRoomService) RollWithCommand(ctx context.Context, actor identity.Actor, roomID string, _ room.Command) (room.Transition, error) {
	return f.Roll(ctx, actor, roomID)
}

func (f *fakeRoomService) ResolveAction(context.Context, identity.Actor, string, string) (room.Transition, error) {
	return f.transition, nil
}

func (f *fakeRoomService) ResolveActionWithCommand(ctx context.Context, actor identity.Actor, roomID, actionID string, _ room.Command) (room.Transition, error) {
	return f.ResolveAction(ctx, actor, roomID, actionID)
}

// The inventory and trade commands record what the hub passed down, so a test
// can check that the sender's own seat is used rather than anything from the
// payload.
func (f *fakeRoomService) ManageInventoryWithCommand(_ context.Context, _ identity.Actor, roomID, operation, target string, _ room.Command) (room.Transition, error) {
	f.lastInventory = [3]string{roomID, operation, target}
	return f.transition, nil
}

func (f *fakeRoomService) ProposeTradeWithCommand(_ context.Context, _ identity.Actor, roomID string, offer game.TradeOffer, _ room.Command) (room.Transition, error) {
	f.lastTrade = &offer
	return f.transition, nil
}

func (f *fakeRoomService) SendMessage(context.Context, identity.Actor, string, string) (room.RoomMessage, error) {
	return f.chat, nil
}

func (f *fakeRoomService) SendMessageWithCommand(ctx context.Context, actor identity.Actor, roomID, body string, _ room.Command) (room.RoomMessage, error) {
	return f.SendMessage(ctx, actor, roomID, body)
}

func TestHubCarriesInventoryAndTradeIntents(t *testing.T) {
	// Online players could be handed an item by a cell and never put it on:
	// the protocol carried start, roll, action and chat and nothing else.
	service := &fakeRoomService{
		room:       &room.Room{ID: "room-1", Members: []room.RoomMember{{ActorKind: "user", ActorID: "user-1", PlayerID: "player_1"}}},
		transition: room.Transition{RoomID: "room-1", Sequence: 4},
	}
	hub, err := NewHub(service)
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Close()
	actor := identity.Actor{User: &identity.User{ID: "user-1"}}

	if _, err := hub.Submit(context.Background(), "room-1", actor, Intent{
		Type: IntentInventory, CommandID: "11111111-1111-1111-1111-111111111111",
		Operation: "equip", Target: "rusty_sword",
	}); err != nil {
		t.Fatalf("inventory intent: %v", err)
	}
	if service.lastInventory != [3]string{"room-1", "equip", "rusty_sword"} {
		t.Fatalf("inventory reached the service as %v", service.lastInventory)
	}

	if _, err := hub.Submit(context.Background(), "room-1", actor, Intent{
		Type: IntentTrade, CommandID: "22222222-2222-2222-2222-222222222222",
		Offer: &game.TradeOffer{ToPlayerID: "player_2", OfferResources: map[string]int{"money": 50}},
	}); err != nil {
		t.Fatalf("trade intent: %v", err)
	}
	if service.lastTrade == nil || service.lastTrade.ToPlayerID != "player_2" {
		t.Fatalf("trade reached the service as %+v", service.lastTrade)
	}
}

func TestHubRefusesIncompleteInventoryAndTradeIntents(t *testing.T) {
	service := &fakeRoomService{
		room:       &room.Room{ID: "room-1", Members: []room.RoomMember{{ActorKind: "user", ActorID: "user-1", PlayerID: "player_1"}}},
		transition: room.Transition{RoomID: "room-1", Sequence: 1},
	}
	hub, _ := NewHub(service)
	defer hub.Close()
	actor := identity.Actor{User: &identity.User{ID: "user-1"}}

	for _, intent := range []Intent{
		{Type: IntentInventory, CommandID: "11111111-1111-1111-1111-111111111111"},
		{Type: IntentInventory, CommandID: "11111111-1111-1111-1111-111111111111", Operation: "equip"},
		{Type: IntentTrade, CommandID: "22222222-2222-2222-2222-222222222222"},
		// A command ID is required for these exactly as it is for a roll, so a
		// retried frame cannot equip twice.
		{Type: IntentInventory, Operation: "equip", Target: "sword"},
	} {
		if _, err := hub.Submit(context.Background(), "room-1", actor, intent); err == nil {
			t.Fatalf("accepted an incomplete intent: %+v", intent)
		}
	}
}
