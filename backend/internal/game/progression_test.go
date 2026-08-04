package game

import "testing"

func progressionDefinition() *GameDefinition {
	definition := rpgDefinition()
	definition.Rules.Resources["experience"] = ResourceRule{Initial: 0}
	definition.Rules.Resources["level"] = ResourceRule{Initial: 1}
	definition.Rules.Resources["points"] = ResourceRule{Initial: 0}
	definition.Rules.Progression = &ProgressionRule{
		ExperienceResource: "experience",
		LevelResource:      "level",
		PointsResource:     "points",
		PointsPerLevel:     2,
		Thresholds:         []int{10, 25, 50},
	}
	return definition
}

func TestCrossingAThresholdGrantsALevelAndPoints(t *testing.T) {
	session := StartSession(progressionDefinition(), []PlayerConfig{{Name: "Ada", Color: "#111"}, {Name: "Bob", Color: "#222"}})
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("cave")

	session.executeActions([]ActionDefinition{
		{Type: "gain_resource", Resource: "experience", Amount: intPtr(10)},
	}, player, cell)

	if player.Resources["level"] != 2 {
		t.Fatalf("level = %d, want 2", player.Resources["level"])
	}
	if player.Resources["points"] != 2 {
		t.Fatalf("points = %d, want 2 skill points", player.Resources["points"])
	}
}

// One big reward should not strand a player one level below where they belong.
func TestOneLargeRewardCanGrantSeveralLevelsAtOnce(t *testing.T) {
	session := StartSession(progressionDefinition(), []PlayerConfig{{Name: "Ada", Color: "#111"}, {Name: "Bob", Color: "#222"}})
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("cave")

	session.executeActions([]ActionDefinition{
		{Type: "gain_resource", Resource: "experience", Amount: intPtr(60)},
	}, player, cell)

	if player.Resources["level"] != 4 {
		t.Fatalf("level = %d, want 4 after passing every threshold", player.Resources["level"])
	}
	if player.Resources["points"] != 6 {
		t.Fatalf("points = %d, want 2 per level gained", player.Resources["points"])
	}
}

func TestLevellingStopsAtTheLastThreshold(t *testing.T) {
	session := StartSession(progressionDefinition(), []PlayerConfig{{Name: "Ada", Color: "#111"}, {Name: "Bob", Color: "#222"}})
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("cave")

	session.executeActions([]ActionDefinition{
		{Type: "gain_resource", Resource: "experience", Amount: intPtr(100000)},
	}, player, cell)

	if player.Resources["level"] != 4 {
		t.Fatalf("level = %d, want the maximum of 4 with three thresholds", player.Resources["level"])
	}
}

func TestGamesWithoutProgressionAreUnaffected(t *testing.T) {
	session := rpgSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("cave")

	session.executeActions([]ActionDefinition{
		{Type: "gain_resource", Resource: "gold", Amount: intPtr(1000)},
	}, player, cell)

	if _, exists := player.Resources["level"]; exists {
		t.Fatalf("resources = %v, want no level to have appeared", player.Resources)
	}
}

func freeMovementSession(t *testing.T) *GameSession {
	t.Helper()
	definition := rpgDefinition()
	definition.Rules.Movement = "free"
	// Neighbours are connected both ways, which is what a board meant for free
	// movement declares.
	definition.Board.Edges = append(definition.Board.Edges,
		EdgeDefinition{ID: "b1", From: "armoury", To: "start", Condition: EdgeCondition{Type: "always"}},
		EdgeDefinition{ID: "b2", From: "cave", To: "armoury", Condition: EdgeCondition{Type: "always"}},
		EdgeDefinition{ID: "b3", From: "summit", To: "cave", Condition: EdgeCondition{Type: "always"}},
	)
	return StartSession(definition, []PlayerConfig{{Name: "Ada", Color: "#111"}, {Name: "Bob", Color: "#222"}})
}

func TestFreeMovementOffersEveryCellInRangeButNotStandingStill(t *testing.T) {
	session := freeMovementSession(t)
	player := &session.State.Players[0]

	session.MoveCurrentPlayer(2, []int{2}, 2)

	pending := session.State.PendingAction
	if pending == nil || pending.Type != "free_move" {
		t.Fatalf("pending = %#v, want a free_move choice", pending)
	}
	offered := map[string]bool{}
	for _, option := range pending.Options {
		offered[option.ID] = true
	}
	// Two steps from the camp reaches the armoury and the cave, and the ring
	// also brings the summit within two going backwards.
	if !offered["armoury"] || !offered["cave"] {
		t.Fatalf("options = %v, want cells within two steps", offered)
	}
	// A roll has to move you somewhere.
	if offered[player.PositionCellID] {
		t.Fatalf("options = %v, want standing still not to be offered", offered)
	}
}

func TestFreeMovementRejectsACellOutOfRange(t *testing.T) {
	session := freeMovementSession(t)
	session.MoveCurrentPlayer(1, []int{1}, 1)

	// One step from the camp cannot reach the summit going forwards.
	if _, err := session.ResolvePendingAction("summit"); err == nil {
		t.Fatal("a destination beyond the roll was accepted")
	}
	// The choice is still open rather than being consumed by the bad attempt.
	if session.State.PendingAction == nil {
		t.Fatal("the pending move was discarded by an invalid choice")
	}
}

func TestFreeMovementRunsTheDestinationActions(t *testing.T) {
	session := freeMovementSession(t)
	for i := range session.Definition.Board.Cells {
		if session.Definition.Board.Cells[i].ID == "cave" {
			session.Definition.Board.Cells[i].OnLand = []ActionDefinition{
				{Type: "gain_resource", Resource: "gold", Amount: intPtr(7)},
			}
		}
	}
	player := &session.State.Players[0]

	session.MoveCurrentPlayer(2, []int{2}, 2)
	if _, err := session.ResolvePendingAction("cave"); err != nil {
		t.Fatalf("resolving the move failed: %v", err)
	}

	if player.PositionCellID != "cave" {
		t.Fatalf("position = %q, want the chosen cell", player.PositionCellID)
	}
	if player.Resources["gold"] != 7 {
		t.Fatalf("gold = %d, want the destination's actions to have run", player.Resources["gold"])
	}
	if session.State.CurrentPlayerIndex != 1 {
		t.Fatalf("current player = %d, want the turn to have passed", session.State.CurrentPlayerIndex)
	}
}

// A face-down destination must not announce what is on it.
func TestFreeMovementDoesNotNameUnexploredCells(t *testing.T) {
	definition := rpgDefinition()
	definition.Rules.Movement = "free"
	definition.Rules.HiddenCells = true
	for i := range definition.Board.Cells {
		if definition.Board.Cells[i].ID == "cave" {
			definition.Board.Cells[i].Title = "Dragon's Lair"
		}
	}
	session := StartSession(definition, []PlayerConfig{{Name: "Ada", Color: "#111"}, {Name: "Bob", Color: "#222"}})

	session.MoveCurrentPlayer(3, []int{3}, 3)

	for _, option := range session.State.PendingAction.Options {
		if option.Title == "Dragon's Lair" {
			t.Fatal("the destination list named an unexplored cell")
		}
	}
}
