package catalog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rollboard/internal/game"
)

var (
	ErrNotFound          = errors.New("game not found")
	ErrInvalidDefinition = errors.New("game definition is invalid")
)

type Service struct{ pool *pgxpool.Pool }

type Game struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	OwnerUserID string    `json:"ownerUserId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Version struct {
	ID            string              `json:"id"`
	GameID        string              `json:"gameId"`
	VersionNumber int                 `json:"versionNumber"`
	Definition    game.GameDefinition `json:"definition"`
	PublishedAt   time.Time           `json:"publishedAt"`
}

type Draft struct {
	GameID     string              `json:"gameId"`
	Definition game.GameDefinition `json:"definition"`
	UpdatedAt  time.Time           `json:"updatedAt"`
}

func NewService(pool *pgxpool.Pool) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("catalog service requires a PostgreSQL pool")
	}
	return &Service{pool: pool}, nil
}

func (s *Service) CreateGame(ctx context.Context, ownerID string, definition game.GameDefinition) (Game, error) {
	if strings.TrimSpace(ownerID) == "" {
		return Game{}, fmt.Errorf("owner ID is required")
	}
	definition.Title = strings.TrimSpace(definition.Title)
	if definition.Title == "" {
		return Game{}, fmt.Errorf("game title is required")
	}
	id, err := newUUID()
	if err != nil {
		return Game{}, err
	}
	definition.ID = id
	definition.Version = 1
	raw, err := json.Marshal(definition)
	if err != nil {
		return Game{}, fmt.Errorf("encode draft: %w", err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Game{}, fmt.Errorf("begin game creation: %w", err)
	}
	defer tx.Rollback(ctx)
	var created Game
	err = tx.QueryRow(ctx, `
		INSERT INTO games (id, title, version, definition_json, owner_user_id)
		VALUES ($1, $2, 1, $3::jsonb, $4)
		RETURNING id, title, owner_user_id::text, created_at, updated_at`, id, definition.Title, string(raw), ownerID).
		Scan(&created.ID, &created.Title, &created.OwnerUserID, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return Game{}, fmt.Errorf("insert game: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO game_drafts (game_id, definition_json) VALUES ($1, $2::jsonb)`, id, string(raw)); err != nil {
		return Game{}, fmt.Errorf("insert draft: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Game{}, fmt.Errorf("commit game creation: %w", err)
	}
	return created, nil
}

func (s *Service) SaveDraft(ctx context.Context, ownerID, gameID string, definition game.GameDefinition) error {
	// The identity and version of a draft belong to the server. A client that
	// omits them (or sends somebody else's) must not be able to store a draft
	// that later fails the sessions.game_version check or points at another game.
	definition.ID = gameID
	if definition.Version < 1 {
		definition.Version = 1
	}
	raw, err := json.Marshal(definition)
	if err != nil {
		return fmt.Errorf("encode draft: %w", err)
	}
	command, err := s.pool.Exec(ctx, `
		UPDATE game_drafts AS draft
		SET definition_json = $1::jsonb, updated_at = now()
		FROM games
		WHERE draft.game_id = games.id AND games.id = $2 AND games.owner_user_id = $3`, string(raw), gameID, ownerID)
	if err != nil {
		return fmt.Errorf("save draft: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) GetDraft(ctx context.Context, ownerID, gameID string) (*Draft, error) {
	var draft Draft
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT draft.game_id, draft.definition_json, draft.updated_at
		FROM game_drafts AS draft
		JOIN games ON games.id = draft.game_id
		WHERE draft.game_id = $1 AND games.owner_user_id = $2`, gameID, ownerID).
		Scan(&draft.GameID, &raw, &draft.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get draft: %w", err)
	}
	if err := json.Unmarshal(raw, &draft.Definition); err != nil {
		return nil, fmt.Errorf("decode draft: %w", err)
	}
	return &draft, nil
}

// ListGames returns only the caller's drafts, most recently updated first.
func (s *Service) ListGames(ctx context.Context, ownerID string) ([]Game, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, owner_user_id::text, created_at, updated_at
		FROM games
		WHERE owner_user_id = $1
		ORDER BY updated_at DESC, id DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list owned games: %w", err)
	}
	defer rows.Close()
	games := make([]Game, 0)
	for rows.Next() {
		var listed Game
		if err := rows.Scan(&listed.ID, &listed.Title, &listed.OwnerUserID, &listed.CreatedAt, &listed.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan owned game: %w", err)
		}
		games = append(games, listed)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate owned games: %w", err)
	}
	return games, nil
}

// ListVersions returns immutable published versions belonging to the caller.
func (s *Service) ListVersions(ctx context.Context, ownerID string) ([]Version, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT version.id::text, version.game_id, version.version_number, version.definition_json, version.published_at
		FROM game_versions AS version
		JOIN games ON games.id = version.game_id
		WHERE games.owner_user_id = $1
		ORDER BY version.published_at DESC, version.id DESC`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list owned game versions: %w", err)
	}
	defer rows.Close()
	versions := make([]Version, 0)
	for rows.Next() {
		var listed Version
		var raw []byte
		if err := rows.Scan(&listed.ID, &listed.GameID, &listed.VersionNumber, &raw, &listed.PublishedAt); err != nil {
			return nil, fmt.Errorf("scan owned game version: %w", err)
		}
		if err := json.Unmarshal(raw, &listed.Definition); err != nil {
			return nil, fmt.Errorf("decode owned game version: %w", err)
		}
		versions = append(versions, listed)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate owned game versions: %w", err)
	}
	return versions, nil
}

func (s *Service) Publish(ctx context.Context, ownerID, gameID string) (Version, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Version{}, fmt.Errorf("begin publication: %w", err)
	}
	defer tx.Rollback(ctx)
	var raw []byte
	err = tx.QueryRow(ctx, `
		SELECT draft.definition_json
		FROM games
		JOIN game_drafts AS draft ON draft.game_id = games.id
		WHERE games.id = $1 AND games.owner_user_id = $2
		FOR UPDATE OF games`, gameID, ownerID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, ErrNotFound
	}
	if err != nil {
		return Version{}, fmt.Errorf("load draft for publication: %w", err)
	}
	var definition game.GameDefinition
	if err := json.Unmarshal(raw, &definition); err != nil {
		return Version{}, fmt.Errorf("decode draft: %w", err)
	}
	if validation := game.ValidateDefinition(&definition); validation != nil {
		return Version{}, fmt.Errorf("%w: %s", ErrInvalidDefinition, strings.Join(validation.Errors, "; "))
	}
	var version Version
	err = tx.QueryRow(ctx, `
		INSERT INTO game_versions (game_id, version_number, definition_json)
		SELECT $1, COALESCE(MAX(version_number), 0) + 1, $2::jsonb
		FROM game_versions
		WHERE game_id = $1
		RETURNING id::text, game_id, version_number, definition_json, published_at`, gameID, string(raw)).
		Scan(&version.ID, &version.GameID, &version.VersionNumber, &raw, &version.PublishedAt)
	if err != nil {
		return Version{}, fmt.Errorf("insert published version: %w", err)
	}
	if err := json.Unmarshal(raw, &version.Definition); err != nil {
		return Version{}, fmt.Errorf("decode published version: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE games SET title = $1, version = $2, definition_json = $3::jsonb, updated_at = now() WHERE id = $4`, definition.Title, version.VersionNumber, string(raw), gameID); err != nil {
		return Version{}, fmt.Errorf("update published game: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, fmt.Errorf("commit publication: %w", err)
	}
	return version, nil
}

func (s *Service) GetVersion(ctx context.Context, gameID string, number int) (*Version, error) {
	var version Version
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, game_id, version_number, definition_json, published_at
		FROM game_versions WHERE game_id = $1 AND version_number = $2`, gameID, number).
		Scan(&version.ID, &version.GameID, &version.VersionNumber, &raw, &version.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get game version: %w", err)
	}
	if err := json.Unmarshal(raw, &version.Definition); err != nil {
		return nil, fmt.Errorf("decode published version: %w", err)
	}
	return &version, nil
}

// GetVersionByID returns an immutable published version by its storage ID.
// Rooms use this ID rather than a mutable game draft or legacy game record.
func (s *Service) GetVersionByID(ctx context.Context, id string) (*Version, error) {
	var version Version
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, game_id, version_number, definition_json, published_at
		FROM game_versions WHERE id = $1`, id).
		Scan(&version.ID, &version.GameID, &version.VersionNumber, &raw, &version.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get game version by ID: %w", err)
	}
	if err := json.Unmarshal(raw, &version.Definition); err != nil {
		return nil, fmt.Errorf("decode published version: %w", err)
	}
	return &version, nil
}

func newUUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate game ID: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}
