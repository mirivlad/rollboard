package game

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Auction is an open ascending auction running inside a turn.
//
// Trading solved a negotiation between two players. An auction is the one
// between all of them, and it does not fit the same shape: a pending action is
// addressed to exactly one player, so a simultaneous sealed bid has nowhere to
// live. Bidding in turn does fit — each bidder holds the pending action for as
// long as it takes them to answer, and the room already refuses a command from
// anybody else.
//
// That also makes the auction resumable. It lives in the session state, so a
// player who reloads mid-auction gets it back, and a room replaying its event
// journal replays the bidding with it.
type Auction struct {
	CellID string `json:"cellId,omitempty"`
	Title  string `json:"title,omitempty"`
	// Resource is the currency bid in.
	Resource string `json:"resource"`
	// Increment is the smallest raise.
	Increment int `json:"increment"`
	// MinBid is what the first bid must be at least.
	MinBid       int    `json:"minBid"`
	HighBid      int    `json:"highBid"`
	HighBidderID string `json:"highBidderId,omitempty"`
	// StartedByPlayerID is whoever triggered the auction; the no-sale branch
	// runs as them, since there is no winner to run it as.
	StartedByPlayerID string `json:"startedByPlayerId,omitempty"`
	// Order holds the players still bidding, in seating order, and Index
	// points at whoever must answer now. Passing removes a player from Order
	// rather than marking them, so the auction ends when it empties.
	Order []string `json:"order"`
	Index int      `json:"index"`
	// Then runs for the winner, Else when nobody bids.
	Then []ActionDefinition `json:"then,omitempty"`
	Else []ActionDefinition `json:"else,omitempty"`
}

// nextBid is the smallest amount that would take the lead.
func (au *Auction) nextBid() int {
	if au.HighBidderID == "" {
		return au.MinBid
	}
	return au.HighBid + au.Increment
}

func (au *Auction) removeCurrent() {
	if au.Index < 0 || au.Index >= len(au.Order) {
		au.Index = 0
		return
	}
	au.Order = append(au.Order[:au.Index], au.Order[au.Index+1:]...)
	if au.Index >= len(au.Order) {
		au.Index = 0
	}
}

func (au *Auction) advance() {
	if len(au.Order) == 0 {
		return
	}
	au.Index = (au.Index + 1) % len(au.Order)
}

// bidOptions offers a few round raises rather than a free-form number.
//
// A text box would need validating, translating and defending against a client
// that types a bid it cannot pay. Buttons the server generated are none of
// those things, and they keep the auction usable on a phone.
func (au *Auction) bidOptions(bidder *PlayerState) []ActionOption {
	next := au.nextBid()
	balance := bidder.Resources[au.Resource]
	candidates := []int{next, next + au.Increment*2, next + au.Increment*5, balance}
	seen := map[int]bool{}
	amounts := make([]int, 0, len(candidates))
	for _, amount := range candidates {
		if amount < next || amount > balance || seen[amount] {
			continue
		}
		seen[amount] = true
		amounts = append(amounts, amount)
	}
	sort.Ints(amounts)

	options := make([]ActionOption, 0, len(amounts)+1)
	for _, amount := range amounts {
		options = append(options, ActionOption{
			ID:    fmt.Sprintf("bid_%d", amount),
			Title: fmt.Sprintf("Bid %d %s", amount, au.Resource),
		})
	}
	return append(options, ActionOption{ID: "pass", Title: "Pass"})
}

// auctionParticipants lists who may bid, starting from the acting player.
func (s *GameSession) auctionParticipants(starter *PlayerState, target string) []string {
	start := 0
	for i := range s.State.Players {
		if s.State.Players[i].ID == starter.ID {
			start = i
			break
		}
	}
	var order []string
	for offset := 0; offset < len(s.State.Players); offset++ {
		p := &s.State.Players[(start+offset)%len(s.State.Players)]
		if p.Bankrupt {
			continue
		}
		if target == "others" && p.ID == starter.ID {
			continue
		}
		order = append(order, p.ID)
	}
	return order
}

// startAuction implements the start_auction action.
func (s *GameSession) startAuction(a ActionDefinition, player *PlayerState, cell *CellDefinition) []GameEvent {
	if a.Resource == "" {
		return []GameEvent{NewGameEvent("invalid_action", "start_auction: no currency was chosen", nil)}
	}
	if s.State.PendingAuction != nil {
		return []GameEvent{NewGameEvent("invalid_action", "an auction is already running", nil)}
	}

	minBid := s.amountFor(a, player, cell)
	increment := 0
	if a.Increment != nil {
		increment = *a.Increment
	}
	if increment < 1 {
		// A tenth of the opening bid keeps bidding proportionate to the money
		// in the game, so an author who leaves this blank does not get an
		// auction that climbs one coin at a time.
		increment = minBid / 10
	}
	if increment < 1 {
		increment = 1
	}
	if minBid < 1 {
		minBid = increment
	}

	title := a.Title
	if title == "" {
		what := "this square"
		if cell != nil && cell.Title != "" {
			what = cell.Title
		}
		title = fmt.Sprintf("Auction: %s", what)
	}

	auction := &Auction{
		Title:             title,
		Resource:          a.Resource,
		Increment:         increment,
		MinBid:            minBid,
		StartedByPlayerID: player.ID,
		Order:             s.auctionParticipants(player, a.Target),
		Then:              a.Then,
		Else:              a.Else,
	}
	if cell != nil {
		auction.CellID = cell.ID
	}
	s.State.PendingAuction = auction

	events := []GameEvent{NewGameEvent("auction_started",
		fmt.Sprintf("%s — bidding opens at %d %s", title, minBid, a.Resource),
		map[string]any{"cellId": auction.CellID, "minBid": minBid, "resource": a.Resource})}
	return append(events, s.promptBidder()...)
}

// promptBidder hands the pending action to whoever must answer next, and ends
// the auction when nobody is left to ask.
func (s *GameSession) promptBidder() []GameEvent {
	var events []GameEvent
	for {
		au := s.State.PendingAuction
		if au == nil {
			return events
		}
		if len(au.Order) == 0 {
			return append(events, s.finishAuction()...)
		}
		// The leader is never asked again: everyone else has dropped out, so
		// there is nothing left to answer.
		if au.HighBidderID != "" && len(au.Order) == 1 && au.Order[0] == au.HighBidderID {
			return append(events, s.finishAuction()...)
		}
		if au.Index < 0 || au.Index >= len(au.Order) {
			au.Index = 0
		}

		bidder := s.getPlayerByID(au.Order[au.Index])
		if bidder == nil || bidder.Bankrupt {
			au.removeCurrent()
			continue
		}
		// Somebody who cannot reach the next bid is dropped rather than shown
		// a prompt whose only answer is "pass".
		if bidder.Resources[au.Resource] < au.nextBid() {
			events = append(events, NewGameEvent("auction_pass",
				fmt.Sprintf("%s cannot reach %d %s and drops out", bidder.Name, au.nextBid(), au.Resource), nil))
			au.removeCurrent()
			continue
		}

		standing := fmt.Sprintf("%s — opening bid %d %s", au.Title, au.MinBid, au.Resource)
		if au.HighBidderID != "" {
			leader := s.getPlayerByID(au.HighBidderID)
			name := au.HighBidderID
			if leader != nil {
				name = leader.Name
			}
			standing = fmt.Sprintf("%s — %s leads with %d %s", au.Title, name, au.HighBid, au.Resource)
		}
		s.State.PendingAction = &PendingAction{
			Type:     "auction_bid",
			PlayerID: bidder.ID,
			Title:    standing,
			CellID:   au.CellID,
			Options:  au.bidOptions(bidder),
		}
		return events
	}
}

// resolveAuctionBid applies one bidder's answer.
func (s *GameSession) resolveAuctionBid(optionID string) ([]GameEvent, error) {
	au := s.State.PendingAuction
	pending := s.State.PendingAction
	if au == nil || pending == nil {
		return nil, fmt.Errorf("no auction is running")
	}
	bidder := s.getPlayerByID(pending.PlayerID)
	if bidder == nil {
		return nil, fmt.Errorf("player not found")
	}
	// Only an option the server itself offered a moment ago is accepted: the
	// list is what bounds the bid, and the client is holding a copy of it.
	known := false
	for _, option := range pending.Options {
		if option.ID == optionID {
			known = true
			break
		}
	}
	if !known {
		return nil, fmt.Errorf("unknown option: %s", optionID)
	}

	var events []GameEvent
	if optionID == "pass" {
		events = append(events, NewGameEvent("auction_pass",
			fmt.Sprintf("%s passed", bidder.Name), nil))
		au.removeCurrent()
	} else {
		amount, err := strconv.Atoi(strings.TrimPrefix(optionID, "bid_"))
		if err != nil {
			return nil, fmt.Errorf("unknown option: %s", optionID)
		}
		if amount < au.nextBid() {
			return nil, fmt.Errorf("a bid must be at least %d", au.nextBid())
		}
		// Re-checked against the balance rather than trusted, in case the
		// player's money moved between the prompt and the answer.
		if bidder.Resources[au.Resource] < amount {
			return nil, fmt.Errorf("%s cannot pay %d %s", bidder.Name, amount, au.Resource)
		}
		au.HighBid = amount
		au.HighBidderID = bidder.ID
		events = append(events, NewGameEvent("auction_bid",
			fmt.Sprintf("%s bids %d %s", bidder.Name, amount, au.Resource),
			map[string]any{"playerId": bidder.ID, "amount": amount}))
		au.advance()
	}

	s.State.PendingAction = nil
	events = append(events, s.promptBidder()...)
	// The auction may have ended and its winner's actions may have raised a
	// choice of their own; only a genuinely idle game moves on.
	if s.State.PendingAction == nil {
		s.advanceTurn()
	}
	return events, nil
}

// finishAuction settles the sale and runs the author's follow-up.
func (s *GameSession) finishAuction() []GameEvent {
	au := s.State.PendingAuction
	s.State.PendingAuction = nil
	s.State.PendingAction = nil
	if au == nil {
		return nil
	}
	var cell *CellDefinition
	if au.CellID != "" && s.Definition != nil {
		cell = s.Definition.Board.getCellByID(au.CellID)
	}

	if au.HighBidderID == "" {
		starter := s.getPlayerByID(au.StartedByPlayerID)
		if starter == nil {
			starter = s.CurrentPlayer()
		}
		events := []GameEvent{NewGameEvent("auction_no_sale",
			fmt.Sprintf("%s — nobody bid", au.Title), nil)}
		return append(events, s.executeActions(au.Else, starter, cell)...)
	}

	winner := s.getPlayerByID(au.HighBidderID)
	if winner == nil {
		return []GameEvent{NewGameEvent("auction_no_sale",
			fmt.Sprintf("%s — the winning bidder is gone", au.Title), nil)}
	}
	paid := au.HighBid
	if winner.Resources[au.Resource] < paid {
		paid = winner.Resources[au.Resource]
	}
	winner.Resources[au.Resource] -= paid
	events := []GameEvent{NewGameEvent("auction_won",
		fmt.Sprintf("%s won %s for %d %s", winner.Name, au.Title, paid, au.Resource),
		map[string]any{"playerId": winner.ID, "amount": paid, "cellId": au.CellID})}
	// The follow-up runs as the winner, so "give this cell to the current
	// player" hands it to whoever won the bidding rather than to whoever
	// happened to land on the square.
	return append(events, s.executeActions(au.Then, winner, cell)...)
}
