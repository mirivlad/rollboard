package game

import (
	"strings"
	"testing"
)

func boundedSession(t *testing.T, min, max *int) *GameSession {
	t.Helper()
	definition := propertyDefinition()
	definition.Rules.Resources["money"] = ResourceRule{Initial: 500, Min: min, Max: max}
	return StartSession(definition, []PlayerConfig{{Name: "Ada"}, {Name: "Bob"}})
}

func negative() *AmountFormula {
	return &AmountFormula{Base: &AmountTerm{Kind: "field", Name: "rent"}, Minus: &AmountTerm{Kind: "const", Value: 1000}}
}

func TestANegativeAmountIsRefusedRatherThanReversed(t *testing.T) {
	// A computed amount that comes out negative used to run the action
	// backwards: "gain" took money away, "lose" handed it out, and the log
	// described both as if they had done what the author asked.
	cases := []struct {
		action string
		target string
	}{{"gain_resource", ""}, {"lose_resource", ""}, {"transfer_resource", "owner"}}

	for _, testCase := range cases {
		t.Run(testCase.action, func(t *testing.T) {
			session := propertySession(t)
			ada := &session.State.Players[0]
			bob := &session.State.Players[1]
			own(session, "blue_a", bob.ID)
			cell := session.Definition.Board.getCellByID("blue_a")

			events := session.executeOneAction(ActionDefinition{
				Type: testCase.action, Resource: "money", Target: testCase.target, Formula: negative(),
			}, ada, cell)

			if ada.Resources["money"] != 500 || bob.Resources["money"] != 500 {
				t.Fatalf("money moved: Ada %d, Bob %d", ada.Resources["money"], bob.Resources["money"])
			}
			if len(events) != 1 || events[0].Type != "invalid_amount" {
				t.Fatalf("events = %+v, want one invalid_amount", events)
			}
		})
	}
}

func TestResourceCeilingHoldsOnEveryPath(t *testing.T) {
	max := 600
	session := boundedSession(t, nil, &max)
	ada := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("blue_a")

	events := session.executeOneAction(ActionDefinition{Type: "gain_resource", Resource: "money", Amount: intPtr(1000)}, ada, cell)
	if ada.Resources["money"] != 600 {
		t.Fatalf("money = %d, want the ceiling of 600", ada.Resources["money"])
	}
	// The log has to say the cap bit, or a player sees "gained 1000" and a
	// balance that did not move by 1000.
	if !strings.Contains(events[0].Message, "limit") {
		t.Fatalf("the capped gain was logged as %q", events[0].Message)
	}

	// The start bonus is a separate path and used to bypass the ceiling too.
	session.State.Players[0].Resources["money"] = 590
	definitionBonus := session.Definition.Rules.StartBonus
	if definitionBonus == 0 {
		session.Definition.Rules.StartBonus = 50
		session.Definition.Rules.StartBonusResource = "money"
	}
	session.addResource(ada, "money", session.Definition.Rules.StartBonus)
	if ada.Resources["money"] > 600 {
		t.Fatalf("the start bonus went through the ceiling: %d", ada.Resources["money"])
	}
}

func TestResourceFloorHoldsOnEveryPath(t *testing.T) {
	min := 100
	session := boundedSession(t, &min, nil)
	ada := &session.State.Players[0]
	bob := &session.State.Players[1]
	own(session, "blue_a", bob.ID)
	cell := session.Definition.Board.getCellByID("blue_a")

	session.executeOneAction(ActionDefinition{Type: "lose_resource", Resource: "money", Amount: intPtr(5000)}, ada, cell)
	if ada.Resources["money"] != 100 {
		t.Fatalf("money = %d, want the floor of 100", ada.Resources["money"])
	}

	before := bob.Resources["money"]
	session.executeOneAction(ActionDefinition{Type: "transfer_resource", Resource: "money", Target: "owner", Amount: intPtr(5000)}, ada, cell)
	if ada.Resources["money"] != 100 {
		t.Fatalf("a transfer went below the floor: %d", ada.Resources["money"])
	}
	if bob.Resources["money"] != before {
		t.Fatalf("Bob received %d from a player who had nothing to give", bob.Resources["money"]-before)
	}
}

func TestATransferNeitherCreatesNorDestroys(t *testing.T) {
	max := 520
	session := boundedSession(t, nil, &max)
	ada := &session.State.Players[0]
	bob := &session.State.Players[1]
	own(session, "blue_a", bob.ID)
	cell := session.Definition.Board.getCellByID("blue_a")
	total := ada.Resources["money"] + bob.Resources["money"]

	session.executeOneAction(ActionDefinition{Type: "transfer_resource", Resource: "money", Target: "owner", Amount: intPtr(200)}, ada, cell)

	// Bob can only hold 20 more, so 20 is what Ada pays. Paying 200 into a
	// pocket that holds 20 would have destroyed the other 180.
	if bob.Resources["money"] != 520 {
		t.Fatalf("recipient = %d, want the ceiling of 520", bob.Resources["money"])
	}
	if ada.Resources["money"] != 480 {
		t.Fatalf("payer = %d, want 480", ada.Resources["money"])
	}
	if got := ada.Resources["money"] + bob.Resources["money"]; got != total {
		t.Fatalf("money in the game changed from %d to %d", total, got)
	}
}

func TestPaymentsGoOnlyWhereTheEngineCanResolveThem(t *testing.T) {
	session := propertySession(t)
	ada := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("blue_a")

	for _, target := range []string{"current", "bank", "none", "nobody-at-all"} {
		events := session.executeOneAction(
			ActionDefinition{Type: "transfer_resource", Resource: "money", Target: target, Amount: intPtr(50)}, ada, cell)
		if ada.Resources["money"] != 500 {
			t.Fatalf("target %q moved money", target)
		}
		// Silence was the real problem: the payment simply never happened and
		// the log said nothing, so the game looked like it had no rent.
		if len(events) != 1 || events[0].Type != "invalid_action" {
			t.Fatalf("target %q produced %+v, want an invalid_action", target, events)
		}
	}
}

func TestASquareIsNeverOwnedBySomebodyWhoDoesNotExist(t *testing.T) {
	session := propertySession(t)
	ada := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("blue_a")

	for _, target := range []string{"owner", "bank", "whoever"} {
		events := session.executeOneAction(ActionDefinition{Type: "set_cell_owner", Target: target}, ada, cell)
		if owner := session.State.CellStates["blue_a"].OwnerPlayerID; owner != "" {
			t.Fatalf("target %q stored owner %q, who is not a player", target, owner)
		}
		if len(events) != 1 || events[0].Type != "invalid_action" {
			t.Fatalf("target %q produced %+v, want an invalid_action", target, events)
		}
	}

	session.executeOneAction(ActionDefinition{Type: "set_cell_owner", Target: "current"}, ada, cell)
	if owner := session.State.CellStates["blue_a"].OwnerPlayerID; owner != ada.ID {
		t.Fatalf("owner = %q, want Ada", owner)
	}
}

func TestATradeIsRefusedRatherThanPartlyDone(t *testing.T) {
	max := 520
	session := boundedSession(t, nil, &max)
	ada := &session.State.Players[0]
	bob := &session.State.Players[1]

	if _, err := session.ProposeTrade(TradeOffer{
		FromPlayerID: ada.ID, ToPlayerID: bob.ID,
		OfferResources:   map[string]int{"money": 100},
		RequestResources: map[string]int{"money": 1},
	}); err != nil {
		t.Fatalf("propose: %v", err)
	}
	events, err := session.ResolvePendingAction("accept")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if events[0].Type != "trade_failed" {
		t.Fatalf("events = %+v, want trade_failed", events)
	}
	if ada.Resources["money"] != 500 || bob.Resources["money"] != 500 {
		t.Fatalf("a refused trade still moved money: Ada %d, Bob %d", ada.Resources["money"], bob.Resources["money"])
	}
}
