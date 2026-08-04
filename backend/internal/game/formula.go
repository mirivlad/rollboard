package game

// resolveTerm evaluates one operand of a formula.
func (s *GameSession) resolveTerm(term *AmountTerm, player *PlayerState, cell *CellDefinition) int {
	if term == nil {
		return 0
	}
	switch term.Kind {
	case "const", "":
		return term.Value
	case "field":
		if cell == nil {
			return 0
		}
		return getIntField(cell.Fields, term.Name, 0)
	case "stat":
		if player == nil {
			return 0
		}
		return s.EffectiveResource(player, term.Name)
	case "resource":
		if player == nil {
			return 0
		}
		return player.Resources[term.Name]
	default:
		return 0
	}
}

// resolveFormula computes base (+plus) (-minus), scaled, then clamped.
//
// The order is fixed and documented rather than parenthesised, so an author
// filling in dropdowns always gets the same arithmetic as the person reading
// the game later.
func (s *GameSession) resolveFormula(formula *AmountFormula, player *PlayerState, cell *CellDefinition) int {
	if formula == nil {
		return 0
	}
	value := s.resolveTerm(formula.Base, player, cell)
	value += s.resolveTerm(formula.Plus, player, cell)
	value -= s.resolveTerm(formula.Minus, player, cell)

	if formula.Times != nil {
		value *= *formula.Times
	}
	// Division by zero would panic, so a zero divisor is treated as "no
	// division" rather than taking down the game.
	if formula.DividedBy != nil && *formula.DividedBy != 0 {
		value /= *formula.DividedBy
	}
	// Clamps last, so "at least zero" holds whatever the arithmetic produced.
	if formula.Min != nil && value < *formula.Min {
		value = *formula.Min
	}
	if formula.Max != nil && value > *formula.Max {
		value = *formula.Max
	}
	return value
}

// amountFor is the session-aware amount resolver. A formula wins over a
// literal amount, which in turn wins over a cell field.
func (s *GameSession) amountFor(a ActionDefinition, player *PlayerState, cell *CellDefinition) int {
	if a.Formula != nil {
		return s.resolveFormula(a.Formula, player, cell)
	}
	var fields map[string]any
	if cell != nil {
		fields = cell.Fields
	}
	return resolveAmount(a, fields)
}

// amountForOrOne is the counting variant, where nothing specified means one.
func (s *GameSession) amountForOrOne(a ActionDefinition, player *PlayerState, cell *CellDefinition) int {
	if a.Formula == nil && a.Amount == nil && a.AmountField == "" {
		return 1
	}
	if amount := s.amountFor(a, player, cell); amount >= 1 {
		return amount
	}
	return 1
}
