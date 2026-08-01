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

type Store interface {
	Close()
	Ping(context.Context) error
	ListGames(context.Context) ([]GameSummary, error)
	GetGame(context.Context, string) (*game.GameDefinition, error)
	CreateGame(context.Context, *game.GameDefinition) error
	UpdateGame(context.Context, *game.GameDefinition) error
	SaveSession(context.Context, *game.GameSession) error
	GetSession(context.Context, string) (*game.GameSession, error)
}
