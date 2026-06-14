package game

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	Errors []string `json:"errors"`
}

// validateAction checks a single action definition for obvious errors.
func validateAction(action ActionDefinition) []string {
	var errs []string
	if strings.TrimSpace(action.Type) == "" {
		errs = append(errs, "action type must not be empty")
		return errs
	}
	switch action.Type {
	case "gain_resource", "lose_resource":
		if strings.TrimSpace(action.Resource) == "" {
			errs = append(errs, fmt.Sprintf("%s: resource is required", action.Type))
		}
		if action.Amount != nil && *action.Amount < 0 {
			errs = append(errs, fmt.Sprintf("%s: amount must be non-negative", action.Type))
		}
	case "transfer_resource":
		if strings.TrimSpace(action.Resource) == "" {
			errs = append(errs, "transfer_resource: resource is required")
		}
		if action.Amount != nil && *action.Amount < 0 {
			errs = append(errs, "transfer_resource: amount must be non-negative")
		}
	case "finish_game":
		// no required fields
	case "log_message":
		if strings.TrimSpace(action.Title) == "" {
			errs = append(errs, "log_message: message (title) is required")
		}
	case "set_cell_owner":
		if strings.TrimSpace(action.Target) == "" {
			errs = append(errs, "set_cell_owner: target is required")
		}
	case "if_resource_ge":
		if strings.TrimSpace(action.Resource) == "" {
			errs = append(errs, "if_resource_ge: resource is required")
		}
		if action.Amount != nil && *action.Amount < 0 {
			errs = append(errs, "if_resource_ge: amount must be non-negative")
		}
		// Recursively validate nested then/else actions
		for _, a := range action.Then {
			errs = append(errs, validateAction(a)...)
		}
		for _, a := range action.Else {
			errs = append(errs, validateAction(a)...)
		}
	default:
		// Unknown action types are allowed (forward compatibility)
	}
	return errs
}

func validateActions(cellID string, listName string, actions []ActionDefinition, g *GameDefinition) []string {
	var errs []string
	if actions == nil {
		return errs
	}
	for i, a := range actions {
		aerrs := validateAction(a)
		for _, e := range aerrs {
			errs = append(errs, fmt.Sprintf("cell '%s' %s action[%d]: %s", cellID, listName, i, e))
		}
	}
	return errs
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
		// Validate actions
		actionErrs := validateActions(c.ID, "onLand", c.OnLand, g)
		errs = append(errs, actionErrs...)
		actionErrs = validateActions(c.ID, "onPass", c.OnPass, g)
		errs = append(errs, actionErrs...)
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
