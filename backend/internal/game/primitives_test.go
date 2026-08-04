package game

import (
	"strings"
	"testing"
	"time"
)

// primitiveBoard is a small loop with a side cell to teleport to.
func primitiveBoard() *GameDefinition {
	cell := func(id, title, kind string, x int, onLand ...ActionDefinition) CellDefinition {
		return CellDefinition{
			ID: id, Title: title, Type: kind, X: x, Y: 0,
			Visual: CellVisual{BaseColor: "#cccccc"},
			Fields: map[string]any{},
			OnLand: onLand,
		}
	}
	return &GameDefinition{
		ID: "primitives", Title: "Primitives", Version: 1,
		Board: Board{
			Width: 400, Height: 100, CellSize: 100,
			Cells: []CellDefinition{
				cell("start", "Start", "start", 0),
				cell("plot", "Plot", "property", 100),
				cell("trap", "Trap", "penalty", 200),
				cell("jail", "Jail", "empty", 300),
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "start", To: "plot", Condition: EdgeCondition{Type: "always"}},
				{ID: "e2", From: "plot", To: "trap", Condition: EdgeCondition{Type: "always"}},
				{ID: "e3", From: "trap", To: "jail", Condition: EdgeCondition{Type: "always"}},
				{ID: "e4", From: "jail", To: "start", Condition: EdgeCondition{Type: "always"}},
			},
		},
		Rules: RuleSet{
			Dice:      DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{"money": {Initial: 100}},
			CellTypes: map[string]CellTypeDef{
				"start":    {Title: "Start", Fields: map[string]FieldDef{}},
				"property": {Title: "Property", Fields: map[string]FieldDef{}},
				"penalty":  {Title: "Penalty", Fields: map[string]FieldDef{}},
				"empty":    {Title: "Empty", Fields: map[string]FieldDef{}},
			},
		},
	}
}

func primitiveSession(t *testing.T) *GameSession {
	t.Helper()
	return StartSession(primitiveBoard(), []PlayerConfig{
		{Name: "Ada", Color: "#111111"},
		{Name: "Bob", Color: "#222222"},
	})
}

func TestSetCellLevelAndLevelBranch(t *testing.T) {
	session := primitiveSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("plot")

	session.executeOneAction(ActionDefinition{Type: "set_cell_level", Amount: intPtr(3)}, player, cell)
	if got := session.State.CellStates["plot"].Level; got != 3 {
		t.Fatalf("level = %d, want 3", got)
	}

	// The branch is what makes tiered rent expressible in data alone.
	events := session.executeOneAction(ActionDefinition{
		Type:   "if_cell_level_ge",
		Amount: intPtr(3),
		Then:   []ActionDefinition{{Type: "gain_resource", Resource: "money", Amount: intPtr(50)}},
		Else:   []ActionDefinition{{Type: "lose_resource", Resource: "money", Amount: intPtr(50)}},
	}, player, cell)
	if player.Resources["money"] != 150 {
		t.Fatalf("money = %d, want the then-branch to have run", player.Resources["money"])
	}
	if len(events) == 0 {
		t.Fatal("branch produced no events")
	}

	// A negative level is clamped rather than stored.
	session.executeOneAction(ActionDefinition{Type: "set_cell_level", Amount: intPtr(-5)}, player, cell)
	if got := session.State.CellStates["plot"].Level; got != 0 {
		t.Fatalf("level = %d, want 0 after a negative value", got)
	}
}

func TestChangingOwnerClearsBuildings(t *testing.T) {
	session := primitiveSession(t)
	owner := &session.State.Players[0]
	buyer := &session.State.Players[1]
	cell := session.Definition.Board.getCellByID("plot")

	session.executeOneAction(ActionDefinition{Type: "set_cell_owner", Target: "current"}, owner, cell)
	session.executeOneAction(ActionDefinition{Type: "set_cell_level", Amount: intPtr(4)}, owner, cell)
	session.executeOneAction(ActionDefinition{Type: "set_cell_mortgaged", Target: "true"}, owner, cell)

	session.executeOneAction(ActionDefinition{Type: "set_cell_owner", Target: "current"}, buyer, cell)

	state := session.State.CellStates["plot"]
	if state.OwnerPlayerID != buyer.ID {
		t.Fatalf("owner = %q, want the buyer", state.OwnerPlayerID)
	}
	// A property changing hands must not carry the previous owner's investment.
	if state.Level != 0 || state.Mortgaged {
		t.Fatalf("cell state = %#v, want buildings and mortgage cleared on sale", state)
	}
}

func TestMortgageFlagAndBranch(t *testing.T) {
	session := primitiveSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("plot")

	session.executeOneAction(ActionDefinition{Type: "set_cell_mortgaged", Target: "true"}, player, cell)
	if !session.State.CellStates["plot"].Mortgaged {
		t.Fatal("cell was not mortgaged")
	}
	session.executeOneAction(ActionDefinition{
		Type: "if_cell_mortgaged",
		Then: []ActionDefinition{{Type: "gain_resource", Resource: "money", Amount: intPtr(10)}},
	}, player, cell)
	if player.Resources["money"] != 110 {
		t.Fatalf("money = %d, want the mortgaged branch to have run", player.Resources["money"])
	}

	session.executeOneAction(ActionDefinition{Type: "set_cell_mortgaged", Target: "false"}, player, cell)
	if session.State.CellStates["plot"].Mortgaged {
		t.Fatal("cell was not redeemed")
	}
}

func TestMovePlayerToRunsTheDestinationActions(t *testing.T) {
	definition := primitiveBoard()
	// Landing in jail costs money, which proves the destination's own actions run.
	for i := range definition.Board.Cells {
		if definition.Board.Cells[i].ID == "jail" {
			definition.Board.Cells[i].OnLand = []ActionDefinition{
				{Type: "lose_resource", Resource: "money", Amount: intPtr(25)},
			}
		}
	}
	session := StartSession(definition, []PlayerConfig{{Name: "Ada", Color: "#111"}, {Name: "Bob", Color: "#222"}})
	player := &session.State.Players[0]
	trap := session.Definition.Board.getCellByID("trap")

	session.executeOneAction(ActionDefinition{Type: "move_player_to", To: "jail"}, player, trap)

	if player.PositionCellID != "jail" {
		t.Fatalf("position = %q, want jail", player.PositionCellID)
	}
	if player.Resources["money"] != 75 {
		t.Fatalf("money = %d, want the destination's own actions to have run", player.Resources["money"])
	}
}

func TestMovePlayerToUnknownCellIsReportedNotSilent(t *testing.T) {
	session := primitiveSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("trap")
	before := player.PositionCellID

	events := session.executeOneAction(ActionDefinition{Type: "move_player_to", To: "nowhere"}, player, cell)

	if player.PositionCellID != before {
		t.Fatalf("player moved to a cell that does not exist")
	}
	if len(events) != 1 || events[0].Type != "invalid_action" {
		t.Fatalf("events = %#v, want a reported invalid_action", events)
	}
}

// A jail sentence measured in turns has to actually elapse.
func TestSkipTurnsCostsThatManyTurns(t *testing.T) {
	session := primitiveSession(t)
	session.State.Players[1].SkipTurns = 1

	// Ada finishes her turn; Bob is skipped, so it comes back to Ada.
	session.advanceTurn()
	if session.State.CurrentPlayerIndex != 0 {
		t.Fatalf("current player index = %d, want Ada again while Bob sits out", session.State.CurrentPlayerIndex)
	}
	if session.State.Players[1].SkipTurns != 0 {
		t.Fatalf("Bob still owes %d skipped turns", session.State.Players[1].SkipTurns)
	}

	// Next time round Bob plays normally.
	session.advanceTurn()
	if session.State.CurrentPlayerIndex != 1 {
		t.Fatalf("current player index = %d, want Bob back in the game", session.State.CurrentPlayerIndex)
	}
}

func TestSkipTurnsTerminatesWhenEverybodyIsSkipping(t *testing.T) {
	session := primitiveSession(t)
	session.State.Players[0].SkipTurns = 2
	session.State.Players[1].SkipTurns = 2

	done := make(chan struct{})
	go func() {
		session.advanceTurn()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("advanceTurn did not terminate with every player skipping")
	}
}

func TestRandomBranchAlwaysPicksOneDeclaredOption(t *testing.T) {
	session := primitiveSession(t)
	player := &session.State.Players[0]
	cell := session.Definition.Board.getCellByID("trap")

	action := ActionDefinition{
		Type: "random_branch",
		Options: []ActionOption{
			{ID: "gain", Title: "Found money", Then: []ActionDefinition{{Type: "gain_resource", Resource: "money", Amount: intPtr(10)}}},
			{ID: "lose", Title: "Dropped money", Then: []ActionDefinition{{Type: "lose_resource", Resource: "money", Amount: intPtr(10)}}},
		},
	}

	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		// Reset each draw: lose_resource floors at zero, so an unconstrained
		// random walk from 100 would eventually hit the floor and report a
		// delta this assertion could not distinguish from a bug.
		player.Resources["money"] = 1000
		events := session.executeOneAction(action, player, cell)
		delta := player.Resources["money"] - 1000
		if delta != 10 && delta != -10 {
			t.Fatalf("money changed by %d, want exactly one declared option to have run", delta)
		}
		if len(events) == 0 {
			t.Fatal("random_branch produced no events")
		}
		seen[events[0].Message] = true
	}
	// Over 200 draws both outcomes should appear; a stuck picker is a real bug.
	if len(seen) < 2 {
		t.Fatalf("only ever picked %v, want both options over 200 draws", seen)
	}
}

func TestValidationRejectsBrokenTeleports(t *testing.T) {
	definition := primitiveBoard()
	for i := range definition.Board.Cells {
		switch definition.Board.Cells[i].ID {
		case "trap":
			definition.Board.Cells[i].OnLand = []ActionDefinition{{Type: "move_player_to", To: "atlantis"}}
		case "plot":
			// A cell teleporting onto itself would re-run its own actions forever.
			definition.Board.Cells[i].OnLand = []ActionDefinition{{Type: "move_player_to", To: "plot"}}
		}
	}

	err := ValidateDefinition(definition)
	if err == nil {
		t.Fatal("validation accepted broken teleports")
	}
	joined := strings.Join(err.Errors, "; ")
	if !strings.Contains(joined, "atlantis") {
		t.Fatalf("errors = %v, want the unknown destination reported", err.Errors)
	}
	if !strings.Contains(joined, "onto itself") {
		t.Fatalf("errors = %v, want the self-teleport reported", err.Errors)
	}
}

func TestValidationRejectsMalformedNewActions(t *testing.T) {
	cases := []struct {
		name   string
		action ActionDefinition
		want   string
	}{
		{"mortgage needs a boolean target", ActionDefinition{Type: "set_cell_mortgaged", Target: "maybe"}, "true"},
		{"skip needs an amount", ActionDefinition{Type: "skip_turns"}, "amount"},
		{"random needs options", ActionDefinition{Type: "random_branch", Options: []ActionOption{{ID: "only"}}}, "two options"},
		{"teleport needs a destination", ActionDefinition{Type: "move_player_to"}, "required"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			errs := validateAction(testCase.action, "action", actionContext{})
			if len(errs) == 0 {
				t.Fatalf("validateAction(%#v) accepted it", testCase.action)
			}
			if !strings.Contains(strings.Join(errs, "; "), testCase.want) {
				t.Fatalf("errors = %v, want something mentioning %q", errs, testCase.want)
			}
		})
	}
}

func TestEliminatePlayerEndsTheGameAndReleasesTheirSquares(t *testing.T) {
	session := primitiveSession(t)
	loser := &session.State.Players[0]
	winner := session.State.Players[1]
	plot := session.Definition.Board.getCellByID("plot")
	trap := session.Definition.Board.getCellByID("trap")

	session.executeOneAction(ActionDefinition{Type: "set_cell_owner", Target: "current"}, loser, plot)
	session.executeOneAction(ActionDefinition{Type: "set_cell_owner", Target: "current"}, loser, trap)

	events := session.executeOneAction(ActionDefinition{Type: "eliminate_player"}, loser, plot)

	if !loser.Bankrupt {
		t.Fatal("player was not marked bankrupt")
	}
	// An eliminated player owns nothing, so their squares can be bought again.
	for _, id := range []string{"plot", "trap"} {
		if owner := session.State.CellStates[id].OwnerPlayerID; owner != "" {
			t.Fatalf("cell %s still owned by %q after elimination", id, owner)
		}
	}
	if session.State.Status != "finished" {
		t.Fatalf("status = %q, want the game to end with one player left", session.State.Status)
	}
	if session.State.WinnerPlayerID != winner.ID {
		t.Fatalf("winner = %q, want the surviving player", session.State.WinnerPlayerID)
	}
	if len(events) == 0 || events[0].Type != "player_eliminated" {
		t.Fatalf("events = %#v, want a player_eliminated event", events)
	}
}

func TestEliminatingTheSamePlayerTwiceIsHarmless(t *testing.T) {
	session := primitiveSession(t)
	loser := &session.State.Players[0]
	plot := session.Definition.Board.getCellByID("plot")

	session.executeOneAction(ActionDefinition{Type: "eliminate_player"}, loser, plot)
	if events := session.executeOneAction(ActionDefinition{Type: "eliminate_player"}, loser, plot); len(events) != 0 {
		t.Fatalf("events = %#v, want the repeat elimination to do nothing", events)
	}
}
