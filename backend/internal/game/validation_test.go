package game

import (
	"strings"
	"testing"
)

func TestValidateEmptyID(t *testing.T) {
	g := &GameDefinition{
		ID:    "",
		Title: "Test Game",
		Board: Board{
			CellSize: 96,
			Cells:    []CellDefinition{{ID: "c1", Type: "start"}},
		},
		Rules: RuleSet{
			CellTypes: map[string]CellTypeDef{
				"start": {Title: "Start"},
			},
		},
	}
	err := ValidateDefinition(g)
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestValidateEmptyTitle(t *testing.T) {
	g := &GameDefinition{
		ID:    "test",
		Title: "",
		Board: Board{
			CellSize: 96,
			Cells:    []CellDefinition{{ID: "c1", Type: "start"}},
		},
		Rules: RuleSet{
			CellTypes: map[string]CellTypeDef{
				"start": {Title: "Start"},
			},
		},
	}
	err := ValidateDefinition(g)
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestValidateNoCells(t *testing.T) {
	g := &GameDefinition{
		ID:    "test",
		Title: "Test",
		Board: Board{
			CellSize: 96,
			Cells:    []CellDefinition{},
		},
		Rules: RuleSet{
			CellTypes: map[string]CellTypeDef{},
		},
	}
	err := ValidateDefinition(g)
	if err == nil {
		t.Fatal("expected error for no cells")
	}
}

func TestValidateNoStartCell(t *testing.T) {
	g := &GameDefinition{
		ID:    "test",
		Title: "Test",
		Board: Board{
			CellSize: 96,
			Cells:    []CellDefinition{{ID: "c1", Type: "empty"}},
		},
		Rules: RuleSet{
			CellTypes: map[string]CellTypeDef{
				"empty": {Title: "Empty"},
			},
		},
	}
	err := ValidateDefinition(g)
	if err == nil {
		t.Fatal("expected error for no start cell")
	}
}

func TestValidateDuplicateCellIDs(t *testing.T) {
	g := &GameDefinition{
		ID:    "test",
		Title: "Test",
		Board: Board{
			CellSize: 96,
			Cells: []CellDefinition{
				{ID: "c1", Type: "start"},
				{ID: "c1", Type: "empty"},
			},
		},
		Rules: RuleSet{
			CellTypes: map[string]CellTypeDef{
				"start": {Title: "Start"},
				"empty": {Title: "Empty"},
			},
		},
	}
	err := ValidateDefinition(g)
	if err == nil {
		t.Fatal("expected error for duplicate cell ids")
	}
}

func TestValidateEdgeUnknownCell(t *testing.T) {
	g := &GameDefinition{
		ID:    "test",
		Title: "Test",
		Board: Board{
			CellSize: 96,
			Cells:    []CellDefinition{{ID: "c1", Type: "start"}},
			Edges:    []EdgeDefinition{{ID: "e1", From: "c1", To: "c2"}},
		},
		Rules: RuleSet{
			CellTypes: map[string]CellTypeDef{
				"start": {Title: "Start"},
			},
		},
	}
	err := ValidateDefinition(g)
	if err == nil {
		t.Fatal("expected error for unknown cell in edge")
	}
}

func TestValidateCellSizeZero(t *testing.T) {
	g := &GameDefinition{
		ID:    "test",
		Title: "Test",
		Board: Board{
			CellSize: 0,
			Cells:    []CellDefinition{{ID: "c1", Type: "start"}},
		},
		Rules: RuleSet{
			CellTypes: map[string]CellTypeDef{
				"start": {Title: "Start"},
			},
		},
	}
	err := ValidateDefinition(g)
	if err == nil {
		t.Fatal("expected error for cellSize = 0")
	}
}

func TestValidateValidGame(t *testing.T) {
	g := &GameDefinition{
		ID:    "test",
		Title: "Test",
		Board: Board{
			CellSize: 96,
			Cells: []CellDefinition{
				{ID: "c1", Type: "start"},
				{ID: "c2", Type: "empty"},
			},
			Edges: []EdgeDefinition{
				{ID: "e1", From: "c1", To: "c2"},
			},
		},
		Rules: RuleSet{
			Dice: DiceRule{Count: 1, Sides: 6},
			CellTypes: map[string]CellTypeDef{
				"start": {Title: "Start"},
				"empty": {Title: "Empty"},
			},
		},
	}
	if err := ValidateDefinition(g); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateReservesMiniGameActionUntilRunnerExists(t *testing.T) {
	g := &GameDefinition{
		ID: "test", Title: "Mini-game boundary",
		Board: Board{Width: 96, Height: 96, CellSize: 96, Cells: []CellDefinition{{
			ID: "start", Type: "start", OnLand: []ActionDefinition{{
				Type: "launch_minigame", MiniGame: &MiniGameReference{ModuleID: "dice-duel", Version: 1},
			}},
		}}},
		Rules: RuleSet{Dice: DiceRule{Count: 1, Sides: 6}, CellTypes: map[string]CellTypeDef{"start": {Title: "Start"}}},
	}

	err := ValidateDefinition(g)
	if err == nil || !strings.Contains(err.Error(), "mini-game modules are not enabled") {
		t.Fatalf("ValidateDefinition() error = %v, want explicit disabled mini-game error", err)
	}
}

// definitionWithAction wraps one action in an otherwise valid board, so a
// validation test is about the action and nothing else.
func definitionWithAction(action ActionDefinition) *GameDefinition {
	definition := propertyDefinition()
	definition.Board.Cells[1].OnLand = []ActionDefinition{action}
	return definition
}

func TestValidateRejectsAQueryThatCanNeverMatch(t *testing.T) {
	cases := map[string]ActionDefinition{
		"no query at all": {Type: "if_cells_ge", Amount: intPtr(1)},
		"unknown owner":   {Type: "if_cells_ge", Query: &CellQuery{Owner: "landlord"}},
		"unknown type":    {Type: "for_each_cell", Query: &CellQuery{Type: "hotel"}},
		"value with no field": {
			Type: "for_each_cell", Query: &CellQuery{Value: "blue"},
		},
		"same-as-cell with no field": {
			Type: "if_cells_ge", Query: &CellQuery{SameAsCell: true},
		},
	}
	for name, action := range cases {
		t.Run(name, func(t *testing.T) {
			// A broken filter simply matches nothing at run time, so rent
			// silently comes to zero. Publication is the only place it can
			// still be caught.
			if err := ValidateDefinition(definitionWithAction(action)); err == nil {
				t.Fatal("published a query that can never match")
			}
		})
	}
}

func TestValidateAcceptsAWorkingQuery(t *testing.T) {
	action := ActionDefinition{
		Type:  "if_cells_ge",
		Query: &CellQuery{Type: "property", Field: "group", SameAsCell: true, Owner: "current"},
		Formula: &AmountFormula{
			Base: &AmountTerm{Kind: "cells", Query: &CellQuery{Field: "group", SameAsCell: true}},
		},
		Then: []ActionDefinition{{Type: "log_message", Title: "monopoly"}},
	}
	if err := ValidateDefinition(definitionWithAction(action)); err != nil {
		t.Fatalf("a valid query was rejected: %v", err)
	}
}

func TestValidateAuctionNeedsACurrencyAndAPrize(t *testing.T) {
	cases := map[string]ActionDefinition{
		"no currency": {Type: "start_auction", Then: []ActionDefinition{{Type: "set_cell_owner", Target: "current"}}},
		"unknown currency": {
			Type: "start_auction", Resource: "credits",
			Then: []ActionDefinition{{Type: "set_cell_owner", Target: "current"}},
		},
		// An auction with no follow-up takes the winning bid and hands back
		// nothing, which is never what the author meant.
		"no prize": {Type: "start_auction", Resource: "money"},
	}
	for name, action := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ValidateDefinition(definitionWithAction(action)); err == nil {
				t.Fatal("published a broken auction")
			}
		})
	}

	good := ActionDefinition{
		Type: "start_auction", Resource: "money", Amount: intPtr(50), Increment: intPtr(10),
		Then: []ActionDefinition{{Type: "set_cell_owner", Target: "current"}},
	}
	if err := ValidateDefinition(definitionWithAction(good)); err != nil {
		t.Fatalf("a valid auction was rejected: %v", err)
	}
}
