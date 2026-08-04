package game

import (
	"encoding/json"
	"strings"
	"testing"
)

func auctionSession(t *testing.T) *GameSession {
	t.Helper()
	return StartSession(propertyDefinition(), []PlayerConfig{
		{Name: "Ada", Color: "#111111"},
		{Name: "Bob", Color: "#222222"},
		{Name: "Cleo", Color: "#333333"},
	})
}

// auctionCell is the action an author would build in the editor: put the
// square up for sale, and give it to whoever wins.
func auctionCell() ActionDefinition {
	return ActionDefinition{
		Type:      "start_auction",
		Resource:  "money",
		Amount:    intPtr(50),
		Increment: intPtr(10),
		Then:      []ActionDefinition{{Type: "set_cell_owner", Target: "current"}},
		Else:      []ActionDefinition{{Type: "log_message", Title: "The square stays with the bank"}},
	}
}

func optionIDs(action *PendingAction) []string {
	ids := make([]string, 0, len(action.Options))
	for _, option := range action.Options {
		ids = append(ids, option.ID)
	}
	return ids
}

func TestAuctionAsksEveryPlayerInTurn(t *testing.T) {
	session := auctionSession(t)
	ada := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("blue_a")

	session.executeOneAction(auctionCell(), ada, cell)

	// This is the thing a two-player trade could not do: the decision passes
	// round the table, and each player in turn holds it.
	if session.State.PendingAction == nil || session.State.PendingAction.Type != "auction_bid" {
		t.Fatalf("no auction started: %+v", session.State.PendingAction)
	}
	if session.State.PendingAction.PlayerID != ada.ID {
		t.Fatalf("first bidder = %s, want Ada", session.State.PendingAction.PlayerID)
	}

	if _, err := session.ResolvePendingAction("bid_50"); err != nil {
		t.Fatalf("Ada's bid: %v", err)
	}
	if got := session.State.PendingAction.PlayerID; got != session.State.Players[1].ID {
		t.Fatalf("second bidder = %s, want Bob", got)
	}
	if _, err := session.ResolvePendingAction("pass"); err != nil {
		t.Fatalf("Bob passing: %v", err)
	}
	if got := session.State.PendingAction.PlayerID; got != session.State.Players[2].ID {
		t.Fatalf("third bidder = %s, want Cleo", got)
	}

	// Cleo outbids, so Ada gets asked again — a pass is not a surrender.
	if _, err := session.ResolvePendingAction("bid_60"); err != nil {
		t.Fatalf("Cleo's bid: %v", err)
	}
	if got := session.State.PendingAction.PlayerID; got != ada.ID {
		t.Fatalf("bidding did not come back to Ada, got %s", got)
	}
}

func TestAuctionWinnerPaysAndTheFollowUpRunsAsThem(t *testing.T) {
	session := auctionSession(t)
	ada := &session.State.Players[0]
	cleo := &session.State.Players[2]
	cell := session.Definition.Board.getCellByID("blue_a")
	cleoBefore := cleo.Resources["money"]

	session.executeOneAction(auctionCell(), ada, cell)
	mustResolve(t, session, "bid_50") // Ada
	mustResolve(t, session, "pass")   // Bob
	mustResolve(t, session, "bid_60") // Cleo
	mustResolve(t, session, "pass")   // Ada gives up

	if session.State.PendingAction != nil {
		t.Fatalf("auction did not end: %+v", session.State.PendingAction)
	}
	if session.State.PendingAuction != nil {
		t.Fatalf("auction state was left behind")
	}
	if got := cleo.Resources["money"]; got != cleoBefore-60 {
		t.Fatalf("winner's money = %d, want %d", got, cleoBefore-60)
	}
	if ada.Resources["money"] != 500 {
		t.Fatalf("a losing bidder was charged: %d", ada.Resources["money"])
	}
	// "give this cell to the current player" has to mean the winner, not the
	// player who happened to land on the square.
	if owner := session.State.CellStates["blue_a"].OwnerPlayerID; owner != cleo.ID {
		t.Fatalf("owner = %q, want Cleo", owner)
	}
}

func TestAuctionWithNoBidsRunsTheFallback(t *testing.T) {
	session := auctionSession(t)
	ada := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("blue_a")

	events := session.executeOneAction(auctionCell(), ada, cell)
	for i := 0; i < 3; i++ {
		more, err := session.ResolvePendingAction("pass")
		if err != nil {
			t.Fatalf("pass %d: %v", i, err)
		}
		events = append(events, more...)
	}

	if session.State.PendingAction != nil || session.State.PendingAuction != nil {
		t.Fatalf("auction did not end after everybody passed")
	}
	if owner := session.State.CellStates["blue_a"].OwnerPlayerID; owner != "" {
		t.Fatalf("an unsold square was given to %q", owner)
	}
	if !hasMessage(events, "The square stays with the bank") {
		t.Fatalf("the no-sale branch did not run: %s", messages(events))
	}
	for _, player := range session.State.Players {
		if player.Resources["money"] != 500 {
			t.Fatalf("%s paid for an auction nobody won", player.Name)
		}
	}
}

func TestAuctionDropsPlayersWhoCannotReachTheBid(t *testing.T) {
	session := auctionSession(t)
	ada := &session.State.Players[0]
	session.State.Players[1].Resources["money"] = 5
	cell := session.Definition.Board.getCellByID("blue_a")

	events := session.executeOneAction(auctionCell(), ada, cell)
	more, err := session.ResolvePendingAction("bid_50")
	if err != nil {
		t.Fatalf("Ada's bid: %v", err)
	}
	events = append(events, more...)

	// Bob cannot reach 60, so he is dropped rather than shown a prompt whose
	// only answer is "pass".
	if got := session.State.PendingAction.PlayerID; got != session.State.Players[2].ID {
		t.Fatalf("bidder after Ada = %s, want Cleo", got)
	}
	if !hasMessage(events, "Bob cannot reach 60 money and drops out") {
		t.Fatalf("no drop-out was logged: %s", messages(events))
	}
}

func TestAuctionRefusesBidsTheServerDidNotOffer(t *testing.T) {
	session := auctionSession(t)
	ada := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("blue_a")
	session.executeOneAction(auctionCell(), ada, cell)

	// The offered list is what bounds a bid; the client is only holding a copy.
	if _, err := session.ResolvePendingAction("bid_1"); err == nil {
		t.Fatal("a bid below the opening price was accepted")
	}
	if _, err := session.ResolvePendingAction("bid_99999"); err == nil {
		t.Fatal("a bid nobody could pay was accepted")
	}
	if _, err := session.ResolvePendingAction("nonsense"); err == nil {
		t.Fatal("an unknown option was accepted")
	}
	if session.State.PendingAction == nil || session.State.PendingAction.PlayerID != ada.ID {
		t.Fatalf("a rejected bid disturbed the auction: %+v", session.State.PendingAction)
	}
}

func TestAuctionOffersOnlyAffordableSteps(t *testing.T) {
	session := auctionSession(t)
	ada := &session.State.Players[0]
	ada.Resources["money"] = 55
	cell := session.Definition.Board.getCellByID("blue_a")
	session.executeOneAction(auctionCell(), ada, cell)

	ids := optionIDs(session.State.PendingAction)
	// 50 is reachable and so is spending everything; 70 and 100 are not.
	want := []string{"bid_50", "bid_55", "pass"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("options = %v, want %v", ids, want)
	}
}

func TestAuctionSurvivesASaveAndReload(t *testing.T) {
	session := auctionSession(t)
	ada := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("blue_a")
	session.executeOneAction(auctionCell(), ada, cell)
	mustResolve(t, session, "bid_50")

	encoded, err := json.Marshal(ForStorage{Session: session})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored GameSession
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// A player who reloads mid-auction, or a room replaying its journal, has
	// to come back to the same bidding.
	if restored.State.PendingAuction == nil || restored.State.PendingAuction.HighBid != 50 {
		t.Fatalf("auction did not survive storage: %+v", restored.State.PendingAuction)
	}
	mustResolveOn(t, &restored, "pass") // Bob
	mustResolveOn(t, &restored, "pass") // Cleo
	if owner := restored.State.CellStates["blue_a"].OwnerPlayerID; owner != restored.State.Players[0].ID {
		t.Fatalf("the reloaded auction awarded the cell to %q", owner)
	}
}

func TestAuctionKeepsTheTurnUntilItIsSettled(t *testing.T) {
	session := auctionSession(t)
	ada := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("blue_a")
	turn := session.State.TurnNumber

	session.executeOneAction(auctionCell(), ada, cell)
	mustResolve(t, session, "bid_50")
	if session.State.TurnNumber != turn {
		t.Fatalf("the turn moved on mid-auction: %d → %d", turn, session.State.TurnNumber)
	}
	mustResolve(t, session, "pass")
	mustResolve(t, session, "pass")
	if session.State.TurnNumber != turn+1 {
		t.Fatalf("the turn did not advance after the auction: %d", session.State.TurnNumber)
	}
	if session.State.CurrentPlayerIndex != 1 {
		t.Fatalf("play resumed with player %d, want Bob", session.State.CurrentPlayerIndex)
	}
}

func mustResolve(t *testing.T, session *GameSession, option string) {
	t.Helper()
	mustResolveOn(t, session, option)
}

func mustResolveOn(t *testing.T, session *GameSession, option string) {
	t.Helper()
	if _, err := session.ResolvePendingAction(option); err != nil {
		t.Fatalf("resolve %q: %v", option, err)
	}
}

func hasMessage(events []GameEvent, want string) bool {
	for _, event := range events {
		if event.Message == want {
			return true
		}
	}
	return false
}

func messages(events []GameEvent) string {
	parts := make([]string, 0, len(events))
	for _, event := range events {
		parts = append(parts, event.Message)
	}
	return strings.Join(parts, " | ")
}
