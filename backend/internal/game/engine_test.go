package game

import (
	"testing"
)

var miniMonopolyDemo = &GameDefinition{
	ID:    "mini-monopoly",
	Title: "Mini-Monopoly",
	Board: Board{
		Width: 864, Height: 768, CellSize: 96,
		Cells: []CellDefinition{
			{ID: "start", Title: "Start", Type: "start", X: 0, Y: 576, Visual: CellVisual{BaseColor: "#4CAF50"}, OnLand: []ActionDefinition{}},
			{ID: "cell_2", Title: "Street A", Type: "property", X: 192, Y: 576, Visual: CellVisual{BaseColor: "#E3F2FD"},
				Fields: map[string]any{"cost": float64(100), "rent": float64(20)},
				OnLand: []ActionDefinition{
					{Type: "if_cell_unowned",
						Then: []ActionDefinition{
							{Type: "offer_choice", Title: "Buy this property?",
								Options: []ActionOption{
									{ID: "buy_property", Title: "Buy",
										Then: []ActionDefinition{
											{Type: "lose_resource", Resource: "money", AmountField: "cost"},
											{Type: "set_cell_owner", Target: "current"},
										}},
									{ID: "skip_purchase", Title: "Don't Buy", Then: []ActionDefinition{}},
								}},
						},
						Else: []ActionDefinition{
							{Type: "if_cell_owned_by_other",
								Then: []ActionDefinition{
									{Type: "transfer_resource", Resource: "money", AmountField: "rent", Target: "owner"},
								},
							},
						},
					},
				},
			},
			{ID: "cell_3", Title: "Bonus", Type: "bonus", X: 384, Y: 576, Visual: CellVisual{BaseColor: "#C8E6C9"},
				Fields: map[string]any{"amount": float64(50)},
				OnLand: []ActionDefinition{
					{Type: "gain_resource", Resource: "money", AmountField: "amount"},
				}},
		},
		Edges: []EdgeDefinition{
			{ID: "e1", From: "start", To: "cell_2", Condition: EdgeCondition{Type: "always"}},
			{ID: "e2", From: "cell_2", To: "cell_3", Condition: EdgeCondition{Type: "always"}},
			{ID: "e3", From: "cell_3", To: "start", Condition: EdgeCondition{Type: "always"}},
		},
	},
	Rules: RuleSet{
		Dice: DiceRule{Count: 1, Sides: 6},
		Resources: map[string]ResourceRule{
			"money": {Initial: 500},
		},
		CellTypes: map[string]CellTypeDef{
			"start":    {},
			"property": {},
			"bonus":    {},
		},
		StartBonus:         100,
		StartBonusResource: "money",
	},
}

var dungeonRaceDemo = &GameDefinition{
	ID:    "dungeon-race",
	Title: "Dungeon Race",
	Board: Board{
		Width: 1056, Height: 384, CellSize: 96,
		Cells: []CellDefinition{
			{ID: "start", Title: "Start", Type: "start", X: 0, Y: 96, Visual: CellVisual{BaseColor: "#4CAF50"},
				OnLand: []ActionDefinition{}},
			{ID: "trap", Title: "Trap", Type: "trap", X: 192, Y: 96, Visual: CellVisual{BaseColor: "#FFCDD2"},
				OnLand: []ActionDefinition{
					{Type: "lose_resource", Resource: "health", Amount: intPtr(2)},
				}},
			{ID: "treasure", Title: "Treasure", Type: "treasure", X: 384, Y: 96, Visual: CellVisual{BaseColor: "#FFF9C4"},
				OnLand: []ActionDefinition{
					{Type: "gain_resource", Resource: "gold", Amount: intPtr(5)},
				}},
			{ID: "key", Title: "Key", Type: "key", X: 576, Y: 96, Visual: CellVisual{BaseColor: "#E1BEE7"},
				OnLand: []ActionDefinition{
					{Type: "gain_resource", Resource: "keys", Amount: intPtr(1)},
				}},
			{ID: "heal", Title: "Heal", Type: "heal", X: 768, Y: 96, Visual: CellVisual{BaseColor: "#C8E6C9"},
				OnLand: []ActionDefinition{
					{Type: "gain_resource", Resource: "health", Amount: intPtr(2)},
				}},
			{ID: "finish", Title: "Finish", Type: "finish", X: 960, Y: 96, Visual: CellVisual{BaseColor: "#FFD700"},
				OnLand: []ActionDefinition{
					{Type: "finish_game"},
				}},
		},
		Edges: []EdgeDefinition{
			{ID: "e1", From: "start", To: "trap", Condition: EdgeCondition{Type: "always"}},
			{ID: "e2", From: "trap", To: "treasure", Condition: EdgeCondition{Type: "always"}},
			{ID: "e3", From: "treasure", To: "key", Condition: EdgeCondition{Type: "always"}},
			{ID: "e4", From: "key", To: "heal", Condition: EdgeCondition{Type: "always"}},
			{ID: "e5", From: "heal", To: "finish", Condition: EdgeCondition{Type: "always"}},
		},
	},
	Rules: RuleSet{
		Dice: DiceRule{Count: 1, Sides: 6},
		Resources: map[string]ResourceRule{
			"health": {Initial: 10, Min: intPtr(0), Max: intPtr(10)},
			"gold":   {Initial: 0},
			"keys":   {Initial: 0},
		},
		CellTypes: map[string]CellTypeDef{
			"start":    {},
			"trap":     {},
			"treasure": {},
			"key":      {},
			"heal":     {},
			"finish":   {},
		},
		StartBonus: 0,
	},
}

func intPtr(i int) *int {
	return &i
}

func TestMiniMonopolyDemoValidates(t *testing.T) {
	if err := ValidateDefinition(miniMonopolyDemo); err != nil {
		t.Fatalf("Mini-Monopoly should validate: %v", err)
	}
}

func TestDungeonRaceDemoValidates(t *testing.T) {
	if err := ValidateDefinition(dungeonRaceDemo); err != nil {
		t.Fatalf("Dungeon Race should validate: %v", err)
	}
}

func TestStartSessionDungeonRace(t *testing.T) {
	players := []PlayerConfig{
		{Name: "Hero 1", Color: "#e74c3c"},
		{Name: "Hero 2", Color: "#3498db"},
	}
	session := StartSession(dungeonRaceDemo, players)
	if session == nil {
		t.Fatal("session should not be nil")
	}
	if session.State.Players[0].Resources["health"] != 10 {
		t.Fatalf("expected health 10, got %d", session.State.Players[0].Resources["health"])
	}
}

func TestGainResource(t *testing.T) {
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}, {Name: "P2", Color: "#3498db"}}
	session := StartSession(miniMonopolyDemo, players)
	player := &session.State.Players[0]

	events := session.executeOneAction(ActionDefinition{Type: "gain_resource", Resource: "gold", Amount: intPtr(10)}, player, nil)
	if len(events) == 0 {
		t.Fatal("expected events")
	}
	if player.Resources["gold"] != 10 {
		t.Fatalf("expected gold 10, got %d", player.Resources["gold"])
	}
}

func TestLoseResource(t *testing.T) {
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}, {Name: "P2", Color: "#3498db"}}
	session := StartSession(miniMonopolyDemo, players)
	player := &session.State.Players[0]
	player.Resources["health"] = 10

	events := session.executeOneAction(ActionDefinition{Type: "lose_resource", Resource: "health", Amount: intPtr(3)}, player, nil)
	if len(events) == 0 {
		t.Fatal("expected events")
	}
	if player.Resources["health"] != 7 {
		t.Fatalf("expected health 7, got %d", player.Resources["health"])
	}
}

func TestTransferResource(t *testing.T) {
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}, {Name: "P2", Color: "#3498db"}}
	session := StartSession(miniMonopolyDemo, players)
	p1 := &session.State.Players[0]
	p2 := &session.State.Players[1]
	p1.Resources["gold"] = 20
	p2.Resources["gold"] = 5

	// Transfer by player ID
	session.executeOneAction(ActionDefinition{
		Type: "transfer_resource", Resource: "gold", Amount: intPtr(10), Target: "player_2",
	}, p1, &session.Definition.Board.Cells[0])

	if p1.Resources["gold"] != 10 {
		t.Fatalf("expected p1 gold 10, got %d", p1.Resources["gold"])
	}
	if p2.Resources["gold"] != 15 {
		t.Fatalf("expected p2 gold 15, got %d", p2.Resources["gold"])
	}
}

func TestFinishGame(t *testing.T) {
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}, {Name: "P2", Color: "#3498db"}}
	session := StartSession(dungeonRaceDemo, players)
	player := &session.State.Players[0]

	session.executeOneAction(ActionDefinition{Type: "finish_game"}, player, nil)
	if session.State.Status != "finished" {
		t.Fatal("game should be finished")
	}
	if session.State.WinnerPlayerID != "player_1" {
		t.Fatalf("expected player_1 winner, got %s", session.State.WinnerPlayerID)
	}
}

func TestDungeonRaceRollDoesNotPanic(t *testing.T) {
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}, {Name: "P2", Color: "#3498db"}}
	session := StartSession(dungeonRaceDemo, players)
	if session == nil {
		t.Fatal("session should not be nil")
	}
	// First roll should succeed without panic
	rollResult, diceEvt := session.RollDice()
	if rollResult == nil {
		t.Fatal("rollResult should not be nil")
	}
	if diceEvt == nil {
		t.Fatal("dice event should not be nil")
	}
	if rollResult.Total < 1 || rollResult.Total > 6 {
		t.Fatalf("dice roll out of range: %d", rollResult.Total)
	}
	if len(rollResult.Rolls) != 1 {
		t.Fatalf("expected 1 die, got %d", len(rollResult.Rolls))
	}
	// Move should also succeed
	events := session.MoveCurrentPlayer(rollResult.Total, rollResult.Rolls, rollResult.Total)
	if events == nil {
		t.Fatal("move events should not be nil")
	}
}

func TestMiniMonopolyRollDoesNotPanic(t *testing.T) {
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}, {Name: "P2", Color: "#3498db"}}
	session := StartSession(miniMonopolyDemo, players)
	if session == nil {
		t.Fatal("session should not be nil")
	}
	rollResult, diceEvt := session.RollDice()
	if rollResult == nil {
		t.Fatal("rollResult should not be nil")
	}
	if diceEvt == nil {
		t.Fatal("dice event should not be nil")
	}
	events := session.MoveCurrentPlayer(rollResult.Total, rollResult.Rolls, rollResult.Total)
	if events == nil {
		t.Fatal("move events should not be nil")
	}
}

func TestDungeonRaceWinnerIsCurrentPlayer(t *testing.T) {
	// Regression: finish_game must mark the moving player as winner,
	// not the next player after advanceTurn.
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}, {Name: "P2", Color: "#3498db"}}
	session := StartSession(dungeonRaceDemo, players)
	// Force a roll of exactly 5 to reach finish from start
	// start → trap → treasure → key → heal → finish (5 steps)
	events := session.MoveCurrentPlayer(5, []int{5}, 5)
	if session.State.Status != "finished" {
		t.Fatalf("game should be finished, got status=%s", session.State.Status)
	}
	if session.State.WinnerPlayerID != "player_1" {
		t.Fatalf("expected player_1 as winner, got %s", session.State.WinnerPlayerID)
	}
	// Verify game_over event mentions the correct player
	foundGameOver := false
	for _, evt := range events {
		if evt.Type == "game_over" {
			foundGameOver = true
		}
	}
	if !foundGameOver {
		t.Fatal("expected game_over event in events")
	}
	// Player 1's health should remain 10 (no trap hit)
	if session.State.Players[0].Resources["health"] != 10 {
		t.Fatalf("expected health 10, got %d", session.State.Players[0].Resources["health"])
	}
}

func TestGameWithoutMoney(t *testing.T) {
	g := &GameDefinition{
		ID:    "no-money-game",
		Title: "No Money",
		Board: Board{
			Width: 384, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 96, Visual: CellVisual{BaseColor: "#4CAF50"}},
				{ID: "cell2", Title: "Cell", Type: "empty", X: 192, Y: 96, Visual: CellVisual{BaseColor: "#ccc"},
					OnLand: []ActionDefinition{{Type: "gain_resource", Resource: "health", Amount: intPtr(5)}}},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "cell2", Condition: EdgeCondition{Type: "always"}},
			},
		},
		Rules: RuleSet{
			Dice: DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"health": {Initial: 10},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
			StartBonus: 0,
		},
	}

	if err := ValidateDefinition(g); err != nil {
		t.Fatalf("game without money should validate: %v", err)
	}

	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}, {Name: "P2", Color: "#3498db"}}
	session := StartSession(g, players)
	if session.State.Players[0].Resources["money"] != 0 {
		t.Fatal("should not have money resource")
	}
	if session.State.Players[0].Resources["health"] != 10 {
		t.Fatalf("expected health 10, got %d", session.State.Players[0].Resources["health"])
	}
}

func TestGameWithoutPropertyCells(t *testing.T) {
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}, {Name: "P2", Color: "#3498db"}}
	session := StartSession(dungeonRaceDemo, players)
	if session == nil {
		t.Fatal("session with no property cells should start")
	}
}

func TestResolvePendingAction(t *testing.T) {
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}, {Name: "P2", Color: "#3498db"}}
	session := StartSession(miniMonopolyDemo, players)
	cell := &session.Definition.Board.Cells[1] // Street A

	session.executeOneAction(ActionDefinition{
		Type: "offer_choice", Title: "Test?",
		Options: []ActionOption{
			{ID: "yes", Title: "Yes", Then: []ActionDefinition{
				{Type: "gain_resource", Resource: "money", Amount: intPtr(50)},
			}},
			{ID: "no", Title: "No", Then: []ActionDefinition{}},
		},
	}, &session.State.Players[0], cell)

	if session.State.PendingAction == nil {
		t.Fatal("pending action should be set")
	}

	events, err := session.ResolvePendingAction("yes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected events from resolving")
	}
	if session.State.Players[0].Resources["money"] != 550 {
		t.Fatalf("expected money 550, got %d", session.State.Players[0].Resources["money"])
	}
	if session.State.PendingAction != nil {
		t.Fatal("pending action should be cleared")
	}
}

func TestPlayerConfigNamesAndColors(t *testing.T) {
	players := []PlayerConfig{
		{Name: "Alice", Color: "#ff0000"},
		{Name: "Bob", Color: "#00ff00"},
		{Name: "Charlie", Color: "#0000ff"},
	}
	session := StartSession(miniMonopolyDemo, players)
	if len(session.State.Players) != 3 {
		t.Fatalf("expected 3 players, got %d", len(session.State.Players))
	}
	if session.State.Players[0].Name != "Alice" || session.State.Players[0].Color != "#ff0000" {
		t.Fatalf("player 1 config not applied: %+v", session.State.Players[0])
	}
	if session.State.Players[2].Name != "Charlie" {
		t.Fatalf("player 3 name not applied: %s", session.State.Players[2].Name)
	}
}

// --- Edge condition tests ---

func hasEventType(events []GameEvent, typ string) bool {
	for _, e := range events {
		if e.Type == typ {
			return true
		}
	}
	return false
}

func TestAlwaysEdge(t *testing.T) {
	g := &GameDefinition{
		ID:    "test-always",
		Title: "Test Always",
		Board: Board{
			Width: 384, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 0},
				{ID: "cell2", Title: "Cell 2", Type: "empty", X: 192, Y: 0},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "cell2", Condition: EdgeCondition{Type: "always"}},
			},
		},
		Rules: RuleSet{
			Dice:  DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"money": {Initial: 100},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
			StartBonus: 0,
		},
	}
	if err := ValidateDefinition(g); err != nil {
		t.Fatalf("definition should validate: %v", err)
	}
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}}
	session := StartSession(g, players)
	events := session.MoveCurrentPlayer(1, []int{1}, 1)
	if !hasEventType(events, "move") {
		t.Fatal("expected move event")
	}
	if session.State.Players[0].PositionCellID != "cell2" {
		t.Fatalf("expected position cell2, got %s", session.State.Players[0].PositionCellID)
	}
}

func TestDiceTotalEven(t *testing.T) {
	g := &GameDefinition{
		ID:    "test-dice-even",
		Title: "Test Dice Total Even",
		Board: Board{
			Width: 576, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 0},
				{ID: "branch", Title: "Branch", Type: "empty", X: 192, Y: 0},
				{ID: "even_cell", Title: "Even", Type: "empty", X: 384, Y: 0},
				{ID: "odd_cell", Title: "Odd", Type: "empty", X: 384, Y: 192},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "branch", Condition: EdgeCondition{Type: "always"}},
				{ID: "e2", From: "branch", To: "even_cell", Condition: EdgeCondition{Type: "dice_total_even"}},
				{ID: "e3", From: "branch", To: "odd_cell", Condition: EdgeCondition{Type: "dice_total_odd"}},
			},
		},
		Rules: RuleSet{
			Dice:  DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"money": {Initial: 100},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
			StartBonus: 0,
		},
	}
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}}

	// Even roll (2) → only dice_total_even edge is available → goes to even_cell
	session := StartSession(g, players)
	session.MoveCurrentPlayer(2, []int{2}, 2)
	if session.State.Players[0].PositionCellID != "even_cell" {
		t.Fatalf("even roll: expected even_cell, got %s", session.State.Players[0].PositionCellID)
	}

	// Odd roll (3) → only dice_total_odd edge is available → goes to odd_cell
	session2 := StartSession(g, players)
	session2.MoveCurrentPlayer(2, []int{3}, 3)
	if session2.State.Players[0].PositionCellID != "odd_cell" {
		t.Fatalf("odd roll: expected odd_cell, got %s", session2.State.Players[0].PositionCellID)
	}
}

func TestDiceTotalOdd(t *testing.T) {
	g := &GameDefinition{
		ID:    "test-dice-odd",
		Title: "Test Dice Total Odd",
		Board: Board{
			Width: 576, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 0},
				{ID: "branch", Title: "Branch", Type: "empty", X: 192, Y: 0},
				{ID: "odd_target", Title: "Odd Target", Type: "empty", X: 384, Y: 0},
				{ID: "even_target", Title: "Even Target", Type: "empty", X: 384, Y: 192},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "branch", Condition: EdgeCondition{Type: "always"}},
				{ID: "e_odd", From: "branch", To: "odd_target", Condition: EdgeCondition{Type: "dice_total_odd"}},
				{ID: "e_even", From: "branch", To: "even_target", Condition: EdgeCondition{Type: "dice_total_even"}},
			},
		},
		Rules: RuleSet{
			Dice:  DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"money": {Initial: 100},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
			StartBonus: 0,
		},
	}
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}}

	// Odd roll (5) → only dice_total_odd edge is available → goes to odd_target
	session := StartSession(g, players)
	session.MoveCurrentPlayer(2, []int{5}, 5)
	if session.State.Players[0].PositionCellID != "odd_target" {
		t.Fatalf("odd roll: expected odd_target, got %s", session.State.Players[0].PositionCellID)
	}

	// Even roll (4) → only dice_total_even edge is available → goes to even_target
	session2 := StartSession(g, players)
	session2.MoveCurrentPlayer(2, []int{4}, 4)
	if session2.State.Players[0].PositionCellID != "even_target" {
		t.Fatalf("even roll: expected even_target, got %s", session2.State.Players[0].PositionCellID)
	}
}

func TestDiceTotalIn(t *testing.T) {
	g := &GameDefinition{
		ID:    "test-dice-in",
		Title: "Test Dice Total In",
		Board: Board{
			Width: 576, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 0},
				{ID: "branch", Title: "Branch", Type: "empty", X: 192, Y: 0},
				{ID: "in_cell", Title: "In", Type: "empty", X: 384, Y: 0},
				{ID: "out_cell", Title: "Out", Type: "empty", X: 384, Y: 192},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "branch", Condition: EdgeCondition{Type: "always"}},
				{ID: "e_in", From: "branch", To: "in_cell", Condition: EdgeCondition{Type: "dice_total_in", Values: []int{1, 3, 5}}},
				{ID: "e_out", From: "branch", To: "out_cell", Condition: EdgeCondition{Type: "always"}},
			},
		},
		Rules: RuleSet{
			Dice:  DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"money": {Initial: 100},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
			StartBonus: 0,
		},
	}
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}}

	// Total=3 ∈ [1,3,5] → e_in available (along with always e_out). Both available, sorted by ID:
	// "e_in" < "e_out" → e_in chosen first, goes to in_cell.
	session := StartSession(g, players)
	session.MoveCurrentPlayer(2, []int{3}, 3)
	if session.State.Players[0].PositionCellID != "in_cell" {
		t.Fatalf("total=3: expected in_cell, got %s", session.State.Players[0].PositionCellID)
	}

	// Total=2 ∉ [1,3,5] → only e_out available → goes to out_cell
	session2 := StartSession(g, players)
	session2.MoveCurrentPlayer(2, []int{2}, 2)
	if session2.State.Players[0].PositionCellID != "out_cell" {
		t.Fatalf("total=2: expected out_cell, got %s", session2.State.Players[0].PositionCellID)
	}
}

func TestPlayerResourceAtLeast(t *testing.T) {
	g := &GameDefinition{
		ID:    "test-res-at-least",
		Title: "Test Resource At Least",
		Board: Board{
			Width: 384, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 0},
				{ID: "target", Title: "Target", Type: "empty", X: 192, Y: 0},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "target", Condition: EdgeCondition{Type: "player_resource_at_least", Resource: "gold", Amount: intPtr(5)}},
			},
		},
		Rules: RuleSet{
			Dice:  DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"gold": {Initial: 10},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
			StartBonus: 0,
		},
	}
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}}

	// Player has gold=10 (>=5) → edge available → move succeeds
	session := StartSession(g, players)
	events := session.MoveCurrentPlayer(1, []int{1}, 1)
	if hasEventType(events, "move_blocked") {
		t.Fatal("edge should be available when player has enough gold")
	}
	if session.State.Players[0].PositionCellID != "target" {
		t.Fatalf("expected target, got %s", session.State.Players[0].PositionCellID)
	}

	// Player has gold=0 (<5) → edge unavailable → move_blocked
	session2 := StartSession(g, players)
	session2.State.Players[0].Resources["gold"] = 0
	events2 := session2.MoveCurrentPlayer(1, []int{1}, 1)
	if !hasEventType(events2, "move_blocked") {
		t.Fatal("expected move_blocked when player has insufficient gold")
	}
}

func TestPayResource(t *testing.T) {
	g := &GameDefinition{
		ID:    "test-pay-resource",
		Title: "Test Pay Resource",
		Board: Board{
			Width: 384, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 0},
				{ID: "target", Title: "Target", Type: "empty", X: 192, Y: 0},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "target", Condition: EdgeCondition{Type: "pay_resource", Resource: "gold", Amount: intPtr(3)}},
			},
		},
		Rules: RuleSet{
			Dice:  DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"gold": {Initial: 10},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
			StartBonus: 0,
		},
	}
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}}
	session := StartSession(g, players)
	initialGold := session.State.Players[0].Resources["gold"]
	if initialGold != 10 {
		t.Fatalf("expected initial gold 10, got %d", initialGold)
	}

	events := session.MoveCurrentPlayer(1, []int{1}, 1)
	if hasEventType(events, "move_blocked") {
		t.Fatal("edge should be available when player has enough gold to pay")
	}
	if session.State.Players[0].PositionCellID != "target" {
		t.Fatalf("expected target, got %s", session.State.Players[0].PositionCellID)
	}
	// 3 gold should be deducted
	if session.State.Players[0].Resources["gold"] != 7 {
		t.Fatalf("expected gold 7 after paying 3, got %d", session.State.Players[0].Resources["gold"])
	}
}

func TestPayResourceInsufficient(t *testing.T) {
	g := &GameDefinition{
		ID:    "test-pay-insufficient",
		Title: "Test Pay Insufficient",
		Board: Board{
			Width: 384, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 0},
				{ID: "target", Title: "Target", Type: "empty", X: 192, Y: 0},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "target", Condition: EdgeCondition{Type: "pay_resource", Resource: "gold", Amount: intPtr(5)}},
			},
		},
		Rules: RuleSet{
			Dice:  DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"gold": {Initial: 2},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
			StartBonus: 0,
		},
	}
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}}
	session := StartSession(g, players)
	if session.State.Players[0].Resources["gold"] != 2 {
		t.Fatalf("expected initial gold 2, got %d", session.State.Players[0].Resources["gold"])
	}

	events := session.MoveCurrentPlayer(1, []int{1}, 1)
	if !hasEventType(events, "move_blocked") {
		t.Fatal("expected move_blocked when player has insufficient gold to pay")
	}
	// Position should remain start
	if session.State.Players[0].PositionCellID != "start" {
		t.Fatalf("expected position to remain start, got %s", session.State.Players[0].PositionCellID)
	}
	// Gold should not have been deducted
	if session.State.Players[0].Resources["gold"] != 2 {
		t.Fatalf("expected gold to remain 2, got %d", session.State.Players[0].Resources["gold"])
	}
}

func TestManualChoiceCreatesPendingAction(t *testing.T) {
	g := &GameDefinition{
		ID:    "test-manual-choice",
		Title: "Test Manual Choice",
		Board: Board{
			Width: 576, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 0},
				{ID: "branch", Title: "Branch", Type: "empty", X: 192, Y: 0},
				{ID: "left", Title: "Left Path", Type: "empty", X: 384, Y: 0},
				{ID: "right", Title: "Right Path", Type: "empty", X: 384, Y: 192},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "branch", Condition: EdgeCondition{Type: "always"}},
				{ID: "e_left", From: "branch", To: "left", Condition: EdgeCondition{Type: "manual_choice", Label: "Go Left"}},
				{ID: "e_right", From: "branch", To: "right", Condition: EdgeCondition{Type: "manual_choice", Label: "Go Right"}},
			},
		},
		Rules: RuleSet{
			Dice:  DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"money": {Initial: 100},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
			StartBonus: 0,
		},
	}
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}}
	session := StartSession(g, players)

	// Move 2 steps: start→branch (1 step), then at branch both manual_choice edges are available → pending action created
	session.MoveCurrentPlayer(2, []int{2}, 2)

	// Should not advance turn or move to a final cell — should pause for choice
	if session.State.PendingAction == nil {
		t.Fatal("expected pending action to be set")
	}
	if session.State.PendingAction.Type != "route_choice" {
		t.Fatalf("expected route_choice, got %s", session.State.PendingAction.Type)
	}
	if len(session.State.PendingAction.Options) != 2 {
		t.Fatalf("expected 2 options, got %d", len(session.State.PendingAction.Options))
	}
	if session.State.PendingMovement == nil {
		t.Fatal("expected pending movement to be set")
	}
	if session.State.PendingMovement.RemainingSteps != 1 {
		t.Fatalf("expected 1 remaining step, got %d", session.State.PendingMovement.RemainingSteps)
	}
	// Player should still be at branch
	if session.State.Players[0].PositionCellID != "branch" {
		t.Fatalf("expected position branch, got %s", session.State.Players[0].PositionCellID)
	}

	// Resolve by choosing left path
	resolveEvents, err := session.ResolvePendingAction("e_left")
	if err != nil {
		t.Fatalf("unexpected error resolving route choice: %v", err)
	}
	if !hasEventType(resolveEvents, "route_chosen") {
		t.Fatal("expected route_chosen event")
	}
	if session.State.Players[0].PositionCellID != "left" {
		t.Fatalf("expected position left, got %s", session.State.Players[0].PositionCellID)
	}
	// Pending action and movement should be cleared
	if session.State.PendingAction != nil {
		t.Fatal("pending action should be cleared after resolve")
	}
	if session.State.PendingMovement != nil {
		t.Fatal("pending movement should be cleared after resolve")
	}
}

func TestMoveStepsNoOutgoingEdges(t *testing.T) {
	g := &GameDefinition{
		ID:    "test-no-edges",
		Title: "Test No Outgoing Edges",
		Board: Board{
			Width: 384, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 0},
				{ID: "deadend", Title: "Dead End", Type: "empty", X: 192, Y: 0},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "deadend", Condition: EdgeCondition{Type: "always"}},
				// deadend has no outgoing edges
			},
		},
		Rules: RuleSet{
			Dice:  DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"money": {Initial: 100},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
			StartBonus: 0,
		},
	}
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}}
	session := StartSession(g, players)

	// Move 2 steps: start→deadend (1 step), then deadend has no edges → move_blocked
	events := session.MoveCurrentPlayer(2, []int{2}, 2)
	if !hasEventType(events, "move_blocked") {
		t.Fatal("expected move_blocked event when no outgoing edges")
	}
	// Player should end up at deadend (movement stops but position is still updated to deadend)
	if session.State.Players[0].PositionCellID != "deadend" {
		t.Fatalf("expected position deadend, got %s", session.State.Players[0].PositionCellID)
	}
}

func TestMoveStepsNoAvailableEdges(t *testing.T) {
	g := &GameDefinition{
		ID:    "test-no-available-edges",
		Title: "Test No Available Edges",
		Board: Board{
			Width: 576, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 0},
				{ID: "branch", Title: "Branch", Type: "empty", X: 192, Y: 0},
				{ID: "target", Title: "Target", Type: "empty", X: 384, Y: 0},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "branch", Condition: EdgeCondition{Type: "always"}},
				{ID: "e2", From: "branch", To: "target", Condition: EdgeCondition{Type: "dice_total_even"}},
			},
		},
		Rules: RuleSet{
			Dice:  DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"money": {Initial: 100},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
			StartBonus: 0,
		},
	}
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}}
	session := StartSession(g, players)

	// Move 2 steps with odd total (3): start→branch, then branch→target requires even total, so no available edge → move_blocked
	events := session.MoveCurrentPlayer(2, []int{3}, 3)
	if !hasEventType(events, "move_blocked") {
		t.Fatal("expected move_blocked event when no available edges at fork")
	}
	// Player should end up at branch
	if session.State.Players[0].PositionCellID != "branch" {
		t.Fatalf("expected position branch, got %s", session.State.Players[0].PositionCellID)
	}
}

func TestMoveStepsLeadsToUnknownCell(t *testing.T) {
	g := &GameDefinition{
		ID:    "test-unknown-cell",
		Title: "Test Unknown Cell",
		Board: Board{
			Width: 384, Height: 384, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 0},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "nonexistent", Condition: EdgeCondition{Type: "always"}},
			},
		},
		Rules: RuleSet{
			Dice:  DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"money": {Initial: 100},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
			},
			StartBonus: 0,
		},
	}
	players := []PlayerConfig{{Name: "P1", Color: "#e74c3c"}}
	session := StartSession(g, players)

	events := session.MoveCurrentPlayer(1, []int{1}, 1)
	if !hasEventType(events, "error") {
		t.Fatal("expected error event when edge leads to unknown cell")
	}
	// Player should still be at start since movement was interrupted
	if session.State.Players[0].PositionCellID != "start" {
		t.Fatalf("expected position to remain start, got %s", session.State.Players[0].PositionCellID)
	}
}

func TestBranchingDemoValidates(t *testing.T) {
	branchingDemo := &GameDefinition{
		ID:    "branching-demo",
		Title: "Branching Demo",
		Board: Board{
			Width: 960, Height: 576, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 288},
				{ID: "fork", Title: "Fork", Type: "empty", X: 288, Y: 288},
				{ID: "left_path", Title: "Left Path", Type: "empty", X: 576, Y: 96},
				{ID: "right_path", Title: "Right Path", Type: "empty", X: 576, Y: 480},
				{ID: "merge", Title: "Merge", Type: "empty", X: 864, Y: 288},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "fork", Condition: EdgeCondition{Type: "always"}},
				{ID: "e2", From: "fork", To: "left_path", Condition: EdgeCondition{Type: "dice_total_even"}},
				{ID: "e3", From: "fork", To: "right_path", Condition: EdgeCondition{Type: "dice_total_odd"}},
				{ID: "e4", From: "left_path", To: "merge", Condition: EdgeCondition{Type: "always"}},
				{ID: "e5", From: "right_path", To: "merge", Condition: EdgeCondition{Type: "always"}},
			},
		},
		Rules: RuleSet{
			Dice: DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"money": {Initial: 100},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
		},
	}
	if err := ValidateDefinition(branchingDemo); err != nil {
		t.Fatalf("Branching Demo should validate: %v", err)
	}
}

func TestManualBranchDemoValidates(t *testing.T) {
	manualBranchDemo := &GameDefinition{
		ID:    "manual-branch-demo",
		Title: "Manual Branch Demo",
		Board: Board{
			Width: 960, Height: 576, CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 0, Y: 288},
				{ID: "fork", Title: "Fork", Type: "empty", X: 288, Y: 288},
				{ID: "left_path", Title: "Left Path", Type: "empty", X: 576, Y: 96},
				{ID: "right_path", Title: "Right Path", Type: "empty", X: 576, Y: 480},
				{ID: "merge", Title: "Merge", Type: "empty", X: 864, Y: 288},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "fork", Condition: EdgeCondition{Type: "always"}},
				{ID: "e_left", From: "fork", To: "left_path", Condition: EdgeCondition{Type: "manual_choice", Label: "Go Left"}},
				{ID: "e_right", From: "fork", To: "right_path", Condition: EdgeCondition{Type: "manual_choice", Label: "Go Right"}},
				{ID: "e4", From: "left_path", To: "merge", Condition: EdgeCondition{Type: "always"}},
				{ID: "e5", From: "right_path", To: "merge", Condition: EdgeCondition{Type: "always"}},
			},
		},
		Rules: RuleSet{
			Dice: DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{
				"money": {Initial: 100},
			},
			CellTypes: map[string]CellTypeDef{
				"start": {},
				"empty": {},
			},
		},
	}
	if err := ValidateDefinition(manualBranchDemo); err != nil {
		t.Fatalf("Manual Branch Demo should validate: %v", err)
	}
}
