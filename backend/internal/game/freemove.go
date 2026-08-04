package game

import (
	"fmt"
	"sort"
)

// freeMovementEnabled reports whether a roll buys a destination of the
// player's choosing rather than a walk along a fixed path.
func (s *GameSession) freeMovementEnabled() bool {
	return s != nil && s.Definition != nil && s.Definition.Rules.Movement == "free"
}

// reachableWithin returns every cell reachable from a starting cell in at most
// `steps` edge traversals, with the number of steps it took to get there.
//
// Edges are followed in their declared direction. A board meant for free
// movement therefore declares edges both ways between neighbours; a one-way
// passage stays one-way, which is the point of keeping this on the graph
// rather than on grid coordinates.
func (s *GameSession) reachableWithin(from string, steps int) map[string]int {
	distance := map[string]int{from: 0}
	if steps < 1 {
		return distance
	}
	edges := s.Definition.Board.buildEdgeMap()
	frontier := []string{from}
	for depth := 1; depth <= steps && len(frontier) > 0; depth++ {
		var next []string
		for _, id := range frontier {
			for _, edge := range edges[id] {
				if _, seen := distance[edge.To]; seen {
					continue
				}
				distance[edge.To] = depth
				next = append(next, edge.To)
			}
		}
		frontier = next
	}
	return distance
}

// offerFreeMove asks the player where they want to go.
//
// Standing still is deliberately not offered: a roll has to move you, which
// keeps a player from parking on a good square forever.
func (s *GameSession) offerFreeMove(player *PlayerState, diceRolls []int, total int) []GameEvent {
	reachable := s.reachableWithin(player.PositionCellID, total)
	delete(reachable, player.PositionCellID)

	if len(reachable) == 0 {
		return []GameEvent{NewGameEvent("move_blocked",
			fmt.Sprintf("%s has nowhere to move from %s", player.Name, player.PositionCellID), nil)}
	}

	ids := make([]string, 0, len(reachable))
	for id := range reachable {
		ids = append(ids, id)
	}
	// Nearest first, then by ID, so the option list is stable between rolls
	// instead of reshuffling with Go's map iteration order.
	sort.Slice(ids, func(i, j int) bool {
		if reachable[ids[i]] != reachable[ids[j]] {
			return reachable[ids[i]] < reachable[ids[j]]
		}
		return ids[i] < ids[j]
	})

	options := make([]ActionOption, 0, len(ids))
	for _, id := range ids {
		title := id
		// A face-down cell is offered as a destination but must not announce
		// what is on it; that is the whole point of exploring.
		if cell := s.Definition.Board.getCellByID(id); cell != nil {
			if !s.hiddenCellsEnabled() || s.State.CellStates[id].Revealed {
				title = cell.Title
			} else {
				title = fmt.Sprintf("Unexplored (%d step(s))", reachable[id])
			}
		}
		options = append(options, ActionOption{ID: id, Title: title})
	}

	s.State.PendingAction = &PendingAction{
		Type:     "free_move",
		PlayerID: player.ID,
		Title:    fmt.Sprintf("Move up to %d step(s)", total),
		CellID:   player.PositionCellID,
		Options:  options,
	}
	s.State.PendingMovement = &PendingMovement{
		PlayerID:      player.ID,
		CurrentCellID: player.PositionCellID,
		Dice:          diceRolls,
		Total:         total,
	}
	return []GameEvent{NewGameEvent("free_move_offered",
		fmt.Sprintf("%s may move up to %d step(s)", player.Name, total), nil)}
}

// resolveFreeMove completes a free move once the player has picked a cell.
func (s *GameSession) resolveFreeMove(cellID string) ([]GameEvent, error) {
	pending := s.State.PendingMovement
	if pending == nil {
		return nil, fmt.Errorf("no pending movement context")
	}
	player := s.getPlayerByID(pending.PlayerID)
	if player == nil {
		return nil, fmt.Errorf("player not found")
	}
	target := s.Definition.Board.getCellByID(cellID)
	if target == nil {
		return nil, fmt.Errorf("cell '%s' not found", cellID)
	}
	// Re-check the range on resolution rather than trusting the option list the
	// client was given, since the client is never the authority on a rule.
	reachable := s.reachableWithin(pending.CurrentCellID, pending.Total)
	steps, ok := reachable[cellID]
	if !ok || cellID == pending.CurrentCellID {
		return nil, fmt.Errorf("cell '%s' is not within %d step(s)", cellID, pending.Total)
	}

	s.State.PendingAction = nil
	s.State.PendingMovement = nil

	player.PositionCellID = target.ID
	events := []GameEvent{NewGameEvent("move",
		fmt.Sprintf("%s moved to %s (%d step(s))", player.Name, target.Title, steps),
		map[string]any{
			"from":     pending.CurrentCellID,
			"to":       target.ID,
			"path":     []string{target.ID},
			"playerId": player.ID,
			"dice":     pending.Dice,
			"total":    pending.Total,
		})}

	s.revealCell(target.ID)
	events = append(events, s.executeActions(target.OnLand, player, target)...)
	if s.State.PendingAction == nil {
		s.advanceTurn()
	}
	return events, nil
}
