package game

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	Errors []string `json:"errors"`
}

func (ve *ValidationError) Error() string {
	return strings.Join(ve.Errors, "; ")
}

func ValidateDefinition(g *GameDefinition) *ValidationError {
	var errs []string

	if strings.TrimSpace(g.ID) == "" {
		errs = append(errs, "id must not be empty")
	}
	if strings.TrimSpace(g.Title) == "" {
		errs = append(errs, "title must not be empty")
	}
	if g.Board.CellSize <= 0 {
		errs = append(errs, "board.cellSize must be > 0")
	}
	if len(g.Board.Cells) == 0 {
		errs = append(errs, "board must have at least one cell")
	}

	hasStart := false
	cellIDs := make(map[string]bool)
	for _, c := range g.Board.Cells {
		if c.ID == "" {
			errs = append(errs, "cell id must not be empty")
			continue
		}
		if cellIDs[c.ID] {
			errs = append(errs, fmt.Sprintf("duplicate cell id: %s", c.ID))
		}
		cellIDs[c.ID] = true
		if c.Type == "start" {
			hasStart = true
		}
		if g.Rules.CellTypes != nil {
			if _, ok := g.Rules.CellTypes[c.Type]; !ok {
				errs = append(errs, fmt.Sprintf("cell type '%s' not defined in rules.cellTypes", c.Type))
			}
		}
	}

	if !hasStart {
		errs = append(errs, "board must have at least one cell of type 'start'")
	}

	for i, e := range g.Board.Edges {
		if e.ID == "" {
			errs = append(errs, fmt.Sprintf("edge[%d] id must not be empty", i))
		}
		if !cellIDs[e.From] {
			errs = append(errs, fmt.Sprintf("edge '%s' references unknown cell id '%s' in 'from'", e.ID, e.From))
		}
		if !cellIDs[e.To] {
			errs = append(errs, fmt.Sprintf("edge '%s' references unknown cell id '%s' in 'to'", e.ID, e.To))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{Errors: errs}
}
