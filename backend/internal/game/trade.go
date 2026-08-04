package game

import (
	"fmt"
	"sort"
	"strings"
)

// TradeOffer is a proposal from one player to another.
//
// Trading needs two steps because a PendingAction is addressed to exactly one
// player: the proposer builds the offer on their turn, and the decision then
// belongs to the recipient. Nothing moves until they answer.
type TradeOffer struct {
	FromPlayerID string `json:"fromPlayerId"`
	ToPlayerID   string `json:"toPlayerId"`
	// Offer is what the proposer gives up; Request is what they ask for.
	OfferItems       map[string]int `json:"offerItems,omitempty"`
	OfferResources   map[string]int `json:"offerResources,omitempty"`
	RequestItems     map[string]int `json:"requestItems,omitempty"`
	RequestResources map[string]int `json:"requestResources,omitempty"`
}

func (t TradeOffer) isEmpty() bool {
	return len(t.OfferItems)+len(t.OfferResources)+len(t.RequestItems)+len(t.RequestResources) == 0
}

// canDeliver reports whether a player actually holds everything listed.
//
// Checked when the offer is made and again when it is accepted: a proposer can
// spend the very thing they promised in the turns in between.
func (s *GameSession) canDeliver(player *PlayerState, items, resources map[string]int) error {
	ensureInventory(player)
	for itemID, count := range items {
		if count < 1 {
			return fmt.Errorf("item count must be positive")
		}
		if player.Inventory[itemID] < count {
			title := itemID
			if item, ok := s.Definition.Rules.Items[itemID]; ok {
				title = item.Title
			}
			return fmt.Errorf("%s no longer has %d× %s", player.Name, count, title)
		}
	}
	for resource, amount := range resources {
		if amount < 1 {
			return fmt.Errorf("resource amount must be positive")
		}
		if player.Resources[resource] < amount {
			return fmt.Errorf("%s no longer has %d %s", player.Name, amount, resource)
		}
	}
	return nil
}

func (s *GameSession) moveGoods(from, to *PlayerState, items, resources map[string]int) {
	ensureInventory(from)
	ensureInventory(to)
	for itemID, count := range items {
		s.removeItem(from, itemID, count)
		to.Inventory[itemID] += count
	}
	for resource, amount := range resources {
		from.Resources[resource] -= amount
		to.Resources[resource] += amount
	}
}

func describeGoods(definition *GameDefinition, items, resources map[string]int) string {
	parts := make([]string, 0, len(items)+len(resources))
	for itemID, count := range items {
		title := itemID
		if item, ok := definition.Rules.Items[itemID]; ok {
			title = item.Title
		}
		parts = append(parts, fmt.Sprintf("%d× %s", count, title))
	}
	for resource, amount := range resources {
		parts = append(parts, fmt.Sprintf("%d %s", amount, resource))
	}
	// Sorted so the same offer always reads the same way, rather than
	// reshuffling with Go's map iteration order.
	sort.Strings(parts)
	if len(parts) == 0 {
		return "nothing"
	}
	return strings.Join(parts, ", ")
}

// ProposeTrade puts an offer in front of another player.
func (s *GameSession) ProposeTrade(offer TradeOffer) ([]GameEvent, error) {
	if s.State.Status != "active" {
		return nil, fmt.Errorf("game is not active")
	}
	if s.State.PendingAction != nil {
		return nil, fmt.Errorf("resolve the pending action first")
	}
	current := s.CurrentPlayer()
	if current == nil || current.ID != offer.FromPlayerID {
		return nil, fmt.Errorf("only the player whose turn it is may propose a trade")
	}
	if offer.ToPlayerID == offer.FromPlayerID {
		return nil, fmt.Errorf("cannot trade with yourself")
	}
	recipient := s.getPlayerByID(offer.ToPlayerID)
	if recipient == nil {
		return nil, fmt.Errorf("no such player")
	}
	if recipient.Bankrupt {
		return nil, fmt.Errorf("%s is out of the game", recipient.Name)
	}
	if offer.isEmpty() {
		return nil, fmt.Errorf("a trade must move something")
	}
	if err := s.canDeliver(current, offer.OfferItems, offer.OfferResources); err != nil {
		return nil, err
	}

	stored := offer
	s.State.PendingTrade = &stored
	s.State.PendingAction = &PendingAction{
		Type: "trade_offer",
		// Addressed to the recipient: the whole point is that the decision is
		// theirs, not the proposer's.
		PlayerID: recipient.ID,
		Title: fmt.Sprintf("%s offers %s for %s",
			current.Name,
			describeGoods(s.Definition, offer.OfferItems, offer.OfferResources),
			describeGoods(s.Definition, offer.RequestItems, offer.RequestResources)),
		CellID: recipient.PositionCellID,
		Options: []ActionOption{
			{ID: "accept", Title: "Accept"},
			{ID: "decline", Title: "Decline"},
		},
	}
	return []GameEvent{NewGameEvent("trade_offered",
		fmt.Sprintf("%s proposed a trade to %s", current.Name, recipient.Name), nil)}, nil
}

// resolveTrade completes or discards an offer.
func (s *GameSession) resolveTrade(answer string) ([]GameEvent, error) {
	offer := s.State.PendingTrade
	if offer == nil {
		return nil, fmt.Errorf("no trade is pending")
	}
	proposer := s.getPlayerByID(offer.FromPlayerID)
	recipient := s.getPlayerByID(offer.ToPlayerID)
	if proposer == nil || recipient == nil {
		s.State.PendingTrade = nil
		s.State.PendingAction = nil
		return nil, fmt.Errorf("a player in the trade is gone")
	}

	if answer != "accept" {
		s.State.PendingTrade = nil
		s.State.PendingAction = nil
		return []GameEvent{NewGameEvent("trade_declined",
			fmt.Sprintf("%s declined the trade", recipient.Name), nil)}, nil
	}

	// Both sides are re-checked at acceptance. Turns may have passed, and
	// neither side should be able to promise what they no longer hold.
	if err := s.canDeliver(proposer, offer.OfferItems, offer.OfferResources); err != nil {
		s.State.PendingTrade = nil
		s.State.PendingAction = nil
		return []GameEvent{NewGameEvent("trade_failed", err.Error(), nil)}, nil
	}
	if err := s.canDeliver(recipient, offer.RequestItems, offer.RequestResources); err != nil {
		s.State.PendingTrade = nil
		s.State.PendingAction = nil
		return []GameEvent{NewGameEvent("trade_failed", err.Error(), nil)}, nil
	}

	s.moveGoods(proposer, recipient, offer.OfferItems, offer.OfferResources)
	s.moveGoods(recipient, proposer, offer.RequestItems, offer.RequestResources)

	s.State.PendingTrade = nil
	s.State.PendingAction = nil

	events := []GameEvent{NewGameEvent("trade_accepted",
		fmt.Sprintf("%s and %s completed a trade", proposer.Name, recipient.Name), nil)}
	events = append(events, s.applyProgression(proposer)...)
	events = append(events, s.applyProgression(recipient)...)
	return events, nil
}
