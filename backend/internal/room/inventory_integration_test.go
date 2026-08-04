package room

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"rollboard/internal/catalog"
	"rollboard/internal/game"
	"rollboard/internal/identity"
	"rollboard/internal/storage/postgres"
	"rollboard/internal/testdb"
)

// inventoryDefinition is a one-square game whose players carry a sword.
func inventoryDefinition() game.GameDefinition {
	return game.GameDefinition{
		Title: "Armoury",
		Board: game.Board{
			Width: 96, Height: 96, CellSize: 96,
			Cells: []game.CellDefinition{{
				ID: "start", Title: "Start", Type: "start", Fields: map[string]any{},
				OnLand: []game.ActionDefinition{{Type: "grant_item", Field: "sword"}},
			}},
		},
		Rules: game.RuleSet{
			Dice:           game.DiceRule{Count: 1, Sides: 6},
			Resources:      map[string]game.ResourceRule{"money": {Initial: 100}},
			CellTypes:      map[string]game.CellTypeDef{"start": {Title: "Start", Fields: map[string]game.FieldDef{}}},
			EquipmentSlots: []string{"weapon"},
			Items: map[string]game.ItemDef{
				"sword": {ID: "sword", Title: "Sword", Slot: "weapon"},
			},
		},
	}
}

// TestOnlinePlayersCanManageInventoryAndTrade covers what the room protocol
// could not do at all: an online player handed an item by a cell had no way to
// put it on, and no way to offer it to anybody.
func TestOnlinePlayersCanManageInventoryAndTrade(t *testing.T) {
	dsn := os.Getenv("ROLLBOARD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ROLLBOARD_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	release, err := testdb.AcquireExclusive(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil {
		t.Fatal(err)
	}

	identities, err := identity.NewRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	hostUser := registerRoomUser(t, ctx, identities, "armourer@example.com", "Host")
	guestUser := registerRoomUser(t, ctx, identities, "squire@example.com", "Guest")
	host := identity.Actor{User: &hostUser}
	guest := identity.Actor{User: &guestUser}

	catalogService, err := catalog.NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	created, err := catalogService.CreateGame(ctx, hostUser.ID, inventoryDefinition())
	if err != nil {
		t.Fatal(err)
	}
	version, err := catalogService.Publish(ctx, hostUser.ID, created.ID)
	if err != nil {
		t.Fatal(err)
	}

	rooms, err := NewService(pool, catalogService)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := rooms.Create(ctx, host, version.ID, CreateInput{Title: "Armoury", MaxPlayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.Join(ctx, guest, stored.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.Start(ctx, host, stored.ID); err != nil {
		t.Fatal(err)
	}

	// Both players are given a sword directly, so the test is about the room
	// commands rather than about landing on the right square.
	live, err := rooms.Get(ctx, stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range live.Session.State.Players {
		live.Session.State.Players[i].Inventory = map[string]int{"sword": 1}
	}
	if err := saveRoomSessionForTest(ctx, pool, live); err != nil {
		t.Fatal(err)
	}

	// --- equipping ---------------------------------------------------------
	transition, err := rooms.ManageInventoryWithCommand(ctx, host, stored.ID, "equip", "sword",
		Command{ID: "11111111-1111-1111-1111-111111111111", Type: "inventory"})
	if err != nil {
		t.Fatalf("equip: %v", err)
	}
	if got := transition.Session.State.Players[0].Equipped["weapon"]; got != "sword" {
		t.Fatalf("weapon slot = %q, want the sword", got)
	}

	// A repeated command ID must not equip twice or produce a second journal
	// entry, exactly as for a roll.
	repeat, err := rooms.ManageInventoryWithCommand(ctx, host, stored.ID, "equip", "sword",
		Command{ID: "11111111-1111-1111-1111-111111111111", Type: "inventory"})
	if err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if !repeat.Duplicate {
		t.Fatal("a repeated command ID was executed again")
	}

	// --- another player's turn ---------------------------------------------
	// The guest is not the current player, so their inventory change is
	// refused: the engine decides that, not the client.
	if _, err := rooms.ManageInventoryWithCommand(ctx, guest, stored.ID, "equip", "sword",
		Command{ID: "22222222-2222-2222-2222-222222222222", Type: "inventory"}); err == nil {
		t.Fatal("a player changed their inventory out of turn")
	}

	// --- unequipping -------------------------------------------------------
	transition, err = rooms.ManageInventoryWithCommand(ctx, host, stored.ID, "unequip", "weapon",
		Command{ID: "33333333-3333-3333-3333-333333333333", Type: "inventory"})
	if err != nil {
		t.Fatalf("unequip: %v", err)
	}
	if got := transition.Session.State.Players[0].Equipped["weapon"]; got != "" {
		t.Fatalf("weapon slot = %q after unequipping", got)
	}

	// --- trading -----------------------------------------------------------
	transition, err = rooms.ProposeTradeWithCommand(ctx, host, stored.ID, game.TradeOffer{
		// FromPlayerID is deliberately wrong here: the server must use the
		// sender's own seat, or a player could offer away another's goods.
		FromPlayerID:     "player_2",
		ToPlayerID:       "player_2",
		OfferItems:       map[string]int{"sword": 1},
		RequestResources: map[string]int{"money": 10},
	}, Command{ID: "44444444-4444-4444-4444-444444444444", Type: "trade"})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	pending := transition.Session.State.PendingAction
	if pending == nil || pending.Type != "trade_offer" {
		t.Fatalf("pending action = %+v, want a trade offer", pending)
	}
	if pending.PlayerID != "player_2" {
		t.Fatalf("the offer was addressed to %q, want the recipient", pending.PlayerID)
	}
	if from := transition.Session.State.PendingTrade.FromPlayerID; from != "player_1" {
		t.Fatalf("proposer = %q, want the sender's own seat", from)
	}

	// The recipient accepts through the ordinary action command, and the goods
	// move both ways.
	accepted, err := rooms.ResolveActionWithCommand(ctx, guest, stored.ID, "accept",
		Command{ID: "55555555-5555-5555-5555-555555555555", Type: "action"})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	players := accepted.Session.State.Players
	if players[0].Inventory["sword"] != 0 || players[1].Inventory["sword"] != 2 {
		t.Fatalf("swords after the trade: %d and %d", players[0].Inventory["sword"], players[1].Inventory["sword"])
	}
	if players[0].Resources["money"] != 110 || players[1].Resources["money"] != 90 {
		t.Fatalf("money after the trade: %d and %d", players[0].Resources["money"], players[1].Resources["money"])
	}

	// --- durability --------------------------------------------------------
	// Reloading the room from the database has to bring all of it back, since
	// this is what a page refresh and an application restart both do.
	reloaded, err := rooms.Get(ctx, stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Session.State.Players[1].Inventory["sword"] != 2 {
		t.Fatalf("the trade did not survive a reload: %+v", reloaded.Session.State.Players[1].Inventory)
	}

	// --- a room that is not running ----------------------------------------
	if _, err := rooms.ManageInventoryWithCommand(ctx, host, "00000000-0000-0000-0000-000000000000", "equip", "sword",
		Command{ID: "66666666-6666-6666-6666-666666666666", Type: "inventory"}); err == nil {
		t.Fatal("a command against an unknown room succeeded")
	} else if errors.Is(err, ErrNotYourTurn) {
		t.Fatalf("unexpected error kind: %v", err)
	}
}

// saveRoomSessionForTest writes a session straight back, so a test can set up
// a position the dice would otherwise have to reach.
func saveRoomSessionForTest(ctx context.Context, pool *pgxpool.Pool, stored *Room) error {
	raw, err := json.Marshal(game.ForStorage{Session: stored.Session})
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `UPDATE rooms SET session_json = $2::jsonb WHERE id = $1`, stored.ID, string(raw))
	return err
}
