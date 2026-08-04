package game

import (
	"encoding/json"
	"testing"
)

// propertyDefinition is a small property board: two cells in the blue group,
// three stations, one square nobody can own.
func propertyDefinition() *GameDefinition {
	property := func(id, title, group string, x, rent int) CellDefinition {
		return CellDefinition{
			ID: id, Title: title, Type: "property", X: x, Y: 0,
			Visual: CellVisual{BaseColor: "#dddddd"},
			Fields: map[string]any{"group": group, "rent": rent},
		}
	}
	station := func(id, title string, x int) CellDefinition {
		return CellDefinition{
			ID: id, Title: title, Type: "station", X: x, Y: 0,
			Visual: CellVisual{BaseColor: "#cccccc"},
			Fields: map[string]any{"rent": 25},
		}
	}
	cells := []CellDefinition{
		{ID: "start", Title: "Go", Type: "start", X: 0, Y: 0, Fields: map[string]any{}},
		property("blue_a", "Blue A", "blue", 100, 30),
		property("blue_b", "Blue B", "blue", 200, 30),
		property("red_a", "Red A", "red", 300, 20),
		station("station_a", "North Station", 400),
		station("station_b", "South Station", 500),
		station("station_c", "East Station", 600),
	}
	edges := make([]EdgeDefinition, 0, len(cells))
	for i := range cells {
		next := cells[(i+1)%len(cells)]
		edges = append(edges, EdgeDefinition{
			ID: "e" + cells[i].ID, From: cells[i].ID, To: next.ID,
			Condition: EdgeCondition{Type: "always"},
		})
	}
	return &GameDefinition{
		ID: "props", Title: "Properties", Version: 1,
		Board: Board{Width: 700, Height: 100, CellSize: 100, Cells: cells, Edges: edges},
		Rules: RuleSet{
			Dice:      DiceRule{Count: 1, Sides: 6},
			Resources: map[string]ResourceRule{"money": {Initial: 500}},
			CellTypes: map[string]CellTypeDef{
				"start":    {Title: "Go", Fields: map[string]FieldDef{}},
				"property": {Title: "Property", Fields: map[string]FieldDef{}},
				"station":  {Title: "Station", Fields: map[string]FieldDef{}},
			},
		},
	}
}

func propertySession(t *testing.T) *GameSession {
	t.Helper()
	return StartSession(propertyDefinition(), []PlayerConfig{
		{Name: "Ada", Color: "#111111"},
		{Name: "Bob", Color: "#222222"},
	})
}

func own(session *GameSession, cellID, playerID string) {
	state := session.State.CellStates[cellID]
	state.OwnerPlayerID = playerID
	session.State.CellStates[cellID] = state
}

func TestCountCellsFiltersByTypeAndOwner(t *testing.T) {
	session := propertySession(t)
	ada := &session.State.Players[0]
	bob := &session.State.Players[1]
	own(session, "station_a", ada.ID)
	own(session, "station_b", ada.ID)
	own(session, "station_c", bob.ID)

	mine := session.countCells(&CellQuery{Type: "station", Owner: "current"}, ada, nil)
	if mine != 2 {
		t.Fatalf("Ada's stations = %d, want 2", mine)
	}
	theirs := session.countCells(&CellQuery{Type: "station", Owner: "other"}, ada, nil)
	if theirs != 1 {
		t.Fatalf("other players' stations = %d, want 1", theirs)
	}
	all := session.countCells(&CellQuery{Type: "station"}, ada, nil)
	if all != 3 {
		t.Fatalf("all stations = %d, want 3", all)
	}
	free := session.countCells(&CellQuery{Type: "property", Owner: "none"}, ada, nil)
	if free != 3 {
		t.Fatalf("unowned properties = %d, want 3", free)
	}
}

func TestSameAsCellMatchesTheGroupWithoutNamingIt(t *testing.T) {
	session := propertySession(t)
	ada := &session.State.Players[0]
	own(session, "blue_a", ada.ID)
	own(session, "blue_b", ada.ID)
	own(session, "red_a", ada.ID)

	blueA := session.Definition.Board.getCellByID("blue_a")
	// The same query, run from a blue square, must count only the blue group:
	// that is what makes one query reusable across every group on the board.
	blue := session.countCells(&CellQuery{Field: "group", SameAsCell: true, Owner: "current"}, ada, blueA)
	if blue != 2 {
		t.Fatalf("blue group = %d, want 2", blue)
	}

	redA := session.Definition.Board.getCellByID("red_a")
	red := session.countCells(&CellQuery{Field: "group", SameAsCell: true, Owner: "current"}, ada, redA)
	if red != 1 {
		t.Fatalf("red group = %d, want 1", red)
	}
}

func TestCellOwnerQueryCountsTheLandlordsHoldings(t *testing.T) {
	session := propertySession(t)
	ada := &session.State.Players[0]
	bob := &session.State.Players[1]
	own(session, "station_a", bob.ID)
	own(session, "station_b", bob.ID)

	stationA := session.Definition.Board.getCellByID("station_a")
	// Ada is the visitor; the count that decides the rent is Bob's.
	count := session.countCells(&CellQuery{Type: "station", Owner: "cellOwner"}, ada, stationA)
	if count != 2 {
		t.Fatalf("landlord's stations = %d, want 2", count)
	}

	// An unowned reference cell has no landlord, so nothing matches rather
	// than everything.
	free := session.Definition.Board.getCellByID("station_c")
	if got := session.countCells(&CellQuery{Type: "station", Owner: "cellOwner"}, ada, free); got != 0 {
		t.Fatalf("unowned reference matched %d cells, want 0", got)
	}
}

func TestRentScalesWithAQueryInsteadOfHandWrittenTiers(t *testing.T) {
	session := propertySession(t)
	ada := &session.State.Players[0]
	bob := &session.State.Players[1]
	own(session, "station_a", bob.ID)
	own(session, "station_b", bob.ID)
	own(session, "station_c", bob.ID)

	stationA := session.Definition.Board.getCellByID("station_a")
	adaBefore := ada.Resources["money"]
	bobBefore := bob.Resources["money"]

	// 25 base rent × the three stations Bob owns.
	session.executeOneAction(ActionDefinition{
		Type:     "transfer_resource",
		Resource: "money",
		Target:   "owner",
		Formula: &AmountFormula{
			Base:  &AmountTerm{Kind: "field", Name: "rent"},
			Times: &AmountTerm{Kind: "cells", Query: &CellQuery{Type: "station", Owner: "cellOwner"}},
		},
	}, ada, stationA)

	if paid := adaBefore - ada.Resources["money"]; paid != 75 {
		t.Fatalf("rent paid = %d, want 75", paid)
	}
	if got := bob.Resources["money"] - bobBefore; got != 75 {
		t.Fatalf("rent received = %d, want 75", got)
	}
}

func TestIfCellsGeComparesTwoQueries(t *testing.T) {
	session := propertySession(t)
	ada := &session.State.Players[0]
	own(session, "blue_a", ada.ID)
	blueA := session.Definition.Board.getCellByID("blue_a")

	// "I own every cell in this group": the count of mine against the count of
	// all of them, so adding a third blue square to the board cannot leave a
	// hard-coded 2 behind.
	monopoly := ActionDefinition{
		Type:  "if_cells_ge",
		Query: &CellQuery{Field: "group", SameAsCell: true, Owner: "current"},
		Formula: &AmountFormula{
			Base: &AmountTerm{Kind: "cells", Query: &CellQuery{Field: "group", SameAsCell: true}},
		},
		Then: []ActionDefinition{{Type: "gain_resource", Resource: "money", Amount: intPtr(50)}},
		Else: []ActionDefinition{{Type: "lose_resource", Resource: "money", Amount: intPtr(10)}},
	}

	before := ada.Resources["money"]
	session.executeOneAction(monopoly, ada, blueA)
	if ada.Resources["money"] != before-10 {
		t.Fatalf("half a group took the then branch: money = %d, want %d", ada.Resources["money"], before-10)
	}

	own(session, "blue_b", ada.ID)
	before = ada.Resources["money"]
	session.executeOneAction(monopoly, ada, blueA)
	if ada.Resources["money"] != before+50 {
		t.Fatalf("a complete group took the else branch: money = %d, want %d", ada.Resources["money"], before+50)
	}
}

func TestForEachCellActsOnEveryMatchInTurn(t *testing.T) {
	session := propertySession(t)
	ada := &session.State.Players[0]
	own(session, "blue_a", ada.ID)
	own(session, "blue_b", ada.ID)
	own(session, "red_a", ada.ID)
	blueA := session.Definition.Board.getCellByID("blue_a")

	session.executeOneAction(ActionDefinition{
		Type:  "for_each_cell",
		Query: &CellQuery{Field: "group", SameAsCell: true, Owner: "current"},
		Then:  []ActionDefinition{{Type: "set_cell_level", Amount: intPtr(1)}},
	}, ada, blueA)

	// The level lands on each matched cell, not on the cell the loop started
	// from — the whole point of running the body with the match as context.
	for _, id := range []string{"blue_a", "blue_b"} {
		if level := session.State.CellStates[id].Level; level != 1 {
			t.Fatalf("%s level = %d, want 1", id, level)
		}
	}
	if level := session.State.CellStates["red_a"].Level; level != 0 {
		t.Fatalf("red_a was built on: level = %d, want 0", level)
	}
}

func TestForEachCellFallsBackWhenNothingMatches(t *testing.T) {
	session := propertySession(t)
	ada := &session.State.Players[0]
	blueA := session.Definition.Board.getCellByID("blue_a")

	events := session.executeOneAction(ActionDefinition{
		Type:  "for_each_cell",
		Query: &CellQuery{Type: "station", Owner: "current"},
		Then:  []ActionDefinition{{Type: "gain_resource", Resource: "money", Amount: intPtr(100)}},
		Else:  []ActionDefinition{{Type: "log_message", Title: "nothing to collect"}},
	}, ada, blueA)

	if ada.Resources["money"] != 500 {
		t.Fatalf("the then branch ran with no matches: money = %d", ada.Resources["money"])
	}
	if len(events) != 1 || events[0].Message != "nothing to collect" {
		t.Fatalf("else branch did not run: %+v", events)
	}
}

func TestFieldValuesCompareAsText(t *testing.T) {
	// A field written as a JSON number arrives as a float64, and 3 must not
	// stringify as "3.0000000001" or fail to match a value typed as "3".
	if got := fieldText(float64(3)); got != "3" {
		t.Fatalf("fieldText(3) = %q, want \"3\"", got)
	}
	if got := fieldText(nil); got != "" {
		t.Fatalf("fieldText(nil) = %q, want empty", got)
	}
	if got := fieldText("blue"); got != "blue" {
		t.Fatalf("fieldText(\"blue\") = %q", got)
	}
}

func TestQueryValuesSurviveARoundTripThroughJSON(t *testing.T) {
	// Fields come back from Postgres as float64 whatever they went in as, so
	// the match has to hold across a save.
	session := propertySession(t)
	ada := &session.State.Players[0]
	own(session, "blue_a", ada.ID)

	encoded, err := json.Marshal(ForStorage{Session: session})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored GameSession
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	blueA := restored.Definition.Board.getCellByID("blue_a")
	count := restored.countCells(&CellQuery{Field: "group", SameAsCell: true}, &restored.State.Players[0], blueA)
	if count != 2 {
		t.Fatalf("after a round trip the group counted %d, want 2", count)
	}
}

func TestABareNumberStillDecodesAsAMultiplier(t *testing.T) {
	// Definitions published before times became a term write "times": 2, and a
	// stored game that no longer loads is a destroyed game.
	var formula AmountFormula
	if err := json.Unmarshal([]byte(`{"base":{"kind":"const","value":10},"times":2,"dividedBy":4}`), &formula); err != nil {
		t.Fatalf("legacy formula did not decode: %v", err)
	}
	session := propertySession(t)
	if got := session.resolveFormula(&formula, &session.State.Players[0], nil); got != 5 {
		t.Fatalf("legacy formula = %d, want 5", got)
	}
}

func TestNestedActionsStopAtADepthLimit(t *testing.T) {
	// Two cells that teleport onto each other loop for ever. Before the limit
	// this took the whole server down with a stack overflow, not just the game.
	definition := propertyDefinition()
	definition.Board.Cells[1].OnLand = []ActionDefinition{{Type: "move_player_to", To: "blue_b"}}
	definition.Board.Cells[2].OnLand = []ActionDefinition{{Type: "move_player_to", To: "blue_a"}}
	session := StartSession(definition, []PlayerConfig{{Name: "Ada"}, {Name: "Bob"}})

	events := session.executeActions(definition.Board.Cells[1].OnLand, &session.State.Players[0], &definition.Board.Cells[1])

	stopped := false
	for _, event := range events {
		if event.Type == "action_depth_exceeded" {
			stopped = true
		}
	}
	if !stopped {
		t.Fatalf("the loop was not stopped: %d events", len(events))
	}
	if session.execDepth != 0 {
		t.Fatalf("depth counter leaked: %d", session.execDepth)
	}
}
