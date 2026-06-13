package game

type GameDefinition struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	Version int         `json:"version"`
	Board   Board       `json:"board"`
	Rules   RuleSet     `json:"rules"`
}

type Board struct {
	Width    int              `json:"width"`
	Height   int              `json:"height"`
	CellSize int              `json:"cellSize"`
	Cells    []CellDefinition `json:"cells"`
	Edges    []EdgeDefinition `json:"edges"`
}

type CellDefinition struct {
	ID      string            `json:"id"`
	Title   string            `json:"title"`
	Type    string            `json:"type"`
	X       int               `json:"x"`
	Y       int               `json:"y"`
	Visual  CellVisual        `json:"visual"`
	Fields  map[string]any    `json:"fields"`
	OnLand  []ActionDefinition `json:"onLand,omitempty"`
	OnPass  []ActionDefinition `json:"onPass,omitempty"`
}

type CellVisual struct {
	BaseColor string `json:"baseColor"`
	BaseImage string `json:"baseImage"`
}

type EdgeDefinition struct {
	ID        string         `json:"id"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Condition EdgeCondition  `json:"condition"`
}

type EdgeCondition struct {
	Type     string `json:"type"`
	Values   []int  `json:"values,omitempty"`
	Resource string `json:"resource,omitempty"`
	Amount   *int   `json:"amount,omitempty"`
}

type RuleSet struct {
	Dice      DiceRule                   `json:"dice"`
	Resources map[string]ResourceRule    `json:"resources"`
	CellTypes map[string]CellTypeDef     `json:"cellTypes"`
	StartBonus         int    `json:"startBonus"`
	StartBonusResource string `json:"startBonusResource"`
}

type DiceRule struct {
	Count  int `json:"count"`
	Sides  int `json:"sides"`
}

type ResourceRule struct {
	Initial int  `json:"initial"`
	Min     *int `json:"min,omitempty"`
	Max     *int `json:"max,omitempty"`
}

type CellTypeDef struct {
	Title  string                    `json:"title"`
	Fields map[string]FieldDef       `json:"fields"`
}

type FieldDef struct {
	Type    string   `json:"type"`
	Label   string   `json:"label"`
	Default any      `json:"default,omitempty"`
	Options []string `json:"options,omitempty"`
}

type ActionDefinition struct {
	Type        string             `json:"type"`
	Resource    string             `json:"resource,omitempty"`
	Amount      *int               `json:"amount,omitempty"`
	AmountField string             `json:"amountField,omitempty"`
	Target      string             `json:"target,omitempty"`
	To          string             `json:"to,omitempty"`
	Field       string             `json:"field,omitempty"`
	Title       string             `json:"title,omitempty"`
	ActionID    string             `json:"actionId,omitempty"`
	Then        []ActionDefinition `json:"then,omitempty"`
	Else        []ActionDefinition `json:"else,omitempty"`
	Options     []ActionOption     `json:"options,omitempty"`
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
