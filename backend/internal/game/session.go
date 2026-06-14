package game

import "time"

type GameSession struct {
	ID          string          `json:"id"`
	GameID      string          `json:"gameId"`
	GameVersion int             `json:"gameVersion"`
	Mode        string          `json:"mode"`
	Definition  *GameDefinition `json:"definition"`
	State       GameState       `json:"state"`
}

type PendingMovement struct {
	PlayerID       string   `json:"playerId"`
	CurrentCellID  string   `json:"currentCellId"`
	RemainingSteps int      `json:"remainingSteps"`
	Dice           []int    `json:"dice"`
	Total          int      `json:"total"`
	PathSoFar      []string `json:"pathSoFar"`
}

type GameState struct {
	CurrentPlayerIndex int                  `json:"currentPlayerIndex"`
	Players            []PlayerState        `json:"players"`
	CellStates         map[string]CellState `json:"cellStates"`
	TurnNumber         int                  `json:"turnNumber"`
	RoundNumber        int                  `json:"roundNumber"`
	Status             string               `json:"status"`
	WinnerPlayerID     string               `json:"winnerPlayerId,omitempty"`
	Log                []GameEvent          `json:"log"`
	PendingAction      *PendingAction       `json:"pendingAction,omitempty"`
	PendingMovement    *PendingMovement     `json:"pendingMovement,omitempty"`
}

type PlayerState struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Color          string         `json:"color"`
	PositionCellID string         `json:"positionCellId"`
	Resources      map[string]int `json:"resources"`
	Bankrupt       bool           `json:"bankrupt"`
}

type CellState struct {
	OwnerPlayerID string `json:"ownerPlayerId,omitempty"`
	Mortgaged     bool   `json:"mortgaged,omitempty"`
	Level         int    `json:"level,omitempty"`
}

type GameEvent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
	Payload   any    `json:"payload,omitempty"`
}

type PendingAction struct {
	Type     string         `json:"type"`
	PlayerID string         `json:"playerId"`
	Title    string         `json:"title,omitempty"`
	CellID   string         `json:"cellId,omitempty"`
	Options  []ActionOption `json:"options,omitempty"`
}

func NewGameEvent(typ, msg string, payload any) GameEvent {
	return GameEvent{
		ID:        generateID(),
		Type:      typ,
		Message:   msg,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Payload:   payload,
	}
}
