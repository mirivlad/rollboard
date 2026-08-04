package game

import (
	"fmt"
	"strconv"
)

// CellQuery selects a set of cells on the board.
//
// Rent that scales with how many stations one player holds, and a colour group
// that pays double once somebody owns all of it, are the same question asked
// twice: "how many cells match this description?". Neither could be expressed
// before, because an action could only ever see the single cell it was
// attached to.
//
// The filters are deliberately the ones an author can fill in from dropdowns:
// a cell type, a field they defined themselves, and who owns it. There is no
// query language to learn.
type CellQuery struct {
	// Type restricts the count to one cell type. Empty means every type.
	Type string `json:"type,omitempty"`
	// Field names a field on the cell and Value the value it must hold.
	// Values compare as text, so a group named "blue" and a price written as
	// 200 both work without the author choosing a type.
	Field string `json:"field,omitempty"`
	Value string `json:"value,omitempty"`
	// SameAsCell compares Field against the same field on the cell the action
	// is running for, instead of against Value. That is what "the rest of my
	// colour group" means without having to name the colour: one query works
	// on every cell in every group.
	SameAsCell bool `json:"sameAsCell,omitempty"`
	// Owner filters by ownership:
	//
	//	""/"any"    no filter
	//	"none"      nobody owns it
	//	"current"   the acting player owns it
	//	"cellOwner" whoever owns the cell being resolved owns it
	//	"other"     somebody other than the acting player owns it
	//
	// "cellOwner" is the one rent needs: the visitor is asking how much the
	// landlord owns, not how much they themselves own.
	Owner string `json:"owner,omitempty"`
	// MinLevel counts only cells built up to at least this level.
	MinLevel *int `json:"minLevel,omitempty"`
	// ExcludeCurrentCell leaves the cell being resolved out of the count, for
	// "how many others" questions.
	ExcludeCurrentCell bool `json:"excludeCurrentCell,omitempty"`
}

// fieldText renders a cell field as text so values compare without the author
// having to declare a type. A JSON number arrives as a float64 and would
// otherwise print as "3" or "3.0000001" depending on how it was written.
func fieldText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ownerMatches applies the ownership filter for one cell.
func (s *GameSession) ownerMatches(filter, cellOwnerID string, player *PlayerState, reference *CellDefinition) bool {
	switch filter {
	case "", "any":
		return true
	case "none":
		return cellOwnerID == ""
	case "current":
		return player != nil && cellOwnerID == player.ID
	case "other":
		return cellOwnerID != "" && (player == nil || cellOwnerID != player.ID)
	case "cellOwner":
		if reference == nil {
			return false
		}
		landlord := s.State.CellStates[reference.ID].OwnerPlayerID
		// An unowned reference cell has no landlord, so nothing can match it.
		// Counting every unowned cell instead would make rent on a free square
		// scale with the size of the board.
		return landlord != "" && cellOwnerID == landlord
	default:
		return false
	}
}

// matchesQuery reports whether one cell satisfies the query.
func (s *GameSession) matchesQuery(q *CellQuery, candidate CellDefinition, player *PlayerState, reference *CellDefinition) bool {
	if q.Type != "" && candidate.Type != q.Type {
		return false
	}
	if q.ExcludeCurrentCell && reference != nil && candidate.ID == reference.ID {
		return false
	}
	if q.Field != "" {
		want := q.Value
		if q.SameAsCell {
			if reference == nil {
				return false
			}
			want = fieldText(reference.Fields[q.Field])
			// A reference cell that does not carry the field at all matches
			// nothing, rather than matching every other cell that is also
			// missing it.
			if want == "" {
				return false
			}
		}
		if fieldText(candidate.Fields[q.Field]) != want {
			return false
		}
	}
	state := s.State.CellStates[candidate.ID]
	if q.MinLevel != nil && state.Level < *q.MinLevel {
		return false
	}
	return s.ownerMatches(q.Owner, state.OwnerPlayerID, player, reference)
}

// matchingCells returns every cell the query selects, in board order.
func (s *GameSession) matchingCells(q *CellQuery, player *PlayerState, reference *CellDefinition) []*CellDefinition {
	if q == nil || s.Definition == nil {
		return nil
	}
	var found []*CellDefinition
	for i := range s.Definition.Board.Cells {
		if s.matchesQuery(q, s.Definition.Board.Cells[i], player, reference) {
			found = append(found, &s.Definition.Board.Cells[i])
		}
	}
	return found
}

// countCells is the query as a number, which is what a formula needs.
func (s *GameSession) countCells(q *CellQuery, player *PlayerState, reference *CellDefinition) int {
	return len(s.matchingCells(q, player, reference))
}

// describeQuery names a query in a log line. Authors read the log to work out
// why a rent came to what it did, so "3 cells" alone would not help.
func describeQuery(q *CellQuery) string {
	if q == nil {
		return "cells"
	}
	what := "cells"
	if q.Type != "" {
		what = q.Type + " cells"
	}
	switch q.Owner {
	case "current":
		what += " they own"
	case "cellOwner":
		what += " the owner holds"
	case "none":
		what += " nobody owns"
	case "other":
		what += " somebody else owns"
	}
	if q.Field != "" {
		if q.SameAsCell {
			what += fmt.Sprintf(" in the same %s", q.Field)
		} else {
			what += fmt.Sprintf(" with %s %s", q.Field, q.Value)
		}
	}
	return what
}
