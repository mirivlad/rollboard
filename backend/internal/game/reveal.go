package game

import (
	"encoding/json"
	"fmt"
)

// hiddenCellsEnabled reports whether this game plays with face-down cells.
func (s *GameSession) hiddenCellsEnabled() bool {
	// A session can legitimately exist without a definition attached — a room
	// snapshot being assembled, a zero value in a test — and marshalling one
	// must not panic.
	return s != nil && s.Definition != nil && s.Definition.Rules.HiddenCells
}

// revealCell turns one cell face up and reports whether it changed anything.
func (s *GameSession) revealCell(cellID string) bool {
	if !s.hiddenCellsEnabled() {
		return false
	}
	state, known := s.State.CellStates[cellID]
	if !known {
		return false
	}
	if state.Revealed {
		return false
	}
	state.Revealed = true
	s.State.CellStates[cellID] = state
	return true
}

// revealCells implements the reveal_cells action.
//
// Without a target it turns over everything within `amount` steps of the
// player, following edges in the direction they can actually be walked. That
// is what a scouting perk means: you see ahead, not behind.
func (s *GameSession) revealCells(player *PlayerState, a ActionDefinition, cell *CellDefinition) []GameEvent {
	if !s.hiddenCellsEnabled() {
		return nil
	}

	if to := a.To; to != "" {
		if s.Definition.Board.getCellByID(to) == nil {
			return []GameEvent{NewGameEvent("invalid_action",
				fmt.Sprintf("Cannot reveal: no cell %q", to), nil)}
		}
		if !s.revealCell(to) {
			return nil
		}
		return []GameEvent{NewGameEvent("reveal_cells",
			fmt.Sprintf("%s scouted %s", player.Name, to), nil)}
	}

	radius := s.amountForOrOne(a, player, cell)
	edges := s.Definition.Board.buildEdgeMap()

	// Breadth-first so "within N steps" means exactly that.
	frontier := []string{player.PositionCellID}
	seen := map[string]bool{player.PositionCellID: true}
	revealed := 0
	for depth := 0; depth < radius && len(frontier) > 0; depth++ {
		var next []string
		for _, id := range frontier {
			for _, edge := range edges[id] {
				if seen[edge.To] {
					continue
				}
				seen[edge.To] = true
				if s.revealCell(edge.To) {
					revealed++
				}
				next = append(next, edge.To)
			}
		}
		frontier = next
	}
	if revealed == 0 {
		return nil
	}
	return []GameEvent{NewGameEvent("reveal_cells",
		fmt.Sprintf("%s scouted %d cell(s) ahead", player.Name, revealed), nil)}
}

// VisibleDefinition returns the board as this session should be shown.
//
// A hidden cell keeps its position and size, because the shape of the map is
// not the secret, but loses its title, colour, image, fields and actions.
// Stripping happens on the server: a client that never receives the contents
// cannot reveal them by reading its own memory.
func (s *GameSession) VisibleDefinition() *GameDefinition {
	if !s.hiddenCellsEnabled() {
		return s.Definition
	}
	if s.State.CellStates == nil {
		return s.Definition
	}
	shown := *s.Definition
	shown.Board = s.Definition.Board
	cells := make([]CellDefinition, len(s.Definition.Board.Cells))
	for i, cell := range s.Definition.Board.Cells {
		if s.State.CellStates[cell.ID].Revealed {
			cells[i] = cell
			continue
		}
		cells[i] = CellDefinition{
			ID:     cell.ID,
			Title:  "",
			Type:   hiddenCellType,
			X:      cell.X,
			Y:      cell.Y,
			Visual: CellVisual{},
			Fields: map[string]any{},
		}
	}
	shown.Board.Cells = cells
	return &shown
}

// hiddenCellType is the type a face-down cell reports, so the interface can
// draw a card back without knowing what is underneath.
const hiddenCellType = "__hidden"

// MarshalJSON substitutes the visible definition whenever a session is sent
// anywhere.
//
// Doing this in the marshaller rather than at each call site is deliberate:
// the hotseat API, the room snapshot, the WebSocket transitions and the stored
// event journal all serialise sessions, and a face-down cell whose contents
// leak through any one of them is not hidden at all.
//
// Storage is the one place that must NOT go through here — see ForStorage.
func (s *GameSession) MarshalJSON() ([]byte, error) {
	type sessionAlias GameSession
	shown := sessionAlias(*s)
	shown.Definition = s.VisibleDefinition()
	return json.Marshal(shown)
}

// ForStorage wraps a session so it serialises in full.
//
// Persisting the stripped view would destroy the game: a face-down cell would
// come back from the database with no title, no fields and no actions, and the
// next reveal would turn over an empty square. Found by playing a dungeon
// where every chest turned out to be empty after the first save.
type ForStorage struct{ Session *GameSession }

func (f ForStorage) MarshalJSON() ([]byte, error) {
	if f.Session == nil {
		return []byte("null"), nil
	}
	type sessionAlias GameSession
	return json.Marshal((*sessionAlias)(f.Session))
}
