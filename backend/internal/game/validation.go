package game

import (
	"fmt"
	"slices"
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
	case "set_cell_level":
		if action.Amount != nil && *action.Amount < 0 {
			errs = append(errs, "set_cell_level: amount must be non-negative")
		}
	case "if_cell_level_ge":
		if action.Amount != nil && *action.Amount < 0 {
			errs = append(errs, "if_cell_level_ge: amount must be non-negative")
		}
		errs = append(errs, validateBranches(action)...)
	case "set_cell_mortgaged":
		if action.Target != "true" && action.Target != "false" {
			errs = append(errs, `set_cell_mortgaged: target must be "true" or "false"`)
		}
	case "if_cell_mortgaged":
		errs = append(errs, validateBranches(action)...)
	case "move_player_to":
		if strings.TrimSpace(action.To) == "" {
			errs = append(errs, "move_player_to: to (target cell ID) is required")
		}
	case "skip_turns":
		if action.Amount == nil && strings.TrimSpace(action.AmountField) == "" {
			errs = append(errs, "skip_turns: amount or amountField is required")
		}
		if action.Amount != nil && *action.Amount < 0 {
			errs = append(errs, "skip_turns: amount must be non-negative")
		}
	case "random_branch":
		if len(action.Options) < 2 {
			errs = append(errs, "random_branch: at least two options are required")
		}
		for _, option := range action.Options {
			if strings.TrimSpace(option.ID) == "" {
				errs = append(errs, "random_branch: every option needs an id")
			}
			for _, nested := range option.Then {
				errs = append(errs, validateAction(nested)...)
			}
		}
	case "grant_item", "remove_item", "equip_item", "use_item":
		if strings.TrimSpace(action.Field) == "" {
			errs = append(errs, fmt.Sprintf("%s: field (item id) is required", action.Type))
		}
	case "unequip_slot":
		if strings.TrimSpace(action.Target) == "" {
			errs = append(errs, "unequip_slot: target (slot name) is required")
		}
	case "if_has_item":
		if strings.TrimSpace(action.Field) == "" {
			errs = append(errs, "if_has_item: field (item id) is required")
		}
		errs = append(errs, validateBranches(action)...)
	case "if_stat_ge":
		if strings.TrimSpace(action.Resource) == "" {
			errs = append(errs, "if_stat_ge: resource is required")
		}
		errs = append(errs, validateBranches(action)...)
	case "reveal_cells":
		if action.Amount != nil && *action.Amount < 0 {
			errs = append(errs, "reveal_cells: amount must be non-negative")
		}
	case "eliminate_player":
		// no required fields
	case "if_cells_ge", "for_each_cell":
		if action.Query == nil {
			errs = append(errs, fmt.Sprintf("%s: query is required", action.Type))
		} else {
			errs = append(errs, validateQuery(action.Type, action.Query)...)
		}
		errs = append(errs, validateBranches(action)...)
	case "start_auction":
		if strings.TrimSpace(action.Resource) == "" {
			errs = append(errs, "start_auction: resource is required")
		}
		if action.Increment != nil && *action.Increment < 0 {
			errs = append(errs, "start_auction: increment must be non-negative")
		}
		if action.Amount != nil && *action.Amount < 0 {
			errs = append(errs, "start_auction: amount must be non-negative")
		}
		if len(action.Then) == 0 {
			// An auction that awards nothing takes money off the winner and
			// gives them nothing back, which is never what the author meant.
			errs = append(errs, "start_auction: then (what the winner receives) must not be empty")
		}
		errs = append(errs, validateBranches(action)...)
	case "launch_minigame":
		if action.MiniGame == nil || strings.TrimSpace(action.MiniGame.ModuleID) == "" || action.MiniGame.Version < 1 {
			errs = append(errs, "launch_minigame: miniGame.moduleId and positive miniGame.version are required")
			return errs
		}
		errs = append(errs, "launch_minigame: mini-game modules are not enabled in this build")
	default:
		// Unknown action types are allowed (forward compatibility)
	}
	return errs
}

// validateQuery checks a cell query on its own terms.
//
// A query is a filter, so every mistake in one is silent: it simply matches
// nothing, and the author sees rent of zero with no idea why.
func validateQuery(actionType string, q *CellQuery) []string {
	var errs []string
	switch q.Owner {
	case "", "any", "none", "current", "other", "cellOwner":
	default:
		errs = append(errs, fmt.Sprintf("%s: query owner %q is not one of any, none, current, other, cellOwner", actionType, q.Owner))
	}
	if q.SameAsCell && strings.TrimSpace(q.Field) == "" {
		errs = append(errs, fmt.Sprintf("%s: query matches the same field on this cell but names no field", actionType))
	}
	if strings.TrimSpace(q.Value) != "" && strings.TrimSpace(q.Field) == "" {
		errs = append(errs, fmt.Sprintf("%s: query has a value but no field to compare it against", actionType))
	}
	if q.MinLevel != nil && *q.MinLevel < 0 {
		errs = append(errs, fmt.Sprintf("%s: query minLevel must be non-negative", actionType))
	}
	return errs
}

// validateBranches checks the then/else arms of a conditional action.
func validateBranches(action ActionDefinition) []string {
	var errs []string
	for _, a := range action.Then {
		errs = append(errs, validateAction(a)...)
	}
	for _, a := range action.Else {
		errs = append(errs, validateAction(a)...)
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
		aerrs = append(aerrs, validateCellReferences(a, cellID, g)...)
		for _, e := range aerrs {
			errs = append(errs, fmt.Sprintf("cell '%s' %s action[%d]: %s", cellID, listName, i, e))
		}
	}
	return errs
}

// validateCellReferences resolves every destination a teleport can reach,
// including the ones nested inside branches and random options.
//
// A teleport to a missing cell would only surface at play time, and a teleport
// to the cell that owns the action would re-run that same action forever, so
// both are rejected at publication.
func validateCellReferences(action ActionDefinition, owningCellID string, g *GameDefinition) []string {
	var errs []string
	switch action.Type {
	case "grant_item", "remove_item", "equip_item", "use_item", "if_has_item":
		// A typo in an item id would otherwise be silent: the action would run
		// and simply do nothing.
		if id := strings.TrimSpace(action.Field); id != "" {
			if _, ok := g.Rules.Items[id]; !ok {
				errs = append(errs, fmt.Sprintf("%s: no item %q is defined", action.Type, id))
			}
		}
	case "unequip_slot":
		if slot := strings.TrimSpace(action.Target); slot != "" && !slices.Contains(g.Rules.EquipmentSlots, slot) {
			errs = append(errs, fmt.Sprintf("unequip_slot: no equipment slot %q is defined", slot))
		}
	case "if_cells_ge", "for_each_cell":
		// A query naming a cell type that does not exist matches nothing, and
		// a rent that silently comes to zero is the hardest kind of mistake to
		// find by playing.
		if action.Query != nil && g.Rules.CellTypes != nil {
			if typ := strings.TrimSpace(action.Query.Type); typ != "" {
				if _, ok := g.Rules.CellTypes[typ]; !ok {
					errs = append(errs, fmt.Sprintf("%s: query names unknown cell type %q", action.Type, typ))
				}
			}
		}
	case "start_auction":
		if res := strings.TrimSpace(action.Resource); res != "" && g.Rules.Resources != nil {
			if _, ok := g.Rules.Resources[res]; !ok {
				errs = append(errs, fmt.Sprintf("start_auction: no resource %q is defined", res))
			}
		}
	}
	if action.Type == "move_player_to" {
		to := strings.TrimSpace(action.To)
		if to != "" {
			if g.Board.getCellByID(to) == nil {
				errs = append(errs, fmt.Sprintf("move_player_to: no cell %q", to))
			} else if to == owningCellID {
				errs = append(errs, "move_player_to: a cell cannot move a player onto itself")
			}
		}
	}
	for _, nested := range action.Then {
		errs = append(errs, validateCellReferences(nested, owningCellID, g)...)
	}
	for _, nested := range action.Else {
		errs = append(errs, validateCellReferences(nested, owningCellID, g)...)
	}
	for _, option := range action.Options {
		for _, nested := range option.Then {
			errs = append(errs, validateCellReferences(nested, owningCellID, g)...)
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

	for id, item := range g.Rules.Items {
		if strings.TrimSpace(item.Title) == "" {
			errs = append(errs, fmt.Sprintf("item %q: title is required", id))
		}
		if item.Slot != "" && !slices.Contains(g.Rules.EquipmentSlots, item.Slot) {
			errs = append(errs, fmt.Sprintf("item %q: slot %q is not in rules.equipmentSlots", id, item.Slot))
		}
		for resource := range item.Bonuses {
			if _, ok := g.Rules.Resources[resource]; !ok {
				errs = append(errs, fmt.Sprintf("item %q: bonus refers to unknown resource %q", id, resource))
			}
		}
		for _, use := range item.Use {
			for _, e := range validateAction(use) {
				errs = append(errs, fmt.Sprintf("item %q use: %s", id, e))
			}
		}
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
