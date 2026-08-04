package game

import (
	"strings"
	"testing"
)

func tradeSession(t *testing.T) (*GameSession, *PlayerState, *PlayerState) {
	t.Helper()
	session := rpgSession(t)
	ada := &session.State.Players[0]
	bob := &session.State.Players[1]
	cell := session.Definition.Board.getCellByID("armoury")

	session.executeOneAction(ActionDefinition{Type: "grant_item", Field: "great_axe"}, ada, cell)
	session.executeOneAction(ActionDefinition{Type: "grant_item", Field: "leather"}, bob, cell)
	ada.Resources["gold"] = 100
	bob.Resources["gold"] = 50
	return session, ada, bob
}

func TestATradeMovesGoodsBothWaysOnlyAfterTheOtherPlayerAgrees(t *testing.T) {
	session, ada, bob := tradeSession(t)

	events, err := session.ProposeTrade(TradeOffer{
		FromPlayerID:     ada.ID,
		ToPlayerID:       bob.ID,
		OfferItems:       map[string]int{"great_axe": 1},
		RequestResources: map[string]int{"gold": 30},
	})
	if err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	if len(events) == 0 {
		t.Fatal("proposing produced no events")
	}

	// Nothing moves on the offer alone.
	if ada.Inventory["great_axe"] != 1 || bob.Resources["gold"] != 50 {
		t.Fatal("goods moved before the recipient answered")
	}
	// The decision belongs to the recipient, not the proposer.
	if session.State.PendingAction.PlayerID != bob.ID {
		t.Fatalf("pending action addressed to %q, want the recipient", session.State.PendingAction.PlayerID)
	}

	if _, err := session.ResolvePendingAction("accept"); err != nil {
		t.Fatalf("accepting failed: %v", err)
	}

	if bob.Inventory["great_axe"] != 1 || ada.Inventory["great_axe"] != 0 {
		t.Fatalf("the axe did not change hands: ada=%v bob=%v", ada.Inventory, bob.Inventory)
	}
	if ada.Resources["gold"] != 130 || bob.Resources["gold"] != 20 {
		t.Fatalf("gold = ada %d, bob %d; want 130 and 20", ada.Resources["gold"], bob.Resources["gold"])
	}
	if session.State.PendingTrade != nil || session.State.PendingAction != nil {
		t.Fatal("the trade stayed pending after completing")
	}
}

func TestDecliningATradeChangesNothing(t *testing.T) {
	session, ada, bob := tradeSession(t)
	if _, err := session.ProposeTrade(TradeOffer{
		FromPlayerID: ada.ID, ToPlayerID: bob.ID,
		OfferItems: map[string]int{"great_axe": 1}, RequestResources: map[string]int{"gold": 30},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := session.ResolvePendingAction("decline"); err != nil {
		t.Fatalf("declining failed: %v", err)
	}
	if ada.Inventory["great_axe"] != 1 || ada.Resources["gold"] != 100 || bob.Resources["gold"] != 50 {
		t.Fatal("declining moved goods anyway")
	}
	if session.State.PendingTrade != nil {
		t.Fatal("a declined trade stayed pending")
	}
}

// Turns can pass between offer and answer, so neither side may promise what
// they no longer hold.
func TestATradeFailsIfEitherSideCanNoLongerDeliver(t *testing.T) {
	session, ada, bob := tradeSession(t)
	cell := session.Definition.Board.getCellByID("armoury")

	if _, err := session.ProposeTrade(TradeOffer{
		FromPlayerID: ada.ID, ToPlayerID: bob.ID,
		OfferItems: map[string]int{"great_axe": 1}, RequestResources: map[string]int{"gold": 30},
	}); err != nil {
		t.Fatal(err)
	}
	// The proposer loses the axe before the answer arrives.
	session.executeOneAction(ActionDefinition{Type: "remove_item", Field: "great_axe"}, ada, cell)

	events, err := session.ResolvePendingAction("accept")
	if err != nil {
		t.Fatalf("accepting returned an error rather than a failed trade: %v", err)
	}
	if len(events) == 0 || events[0].Type != "trade_failed" {
		t.Fatalf("events = %#v, want the trade to fail cleanly", events)
	}
	if bob.Resources["gold"] != 50 {
		t.Fatalf("bob paid %d gold for nothing", 50-bob.Resources["gold"])
	}
}

func TestTradeProposalsAreRejectedWhenTheyMakeNoSense(t *testing.T) {
	session, ada, bob := tradeSession(t)

	cases := []struct {
		name  string
		offer TradeOffer
		want  string
	}{
		{"with yourself", TradeOffer{FromPlayerID: ada.ID, ToPlayerID: ada.ID, OfferResources: map[string]int{"gold": 1}}, "yourself"},
		{"nothing on either side", TradeOffer{FromPlayerID: ada.ID, ToPlayerID: bob.ID}, "must move something"},
		{"offering what you lack", TradeOffer{FromPlayerID: ada.ID, ToPlayerID: bob.ID, OfferItems: map[string]int{"rusty_sword": 1}}, "no longer has"},
		{"offering more gold than you hold", TradeOffer{FromPlayerID: ada.ID, ToPlayerID: bob.ID, OfferResources: map[string]int{"gold": 5000}}, "no longer has"},
		{"a player who does not exist", TradeOffer{FromPlayerID: ada.ID, ToPlayerID: "nobody", OfferResources: map[string]int{"gold": 1}}, "no such player"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := session.ProposeTrade(testCase.offer)
			if err == nil {
				t.Fatalf("ProposeTrade(%#v) was accepted", testCase.offer)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %q, want it to mention %q", err, testCase.want)
			}
		})
	}
}

func TestOnlyTheCurrentPlayerMayProposeATrade(t *testing.T) {
	session, ada, bob := tradeSession(t)

	// Bob is not the current player.
	_, err := session.ProposeTrade(TradeOffer{
		FromPlayerID: bob.ID, ToPlayerID: ada.ID, OfferResources: map[string]int{"gold": 10},
	})
	if err == nil || !strings.Contains(err.Error(), "turn") {
		t.Fatalf("error = %v, want a turn-order refusal", err)
	}
}

func TestComputedAmountsSubtractOneStatFromAnother(t *testing.T) {
	session := rpgSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("cave")
	cell.Fields = map[string]any{"damage": 10}
	player.Resources["health"] = 30

	// Armour must actually reduce damage: this is what the engine could not
	// express before, since actions could compare but never combine.
	session.executeOneAction(ActionDefinition{Type: "grant_item", Field: "leather"}, player, cell)
	session.executeOneAction(ActionDefinition{Type: "equip_item", Field: "leather"}, player, cell)

	session.executeOneAction(ActionDefinition{
		Type:     "lose_resource",
		Resource: "health",
		Formula: &AmountFormula{
			Base:  &AmountTerm{Kind: "field", Name: "damage"},
			Minus: &AmountTerm{Kind: "stat", Name: "strength"},
			Min:   intPtr(0),
		},
	}, player, cell)

	// health starts at 30 (+5 from the armour is effective only, not stored),
	// damage 10 minus effective strength 5 = 5.
	if player.Resources["health"] != 25 {
		t.Fatalf("health = %d, want 30 - (10 - 5)", player.Resources["health"])
	}
}

func TestComputedAmountsClampAndScale(t *testing.T) {
	session := rpgSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("cave")
	cell.Fields = map[string]any{"base": 7}

	// A floor keeps a heavily armoured hit from healing the victim.
	got := session.resolveFormula(&AmountFormula{
		Base:  &AmountTerm{Kind: "field", Name: "base"},
		Minus: &AmountTerm{Kind: "const", Value: 100},
		Min:   intPtr(0),
	}, player, cell)
	if got != 0 {
		t.Fatalf("floored value = %d, want 0", got)
	}

	got = session.resolveFormula(&AmountFormula{
		Base:      &AmountTerm{Kind: "const", Value: 10},
		Times:     &AmountTerm{Kind: "const", Value: 3},
		DividedBy: &AmountTerm{Kind: "const", Value: 4},
		Max:       intPtr(6),
	}, player, cell)
	if got != 6 {
		t.Fatalf("capped value = %d, want 6", got)
	}

	// A zero divisor must not take the game down.
	got = session.resolveFormula(&AmountFormula{
		Base:      &AmountTerm{Kind: "const", Value: 9},
		DividedBy: &AmountTerm{Kind: "const", Value: 0},
	}, player, cell)
	if got != 9 {
		t.Fatalf("value = %d, want the zero divisor ignored", got)
	}
}
