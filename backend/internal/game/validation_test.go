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
