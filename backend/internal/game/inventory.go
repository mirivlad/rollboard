package game

import "fmt"

// EffectiveResource is a player's base value for a resource plus every bonus
// from what they are currently wearing.
//
// Bonuses are kept out of Resources rather than folded into it, because a
// player who takes off a sword must lose its strength again. Storing the sum
// would make unequipping lossy.
func (s *GameSession) EffectiveResource(player *PlayerState, resource string) int {
	total := player.Resources[resource]
	for _, itemID := range player.Equipped {
		item, ok := s.Definition.Rules.Items[itemID]
		if !ok {
			continue
		}
		total += item.Bonuses[resource]
	}
	return total
}

// EffectiveResources is the whole set, for display.
func (s *GameSession) EffectiveResources(player *PlayerState) map[string]int {
	effective := make(map[string]int, len(player.Resources))
	for name, value := range player.Resources {
		effective[name] = value
	}
	for _, itemID := range player.Equipped {
		item, ok := s.Definition.Rules.Items[itemID]
		if !ok {
			continue
		}
		for name, bonus := range item.Bonuses {
			effective[name] += bonus
		}
	}
	return effective
}

func ensureInventory(player *PlayerState) {
	if player.Inventory == nil {
		player.Inventory = map[string]int{}
	}
	if player.Equipped == nil {
		player.Equipped = map[string]string{}
	}
}

// grantItem adds copies of an item to a player's pack.
func (s *GameSession) grantItem(player *PlayerState, itemID string, count int) []GameEvent {
	item, ok := s.Definition.Rules.Items[itemID]
	if !ok {
		return []GameEvent{NewGameEvent("invalid_action",
			fmt.Sprintf("No item %q is defined", itemID), nil)}
	}
	if count < 1 {
		count = 1
	}
	ensureInventory(player)
	player.Inventory[itemID] += count
	return []GameEvent{NewGameEvent("grant_item",
		fmt.Sprintf("%s picked up %s x%d", player.Name, item.Title, count), nil)}
}

// removeItem takes items away, unequipping first if the pack would otherwise
// run out from under a worn item.
func (s *GameSession) removeItem(player *PlayerState, itemID string, count int) []GameEvent {
	ensureInventory(player)
	if count < 1 {
		count = 1
	}
	held := player.Inventory[itemID]
	if held == 0 {
		return nil
	}
	if count >= held {
		count = held
		delete(player.Inventory, itemID)
	} else {
		player.Inventory[itemID] -= count
	}
	// Losing the last copy of something you are wearing takes it off, so a
	// player can never keep a bonus from an item they no longer own.
	if player.Inventory[itemID] == 0 {
		for slot, equipped := range player.Equipped {
			if equipped == itemID {
				delete(player.Equipped, slot)
			}
		}
	}
	title := itemID
	if item, ok := s.Definition.Rules.Items[itemID]; ok {
		title = item.Title
	}
	return []GameEvent{NewGameEvent("remove_item",
		fmt.Sprintf("%s lost %s x%d", player.Name, title, count), nil)}
}

// equipItem moves an item from the pack into its slot.
func (s *GameSession) equipItem(player *PlayerState, itemID string) []GameEvent {
	item, ok := s.Definition.Rules.Items[itemID]
	if !ok {
		return []GameEvent{NewGameEvent("invalid_action",
			fmt.Sprintf("No item %q is defined", itemID), nil)}
	}
	if item.Slot == "" {
		return []GameEvent{NewGameEvent("invalid_action",
			fmt.Sprintf("%s cannot be worn", item.Title), nil)}
	}
	ensureInventory(player)
	if player.Inventory[itemID] < 1 {
		return []GameEvent{NewGameEvent("invalid_action",
			fmt.Sprintf("%s is not carrying %s", player.Name, item.Title), nil)}
	}
	previous := player.Equipped[item.Slot]
	player.Equipped[item.Slot] = itemID
	message := fmt.Sprintf("%s equipped %s", player.Name, item.Title)
	if previous != "" && previous != itemID {
		// The replaced item goes back in the pack rather than vanishing.
		if old, ok := s.Definition.Rules.Items[previous]; ok {
			message = fmt.Sprintf("%s equipped %s, stowing %s", player.Name, item.Title, old.Title)
		}
	}
	return []GameEvent{NewGameEvent("equip_item", message, nil)}
}

func (s *GameSession) unequipSlot(player *PlayerState, slot string) []GameEvent {
	ensureInventory(player)
	itemID, worn := player.Equipped[slot]
	if !worn {
		return nil
	}
	delete(player.Equipped, slot)
	title := itemID
	if item, ok := s.Definition.Rules.Items[itemID]; ok {
		title = item.Title
	}
	return []GameEvent{NewGameEvent("unequip_item",
		fmt.Sprintf("%s put away %s", player.Name, title), nil)}
}

// useItem runs a consumable's effect and destroys it.
func (s *GameSession) useItem(player *PlayerState, itemID string, cell *CellDefinition) []GameEvent {
	item, ok := s.Definition.Rules.Items[itemID]
	if !ok {
		return []GameEvent{NewGameEvent("invalid_action",
			fmt.Sprintf("No item %q is defined", itemID), nil)}
	}
	ensureInventory(player)
	if player.Inventory[itemID] < 1 {
		return []GameEvent{NewGameEvent("invalid_action",
			fmt.Sprintf("%s is not carrying %s", player.Name, item.Title), nil)}
	}
	events := []GameEvent{NewGameEvent("use_item",
		fmt.Sprintf("%s used %s", player.Name, item.Title), nil)}
	events = append(events, s.executeActions(item.Use, player, cell)...)
	if item.Consumable {
		events = append(events, s.removeItem(player, itemID, 1)...)
	}
	return events
}
