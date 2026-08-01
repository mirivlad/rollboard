// Package realtime coordinates ordered in-process room broadcasts.
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"rollboard/internal/identity"
	"rollboard/internal/room"
)

const (
	EventRoomState   = "room_state"
	EventRoomEvent   = "room_event"
	EventChatMessage = "chat_message"

	IntentStart  = "start"
	IntentRoll   = "roll"
	IntentAction = "action"
	IntentChat   = "chat"
)

var (
	ErrNotMember         = errors.New("room membership is required")
	ErrUnsupportedIntent = errors.New("unsupported room intention")
)

type RoomService interface {
	Get(context.Context, string) (*room.Room, error)
	Start(context.Context, identity.Actor, string) (*room.Room, error)
	Roll(context.Context, identity.Actor, string) (room.Transition, error)
	ResolveAction(context.Context, identity.Actor, string, string) (room.Transition, error)
	SendMessage(context.Context, identity.Actor, string, string) (room.RoomMessage, error)
}

type Intent struct {
	Type     string `json:"type"`
	ActionID string `json:"actionId,omitempty"`
	Body     string `json:"body,omitempty"`
}

type Envelope struct {
	Type     string          `json:"type"`
	Sequence uint64          `json:"sequence"`
	Payload  json.RawMessage `json:"payload"`
}

type Client struct {
	Events <-chan Envelope

	room *roomClients
	self *client
}

// Close disconnects the client and releases its bounded event channel.
func (c *Client) Close() {
	if c == nil || c.self == nil {
		return
	}
	c.room.remove(c.self)
}

type Hub struct {
	service RoomService

	mu    sync.Mutex
	rooms map[string]*roomClients
}

type roomClients struct {
	mu      sync.Mutex
	clients map[*client]struct{}
}

type client struct {
	events chan Envelope
	once   sync.Once
}

func NewHub(service RoomService) (*Hub, error) {
	if service == nil {
		return nil, fmt.Errorf("realtime hub requires a room service")
	}
	return &Hub{service: service, rooms: make(map[string]*roomClients)}, nil
}

// Connect validates membership and sends a current sequenced snapshot before
// subscribing the client to later transitions.
func (h *Hub) Connect(ctx context.Context, roomID string, actor identity.Actor, _ uint64) (*Client, error) {
	clients := h.clientsFor(roomID)
	clients.mu.Lock()
	defer clients.mu.Unlock()
	stored, err := h.service.Get(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !isMember(actor, stored.Members) {
		return nil, ErrNotMember
	}
	payload, err := json.Marshal(stored)
	if err != nil {
		return nil, fmt.Errorf("encode room snapshot: %w", err)
	}
	c := &client{events: make(chan Envelope, 32)}
	c.events <- Envelope{Type: EventRoomState, Sequence: stored.Sequence, Payload: payload}
	clients.clients[c] = struct{}{}
	return &Client{Events: c.events, room: clients, self: c}, nil
}

// Submit serializes an authenticated player intention and broadcasts only the
// transition that was durably accepted by the room service.
func (h *Hub) Submit(ctx context.Context, roomID string, actor identity.Actor, intent Intent) (room.Transition, error) {
	clients := h.clientsFor(roomID)
	clients.mu.Lock()
	defer clients.mu.Unlock()

	var transition room.Transition
	var err error
	switch intent.Type {
	case IntentStart:
		started, startErr := h.service.Start(ctx, actor, roomID)
		if startErr != nil {
			return room.Transition{}, startErr
		}
		transition = room.Transition{RoomID: started.ID, Sequence: started.Sequence, Session: started.Session}
		payload, marshalErr := json.Marshal(started)
		if marshalErr != nil {
			return room.Transition{}, fmt.Errorf("encode started room snapshot: %w", marshalErr)
		}
		clients.broadcast(Envelope{Type: EventRoomState, Sequence: started.Sequence, Payload: payload})
		return transition, nil
	case IntentRoll:
		transition, err = h.service.Roll(ctx, actor, roomID)
	case IntentAction:
		if intent.ActionID == "" {
			return room.Transition{}, fmt.Errorf("%w: action ID is required", ErrUnsupportedIntent)
		}
		transition, err = h.service.ResolveAction(ctx, actor, roomID, intent.ActionID)
	case IntentChat:
		message, messageErr := h.service.SendMessage(ctx, actor, roomID, intent.Body)
		if messageErr != nil {
			return room.Transition{}, messageErr
		}
		payload, err := json.Marshal(message)
		if err != nil {
			return room.Transition{}, fmt.Errorf("encode chat message: %w", err)
		}
		clients.broadcast(Envelope{Type: EventChatMessage, Sequence: message.Sequence, Payload: payload})
		return room.Transition{RoomID: roomID, Sequence: message.Sequence}, nil
	default:
		return room.Transition{}, fmt.Errorf("%w: %s", ErrUnsupportedIntent, intent.Type)
	}
	if err != nil {
		return room.Transition{}, err
	}
	payload, err := json.Marshal(transition)
	if err != nil {
		return room.Transition{}, fmt.Errorf("encode room transition: %w", err)
	}
	clients.broadcast(Envelope{Type: EventRoomEvent, Sequence: transition.Sequence, Payload: payload})
	return transition, nil
}

// Refresh broadcasts the latest durable room snapshot after a membership or
// moderation change made through the HTTP API.
func (h *Hub) Refresh(ctx context.Context, roomID string) error {
	clients := h.clientsFor(roomID)
	clients.mu.Lock()
	defer clients.mu.Unlock()

	stored, err := h.service.Get(ctx, roomID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("encode refreshed room snapshot: %w", err)
	}
	clients.broadcast(Envelope{Type: EventRoomState, Sequence: stored.Sequence, Payload: payload})
	return nil
}

func (h *Hub) clientsFor(roomID string) *roomClients {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := h.rooms[roomID]
	if clients == nil {
		clients = &roomClients{clients: make(map[*client]struct{})}
		h.rooms[roomID] = clients
	}
	return clients
}

func (r *roomClients) broadcast(event Envelope) {
	for c := range r.clients {
		select {
		case c.events <- event:
		default:
			r.removeLocked(c)
		}
	}
}

func (r *roomClients) remove(c *client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeLocked(c)
}

func (r *roomClients) removeLocked(c *client) {
	c.once.Do(func() {
		delete(r.clients, c)
		close(c.events)
	})
}

func isMember(actor identity.Actor, members []room.RoomMember) bool {
	for _, member := range members {
		if actor.User != nil && member.ActorKind == "user" && member.ActorID == actor.User.ID {
			return true
		}
		if actor.Guest != nil && member.ActorKind == "guest" && member.ActorID == actor.Guest.ID {
			return true
		}
	}
	return false
}
