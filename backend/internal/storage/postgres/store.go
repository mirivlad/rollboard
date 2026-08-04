package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rollboard/internal/game"
	"rollboard/internal/storage"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string, maxConns int32) (*Store, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL pool configuration: %w", err)
	}
	poolConfig.MaxConns = maxConns
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL pool: %w", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) ListGames(ctx context.Context) ([]storage.GameSummary, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, title, version, updated_at FROM games ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list games: %w", err)
	}
	defer rows.Close()

	games := make([]storage.GameSummary, 0)
	for rows.Next() {
		var game storage.GameSummary
		var updatedAt time.Time
		if err := rows.Scan(&game.ID, &game.Title, &game.Version, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan game summary: %w", err)
		}
		game.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate games: %w", err)
	}
	return games, nil
}

func (s *Store) GetGame(ctx context.Context, id string) (*game.GameDefinition, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT definition_json FROM games WHERE id = $1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get game: %w", err)
	}

	var definition game.GameDefinition
	if err := json.Unmarshal(raw, &definition); err != nil {
		return nil, fmt.Errorf("decode game definition: %w", err)
	}
	return &definition, nil
}

func (s *Store) CreateGame(ctx context.Context, definition *game.GameDefinition) error {
	raw, err := json.Marshal(definition)
	if err != nil {
		return fmt.Errorf("encode game definition: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO games (id, title, version, definition_json)
		VALUES ($1, $2, $3, $4::jsonb)`, definition.ID, definition.Title, definition.Version, string(raw))
	if err != nil {
		return fmt.Errorf("create game: %w", err)
	}
	return nil
}

func (s *Store) UpdateGame(ctx context.Context, definition *game.GameDefinition) error {
	next := *definition
	next.Version++
	raw, err := json.Marshal(&next)
	if err != nil {
		return fmt.Errorf("encode game definition: %w", err)
	}
	err = s.pool.QueryRow(ctx, `
		UPDATE games
		SET title = $1, version = $2, definition_json = $3::jsonb, updated_at = now()
		WHERE id = $4 AND version = $5
		RETURNING version`, next.Title, next.Version, string(raw), next.ID, definition.Version).Scan(&next.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("update game: stale version")
	}
	if err != nil {
		return fmt.Errorf("update game: %w", err)
	}
	definition.Version = next.Version
	return nil
}

func (s *Store) SaveSession(ctx context.Context, ownerUserID string, session *game.GameSession) error {
	raw, err := json.Marshal(game.ForStorage{Session: session})
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	// The owner predicate on UPDATE keeps a second account from overwriting a
	// session it does not own even if it learns the session ID.
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (id, game_id, game_version, session_json, owner_user_id)
		VALUES ($1, $2, $3, $4::jsonb, $5)
		ON CONFLICT (id) DO UPDATE
		SET game_version = EXCLUDED.game_version,
			session_json = EXCLUDED.session_json,
			updated_at = now()
		WHERE sessions.owner_user_id = EXCLUDED.owner_user_id`,
		session.ID, session.GameID, session.GameVersion, string(raw), ownerUserID)
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("save session: not owned by the requesting account")
	}
	return nil
}

func (s *Store) GetSession(ctx context.Context, id, ownerUserID string) (*game.GameSession, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx,
		`SELECT session_json FROM sessions WHERE id = $1 AND owner_user_id = $2`,
		id, ownerUserID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	var session game.GameSession
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}
	return &session, nil
}

var _ storage.Store = (*Store)(nil)
