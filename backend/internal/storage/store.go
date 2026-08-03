package storage

import (
	"context"

	"rollboard/internal/game"
)

type GameSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Version   int    `json:"version"`
	UpdatedAt string `json:"updatedAt"`
}

// Store is the persistence surface the HTTP API is allowed to reach directly.
// Game rows are deliberately absent: every read and write of a game definition
// must go through the catalog service so that ownership is always enforced.
type Store interface {
	Close()
	Ping(context.Context) error
	SaveSession(ctx context.Context, ownerUserID string, session *game.GameSession) error
	// GetSession returns the session only when ownerUserID owns it, and
	// (nil, nil) otherwise, so callers cannot distinguish "not yours" from
	// "does not exist".
	GetSession(ctx context.Context, id, ownerUserID string) (*game.GameSession, error)
}
