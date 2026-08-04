// Package room persists multiplayer lobbies and their membership.
package room

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rollboard/internal/catalog"
	"rollboard/internal/game"
	"rollboard/internal/identity"
)

var (
	ErrNotFound            = errors.New("room not found")
	ErrGameVersionNotFound = errors.New("published game version not found")
	ErrAccountRequired     = errors.New("account is required")
	ErrAlreadyMember       = errors.New("actor is already a room member")
	ErrRoomFull            = errors.New("room is full")
	ErrRoomNotJoinable     = errors.New("room is not accepting new members")
	ErrNotHost             = errors.New("host permission is required")
	ErrMemberNotFound      = errors.New("room member not found")
	ErrCannotRemoveHost    = errors.New("host cannot be removed")
	ErrNotMember           = errors.New("room membership is required")
	ErrRoomNotReady        = errors.New("room needs at least two members to start")
	ErrGameNotActive       = errors.New("room game is not active")
	ErrNotYourTurn         = errors.New("it is not this actor's turn")
	ErrPendingAction       = errors.New("a pending action must be resolved first")
	ErrNoPendingAction     = errors.New("there is no pending action")
	ErrMemberMuted         = errors.New("room member is muted")
	ErrInvalidMessage      = errors.New("chat message must contain 1 to 1000 characters")
	ErrInvalidCommand      = errors.New("room command is invalid")
)

const (
	StatusLobby    = "lobby"
	StatusActive   = "active"
	StatusFinished = "finished"

	maxJournalReplayEvents = 64
)

type Service struct {
	pool    *pgxpool.Pool
	catalog *catalog.Service
}

type CreateInput struct {
	Title      string
	MaxPlayers int16
}

type Room struct {
	ID            string            `json:"id"`
	GameVersionID string            `json:"gameVersionId"`
	HostUserID    string            `json:"hostUserId"`
	HostMemberID  string            `json:"hostMemberId"`
	Title         string            `json:"title"`
	MaxPlayers    int16             `json:"maxPlayers"`
	Status        string            `json:"status"`
	Sequence      uint64            `json:"sequence"`
	Session       *game.GameSession `json:"session,omitempty"`
	Members       []RoomMember      `json:"members"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
	StoredEvent   *StoredEvent      `json:"-"`
	Duplicate     bool              `json:"-"`
}

type RoomMember struct {
	ID          string     `json:"id"`
	RoomID      string     `json:"roomId"`
	ActorKind   string     `json:"actorKind"`
	ActorID     string     `json:"actorId"`
	PlayerID    string     `json:"playerId,omitempty"`
	DisplayName string     `json:"displayName"`
	MutedAt     *time.Time `json:"mutedAt,omitempty"`
	JoinedAt    time.Time  `json:"joinedAt"`
}

type RoomMessage struct {
	ID          string       `json:"id"`
	RoomID      string       `json:"roomId"`
	MemberID    string       `json:"memberId"`
	DisplayName string       `json:"displayName"`
	Body        string       `json:"body"`
	CreatedAt   time.Time    `json:"createdAt"`
	Sequence    uint64       `json:"sequence"`
	StoredEvent *StoredEvent `json:"-"`
	Duplicate   bool         `json:"-"`
}

// Transition is an authoritative room state change suitable for realtime broadcast.
type Transition struct {
	RoomID      string            `json:"roomId"`
	Sequence    uint64            `json:"sequence"`
	Session     *game.GameSession `json:"session"`
	Events      []game.GameEvent  `json:"events"`
	StoredEvent *StoredEvent      `json:"-"`
	Duplicate   bool              `json:"-"`
}

// Command identifies one client mutation. An empty ID is supported only by
// legacy callers while the websocket contract is migrated to idempotent IDs.
type Command struct {
	ID   string
	Type string
}

// StoredEvent is a durable, wire-compatible room envelope payload.
type StoredEvent struct {
	RoomID    string          `json:"roomId"`
	Sequence  uint64          `json:"sequence"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
}

func NewService(pool *pgxpool.Pool, catalogService *catalog.Service) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("room service requires a PostgreSQL pool")
	}
	if catalogService == nil {
		return nil, fmt.Errorf("room service requires a catalog service")
	}
	return &Service{pool: pool, catalog: catalogService}, nil
}

func (s *Service) Create(ctx context.Context, host identity.Actor, gameVersionID string, input CreateInput) (Room, error) {
	if host.User == nil {
		return Room{}, ErrAccountRequired
	}
	if err := validateCreateInput(&input); err != nil {
		return Room{}, err
	}
	version, err := s.catalog.GetVersionByID(ctx, gameVersionID)
	if err != nil {
		return Room{}, fmt.Errorf("load game version: %w", err)
	}
	if version == nil {
		return Room{}, ErrGameVersionNotFound
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Room{}, fmt.Errorf("begin room creation: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	inviteToken, err := newInviteToken()
	if err != nil {
		return Room{}, err
	}

	var created Room
	err = tx.QueryRow(ctx, `
		INSERT INTO rooms (game_version_id, host_user_id, title, max_players, invite_token)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text, game_version_id::text, host_user_id::text, title, max_players, status, sequence, created_at, updated_at`,
		version.ID, host.User.ID, input.Title, input.MaxPlayers, inviteToken).
		Scan(&created.ID, &created.GameVersionID, &created.HostUserID, &created.Title, &created.MaxPlayers, &created.Status, &created.Sequence, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return Room{}, fmt.Errorf("insert room: %w", err)
	}
	hostMember, err := insertMember(ctx, tx, created.ID, actorReference(host))
	if err != nil {
		return Room{}, err
	}
	created.HostMemberID = hostMember.ID
	created.Members = []RoomMember{hostMember}
	if err := tx.Commit(ctx); err != nil {
		return Room{}, fmt.Errorf("commit room creation: %w", err)
	}
	return created, nil
}

func (s *Service) Get(ctx context.Context, roomID string) (*Room, error) {
	var room Room
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, game_version_id::text, host_user_id::text, title, max_players, status, sequence, session_json, created_at, updated_at
		FROM rooms WHERE id = $1`, roomID).
		Scan(&room.ID, &room.GameVersionID, &room.HostUserID, &room.Title, &room.MaxPlayers, &room.Status, &room.Sequence, &raw, &room.CreatedAt, &room.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	if len(raw) > 0 {
		var session game.GameSession
		if err := json.Unmarshal(raw, &session); err != nil {
			return nil, fmt.Errorf("decode room session: %w", err)
		}
		room.Session = &session
	}
	members, err := listMembers(ctx, s.pool, room.ID)
	if err != nil {
		return nil, err
	}
	room.Members = members
	for _, member := range members {
		if member.ActorKind == "user" && member.ActorID == room.HostUserID {
			room.HostMemberID = member.ID
			break
		}
	}
	return &room, nil
}

func (s *Service) Join(ctx context.Context, actor identity.Actor, roomID string) (RoomMember, error) {
	ref := actorReference(actor)
	if ref.id == "" {
		return RoomMember{}, ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RoomMember{}, fmt.Errorf("begin room join: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var maxPlayers int16
	var status string
	err = tx.QueryRow(ctx, `SELECT max_players, status FROM rooms WHERE id = $1 FOR UPDATE`, roomID).Scan(&maxPlayers, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return RoomMember{}, ErrNotFound
	}
	if err != nil {
		return RoomMember{}, fmt.Errorf("lock room for join: %w", err)
	}
	if status != StatusLobby {
		return RoomMember{}, ErrRoomNotJoinable
	}
	var alreadyMember bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND actor_kind = $2 AND actor_id = $3)`, roomID, ref.kind, ref.id).Scan(&alreadyMember); err != nil {
		return RoomMember{}, fmt.Errorf("check room membership: %w", err)
	}
	if alreadyMember {
		return RoomMember{}, ErrAlreadyMember
	}
	var memberCount int16
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM room_members WHERE room_id = $1`, roomID).Scan(&memberCount); err != nil {
		return RoomMember{}, fmt.Errorf("count room members: %w", err)
	}
	if memberCount >= maxPlayers {
		return RoomMember{}, ErrRoomFull
	}
	member, err := insertMember(ctx, tx, roomID, ref)
	if err != nil {
		return RoomMember{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE rooms SET sequence = sequence + 1, updated_at = now() WHERE id = $1`, roomID); err != nil {
		return RoomMember{}, fmt.Errorf("touch room after join: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return RoomMember{}, fmt.Errorf("commit room join: %w", err)
	}
	return member, nil
}

// Start turns a lobby into a multiplayer session using only its pinned published
// definition and the currently persisted room members.
func (s *Service) Start(ctx context.Context, actor identity.Actor, roomID string) (*Room, error) {
	return s.StartWithCommand(ctx, actor, roomID, Command{})
}

// StartWithCommand starts a room once for a client command ID and returns the
// original persisted room snapshot when the command is retried.
func (s *Service) StartWithCommand(ctx context.Context, actor identity.Actor, roomID string, command Command) (*Room, error) {
	if err := validateCommand(command, "start"); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin room start: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := assertHost(ctx, tx, actor, roomID); err != nil {
		return nil, err
	}
	stored, err := loadRoom(ctx, tx, roomID, true)
	if err != nil {
		return nil, err
	}
	if command.ID != "" {
		receipt, err := commandReceipt(ctx, tx, stored.ID, actorReference(actor), command)
		if err != nil {
			return nil, err
		}
		if receipt != nil {
			var duplicate Room
			if err := json.Unmarshal(receipt.Payload, &duplicate); err != nil {
				return nil, fmt.Errorf("decode room command receipt: %w", err)
			}
			duplicate.StoredEvent = receipt
			duplicate.Duplicate = true
			return &duplicate, nil
		}
	}
	if stored.Status != StatusLobby {
		return nil, ErrRoomNotJoinable
	}
	if len(stored.Members) < 2 {
		return nil, ErrRoomNotReady
	}
	version, err := s.catalog.GetVersionByID(ctx, stored.GameVersionID)
	if err != nil {
		return nil, fmt.Errorf("load pinned game version: %w", err)
	}
	if version == nil {
		return nil, ErrGameVersionNotFound
	}
	configs := make([]game.PlayerConfig, len(stored.Members))
	for i, member := range stored.Members {
		configs[i] = game.PlayerConfig{Name: member.DisplayName, Color: roomPlayerColors[i%len(roomPlayerColors)]}
	}
	session := game.StartSession(&version.Definition, configs)
	session.Mode = "multiplayer"
	startEvent := game.NewGameEvent("room_started", "Room game started", nil)
	session.State.Log = append(session.State.Log, startEvent)
	for i := range stored.Members {
		stored.Members[i].PlayerID = session.State.Players[i].ID
		if _, err := tx.Exec(ctx, `UPDATE room_members SET player_id = $3 WHERE room_id = $1 AND id = $2`, roomID, stored.Members[i].ID, stored.Members[i].PlayerID); err != nil {
			return nil, fmt.Errorf("assign room player slot: %w", err)
		}
	}
	stored.Session = session
	stored.Status = StatusActive
	if err := persistSession(ctx, tx, stored); err != nil {
		return nil, err
	}
	event, err := recordEvent(ctx, tx, stored.ID, stored.Sequence, "room_state", stored)
	if err != nil {
		return nil, err
	}
	if command.ID != "" {
		if err := recordCommandReceipt(ctx, tx, stored.ID, actorReference(actor), command, event); err != nil {
			return nil, err
		}
	}
	stored.StoredEvent = &event
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit room start: %w", err)
	}
	return stored, nil
}

// Roll validates the current player, resolves movement server-side and persists
// one sequenced room transition.
func (s *Service) Roll(ctx context.Context, actor identity.Actor, roomID string) (Transition, error) {
	return s.RollWithCommand(ctx, actor, roomID, Command{})
}

// RollWithCommand performs a server-authoritative roll once for a client
// command ID and returns the original persisted transition on a retry.
func (s *Service) RollWithCommand(ctx context.Context, actor identity.Actor, roomID string, command Command) (Transition, error) {
	if err := validateCommand(command, "roll"); err != nil {
		return Transition{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Transition{}, fmt.Errorf("begin room roll: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	stored, err := loadRoom(ctx, tx, roomID, true)
	if err != nil {
		return Transition{}, err
	}
	member, err := currentMember(actor, stored.Members)
	if err != nil {
		return Transition{}, err
	}
	if command.ID != "" {
		receipt, err := commandReceipt(ctx, tx, stored.ID, actorReference(actor), command)
		if err != nil {
			return Transition{}, err
		}
		if receipt != nil {
			var transition Transition
			if err := json.Unmarshal(receipt.Payload, &transition); err != nil {
				return Transition{}, fmt.Errorf("decode room command receipt: %w", err)
			}
			transition.StoredEvent = receipt
			transition.Duplicate = true
			return transition, nil
		}
	}
	if stored.Status != StatusActive || stored.Session == nil || stored.Session.State.Status != "active" {
		return Transition{}, ErrGameNotActive
	}
	if stored.Session.State.PendingAction != nil {
		return Transition{}, ErrPendingAction
	}
	if stored.Session.CurrentPlayer().ID != member.PlayerID {
		return Transition{}, ErrNotYourTurn
	}
	roll, diceEvent := stored.Session.RollDice()
	events := []game.GameEvent{*diceEvent}
	moveEvents := stored.Session.MoveCurrentPlayer(roll.Total, roll.Rolls, roll.Total)
	events = append(events, moveEvents...)
	stored.Session.State.Log = append(stored.Session.State.Log, events...)
	if stored.Session.State.Status == "finished" {
		stored.Status = StatusFinished
	}
	if err := persistSession(ctx, tx, stored); err != nil {
		return Transition{}, err
	}
	transition := Transition{RoomID: stored.ID, Sequence: stored.Sequence, Session: stored.Session, Events: events}
	event, err := recordEvent(ctx, tx, stored.ID, stored.Sequence, "room_event", transition)
	if err != nil {
		return Transition{}, err
	}
	if command.ID != "" {
		if err := recordCommandReceipt(ctx, tx, stored.ID, actorReference(actor), command, event); err != nil {
			return Transition{}, err
		}
	}
	transition.StoredEvent = &event
	if err := tx.Commit(ctx); err != nil {
		return Transition{}, fmt.Errorf("commit room roll: %w", err)
	}
	return transition, nil
}

// ResolveAction resolves the current member's server-side pending choice.
func (s *Service) ResolveAction(ctx context.Context, actor identity.Actor, roomID, actionID string) (Transition, error) {
	return s.ResolveActionWithCommand(ctx, actor, roomID, actionID, Command{})
}

// ResolveActionWithCommand resolves one pending action for a client command ID
// and returns the original persisted transition on a retry.
func (s *Service) ResolveActionWithCommand(ctx context.Context, actor identity.Actor, roomID, actionID string, command Command) (Transition, error) {
	if err := validateCommand(command, "action"); err != nil {
		return Transition{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Transition{}, fmt.Errorf("begin room action: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	stored, err := loadRoom(ctx, tx, roomID, true)
	if err != nil {
		return Transition{}, err
	}
	member, err := currentMember(actor, stored.Members)
	if err != nil {
		return Transition{}, err
	}
	if command.ID != "" {
		receipt, err := commandReceipt(ctx, tx, stored.ID, actorReference(actor), command)
		if err != nil {
			return Transition{}, err
		}
		if receipt != nil {
			var transition Transition
			if err := json.Unmarshal(receipt.Payload, &transition); err != nil {
				return Transition{}, fmt.Errorf("decode room command receipt: %w", err)
			}
			transition.StoredEvent = receipt
			transition.Duplicate = true
			return transition, nil
		}
	}
	if stored.Status != StatusActive || stored.Session == nil || stored.Session.State.Status != "active" {
		return Transition{}, ErrGameNotActive
	}
	if stored.Session.State.PendingAction == nil {
		return Transition{}, ErrNoPendingAction
	}
	if stored.Session.State.PendingAction.PlayerID != member.PlayerID {
		return Transition{}, ErrNotYourTurn
	}
	events, err := stored.Session.ResolvePendingAction(actionID)
	if err != nil {
		return Transition{}, fmt.Errorf("resolve room action: %w", err)
	}
	stored.Session.State.Log = append(stored.Session.State.Log, events...)
	if stored.Session.State.Status == "finished" {
		stored.Status = StatusFinished
	}
	if err := persistSession(ctx, tx, stored); err != nil {
		return Transition{}, err
	}
	transition := Transition{RoomID: stored.ID, Sequence: stored.Sequence, Session: stored.Session, Events: events}
	event, err := recordEvent(ctx, tx, stored.ID, stored.Sequence, "room_event", transition)
	if err != nil {
		return Transition{}, err
	}
	if command.ID != "" {
		if err := recordCommandReceipt(ctx, tx, stored.ID, actorReference(actor), command, event); err != nil {
			return Transition{}, err
		}
	}
	transition.StoredEvent = &event
	if err := tx.Commit(ctx); err != nil {
		return Transition{}, fmt.Errorf("commit room action: %w", err)
	}
	return transition, nil
}

func (s *Service) SendMessage(ctx context.Context, actor identity.Actor, roomID, body string) (RoomMessage, error) {
	return s.SendMessageWithCommand(ctx, actor, roomID, body, Command{})
}

// SendMessageWithCommand stores a room chat message once for a client command
// ID and returns the original persisted message when the command is retried.
func (s *Service) SendMessageWithCommand(ctx context.Context, actor identity.Actor, roomID, body string, command Command) (RoomMessage, error) {
	if err := validateCommand(command, "chat"); err != nil {
		return RoomMessage{}, err
	}
	body = strings.TrimSpace(body)
	if len([]rune(body)) < 1 || len([]rune(body)) > 1000 {
		return RoomMessage{}, ErrInvalidMessage
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RoomMessage{}, fmt.Errorf("begin chat message: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := loadRoom(ctx, tx, roomID, true); err != nil {
		return RoomMessage{}, err
	}
	members, err := listMembers(ctx, tx, roomID)
	if err != nil {
		return RoomMessage{}, err
	}
	member, err := currentMember(actor, members)
	if err != nil {
		return RoomMessage{}, err
	}
	if command.ID != "" {
		receipt, err := commandReceipt(ctx, tx, roomID, actorReference(actor), command)
		if err != nil {
			return RoomMessage{}, err
		}
		if receipt != nil {
			var message RoomMessage
			if err := json.Unmarshal(receipt.Payload, &message); err != nil {
				return RoomMessage{}, fmt.Errorf("decode room command receipt: %w", err)
			}
			message.StoredEvent = receipt
			message.Duplicate = true
			return message, nil
		}
	}
	if member.MutedAt != nil {
		return RoomMessage{}, ErrMemberMuted
	}
	var message RoomMessage
	err = tx.QueryRow(ctx, `
		INSERT INTO room_messages (room_id, member_id, body)
		VALUES ($1, $2, $3)
		RETURNING id::text, room_id::text, member_id::text, body, created_at`, roomID, member.ID, body).
		Scan(&message.ID, &message.RoomID, &message.MemberID, &message.Body, &message.CreatedAt)
	if err != nil {
		return RoomMessage{}, fmt.Errorf("insert chat message: %w", err)
	}
	message.DisplayName = member.DisplayName
	if err := tx.QueryRow(ctx, `UPDATE rooms SET sequence = sequence + 1, updated_at = now() WHERE id = $1 RETURNING sequence`, roomID).Scan(&message.Sequence); err != nil {
		return RoomMessage{}, fmt.Errorf("sequence chat message: %w", err)
	}
	event, err := recordEvent(ctx, tx, roomID, message.Sequence, "chat_message", message)
	if err != nil {
		return RoomMessage{}, err
	}
	if command.ID != "" {
		if err := recordCommandReceipt(ctx, tx, roomID, actorReference(actor), command, event); err != nil {
			return RoomMessage{}, err
		}
	}
	message.StoredEvent = &event
	if err := tx.Commit(ctx); err != nil {
		return RoomMessage{}, fmt.Errorf("commit chat message: %w", err)
	}
	return message, nil
}

func (s *Service) ListMessages(ctx context.Context, actor identity.Actor, roomID string, limit int) ([]RoomMessage, error) {
	stored, err := s.Get(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if _, err := currentMember(actor, stored.Members); err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, room_id, member_id, display_name, body, created_at
		FROM (
			SELECT messages.id::text AS id, messages.room_id::text AS room_id, messages.member_id::text AS member_id,
				members.display_name, messages.body, messages.created_at
			FROM room_messages AS messages
			JOIN room_members AS members ON members.id = messages.member_id
			WHERE messages.room_id = $1
			ORDER BY messages.created_at DESC, messages.id DESC
			LIMIT $2
		) AS latest
		ORDER BY created_at, id`, roomID, limit)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	defer rows.Close()
	messages := make([]RoomMessage, 0)
	for rows.Next() {
		var message RoomMessage
		if err := rows.Scan(&message.ID, &message.RoomID, &message.MemberID, &message.DisplayName, &message.Body, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat messages: %w", err)
	}
	return messages, nil
}

func (s *Service) Mute(ctx context.Context, actor identity.Actor, roomID, memberID string, muted bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin member moderation: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := assertHost(ctx, tx, actor, roomID); err != nil {
		return err
	}
	command, err := tx.Exec(ctx, `
		UPDATE room_members
		SET muted_at = CASE WHEN $3 THEN now() ELSE NULL END
		WHERE room_id = $1 AND id = $2`, roomID, memberID, muted)
	if err != nil {
		return fmt.Errorf("mute room member: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE rooms SET sequence = sequence + 1, updated_at = now() WHERE id = $1`, roomID); err != nil {
		return fmt.Errorf("touch room after mute: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit member moderation: %w", err)
	}
	return nil
}

func (s *Service) Remove(ctx context.Context, actor identity.Actor, roomID, memberID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin member removal: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := assertHost(ctx, tx, actor, roomID); err != nil {
		return err
	}
	var kind, actorID string
	err = tx.QueryRow(ctx, `SELECT actor_kind, actor_id::text FROM room_members WHERE room_id = $1 AND id = $2`, roomID, memberID).Scan(&kind, &actorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMemberNotFound
	}
	if err != nil {
		return fmt.Errorf("get member for removal: %w", err)
	}
	if kind == "user" && actor.User != nil && actorID == actor.User.ID {
		return ErrCannotRemoveHost
	}
	if _, err := tx.Exec(ctx, `DELETE FROM room_members WHERE room_id = $1 AND id = $2`, roomID, memberID); err != nil {
		return fmt.Errorf("remove room member: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE rooms SET sequence = sequence + 1, updated_at = now() WHERE id = $1`, roomID); err != nil {
		return fmt.Errorf("touch room after removal: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit member removal: %w", err)
	}
	return nil
}

var roomPlayerColors = []string{
	"#e74c3c", "#3498db", "#2ecc71", "#f39c12",
	"#9b59b6", "#1abc9c", "#e67e22", "#34495e",
}

type roomQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadRoom(ctx context.Context, queryer roomQuerier, roomID string, lock bool) (*Room, error) {
	query := `
		SELECT id::text, game_version_id::text, host_user_id::text, title, max_players, status, sequence, session_json, created_at, updated_at
		FROM rooms WHERE id = $1`
	if lock {
		query += " FOR UPDATE"
	}
	var stored Room
	var raw []byte
	err := queryer.QueryRow(ctx, query, roomID).
		Scan(&stored.ID, &stored.GameVersionID, &stored.HostUserID, &stored.Title, &stored.MaxPlayers, &stored.Status, &stored.Sequence, &raw, &stored.CreatedAt, &stored.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	if len(raw) > 0 {
		var session game.GameSession
		if err := json.Unmarshal(raw, &session); err != nil {
			return nil, fmt.Errorf("decode room session: %w", err)
		}
		stored.Session = &session
	}
	members, err := listMembers(ctx, queryer, stored.ID)
	if err != nil {
		return nil, err
	}
	stored.Members = members
	for _, member := range members {
		if member.ActorKind == "user" && member.ActorID == stored.HostUserID {
			stored.HostMemberID = member.ID
			break
		}
	}
	return &stored, nil
}

func persistSession(ctx context.Context, tx pgx.Tx, stored *Room) error {
	raw, err := json.Marshal(stored.Session)
	if err != nil {
		return fmt.Errorf("encode room session: %w", err)
	}
	err = tx.QueryRow(ctx, `
		UPDATE rooms
		SET status = $2, session_json = $3::jsonb, sequence = sequence + 1, updated_at = now()
		WHERE id = $1
		RETURNING sequence, updated_at`, stored.ID, stored.Status, string(raw)).
		Scan(&stored.Sequence, &stored.UpdatedAt)
	if err != nil {
		return fmt.Errorf("persist room session: %w", err)
	}
	return nil
}

func recordEvent(ctx context.Context, tx pgx.Tx, roomID string, sequence uint64, eventType string, payload any) (StoredEvent, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return StoredEvent{}, fmt.Errorf("encode room event: %w", err)
	}
	var event StoredEvent
	err = tx.QueryRow(ctx, `
		INSERT INTO room_events (room_id, sequence, event_type, payload_json)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING room_id::text, sequence, event_type, payload_json, created_at`, roomID, sequence, eventType, string(raw)).
		Scan(&event.RoomID, &event.Sequence, &event.Type, &event.Payload, &event.CreatedAt)
	if err != nil {
		return StoredEvent{}, fmt.Errorf("record room event: %w", err)
	}
	return event, nil
}

func commandReceipt(ctx context.Context, tx pgx.Tx, roomID string, actor actorRef, command Command) (*StoredEvent, error) {
	var event StoredEvent
	var receiptType string
	err := tx.QueryRow(ctx, `
		SELECT events.room_id::text, events.sequence, events.event_type, events.payload_json, events.created_at, receipts.command_type
		FROM room_command_receipts AS receipts
		JOIN room_events AS events ON events.room_id = receipts.room_id AND events.sequence = receipts.event_sequence
		WHERE receipts.room_id = $1 AND receipts.actor_kind = $2 AND receipts.actor_id = $3 AND receipts.command_id = $4`,
		roomID, actor.kind, actor.id, command.ID).
		Scan(&event.RoomID, &event.Sequence, &event.Type, &event.Payload, &event.CreatedAt, &receiptType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load room command receipt: %w", err)
	}
	if receiptType != command.Type {
		return nil, fmt.Errorf("%w: command ID was already used for %s", ErrInvalidCommand, receiptType)
	}
	return &event, nil
}

func recordCommandReceipt(ctx context.Context, tx pgx.Tx, roomID string, actor actorRef, command Command, event StoredEvent) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO room_command_receipts (room_id, actor_kind, actor_id, command_id, command_type, event_sequence)
		VALUES ($1, $2, $3, $4, $5, $6)`, roomID, actor.kind, actor.id, command.ID, command.Type, event.Sequence)
	if err != nil {
		return fmt.Errorf("record room command receipt: %w", err)
	}
	return nil
}

// EventsSince returns only a contiguous event range ending at the current room
// sequence. A caller must use a snapshot instead when the journal has a gap.
func (s *Service) EventsSince(ctx context.Context, actor identity.Actor, roomID string, since uint64) ([]StoredEvent, bool, error) {
	stored, err := s.Get(ctx, roomID)
	if err != nil {
		return nil, false, err
	}
	if _, err := currentMember(actor, stored.Members); err != nil {
		return nil, false, err
	}
	if since >= stored.Sequence {
		return []StoredEvent{}, true, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT room_id::text, sequence, event_type, payload_json, created_at
		FROM room_events
		WHERE room_id = $1 AND sequence > $2 AND sequence <= $3
		ORDER BY sequence
		LIMIT $4`, roomID, since, stored.Sequence, maxJournalReplayEvents+1)
	if err != nil {
		return nil, false, fmt.Errorf("list room events: %w", err)
	}
	defer rows.Close()
	events := make([]StoredEvent, 0)
	expected := since + 1
	for rows.Next() {
		var event StoredEvent
		if err := rows.Scan(&event.RoomID, &event.Sequence, &event.Type, &event.Payload, &event.CreatedAt); err != nil {
			return nil, false, fmt.Errorf("scan room event: %w", err)
		}
		if event.Sequence != expected {
			return nil, false, nil
		}
		events = append(events, event)
		if len(events) > maxJournalReplayEvents {
			return nil, false, nil
		}
		expected++
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate room events: %w", err)
	}
	return events, expected == stored.Sequence+1, nil
}

func currentMember(actor identity.Actor, members []RoomMember) (*RoomMember, error) {
	ref := actorReference(actor)
	for i := range members {
		if members[i].ActorKind == ref.kind && members[i].ActorID == ref.id {
			return &members[i], nil
		}
	}
	return nil, ErrNotMember
}

type actorRef struct {
	kind        string
	id          string
	displayName string
}

func actorReference(actor identity.Actor) actorRef {
	if actor.User != nil {
		return actorRef{kind: "user", id: actor.User.ID, displayName: actor.User.DisplayName}
	}
	if actor.Guest != nil {
		return actorRef{kind: "guest", id: actor.Guest.ID, displayName: actor.Guest.DisplayName}
	}
	return actorRef{}
}

func validateCreateInput(input *CreateInput) error {
	input.Title = strings.TrimSpace(input.Title)
	if len([]rune(input.Title)) < 1 || len([]rune(input.Title)) > 120 {
		return fmt.Errorf("room title must contain 1 to 120 characters")
	}
	if input.MaxPlayers < 2 || input.MaxPlayers > 8 {
		return fmt.Errorf("room max players must be between 2 and 8")
	}
	return nil
}

func validateCommand(command Command, expectedType string) error {
	if command.ID == "" {
		return nil
	}
	if command.Type != expectedType || !isUUID(command.ID) {
		return fmt.Errorf("%w: %s command requires a UUID ID", ErrInvalidCommand, expectedType)
	}
	return nil
}

func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func insertMember(ctx context.Context, tx pgx.Tx, roomID string, ref actorRef) (RoomMember, error) {
	var member RoomMember
	err := tx.QueryRow(ctx, `
		INSERT INTO room_members (room_id, actor_kind, actor_id, display_name)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, room_id::text, actor_kind, actor_id::text, COALESCE(player_id, ''), display_name, muted_at, joined_at`,
		roomID, ref.kind, ref.id, ref.displayName).
		Scan(&member.ID, &member.RoomID, &member.ActorKind, &member.ActorID, &member.PlayerID, &member.DisplayName, &member.MutedAt, &member.JoinedAt)
	if err != nil {
		return RoomMember{}, fmt.Errorf("insert room member: %w", err)
	}
	return member, nil
}

type memberQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func listMembers(ctx context.Context, queryer memberQuerier, roomID string) ([]RoomMember, error) {
	rows, err := queryer.Query(ctx, `
		SELECT id::text, room_id::text, actor_kind, actor_id::text, COALESCE(player_id, ''), display_name, muted_at, joined_at
		FROM room_members WHERE room_id = $1 ORDER BY joined_at, id`, roomID)
	if err != nil {
		return nil, fmt.Errorf("list room members: %w", err)
	}
	defer rows.Close()
	members := make([]RoomMember, 0)
	for rows.Next() {
		var member RoomMember
		if err := rows.Scan(&member.ID, &member.RoomID, &member.ActorKind, &member.ActorID, &member.PlayerID, &member.DisplayName, &member.MutedAt, &member.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan room member: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate room members: %w", err)
	}
	return members, nil
}

func assertHost(ctx context.Context, tx pgx.Tx, actor identity.Actor, roomID string) error {
	if actor.User == nil {
		return ErrNotHost
	}
	var hostID string
	err := tx.QueryRow(ctx, `SELECT host_user_id::text FROM rooms WHERE id = $1 FOR UPDATE`, roomID).Scan(&hostID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock room for moderation: %w", err)
	}
	if hostID != actor.User.ID {
		return ErrNotHost
	}
	return nil
}
