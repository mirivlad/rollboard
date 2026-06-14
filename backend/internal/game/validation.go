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
	if g.Rules.Dice.Count < 1 {
		errs = append(errs, "dice.count must be at least 1")
	}
	if g.Rules.Dice.Count > 10 {
		errs = append(errs, "dice.count must be at most 10")
	}
	if g.Rules.Dice.Sides < 2 {
		errs = append(errs, "dice.sides must be at least 2")
	}
	if g.Rules.Dice.Sides > 100 {
		errs = append(errs, "dice.sides must be at most 100")
	}
	if len(g.Board.Cells) == 0 {
		errs = append(errs, "board must have at least one cell")
	}

	if g.Board.CellSize > 0 {
		if g.Board.Width%g.Board.CellSize != 0 {
			errs = append(errs, fmt.Sprintf("board.width %d must be divisible by cellSize %d", g.Board.Width, g.Board.CellSize))
		}
		if g.Board.Height%g.Board.CellSize != 0 {
			errs = append(errs, fmt.Sprintf("board.height %d must be divisible by cellSize %d", g.Board.Height, g.Board.CellSize))
		}
	}

	maxCols := 0
	maxRows := 0
	if g.Board.CellSize > 0 {
		maxCols = g.Board.Width / g.Board.CellSize
		maxRows = g.Board.Height / g.Board.CellSize
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
		if g.Board.CellSize > 0 {
			if c.X%g.Board.CellSize != 0 {
				errs = append(errs, fmt.Sprintf("cell '%s' x=%d is not aligned to cellSize=%d", c.ID, c.X, g.Board.CellSize))
			}
			if c.Y%g.Board.CellSize != 0 {
				errs = append(errs, fmt.Sprintf("cell '%s' y=%d is not aligned to cellSize=%d", c.ID, c.Y, g.Board.CellSize))
			}
			if maxCols > 0 {
				cellCol := c.X / g.Board.CellSize
				if cellCol >= maxCols {
					errs = append(errs, fmt.Sprintf("cell '%s' col=%d exceeds board width (max col=%d)", c.ID, cellCol, maxCols-1))
				}
			}
			if maxRows > 0 {
				cellRow := c.Y / g.Board.CellSize
				if cellRow >= maxRows {
					errs = append(errs, fmt.Sprintf("cell '%s' row=%d exceeds board height (max row=%d)", c.ID, cellRow, maxRows-1))
				}
			}
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
