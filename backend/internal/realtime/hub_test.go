package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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

	if _, err := hub.Submit(context.Background(), "room-id", identity.Actor{User: &host}, Intent{Type: IntentRoll}); err != nil {
		t.Fatal(err)
	}
	hostEvent := receiveEnvelope(t, hostClient)
	guestEvent := receiveEnvelope(t, guestClient)
	if hostEvent.Type != EventRoomEvent || hostEvent.Sequence != 5 || !reflect.DeepEqual(hostEvent, guestEvent) {
		t.Fatalf("broadcasts = %#v and %#v, want equal ordered room event", hostEvent, guestEvent)
	}
	if _, err := hub.Submit(context.Background(), "room-id", identity.Actor{Guest: &guest}, Intent{Type: IntentRoll}); !errors.Is(err, room.ErrNotYourTurn) {
		t.Fatalf("out-of-turn Submit() error = %v, want ErrNotYourTurn", err)
	}
	if service.rollCalls != 2 {
		t.Fatalf("roll calls = %d, want both intentions checked by service", service.rollCalls)
	}
	assertNoEnvelope(t, hostClient)
	assertNoEnvelope(t, guestClient)
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

	if _, err := hub.Submit(context.Background(), "room-id", identity.Actor{Guest: &guest}, Intent{Type: IntentChat, Body: "Hello"}); err != nil {
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

	if _, err := hub.Submit(context.Background(), "room-id", identity.Actor{User: &host}, Intent{Type: IntentStart}); err != nil {
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

type fakeRoomService struct {
	room       *room.Room
	startRoom  *room.Room
	transition room.Transition
	rollCalls  int
	chat       room.RoomMessage
}

func (f *fakeRoomService) Get(context.Context, string) (*room.Room, error) { return f.room, nil }

func (f *fakeRoomService) Start(context.Context, identity.Actor, string) (*room.Room, error) {
	if f.startRoom != nil {
		f.room = f.startRoom
	}
	return f.room, nil
}

func (f *fakeRoomService) Roll(_ context.Context, actor identity.Actor, _ string) (room.Transition, error) {
	f.rollCalls++
	if actor.Guest != nil {
		return room.Transition{}, room.ErrNotYourTurn
	}
	return f.transition, nil
}

func (f *fakeRoomService) ResolveAction(context.Context, identity.Actor, string, string) (room.Transition, error) {
	return f.transition, nil
}

func (f *fakeRoomService) SendMessage(context.Context, identity.Actor, string, string) (room.RoomMessage, error) {
	return f.chat, nil
}
