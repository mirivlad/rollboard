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

func TestValidateRejectsFormulasThatCanOnlyBeWrong(t *testing.T) {
	// resolveTerm answers zero for anything it does not understand, so every
	// one of these used to publish cleanly and then quietly do nothing.
	cases := map[string]struct {
		action ActionDefinition
		want   string
	}{
		"unknown kind": {
			ActionDefinition{Type: "gain_resource", Resource: "money", Formula: &AmountFormula{Base: &AmountTerm{Kind: "wobble"}}},
			"not a kind of value",
		},
		"unknown resource": {
			ActionDefinition{Type: "gain_resource", Resource: "money", Formula: &AmountFormula{Base: &AmountTerm{Kind: "stat", Name: "charisma"}}},
			`no resource "charisma"`,
		},
		"nameless stat": {
			ActionDefinition{Type: "gain_resource", Resource: "money", Formula: &AmountFormula{Base: &AmountTerm{Kind: "stat"}}},
			"needs a name",
		},
		"unknown field": {
			ActionDefinition{Type: "gain_resource", Resource: "money", Formula: &AmountFormula{Base: &AmountTerm{Kind: "field", Name: "rnet"}}},
			`field "rnet"`,
		},
		"cells without a query": {
			ActionDefinition{Type: "gain_resource", Resource: "money", Formula: &AmountFormula{Base: &AmountTerm{Kind: "cells"}}},
			"needs a query",
		},
		"query inside a formula": {
			ActionDefinition{Type: "gain_resource", Resource: "money", Formula: &AmountFormula{
				Base: &AmountTerm{Kind: "cells", Query: &CellQuery{Type: "railrod"}}}},
			`unknown cell type "railrod"`,
		},
		"impossible clamps": {
			ActionDefinition{Type: "gain_resource", Resource: "money", Formula: &AmountFormula{
				Base: &AmountTerm{Kind: "const", Value: 5}, Min: intPtr(100), Max: intPtr(10)}},
			"can never hold",
		},
		"always negative": {
			ActionDefinition{Type: "gain_resource", Resource: "money", Formula: &AmountFormula{
				Base: &AmountTerm{Kind: "const", Value: 5}, Minus: &AmountTerm{Kind: "const", Value: 15}}},
			"cannot be negative",
		},
		"undeclared resource": {
			ActionDefinition{Type: "gain_resource", Resource: "credits", Amount: intPtr(5)},
			`no resource "credits"`,
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateDefinition(definitionWithAction(testCase.action))
			if err == nil {
				t.Fatal("published a formula that can only be wrong")
			}
			joined := strings.Join(err.Errors, "; ")
			if !strings.Contains(joined, testCase.want) {
				t.Fatalf("errors = %q, want something mentioning %q", joined, testCase.want)
			}
			// The author has to be able to find the action being complained
			// about; "unknown cell type" on its own names nothing.
			if !strings.Contains(joined, "cell 'blue_a' onLand action[0]") {
				t.Fatalf("error gives no path to the action: %q", joined)
			}
		})
	}
}

func TestValidateNamesThePathIntoNestedActions(t *testing.T) {
	action := ActionDefinition{
		Type: "if_cell_unowned",
		Then: []ActionDefinition{{
			Type: "offer_choice", Title: "Pick",
			Options: []ActionOption{{ID: "a", Title: "A", Then: []ActionDefinition{
				{Type: "gain_resource", Resource: "money", Formula: &AmountFormula{Times: &AmountTerm{Kind: "cells", Query: &CellQuery{Owner: "landlord"}}}},
			}}},
		}},
	}
	err := ValidateDefinition(definitionWithAction(action))
	if err == nil {
		t.Fatal("a broken formula three levels down was published")
	}
	want := "cell 'blue_a' onLand action[0].then[0].options[0].then[0].formula.times.query"
	if !strings.Contains(strings.Join(err.Errors, "; "), want) {
		t.Fatalf("errors = %v, want a path like %q", err.Errors, want)
	}
}

func TestValidateRejectsDuplicateOptionIDs(t *testing.T) {
	// The editor named options after the length of the list, so adding,
	// removing and adding again produced two options with one ID — and the
	// engine only ever resolves the first.
	action := ActionDefinition{
		Type: "offer_choice", Title: "Pick",
		Options: []ActionOption{
			{ID: "option_1", Title: "First"},
			{ID: "option_3", Title: "Second"},
			{ID: "option_3", Title: "Third"},
		},
	}
	err := ValidateDefinition(definitionWithAction(action))
	if err == nil || !strings.Contains(strings.Join(err.Errors, "; "), "used twice") {
		t.Fatalf("duplicate option ids were accepted: %v", err)
	}
}

func TestValidateAcceptsAComputedNumberOfTurns(t *testing.T) {
	// The editor offered a formula for skip_turns and the validator refused it,
	// so the control was there and could not be used.
	action := ActionDefinition{Type: "skip_turns", Formula: &AmountFormula{Base: &AmountTerm{Kind: "const", Value: 2}}}
	if err := ValidateDefinition(definitionWithAction(action)); err != nil {
		t.Fatalf("a computed number of turns was rejected: %v", err)
	}
}

func TestValidateRejectsRecipientsTheEngineCannotResolve(t *testing.T) {
	for _, target := range []string{"", "current", "bank", "none"} {
		action := ActionDefinition{Type: "transfer_resource", Resource: "money", Target: target, Amount: intPtr(10)}
		if err := ValidateDefinition(definitionWithAction(action)); err == nil {
			t.Fatalf("transfer to %q was published, and would have done nothing at all", target)
		}
	}
	good := ActionDefinition{Type: "transfer_resource", Resource: "money", Target: "owner", Amount: intPtr(10)}
	if err := ValidateDefinition(definitionWithAction(good)); err != nil {
		t.Fatalf("paying the cell owner was rejected: %v", err)
	}
}

func TestValidateRejectsImpossibleResourceRules(t *testing.T) {
	definition := propertyDefinition()
	min, max := 200, 100
	definition.Rules.Resources["money"] = ResourceRule{Initial: 500, Min: &min, Max: &max}
	if err := ValidateDefinition(definition); err == nil {
		t.Fatal("a resource whose minimum is above its maximum was published")
	}
}
