package game

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type ValidationError struct {
	Errors []string `json:"errors"`
}

// actionContext is what an action needs to be checked against: the definition
// it belongs to, and the cell that owns it.
//
// A definition is always available, because most of what can go wrong in an
// action is a reference to something that does not exist — a resource, an item,
// a cell type, a field. Those mistakes are invisible at play time: the action
// runs, resolves to zero or matches nothing, and the game is quietly wrong.
type actionContext struct {
	game         *GameDefinition
	owningCellID string
}

// playerSeat matches the IDs the engine hands out, so a definition can name a
// seat directly without being able to invent a player that never exists.
var playerSeat = regexp.MustCompile(`^player_[0-9]+$`)

// quantityActions take an amount that is a count, never a direction.
//
// The engine refuses a negative one at run time; publication refuses the ones
// that can be seen to be negative before the game is ever played.
var quantityActions = map[string]bool{
	"gain_resource": true, "lose_resource": true, "transfer_resource": true,
	"skip_turns": true, "set_cell_level": true, "grant_item": true,
	"remove_item": true, "if_has_item": true, "if_cell_level_ge": true,
	"if_resource_ge": true, "if_stat_ge": true, "if_cells_ge": true,
	"reveal_cells": true, "start_auction": true,
}

func validateAction(action ActionDefinition, path string, ctx actionContext) []string {
	var errs []string
	at := func(format string, args ...any) string {
		return fmt.Sprintf("%s: %s", path, fmt.Sprintf(format, args...))
	}

	if strings.TrimSpace(action.Type) == "" {
		return []string{at("action type must not be empty")}
	}

	// --- fields shared by many actions ---------------------------------------
	if action.Amount != nil && *action.Amount < 0 && quantityActions[action.Type] {
		errs = append(errs, at("%s: amount must be non-negative", action.Type))
	}
	errs = append(errs, validateFormula(action.Formula, path+".formula", ctx, quantityActions[action.Type])...)

	switch action.Type {
	case "gain_resource", "lose_resource":
		errs = append(errs, requireResource(action.Resource, action.Type, at, ctx)...)
	case "transfer_resource":
		errs = append(errs, requireResource(action.Resource, action.Type, at, ctx)...)
		errs = append(errs, validateTransferTarget(action.Target, at)...)
	case "finish_game", "eliminate_player":
		// no required fields
	case "log_message":
		if strings.TrimSpace(action.Title) == "" {
			errs = append(errs, at("log_message: message (title) is required"))
		}
	case "set_cell_owner":
		errs = append(errs, validateCellOwnerTarget(action.Target, at)...)
	case "if_resource_ge", "if_stat_ge":
		errs = append(errs, requireResource(action.Resource, action.Type, at, ctx)...)
	case "set_cell_level", "if_cell_level_ge", "if_cell_mortgaged", "if_cell_unowned",
		"if_cell_owned_by_current", "if_cell_owned_by_other":
		// amounts and branches are handled outside the switch
	case "set_cell_mortgaged":
		if action.Target != "true" && action.Target != "false" {
			errs = append(errs, at(`set_cell_mortgaged: target must be "true" or "false"`))
		}
	case "move_player_to":
		to := strings.TrimSpace(action.To)
		if to == "" {
			errs = append(errs, at("move_player_to: to (target cell ID) is required"))
		} else if ctx.game != nil {
			if ctx.game.Board.getCellByID(to) == nil {
				errs = append(errs, at("move_player_to: no cell %q", to))
			} else if to == ctx.owningCellID {
				errs = append(errs, at("move_player_to: a cell cannot move a player onto itself"))
			}
		}
	case "skip_turns":
		// A formula counts: the editor offers one, and the engine resolves it.
		if action.Amount == nil && strings.TrimSpace(action.AmountField) == "" && action.Formula == nil {
			errs = append(errs, at("skip_turns: amount, amountField or a computed amount is required"))
		}
	case "random_branch":
		if len(action.Options) < 2 {
			errs = append(errs, at("random_branch: at least two options are required"))
		}
	case "offer_choice":
		if strings.TrimSpace(action.Title) == "" {
			errs = append(errs, at("offer_choice: title is required — the player reads it"))
		}
		if len(action.Options) == 0 {
			errs = append(errs, at("offer_choice: at least one option is required"))
		}
		for i, option := range action.Options {
			if strings.TrimSpace(option.Title) == "" {
				errs = append(errs, at("offer_choice: option[%d] needs a title the player can read", i))
			}
		}
	case "grant_item", "remove_item", "equip_item", "use_item", "if_has_item":
		id := strings.TrimSpace(action.Field)
		if id == "" {
			errs = append(errs, at("%s: field (item id) is required", action.Type))
		} else if ctx.game != nil {
			if _, ok := ctx.game.Rules.Items[id]; !ok {
				errs = append(errs, at("%s: no item %q is defined", action.Type, id))
			}
		}
	case "unequip_slot":
		slot := strings.TrimSpace(action.Target)
		if slot == "" {
			errs = append(errs, at("unequip_slot: target (slot name) is required"))
		} else if ctx.game != nil && !slices.Contains(ctx.game.Rules.EquipmentSlots, slot) {
			errs = append(errs, at("unequip_slot: no equipment slot %q is defined", slot))
		}
	case "reveal_cells":
		if to := strings.TrimSpace(action.To); to != "" && ctx.game != nil && ctx.game.Board.getCellByID(to) == nil {
			errs = append(errs, at("reveal_cells: no cell %q", to))
		}
	case "if_cells_ge", "for_each_cell":
		if action.Query == nil {
			errs = append(errs, at("%s: query is required", action.Type))
		} else {
			errs = append(errs, validateQuery(action.Query, path+".query", ctx)...)
		}
	case "start_auction":
		errs = append(errs, requireResource(action.Resource, "start_auction", at, ctx)...)
		if action.Increment != nil && *action.Increment < 0 {
			errs = append(errs, at("start_auction: increment must be non-negative"))
		}
		if action.Target != "" && action.Target != "others" {
			errs = append(errs, at(`start_auction: bidders must be empty (everyone) or "others"`))
		}
		if len(action.Then) == 0 {
			// An auction that awards nothing takes money off the winner and
			// gives them nothing back, which is never what the author meant.
			errs = append(errs, at("start_auction: then (what the winner receives) must not be empty"))
		}
	case "launch_minigame":
		if action.MiniGame == nil || strings.TrimSpace(action.MiniGame.ModuleID) == "" || action.MiniGame.Version < 1 {
			errs = append(errs, at("launch_minigame: miniGame.moduleId and positive miniGame.version are required"))
			return errs
		}
		errs = append(errs, at("launch_minigame: mini-game modules are not enabled in this build"))
	default:
		// Unknown action types are allowed (forward compatibility)
	}

	// --- option ids ----------------------------------------------------------
	// The editor used to name a new option after the length of the list, so
	// adding, removing and adding again produced two options with one ID. The
	// engine resolves the first match, which makes the second one unreachable
	// with nothing to show for it.
	seen := map[string]bool{}
	for i, option := range action.Options {
		id := strings.TrimSpace(option.ID)
		if id == "" {
			errs = append(errs, at("%s: option[%d] needs an id", action.Type, i))
			continue
		}
		if seen[id] {
			errs = append(errs, at("%s: option id %q is used twice — only the first would ever run", action.Type, id))
		}
		seen[id] = true
	}

	// --- nested actions ------------------------------------------------------
	for i, nested := range action.Then {
		errs = append(errs, validateAction(nested, fmt.Sprintf("%s.then[%d]", path, i), ctx)...)
	}
	for i, nested := range action.Else {
		errs = append(errs, validateAction(nested, fmt.Sprintf("%s.else[%d]", path, i), ctx)...)
	}
	for i, option := range action.Options {
		for j, nested := range option.Then {
			errs = append(errs, validateAction(nested, fmt.Sprintf("%s.options[%d].then[%d]", path, i, j), ctx)...)
		}
	}
	return errs
}

func requireResource(name, actionType string, at func(string, ...any) string, ctx actionContext) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return []string{at("%s: resource is required", actionType)}
	}
	if ctx.game != nil && ctx.game.Rules.Resources != nil {
		if _, ok := ctx.game.Rules.Resources[name]; !ok {
			return []string{at("%s: no resource %q is declared in the rules", actionType, name)}
		}
	}
	return nil
}

// validateTransferTarget accepts only the recipients the engine can resolve.
func validateTransferTarget(target string, at func(string, ...any) string) []string {
	switch {
	case target == "owner":
		return nil
	case playerSeat.MatchString(target):
		return nil
	case target == "":
		return []string{at("transfer_resource: target (who is paid) is required")}
	default:
		return []string{at(`transfer_resource: target %q is not a recipient — use "owner" or a seat like "player_2"`, target)}
	}
}

func validateCellOwnerTarget(target string, at func(string, ...any) string) []string {
	switch {
	case target == "current" || target == "none":
		return nil
	case playerSeat.MatchString(target):
		return nil
	case target == "":
		return []string{at("set_cell_owner: target is required")}
	default:
		return []string{at(`set_cell_owner: target %q is not an owner — use "current", "none" or a seat like "player_2"`, target)}
	}
}

// validateFormula checks a computed amount all the way down.
//
// resolveTerm answers zero for anything it does not understand, so a typo in a
// term used to become a rule that silently did nothing rather than a game that
// refused to publish.
func validateFormula(f *AmountFormula, path string, ctx actionContext, quantity bool) []string {
	if f == nil {
		return nil
	}
	var errs []string
	for name, term := range map[string]*AmountTerm{
		"base": f.Base, "plus": f.Plus, "minus": f.Minus, "times": f.Times, "dividedBy": f.DividedBy,
	} {
		errs = append(errs, validateTerm(term, path+"."+name, ctx)...)
	}
	if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
		errs = append(errs, fmt.Sprintf("%s: at least %d is above at most %d, so this can never hold", path, *f.Min, *f.Max))
	}
	if quantity {
		if f.Max != nil && *f.Max < 0 {
			errs = append(errs, fmt.Sprintf("%s: at most %d forces a negative amount", path, *f.Max))
		}
		if value, static := staticValue(f); static && value < 0 {
			errs = append(errs, fmt.Sprintf("%s: this always works out to %d, and an amount cannot be negative", path, value))
		}
	}
	// Sorted so the same definition always reports its problems in the same
	// order; the map above iterates at random.
	slices.Sort(errs)
	return errs
}

// staticValue evaluates a formula made only of constants.
func staticValue(f *AmountFormula) (int, bool) {
	constant := func(term *AmountTerm) (int, bool) {
		if term == nil {
			return 0, true
		}
		if term.Kind != "const" && term.Kind != "" {
			return 0, false
		}
		return term.Value, true
	}
	base, ok1 := constant(f.Base)
	plus, ok2 := constant(f.Plus)
	minus, ok3 := constant(f.Minus)
	if !ok1 || !ok2 || !ok3 {
		return 0, false
	}
	value := base + plus - minus
	if f.Times != nil {
		times, ok := constant(f.Times)
		if !ok {
			return 0, false
		}
		value *= times
	}
	if f.DividedBy != nil {
		divisor, ok := constant(f.DividedBy)
		if !ok {
			return 0, false
		}
		if divisor != 0 {
			value /= divisor
		}
	}
	if f.Min != nil && value < *f.Min {
		value = *f.Min
	}
	if f.Max != nil && value > *f.Max {
		value = *f.Max
	}
	return value, true
}

func validateTerm(t *AmountTerm, path string, ctx actionContext) []string {
	if t == nil {
		return nil
	}
	switch t.Kind {
	case "const", "":
		return nil
	case "stat", "resource":
		if strings.TrimSpace(t.Name) == "" {
			return []string{fmt.Sprintf("%s: a %s term needs a name", path, t.Kind)}
		}
		if ctx.game != nil && ctx.game.Rules.Resources != nil {
			if _, ok := ctx.game.Rules.Resources[t.Name]; !ok {
				return []string{fmt.Sprintf("%s: no resource %q is declared in the rules", path, t.Name)}
			}
		}
		return nil
	case "field":
		if strings.TrimSpace(t.Name) == "" {
			return []string{fmt.Sprintf("%s: a field term needs a field name", path)}
		}
		if ctx.game != nil && !fieldExists(ctx.game, t.Name) {
			return []string{fmt.Sprintf("%s: no cell type or cell carries a field %q", path, t.Name)}
		}
		return nil
	case "cells":
		if t.Query == nil {
			return []string{fmt.Sprintf("%s: a cells term needs a query saying which cells to count", path)}
		}
		return validateQuery(t.Query, path+".query", ctx)
	default:
		return []string{fmt.Sprintf("%s: %q is not a kind of value (use const, field, stat, resource or cells)", path, t.Kind)}
	}
}

// fieldExists reports whether any cell type declares the field, or any cell
// actually carries it.
//
// Authors set fields on cells directly as well as through the type, so
// requiring a declaration would reject working boards.
func fieldExists(g *GameDefinition, name string) bool {
	for _, cellType := range g.Rules.CellTypes {
		if _, ok := cellType.Fields[name]; ok {
			return true
		}
	}
	for _, cell := range g.Board.Cells {
		if _, ok := cell.Fields[name]; ok {
			return true
		}
	}
	return false
}

// validateQuery checks a cell query on its own terms.
//
// A query is a filter, so every mistake in one is silent: it simply matches
// nothing, and the author sees rent of zero with no idea why.
func validateQuery(q *CellQuery, path string, ctx actionContext) []string {
	var errs []string
	switch q.Owner {
	case "", "any", "none", "current", "other", "cellOwner":
	default:
		errs = append(errs, fmt.Sprintf("%s: owner %q is not one of any, none, current, other, cellOwner", path, q.Owner))
	}
	if q.SameAsCell && strings.TrimSpace(q.Field) == "" {
		errs = append(errs, fmt.Sprintf("%s: matches the same field on this cell but names no field", path))
	}
	if strings.TrimSpace(q.Value) != "" && strings.TrimSpace(q.Field) == "" {
		errs = append(errs, fmt.Sprintf("%s: has a value but no field to compare it against", path))
	}
	if q.MinLevel != nil && *q.MinLevel < 0 {
		errs = append(errs, fmt.Sprintf("%s: minLevel must be non-negative", path))
	}
	if ctx.game != nil {
		if typ := strings.TrimSpace(q.Type); typ != "" && ctx.game.Rules.CellTypes != nil {
			if _, ok := ctx.game.Rules.CellTypes[typ]; !ok {
				errs = append(errs, fmt.Sprintf("%s: unknown cell type %q", path, typ))
			}
		}
		if field := strings.TrimSpace(q.Field); field != "" && !fieldExists(ctx.game, field) {
			errs = append(errs, fmt.Sprintf("%s: no cell type or cell carries a field %q", path, field))
		}
	}
	return errs
}

func validateActions(cellID string, listName string, actions []ActionDefinition, g *GameDefinition) []string {
	var errs []string
	ctx := actionContext{game: g, owningCellID: cellID}
	for i, a := range actions {
		errs = append(errs, validateAction(a, fmt.Sprintf("cell '%s' %s action[%d]", cellID, listName, i), ctx)...)
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

	for name, rule := range g.Rules.Resources {
		if rule.Min != nil && rule.Max != nil && *rule.Min > *rule.Max {
			errs = append(errs, fmt.Sprintf("resource %q: min %d is above max %d", name, *rule.Min, *rule.Max))
		}
		if rule.Max != nil && rule.Initial > *rule.Max {
			errs = append(errs, fmt.Sprintf("resource %q: every player starts with %d, above the maximum of %d", name, rule.Initial, *rule.Max))
		}
		if rule.Min != nil && rule.Initial < *rule.Min {
			errs = append(errs, fmt.Sprintf("resource %q: every player starts with %d, below the minimum of %d", name, rule.Initial, *rule.Min))
		}
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
		for i, use := range item.Use {
			errs = append(errs, validateAction(use, fmt.Sprintf("item '%s' use action[%d]", id, i), actionContext{game: g})...)
		}
	}

	if g.Rules.Progression != nil {
		p := g.Rules.Progression
		for label, name := range map[string]string{
			"experienceResource": p.ExperienceResource,
			"levelResource":      p.LevelResource,
			"pointsResource":     p.PointsResource,
		} {
			if name == "" {
				continue
			}
			if _, ok := g.Rules.Resources[name]; !ok {
				errs = append(errs, fmt.Sprintf("progression.%s: no resource %q is declared in the rules", label, name))
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
		errs = append(errs, validateActions(c.ID, "onLand", c.OnLand, g)...)
		errs = append(errs, validateActions(c.ID, "onPass", c.OnPass, g)...)
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
		if e.Condition.Resource != "" && g.Rules.Resources != nil {
			if _, ok := g.Rules.Resources[e.Condition.Resource]; !ok {
				errs = append(errs, fmt.Sprintf("edge '%s': no resource %q is declared in the rules", e.ID, e.Condition.Resource))
			}
		}
		if e.Condition.Amount != nil && *e.Condition.Amount < 0 {
			errs = append(errs, fmt.Sprintf("edge '%s': amount must be non-negative", e.ID))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{Errors: errs}
}
