package room

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"rollboard/internal/identity"
)

// Invite is what somebody holding a link is allowed to learn before joining.
//
// It deliberately excludes the member list and the session: the link tells you
// what you are being invited to, not who is already inside or how the game is
// going. Everything beyond this requires membership.
type Invite struct {
	RoomID      string `json:"roomId"`
	Title       string `json:"title"`
	GameTitle   string `json:"gameTitle"`
	Status      string `json:"status"`
	MemberCount int16  `json:"memberCount"`
	MaxPlayers  int16  `json:"maxPlayers"`
	Joinable    bool   `json:"joinable"`
}

// newInviteToken produces a URL-safe capability. 24 bytes because the token is
// the only thing standing between a stranger and the room, and it travels
// through chat applications that may log it.
func newInviteToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate invite token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// ResolveInvite describes the room behind a token without joining it, so the
// interface can show what is being joined before asking for a display name.
func (s *Service) ResolveInvite(ctx context.Context, token string) (*Invite, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	var invite Invite
	err := s.pool.QueryRow(ctx, `
		SELECT rooms.id::text,
		       rooms.title,
		       COALESCE(versions.definition_json->>'title', ''),
		       rooms.status,
		       rooms.max_players,
		       (SELECT count(*) FROM room_members WHERE room_members.room_id = rooms.id)
		FROM rooms
		JOIN game_versions AS versions ON versions.id = rooms.game_version_id
		WHERE rooms.invite_token = $1`, token).
		Scan(&invite.RoomID, &invite.Title, &invite.GameTitle, &invite.Status, &invite.MaxPlayers, &invite.MemberCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolve invite: %w", err)
	}
	invite.Joinable = invite.Status == StatusLobby && invite.MemberCount < invite.MaxPlayers
	return &invite, nil
}

// JoinByInvite exchanges a token for membership.
//
// An actor who is already a member succeeds rather than failing, because
// following your own link a second time should simply put you back in the room.
func (s *Service) JoinByInvite(ctx context.Context, actor identity.Actor, token string) (string, error) {
	invite, err := s.ResolveInvite(ctx, token)
	if err != nil {
		return "", err
	}
	if _, err := s.Join(ctx, actor, invite.RoomID); err != nil {
		if errors.Is(err, ErrAlreadyMember) {
			return invite.RoomID, nil
		}
		return "", err
	}
	return invite.RoomID, nil
}

// InviteToken returns the room's current token. Host only: anyone who can read
// it can hand out access.
func (s *Service) InviteToken(ctx context.Context, actor identity.Actor, roomID string) (string, error) {
	if actor.User == nil {
		return "", ErrAccountRequired
	}
	var token, hostUserID string
	err := s.pool.QueryRow(ctx, `SELECT invite_token, host_user_id::text FROM rooms WHERE id = $1`, roomID).Scan(&token, &hostUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read invite token: %w", err)
	}
	if hostUserID != actor.User.ID {
		return "", ErrNotHost
	}
	return token, nil
}

// RotateInvite issues a new token and invalidates the old one, which is the
// only way to withdraw a link that has been shared too widely.
func (s *Service) RotateInvite(ctx context.Context, actor identity.Actor, roomID string) (string, error) {
	if actor.User == nil {
		return "", ErrAccountRequired
	}
	token, err := newInviteToken()
	if err != nil {
		return "", err
	}
	command, err := s.pool.Exec(ctx,
		`UPDATE rooms SET invite_token = $1, updated_at = now() WHERE id = $2 AND host_user_id = $3`,
		token, roomID, actor.User.ID)
	if err != nil {
		return "", fmt.Errorf("rotate invite token: %w", err)
	}
	if command.RowsAffected() == 0 {
		// Either the room does not exist or the caller does not host it. Both
		// answer the same way so the endpoint cannot confirm a room ID.
		return "", ErrNotFound
	}
	return token, nil
}
