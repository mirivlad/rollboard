package game

import (
	"strings"
	"testing"
)

func rpgDefinition() *GameDefinition {
	cell := func(id, title string, x int, onLand ...ActionDefinition) CellDefinition {
		return CellDefinition{
			ID: id, Title: title, Type: "empty", X: x, Y: 0,
			Visual: CellVisual{BaseColor: "#cccccc"},
			Fields: map[string]any{},
			OnLand: onLand,
		}
	}
	cells := []CellDefinition{
		{ID: "start", Title: "Camp", Type: "start", X: 0, Y: 0, Visual: CellVisual{BaseColor: "#4CAF50"}, Fields: map[string]any{}},
		cell("armoury", "Armoury", 100),
		cell("cave", "Cave", 200),
		cell("summit", "Summit", 300),
	}
	edges := []EdgeDefinition{
		{ID: "e1", From: "start", To: "armoury", Condition: EdgeCondition{Type: "always"}},
		{ID: "e2", From: "armoury", To: "cave", Condition: EdgeCondition{Type: "always"}},
		{ID: "e3", From: "cave", To: "summit", Condition: EdgeCondition{Type: "always"}},
		{ID: "e4", From: "summit", To: "start", Condition: EdgeCondition{Type: "always"}},
	}
	return &GameDefinition{
		ID: "rpg", Title: "RPG", Version: 1,
		Board: Board{Width: 400, Height: 100, CellSize: 100, Cells: cells, Edges: edges},
		Rules: RuleSet{
			Dice: DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"health":   {Initial: 20},
				"strength": {Initial: 5},
				"gold":     {Initial: 0},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {Title: "Camp", Fields: map[string]FieldDef{}},
				"empty": {Title: "Empty", Fields: map[string]FieldDef{}},
			},
			EquipmentSlots: []string{"weapon", "armour"},
			Items: map[string]ItemDef{
				"rusty_sword": {ID: "rusty_sword", Title: "Rusty Sword", Slot: "weapon", Bonuses: map[string]int{"strength": 3}},
				"great_axe":   {ID: "great_axe", Title: "Great Axe", Slot: "weapon", Bonuses: map[string]int{"strength": 7}},
				"leather":     {ID: "leather", Title: "Leather Armour", Slot: "armour", Bonuses: map[string]int{"health": 5}},
				"potion": {
					ID: "potion", Title: "Healing Potion", Consumable: true,
					Use: []ActionDefinition{{Type: "gain_resource", Resource: "health", Amount: intPtr(8)}},
				},
			},
		},
	}
}

func rpgSession(t *testing.T) *GameSession {
	t.Helper()
	return StartSession(rpgDefinition(), []PlayerConfig{
		{Name: "Ada", Color: "#111111"},
		{Name: "Bob", Color: "#222222"},
	})
}

func TestEquippedItemsRaiseEffectiveStatsWithoutTouchingTheBase(t *testing.T) {
	session := rpgSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("armoury")

	session.executeOneAction(ActionDefinition{Type: "grant_item", Field: "rusty_sword"}, player, cell)
	session.executeOneAction(ActionDefinition{Type: "equip_item", Field: "rusty_sword"}, player, cell)

	if got := session.EffectiveResource(player, "strength"); got != 8 {
		t.Fatalf("effective strength = %d, want 5 base plus 3 from the sword", got)
	}
	// The base must stay untouched, otherwise taking the sword off would be lossy.
	if player.Resources["strength"] != 5 {
		t.Fatalf("base strength = %d, want it unchanged at 5", player.Resources["strength"])
	}

	session.executeOneAction(ActionDefinition{Type: "unequip_slot", Target: "weapon"}, player, cell)
	if got := session.EffectiveResource(player, "strength"); got != 5 {
		t.Fatalf("effective strength = %d after unequipping, want the bonus gone", got)
	}
}

func TestEquippingASecondItemInTheSameSlotReplacesTheFirst(t *testing.T) {
	session := rpgSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("armoury")

	for _, id := range []string{"rusty_sword", "great_axe"} {
		session.executeOneAction(ActionDefinition{Type: "grant_item", Field: id}, player, cell)
		session.executeOneAction(ActionDefinition{Type: "equip_item", Field: id}, player, cell)
	}

	if player.Equipped["weapon"] != "great_axe" {
		t.Fatalf("weapon slot = %q, want the axe", player.Equipped["weapon"])
	}
	// Only one weapon counts, so bonuses must not stack across a slot.
	if got := session.EffectiveResource(player, "strength"); got != 12 {
		t.Fatalf("effective strength = %d, want 5 + 7 from the axe alone", got)
	}
	// The replaced sword is still carried.
	if player.Inventory["rusty_sword"] != 1 {
		t.Fatalf("inventory = %v, want the sword stowed rather than destroyed", player.Inventory)
	}
}

func TestBonusesFromDifferentSlotsAddUp(t *testing.T) {
	session := rpgSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("armoury")

	for _, id := range []string{"great_axe", "leather"} {
		session.executeOneAction(ActionDefinition{Type: "grant_item", Field: id}, player, cell)
		session.executeOneAction(ActionDefinition{Type: "equip_item", Field: id}, player, cell)
	}

	effective := session.EffectiveResources(player)
	if effective["strength"] != 12 || effective["health"] != 25 {
		t.Fatalf("effective = %v, want strength 12 and health 25", effective)
	}
}

// Losing the last copy of a worn item must take it off, or the player would
// keep a bonus from equipment they no longer own.
func TestLosingAWornItemUnequipsIt(t *testing.T) {
	session := rpgSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("armoury")

	session.executeOneAction(ActionDefinition{Type: "grant_item", Field: "rusty_sword"}, player, cell)
	session.executeOneAction(ActionDefinition{Type: "equip_item", Field: "rusty_sword"}, player, cell)
	session.executeOneAction(ActionDefinition{Type: "remove_item", Field: "rusty_sword"}, player, cell)

	if _, worn := player.Equipped["weapon"]; worn {
		t.Fatalf("equipped = %v, want the slot emptied", player.Equipped)
	}
	if got := session.EffectiveResource(player, "strength"); got != 5 {
		t.Fatalf("effective strength = %d, want the bonus gone with the item", got)
	}
}

func TestConsumablesRunTheirEffectAndAreDestroyed(t *testing.T) {
	session := rpgSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("cave")
	player.Resources["health"] = 10

	session.executeOneAction(ActionDefinition{Type: "grant_item", Field: "potion", Amount: intPtr(2)}, player, cell)
	session.executeOneAction(ActionDefinition{Type: "use_item", Field: "potion"}, player, cell)

	if player.Resources["health"] != 18 {
		t.Fatalf("health = %d, want the potion to have healed 8", player.Resources["health"])
	}
	if player.Inventory["potion"] != 1 {
		t.Fatalf("inventory = %v, want one potion consumed", player.Inventory)
	}
}

func TestCannotEquipOrUseWhatYouDoNotCarry(t *testing.T) {
	session := rpgSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("armoury")

	for _, a := range []ActionDefinition{
		{Type: "equip_item", Field: "great_axe"},
		{Type: "use_item", Field: "potion"},
	} {
		events := session.executeOneAction(a, player, cell)
		if len(events) == 0 || events[0].Type != "invalid_action" {
			t.Fatalf("%s on an empty pack gave %#v, want invalid_action", a.Type, events)
		}
	}
	if len(player.Equipped) != 0 {
		t.Fatalf("equipped = %v, want nothing worn", player.Equipped)
	}
}

func TestIfHasItemAndIfStatGe(t *testing.T) {
	session := rpgSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("cave")

	session.executeOneAction(ActionDefinition{
		Type: "if_has_item", Field: "rusty_sword",
		Then: []ActionDefinition{{Type: "gain_resource", Resource: "gold", Amount: intPtr(100)}},
		Else: []ActionDefinition{{Type: "gain_resource", Resource: "gold", Amount: intPtr(1)}},
	}, player, cell)
	if player.Resources["gold"] != 1 {
		t.Fatalf("gold = %d, want the else branch without the sword", player.Resources["gold"])
	}

	session.executeOneAction(ActionDefinition{Type: "grant_item", Field: "great_axe"}, player, cell)
	session.executeOneAction(ActionDefinition{Type: "equip_item", Field: "great_axe"}, player, cell)

	// if_stat_ge counts the axe; if_resource_ge deliberately does not.
	session.executeOneAction(ActionDefinition{
		Type: "if_stat_ge", Resource: "strength", Amount: intPtr(10),
		Then: []ActionDefinition{{Type: "gain_resource", Resource: "gold", Amount: intPtr(10)}},
	}, player, cell)
	if player.Resources["gold"] != 11 {
		t.Fatalf("gold = %d, want if_stat_ge to count the equipped axe", player.Resources["gold"])
	}
	session.executeOneAction(ActionDefinition{
		Type: "if_resource_ge", Resource: "strength", Amount: intPtr(10),
		Then: []ActionDefinition{{Type: "gain_resource", Resource: "gold", Amount: intPtr(1000)}},
	}, player, cell)
	if player.Resources["gold"] != 11 {
		t.Fatalf("gold = %d, want if_resource_ge to ignore equipment", player.Resources["gold"])
	}
}

func TestValidationRejectsUnknownItemsAndSlots(t *testing.T) {
	definition := rpgDefinition()
	definition.Rules.Items["broken"] = ItemDef{ID: "broken", Title: "Broken", Slot: "tail", Bonuses: map[string]int{"mana": 1}}
	for i := range definition.Board.Cells {
		if definition.Board.Cells[i].ID == "cave" {
			definition.Board.Cells[i].OnLand = []ActionDefinition{
				{Type: "grant_item", Field: "no_such_item"},
				{Type: "unequip_slot", Target: "no_such_slot"},
			}
		}
	}

	err := ValidateDefinition(definition)
	if err == nil {
		t.Fatal("validation accepted unknown items and slots")
	}
	joined := strings.Join(err.Errors, "; ")
	for _, want := range []string{"no_such_item", "no_such_slot", "equipmentSlots", "mana"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("errors = %v, want something mentioning %q", err.Errors, want)
		}
	}
}
