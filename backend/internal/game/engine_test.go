package game

import (
	"testing"
)

var miniMonopolyDemo = &GameDefinition{
	ID:    "mini-monopoly",
	Title: "Mini-Monopoly",
	Board: Board{
		Width: 800, Height: 700, CellSize: 96,
		Cells: []CellDefinition{
			{ID: "start", Title: "Start", Type: "start", X: 50, Y: 550, Visual: CellVisual{BaseColor: "#4CAF50"}, OnLand: []ActionDefinition{}},
			{ID: "cell_2", Title: "Street A", Type: "property", X: 200, Y: 550, Visual: CellVisual{BaseColor: "#E3F2FD"},
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
			{ID: "cell_3", Title: "Bonus", Type: "bonus", X: 350, Y: 550, Visual: CellVisual{BaseColor: "#C8E6C9"},
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
		Width: 1200, Height: 400, CellSize: 96,
		Cells: []CellDefinition{
			{ID: "start", Title: "Start", Type: "start", X: 50, Y: 150, Visual: CellVisual{BaseColor: "#4CAF50"},
				OnLand: []ActionDefinition{}},
			{ID: "trap", Title: "Trap", Type: "trap", X: 200, Y: 150, Visual: CellVisual{BaseColor: "#FFCDD2"},
				OnLand: []ActionDefinition{
					{Type: "lose_resource", Resource: "health", Amount: intPtr(2)},
				}},
			{ID: "treasure", Title: "Treasure", Type: "treasure", X: 350, Y: 150, Visual: CellVisual{BaseColor: "#FFF9C4"},
				OnLand: []ActionDefinition{
					{Type: "gain_resource", Resource: "gold", Amount: intPtr(5)},
				}},
			{ID: "key", Title: "Key", Type: "key", X: 500, Y: 150, Visual: CellVisual{BaseColor: "#E1BEE7"},
				OnLand: []ActionDefinition{
					{Type: "gain_resource", Resource: "keys", Amount: intPtr(1)},
				}},
			{ID: "heal", Title: "Heal", Type: "heal", X: 650, Y: 150, Visual: CellVisual{BaseColor: "#C8E6C9"},
				OnLand: []ActionDefinition{
					{Type: "gain_resource", Resource: "health", Amount: intPtr(2)},
				}},
			{ID: "finish", Title: "Finish", Type: "finish", X: 800, Y: 150, Visual: CellVisual{BaseColor: "#FFD700"},
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

func TestGameWithoutMoney(t *testing.T) {
	g := &GameDefinition{
		ID:    "no-money-game",
		Title: "No Money",
		Board: Board{
			CellSize: 96,
			Cells: []CellDefinition{
				{ID: "start", Title: "Start", Type: "start", X: 50, Y: 150, Visual: CellVisual{BaseColor: "#4CAF50"}},
				{ID: "cell2", Title: "Cell", Type: "empty", X: 200, Y: 150, Visual: CellVisual{BaseColor: "#ccc"},
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
