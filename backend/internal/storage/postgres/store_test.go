package postgres

import (
	"context"
	"os"
	"testing"

	"rollboard/internal/game"
	"rollboard/internal/testdb"
)

func TestStoreRoundTripsGameAndSession(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	definition := testDefinition("postgres-round-trip")

	if err := store.CreateGame(ctx, definition); err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}
	loaded, err := store.GetGame(ctx, definition.ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if loaded == nil || loaded.ID != definition.ID || loaded.Title != definition.Title {
		t.Fatalf("GetGame() = %#v, want game %q", loaded, definition.ID)
	}

	session := game.StartSession(definition, []game.PlayerConfig{{Name: "Alice", Color: "#111111"}, {Name: "Bob", Color: "#222222"}})
	if err := store.SaveSession(ctx, session); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	loadedSession, err := store.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if loadedSession == nil || loadedSession.ID != session.ID || loadedSession.GameID != definition.ID {
		t.Fatalf("GetSession() = %#v, want session %q", loadedSession, session.ID)
	}
}

func TestStoreUpdateIncrementsVersion(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	definition := testDefinition("postgres-version")
	if err := store.CreateGame(ctx, definition); err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}

	definition.Title = "Renamed game"
	if err := store.UpdateGame(ctx, definition); err != nil {
		t.Fatalf("UpdateGame() error = %v", err)
	}
	if definition.Version != 2 {
		t.Fatalf("Version = %d, want 2", definition.Version)
	}
	loaded, err := store.GetGame(ctx, definition.ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if loaded == nil || loaded.Title != "Renamed game" || loaded.Version != 2 {
		t.Fatalf("stored game = %#v, want renamed version 2", loaded)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("ROLLBOARD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ROLLBOARD_TEST_DATABASE_URL is required")
	}
	store, err := New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(store.Close)
	release, err := testdb.AcquireExclusive(context.Background(), store.pool)
	if err != nil {
		t.Fatalf("lock test database: %v", err)
	}
	t.Cleanup(release)
	if _, err := store.pool.Exec(context.Background(), `TRUNCATE sessions, games CASCADE`); err != nil {
		t.Fatalf("clear test tables: %v", err)
	}
	return store
}

func testDefinition(id string) *game.GameDefinition {
	return &game.GameDefinition{
		ID:      id,
		Title:   "PostgreSQL test game",
		Version: 1,
		Board: game.Board{
			Width:    96,
			Height:   96,
			CellSize: 96,
			Cells: []game.CellDefinition{{
				ID:     "start",
				Title:  "Start",
				Type:   "start",
				Visual: game.CellVisual{},
				Fields: map[string]any{},
			}},
		},
		Rules: game.RuleSet{Dice: game.DiceRule{Count: 1, Sides: 6}, Resources: map[string]game.ResourceRule{}},
	}
}
