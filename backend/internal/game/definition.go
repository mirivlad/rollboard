package game

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type GameDefinition struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Version int     `json:"version"`
	Board   Board   `json:"board"`
	Rules   RuleSet `json:"rules"`
}

type Board struct {
	Width    int              `json:"width"`
	Height   int              `json:"height"`
	CellSize int              `json:"cellSize"`
	Cells    []CellDefinition `json:"cells"`
	Edges    []EdgeDefinition `json:"edges"`
}

type CellDefinition struct {
	ID     string             `json:"id"`
	Title  string             `json:"title"`
	Type   string             `json:"type"`
	X      int                `json:"x"`
	Y      int                `json:"y"`
	Visual CellVisual         `json:"visual"`
	Fields map[string]any     `json:"fields"`
	OnLand []ActionDefinition `json:"onLand,omitempty"`
	OnPass []ActionDefinition `json:"onPass,omitempty"`
}

type CellVisual struct {
	BaseColor string `json:"baseColor"`
	BaseImage string `json:"baseImage"`
}

type EdgeDefinition struct {
	ID        string        `json:"id"`
	From      string        `json:"from"`
	To        string        `json:"to"`
	Condition EdgeCondition `json:"condition"`
}

type EdgeCondition struct {
	Type     string `json:"type"`
	Values   []int  `json:"values,omitempty"`
	Resource string `json:"resource,omitempty"`
	Amount   *int   `json:"amount,omitempty"`
	Label    string `json:"label,omitempty"`
}

type RuleSet struct {
	Dice               DiceRule                `json:"dice"`
	Resources          map[string]ResourceRule `json:"resources"`
	CellTypes          map[string]CellTypeDef  `json:"cellTypes"`
	StartBonus         int                     `json:"startBonus"`
	StartBonusResource string                  `json:"startBonusResource"`

	// Items are the definition's catalogue of things a player can carry. A
	// resource is a number; an item is a named thing with its own effects,
	// which is what a sword has to be and a counter cannot.
	Items map[string]ItemDef `json:"items,omitempty"`
	// EquipmentSlots names the places an item can be worn, in display order.
	// One item per slot.
	EquipmentSlots []string `json:"equipmentSlots,omitempty"`
	// Progression turns an experience counter into levels and spendable
	// points. Levels are common enough across games to belong in the rules
	// rather than being rebuilt from conditionals in every definition.
	Progression *ProgressionRule `json:"progression,omitempty"`
	// Movement selects how a roll is spent. "path" (the default) walks the
	// graph edge by edge; "free" lets the player choose any cell within range.
	Movement string `json:"movement,omitempty"`
	// HiddenCells starts every cell face down. Landing on one turns it over,
	// and a reveal_cells action can turn over more.
	HiddenCells bool `json:"hiddenCells,omitempty"`
}

// ProgressionRule describes how experience becomes levels.
type ProgressionRule struct {
	ExperienceResource string `json:"experienceResource"`
	LevelResource      string `json:"levelResource"`
	// PointsResource receives PointsPerLevel on each level gained.
	PointsResource string `json:"pointsResource"`
	PointsPerLevel int    `json:"pointsPerLevel"`
	// Thresholds[i] is the total experience needed to reach level i+2. Running
	// past the end of the list simply means the maximum level is reached.
	Thresholds []int `json:"thresholds"`
}

// ItemDef describes one kind of item.
type ItemDef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Slot, when set, names the equipment slot this item occupies. An item
	// with no slot is carried but never worn: a potion, a key, a quest token.
	Slot string `json:"slot,omitempty"`
	// Bonuses apply to the player's effective stats while the item is
	// equipped. They are deliberately not applied while it merely sits in the
	// pack, so equipping is a decision rather than a formality.
	Bonuses map[string]int `json:"bonuses,omitempty"`
	// Consumable items are destroyed when used.
	Consumable bool `json:"consumable,omitempty"`
	// Use runs when a consumable is used.
	Use []ActionDefinition `json:"use,omitempty"`
}

type DiceRule struct {
	Count int `json:"count"`
	Sides int `json:"sides"`
}

type ResourceRule struct {
	Initial int  `json:"initial"`
	Min     *int `json:"min,omitempty"`
	Max     *int `json:"max,omitempty"`
}

type CellTypeDef struct {
	Title  string              `json:"title"`
	Fields map[string]FieldDef `json:"fields"`
}

type FieldDef struct {
	Type    string   `json:"type"`
	Label   string   `json:"label"`
	Default any      `json:"default,omitempty"`
	Options []string `json:"options,omitempty"`
}

type ActionDefinition struct {
	Type        string `json:"type"`
	Resource    string `json:"resource,omitempty"`
	Amount      *int   `json:"amount,omitempty"`
	AmountField string `json:"amountField,omitempty"`
	// Formula computes the amount instead of taking it literally. Actions
	// could previously only compare values, never combine them, so armour
	// could raise a defence stat that nothing was able to subtract.
	Formula  *AmountFormula     `json:"formula,omitempty"`
	Target   string             `json:"target,omitempty"`
	To       string             `json:"to,omitempty"`
	Field    string             `json:"field,omitempty"`
	Title    string             `json:"title,omitempty"`
	ActionID string             `json:"actionId,omitempty"`
	MiniGame *MiniGameReference `json:"miniGame,omitempty"`
	// Query selects other cells on the board, for the actions that ask about
	// more than the square they are attached to.
	Query *CellQuery `json:"query,omitempty"`
	// Increment is the smallest raise in an auction. Blank means a tenth of
	// the opening bid, so an author who fills in nothing still gets sensible
	// bidding steps.
	Increment *int               `json:"increment,omitempty"`
	Then      []ActionDefinition `json:"then,omitempty"`
	Else      []ActionDefinition `json:"else,omitempty"`
	Options   []ActionOption     `json:"options,omitempty"`
}

// AmountFormula is deliberately a fixed shape rather than a general
// expression tree: base, one adjustment, an optional scale and clamps. That
// covers "damage minus defence, at least zero" and stays a handful of
// dropdowns in the editor, which is the point — authors do not write code.
type AmountFormula struct {
	Base  *AmountTerm `json:"base,omitempty"`
	Plus  *AmountTerm `json:"plus,omitempty"`
	Minus *AmountTerm `json:"minus,omitempty"`
	// Times and DividedBy are terms rather than plain numbers, which is what
	// lets rent scale with a count: "the rent field, times the number of
	// stations this owner holds". A bare number still decodes, so definitions
	// written before this read back unchanged.
	Times     *AmountTerm `json:"times,omitempty"`
	DividedBy *AmountTerm `json:"dividedBy,omitempty"`
	// Min and Max stay literal. A clamp is a rule of the game ("never below
	// zero"), not a quantity to compute, and keeping them numbers keeps the
	// editor short.
	Min *int `json:"min,omitempty"`
	Max *int `json:"max,omitempty"`
}

// AmountTerm is one value in a formula.
//
//	const    a literal
//	field    a field on the cell being resolved
//	stat     the acting player's effective value, equipment included
//	resource the acting player's stored value, equipment excluded
//	cells    how many cells match Query
type AmountTerm struct {
	Kind  string     `json:"kind"`
	Name  string     `json:"name,omitempty"`
	Value int        `json:"value,omitempty"`
	Query *CellQuery `json:"query,omitempty"`
}

// UnmarshalJSON accepts a bare number as shorthand for a constant.
//
// Times and DividedBy used to be plain integers. Every definition already
// published writes "times": 2, and a stored game that no longer loads is a
// destroyed game, so the shorthand is part of the format rather than a
// migration.
func (t *AmountTerm) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] != '{' {
		var n int
		if err := json.Unmarshal(trimmed, &n); err != nil {
			return fmt.Errorf("amount term: expected an object or a number: %w", err)
		}
		*t = AmountTerm{Kind: "const", Value: n}
		return nil
	}
	type termAlias AmountTerm
	var alias termAlias
	if err := json.Unmarshal(trimmed, &alias); err != nil {
		return err
	}
	*t = AmountTerm(alias)
	return nil
}

type ActionOption struct {
	ID    string             `json:"id"`
	Title string             `json:"title"`
	Then  []ActionDefinition `json:"then,omitempty"`
}

type PlayerConfig struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}
