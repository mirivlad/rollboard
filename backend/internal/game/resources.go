package game

import "fmt"

// Every change to a player's stored resources goes through this file.
//
// Before it, each action did its own arithmetic on the map. Two things fell
// out of that. A computed amount could be negative, so "gain 10" written as a
// formula that came to -10 quietly took money away and logged it as a gain,
// and "pay rent" with a negative rent paid the tenant. And ResourceRule.Min
// and .Max were declared, documented and never read by anything, so a health
// bar capped at 20 went to 35.
//
// The contract:
//
//   - An amount is a quantity, never a direction. Actions that take one refuse
//     a negative value rather than reversing themselves.
//   - A resource stays between its bounds on every path, including trades,
//     auctions, the start bonus and edge tolls.
//   - A transfer neither creates nor destroys: what leaves one player is
//     exactly what reaches the other.
//   - An operation that could not happen in full says so in the log, rather
//     than silently doing part of it.

// defaultResourceMin is the floor for a resource whose rule sets none.
//
// Zero, because that is what the engine already did when a payment was larger
// than a balance. A definition that wants debt sets a negative Min explicitly.
const defaultResourceMin = 0

func (s *GameSession) resourceBounds(resource string) (int, *int) {
	if s.Definition == nil {
		return defaultResourceMin, nil
	}
	rule, ok := s.Definition.Rules.Resources[resource]
	if !ok {
		return defaultResourceMin, nil
	}
	minimum := defaultResourceMin
	if rule.Min != nil {
		minimum = *rule.Min
	}
	return minimum, rule.Max
}

// headroom is how much more of a resource a player could receive.
func (s *GameSession) headroom(player *PlayerState, resource string) int {
	_, maximum := s.resourceBounds(resource)
	if maximum == nil {
		return int(^uint(0) >> 1) // no ceiling
	}
	room := *maximum - player.Resources[resource]
	if room < 0 {
		return 0
	}
	return room
}

// available is how much of a resource a player could give up.
func (s *GameSession) available(player *PlayerState, resource string) int {
	minimum, _ := s.resourceBounds(resource)
	spare := player.Resources[resource] - minimum
	if spare < 0 {
		return 0
	}
	return spare
}

// addResource credits a player and reports how much actually landed.
func (s *GameSession) addResource(player *PlayerState, resource string, amount int) int {
	if amount <= 0 {
		return 0
	}
	granted := amount
	if room := s.headroom(player, resource); granted > room {
		granted = room
	}
	player.Resources[resource] += granted
	return granted
}

// takeResource debits a player and reports how much was actually taken.
func (s *GameSession) takeResource(player *PlayerState, resource string, amount int) int {
	if amount <= 0 {
		return 0
	}
	taken := amount
	if spare := s.available(player, resource); taken > spare {
		taken = spare
	}
	player.Resources[resource] -= taken
	return taken
}

// moveResource hands a resource from one player to another.
//
// The amount is decided before anything moves, so a recipient who cannot hold
// the whole payment does not let the payer lose the difference.
func (s *GameSession) moveResource(from, to *PlayerState, resource string, amount int) int {
	if amount <= 0 || from == nil || to == nil {
		return 0
	}
	moved := amount
	if spare := s.available(from, resource); moved > spare {
		moved = spare
	}
	if room := s.headroom(to, resource); moved > room {
		moved = room
	}
	if moved <= 0 {
		return 0
	}
	from.Resources[resource] -= moved
	to.Resources[resource] += moved
	return moved
}

// setResource writes an absolute value, clamped to the resource's bounds.
func (s *GameSession) setResource(player *PlayerState, resource string, value int) int {
	minimum, maximum := s.resourceBounds(resource)
	if value < minimum {
		value = minimum
	}
	if maximum != nil && value > *maximum {
		value = *maximum
	}
	player.Resources[resource] = value
	return value
}

// quantityFor resolves an amount that must be a quantity.
//
// A negative result is refused rather than reversed: the author asked for a
// gain, and the safe reading of "gain -10" is that the rule is wrong, not that
// the player should be charged. The refusal is logged, because a rule that
// silently does nothing is the hardest kind to debug from a game log.
func (s *GameSession) quantityFor(a ActionDefinition, player *PlayerState, cell *CellDefinition) (int, []GameEvent) {
	amount := s.amountFor(a, player, cell)
	if amount < 0 {
		return 0, []GameEvent{NewGameEvent("invalid_amount",
			fmt.Sprintf("%s: %s worked out to %d, which is not a quantity — nothing happened",
				player.Name, a.Type, amount), nil)}
	}
	return amount, nil
}

// transferRecipient resolves who a payment is owed to.
//
// The editor used to offer one shared list of targets — current, owner, bank,
// nobody — for actions that read them differently. Everything except "owner"
// fell through to a player lookup, found nothing and returned silently, so a
// rent written against "bank" simply never happened and the log said nothing
// at all. Only the recipients that exist are accepted now, and anything else
// is an error the author can see.
func (s *GameSession) transferRecipient(target string, player *PlayerState, cell *CellDefinition) (*PlayerState, error) {
	switch target {
	case "owner":
		if cell == nil {
			return nil, nil
		}
		ownerID := s.State.CellStates[cell.ID].OwnerPlayerID
		if ownerID == "" {
			// Nobody owns it, so nothing is owed. Not an error: a rent action
			// on an unowned square is a normal state of play.
			return nil, nil
		}
		return s.getPlayerByID(ownerID), nil
	case "":
		return nil, fmt.Errorf("no recipient was chosen")
	default:
		if recipient := s.getPlayerByID(target); recipient != nil {
			return recipient, nil
		}
		return nil, fmt.Errorf("no player %q is in this game", target)
	}
}

// cellOwnerFor resolves who a square should belong to.
func (s *GameSession) cellOwnerFor(target string, player *PlayerState) (string, error) {
	switch target {
	case "current":
		return player.ID, nil
	case "none":
		return "", nil
	case "":
		return "", fmt.Errorf("no owner was chosen")
	default:
		// A seat named directly. Without this check the target was stored
		// verbatim, so a square could end up owned by a player called "bank"
		// who does not exist and can never be paid.
		if s.getPlayerByID(target) == nil {
			return "", fmt.Errorf("no player %q is in this game", target)
		}
		return target, nil
	}
}

// shortfallNote explains a capped operation in the game log.
func shortfallNote(asked, applied int, resource string) string {
	if applied >= asked {
		return ""
	}
	return fmt.Sprintf(" (%d of %d %s — the limit was reached)", applied, asked, resource)
}
