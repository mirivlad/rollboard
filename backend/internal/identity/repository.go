package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

var (
	ErrEmailTaken         = errors.New("email is already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrGuestClaimed       = errors.New("guest has already been claimed")
)

func (r *Repository) Register(ctx context.Context, input RegistrationInput) (User, error) {
	if err := input.Validate(); err != nil {
		return User{}, err
	}
	hash, err := HashPassword(input.Password)
	if err != nil {
		return User{}, err
	}
	var user User
	err = r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id::text, email, display_name, created_at`, input.Email, hash, input.DisplayName).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.CreatedAt)
	if err == nil {
		return user, nil
	}
	if isUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	return User{}, fmt.Errorf("insert user: %w", err)
}

func (r *Repository) CreateGuest(ctx context.Context, displayName string) (Guest, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || len([]rune(displayName)) > 64 {
		return Guest{}, fmt.Errorf("display name must contain 1 to 64 characters")
	}

	var guest Guest
	err := r.pool.QueryRow(ctx, `
		INSERT INTO guest_identities (display_name)
		VALUES ($1)
		RETURNING id::text, display_name, created_at`, displayName).
		Scan(&guest.ID, &guest.DisplayName, &guest.CreatedAt)
	if err != nil {
		return Guest{}, fmt.Errorf("insert guest: %w", err)
	}
	return guest, nil
}

func (r *Repository) Authenticate(ctx context.Context, email, password string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var user User
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, email, display_name, password_hash, created_at
		FROM users
		WHERE email = $1`, email).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.PasswordHash, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("get user for authentication: %w", err)
	}
	valid := VerifyPassword(user.PasswordHash, password)
	if !valid {
		return User{}, ErrInvalidCredentials
	}
	user.PasswordHash = ""
	return user, nil
}

func (r *Repository) ClaimGuest(ctx context.Context, guestID, userID string) (Guest, error) {
	var guest Guest
	err := r.pool.QueryRow(ctx, `
		UPDATE guest_identities
		SET claimed_by_user_id = $2, claimed_at = now()
		WHERE id = $1 AND claimed_by_user_id IS NULL
		RETURNING id::text, display_name, claimed_by_user_id::text, created_at, claimed_at`, guestID, userID).
		Scan(&guest.ID, &guest.DisplayName, &guest.ClaimedByUserID, &guest.CreatedAt, &guest.ClaimedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Guest{}, ErrGuestClaimed
	}
	if err != nil {
		return Guest{}, fmt.Errorf("claim guest: %w", err)
	}
	return guest, nil
}

func (r *Repository) CreateGuestSession(ctx context.Context, guestID string, expiresAt time.Time) (Session, string, error) {
	if strings.TrimSpace(guestID) == "" {
		return Session{}, "", fmt.Errorf("guest ID is required")
	}
	if !expiresAt.After(time.Now()) {
		return Session{}, "", fmt.Errorf("session expiry must be in the future")
	}
	token, digest, err := NewToken()
	if err != nil {
		return Session{}, "", err
	}

	var session Session
	err = r.pool.QueryRow(ctx, `
		INSERT INTO auth_sessions (guest_id, token_digest, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id::text, guest_id::text, token_digest, expires_at, created_at`, guestID, digest, expiresAt).
		Scan(&session.ID, &session.GuestID, &session.TokenDigest, &session.ExpiresAt, &session.CreatedAt)
	if err != nil {
		return Session{}, "", fmt.Errorf("insert guest session: %w", err)
	}
	return session, token, nil
}

func (r *Repository) CreateUserSession(ctx context.Context, userID string, expiresAt time.Time) (Session, string, error) {
	if strings.TrimSpace(userID) == "" {
		return Session{}, "", fmt.Errorf("user ID is required")
	}
	if !expiresAt.After(time.Now()) {
		return Session{}, "", fmt.Errorf("session expiry must be in the future")
	}
	token, digest, err := NewToken()
	if err != nil {
		return Session{}, "", err
	}

	var session Session
	err = r.pool.QueryRow(ctx, `
		INSERT INTO auth_sessions (user_id, token_digest, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id::text, user_id::text, token_digest, expires_at, created_at`, userID, digest, expiresAt).
		Scan(&session.ID, &session.UserID, &session.TokenDigest, &session.ExpiresAt, &session.CreatedAt)
	if err != nil {
		return Session{}, "", fmt.Errorf("insert user session: %w", err)
	}
	return session, token, nil
}

func (r *Repository) LookupSession(ctx context.Context, token string, now time.Time) (*Actor, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	digest := DigestToken(token)

	var actor Actor
	var user User
	var guest Guest
	var userID, userEmail, userDisplayName *string
	var userCreatedAt *time.Time
	var guestID, guestDisplayName, claimedByUserID *string
	var guestCreatedAt, claimedAt *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT s.id::text,
			u.id::text, u.email, u.display_name, u.created_at,
			g.id::text, g.display_name, g.claimed_by_user_id::text, g.created_at, g.claimed_at
		FROM auth_sessions s
		LEFT JOIN users u ON u.id = s.user_id
		LEFT JOIN guest_identities g ON g.id = s.guest_id
		WHERE s.token_digest = $1 AND s.expires_at > $2`, digest, now).
		Scan(&actor.SessionID, &userID, &userEmail, &userDisplayName, &userCreatedAt,
			&guestID, &guestDisplayName, &claimedByUserID, &guestCreatedAt, &claimedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup session: %w", err)
	}
	if userID != nil {
		user = User{ID: *userID, Email: *userEmail, DisplayName: *userDisplayName, CreatedAt: *userCreatedAt}
		actor.User = &user
		return &actor, nil
	}
	if guestID != nil {
		guest = Guest{ID: *guestID, DisplayName: *guestDisplayName, ClaimedByUserID: claimedByUserID, CreatedAt: *guestCreatedAt, ClaimedAt: claimedAt}
		actor.Guest = &guest
		return &actor, nil
	}
	return nil, fmt.Errorf("session has no actor")
}

func (r *Repository) DeleteSession(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE token_digest = $1`, DigestToken(token))
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func NewRepository(pool *pgxpool.Pool) (*Repository, error) {
	if pool == nil {
		return nil, fmt.Errorf("identity repository requires a PostgreSQL pool")
	}
	return &Repository{pool: pool}, nil
}
