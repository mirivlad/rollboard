package game

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

func generateID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func StartSession(gameDef *GameDefinition, players []PlayerConfig) *GameSession {
	if len(players) < 2 {
		players = []PlayerConfig{
			{Name: "Player 1", Color: "#e74c3c"},
			{Name: "Player 2", Color: "#3498db"},
		}
	}

	var startCell *CellDefinition
	for _, c := range gameDef.Board.Cells {
		if c.Type == "start" {
			startCell = &c
			break
		}
	}

	playerStates := make([]PlayerState, len(players))
	for i, cfg := range players {
		resources := make(map[string]int)
		for name, rule := range gameDef.Rules.Resources {
			resources[name] = rule.Initial
		}
		playerStates[i] = PlayerState{
			ID:             fmt.Sprintf("player_%d", i+1),
			Name:           cfg.Name,
			Color:          cfg.Color,
			PositionCellID: startCell.ID,
			Resources:      resources,
			Bankrupt:       false,
		}
	}

	cellStates := make(map[string]CellState)
	for _, c := range gameDef.Board.Cells {
		cellStates[c.ID] = CellState{}
	}

	log := []GameEvent{
		NewGameEvent("game_start", fmt.Sprintf("Game started with %d players", len(players)), nil),
	}

	return &GameSession{
		ID:          generateID(),
		GameID:      gameDef.ID,
		GameVersion: gameDef.Version,
		Mode:        "hotseat",
		Definition:  gameDef,
		State: GameState{
			CurrentPlayerIndex: 0,
			Players:            playerStates,
			CellStates:         cellStates,
			TurnNumber:         1,
			RoundNumber:        1,
			Status:             "active",
			Log:                log,
		},
	}
}

type RollResult struct {
	Rolls []int `json:"rolls"`
	Total int   `json:"total"`
}

func (s *GameSession) RollDice() (*RollResult, *GameEvent) {
	dice := s.Definition.Rules.Dice
	rolls := make([]int, dice.Count)
	total := 0
	for i := 0; i < dice.Count; i++ {
		rolls[i] = rand.Intn(dice.Sides) + 1
		total += rolls[i]
	}

	evt := NewGameEvent("dice_roll",
		fmt.Sprintf("%s rolled %d (dice: %s)",
			s.State.Players[s.State.CurrentPlayerIndex].Name,
			total,
			joinInts(rolls, "+")),
		map[string]any{"rolls": rolls, "total": total, "playerId": s.State.Players[s.State.CurrentPlayerIndex].ID})

	return &RollResult{Rolls: rolls, Total: total}, &evt
}

func (s *GameSession) MoveCurrentPlayer(steps int) []GameEvent {
	var events []GameEvent
	player := &s.State.Players[s.State.CurrentPlayerIndex]

	if player.Bankrupt {
		s.advanceTurn()
		return nil
	}

	edgeMap := s.Definition.Board.buildEdgeMap()

	passedStart := false
	pathCells := []*CellDefinition{}

	currentCell := s.Definition.Board.getCellByID(player.PositionCellID)
	if currentCell == nil {
		return events
	}

	for step := 0; step < steps; step++ {
		edges := edgeMap[currentCell.ID]
		if len(edges) == 0 {
			break
		}

		var nextEdge *EdgeDefinition
		for _, e := range edges {
			if e.Condition.Type == "always" || e.Condition.Type == "" {
				nextEdge = &e
				break
			}
		}
		if nextEdge == nil {
			nextEdge = &edges[0]
		}

		nextCell := s.Definition.Board.getCellByID(nextEdge.To)
		if nextCell == nil {
			break
		}

		pathCells = append(pathCells, nextCell)
		currentCell = nextCell
	}

	finalCell := currentCell

	// Check for start pass-through (not starting position)
	for _, pc := range pathCells[:len(pathCells)-1] {
		if pc != nil && pc.Type == "start" && pc.ID != player.PositionCellID {
			passedStart = true
		}
	}

	oldPos := player.PositionCellID
	player.PositionCellID = finalCell.ID

	pathIDs := make([]string, len(pathCells))
	for i, pc := range pathCells {
		pathIDs[i] = pc.ID
	}

	events = append(events, NewGameEvent("move",
		fmt.Sprintf("%s moved from %s to %s",
			player.Name, oldPos, finalCell.ID),
		map[string]any{
			"from":     oldPos,
			"to":       finalCell.ID,
			"path":     pathIDs,
			"playerId": player.ID,
		}))

	// Start pass-through bonus
	if passedStart {
		bonus := s.Definition.Rules.StartBonus
		res := s.Definition.Rules.StartBonusResource
		if res != "" && bonus != 0 {
			player.Resources[res] += bonus
			events = append(events, NewGameEvent("start_bonus",
				fmt.Sprintf("%s passed START and received %d %s", player.Name, bonus, res), nil))
		}
	}

	// Execute OnPass actions for each passed cell (except final)
	for _, pc := range pathCells[:len(pathCells)-1] {
		if pc != nil && len(pc.OnPass) > 0 {
			passEvents := s.executeActions(pc.OnPass, player, pc)
			events = append(events, passEvents...)
		}
	}

	// Execute OnLand actions for final cell
	if finalCell != nil && len(finalCell.OnLand) > 0 {
		landEvents := s.executeActions(finalCell.OnLand, player, finalCell)
		events = append(events, landEvents...)
	}

	// Advance turn if no pending action was set
	if s.State.PendingAction == nil {
		s.advanceTurn()
	}

	return events
}

// executeActions runs a list of action definitions and returns events.
// If a pending action is set during execution, it stops processing further actions.
func (s *GameSession) executeActions(actions []ActionDefinition, player *PlayerState, cell *CellDefinition) []GameEvent {
	var events []GameEvent
	for _, a := range actions {
		if s.State.PendingAction != nil {
			break
		}
		actionEvents := s.executeOneAction(a, player, cell)
		events = append(events, actionEvents...)
	}
	return events
}

func (s *GameSession) executeOneAction(a ActionDefinition, player *PlayerState, cell *CellDefinition) []GameEvent {
	switch a.Type {
	case "log_message":
		msg := a.Title
		if msg == "" {
			msg = fmt.Sprintf("%s: cell %s", player.Name, cell.Title)
		}
		return []GameEvent{NewGameEvent("log_message", msg, nil)}

	case "gain_resource":
		res := a.Resource
		amount := resolveAmount(a, cell.Fields)
		if res == "" {
			return nil
		}
		player.Resources[res] += amount
		return []GameEvent{NewGameEvent("gain_resource",
			fmt.Sprintf("%s gained %d %s", player.Name, amount, res), nil)}

	case "lose_resource":
		res := a.Resource
		amount := resolveAmount(a, cell.Fields)
		if res == "" {
			return nil
		}
		if player.Resources[res] > amount {
			player.Resources[res] -= amount
		} else {
			player.Resources[res] = 0
		}
		return []GameEvent{NewGameEvent("lose_resource",
			fmt.Sprintf("%s lost %d %s", player.Name, amount, res), nil)}

	case "transfer_resource":
		res := a.Resource
		amount := resolveAmount(a, cell.Fields)
		if res == "" {
			return nil
		}
		var targetPlayer *PlayerState
		switch a.Target {
		case "owner":
			cs := s.State.CellStates[cell.ID]
			if cs.OwnerPlayerID != "" {
				targetPlayer = s.getPlayerByID(cs.OwnerPlayerID)
			}
		default:
			targetPlayer = s.getPlayerByID(a.Target)
		}
		if targetPlayer == nil || targetPlayer.Bankrupt {
			return nil
		}

		actual := amount
		if player.Resources[res] < actual {
			actual = player.Resources[res]
		}
		player.Resources[res] -= actual
		targetPlayer.Resources[res] += actual
		return []GameEvent{NewGameEvent("transfer_resource",
			fmt.Sprintf("%s paid %d %s to %s", player.Name, actual, res, targetPlayer.Name), nil)}

	case "set_cell_owner":
		switch a.Target {
		case "current":
			s.State.CellStates[cell.ID] = CellState{OwnerPlayerID: player.ID}
		case "none":
			s.State.CellStates[cell.ID] = CellState{}
		default:
			s.State.CellStates[cell.ID] = CellState{OwnerPlayerID: a.Target}
		}
		return []GameEvent{NewGameEvent("set_cell_owner",
			fmt.Sprintf("%s now owns %s", player.Name, cell.Title), nil)}

	case "offer_choice":
		s.State.PendingAction = &PendingAction{
			Type:     "choice",
			PlayerID: player.ID,
			Title:    a.Title,
			CellID:   cell.ID,
			Options:  a.Options,
		}
		return []GameEvent{NewGameEvent("choice_offered",
			fmt.Sprintf("%s: %s", player.Name, a.Title), nil)}

	case "finish_game":
		s.State.Status = "finished"
		s.State.WinnerPlayerID = player.ID
		return []GameEvent{NewGameEvent("game_over",
			fmt.Sprintf("%s wins the game!", player.Name), nil)}

	case "if_cell_unowned":
		cs := s.State.CellStates[cell.ID]
		if cs.OwnerPlayerID == "" {
			return s.executeActions(a.Then, player, cell)
		}
		return s.executeActions(a.Else, player, cell)

	case "if_cell_owned_by_current":
		cs := s.State.CellStates[cell.ID]
		if cs.OwnerPlayerID == player.ID {
			return s.executeActions(a.Then, player, cell)
		}
		return s.executeActions(a.Else, player, cell)

	case "if_cell_owned_by_other":
		cs := s.State.CellStates[cell.ID]
		if cs.OwnerPlayerID != "" && cs.OwnerPlayerID != player.ID {
			return s.executeActions(a.Then, player, cell)
		}
		return s.executeActions(a.Else, player, cell)

	case "if_resource_ge":
		amount := resolveAmount(a, cell.Fields)
		if player.Resources[a.Resource] >= amount {
			return s.executeActions(a.Then, player, cell)
		}
		return s.executeActions(a.Else, player, cell)

	default:
		return []GameEvent{NewGameEvent("unknown_action",
			fmt.Sprintf("Unknown action type: %s", a.Type), nil)}
	}
}

// ResolvePendingAction executes the chosen option's Then actions.
func (s *GameSession) ResolvePendingAction(actionID string) ([]GameEvent, error) {
	if s.State.PendingAction == nil {
		return nil, fmt.Errorf("no pending action")
	}

	var chosen *ActionOption
	for i := range s.State.PendingAction.Options {
		if s.State.PendingAction.Options[i].ID == actionID {
			chosen = &s.State.PendingAction.Options[i]
			break
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("unknown option: %s", actionID)
	}

	player := s.getPlayerByID(s.State.PendingAction.PlayerID)
	if player == nil {
		return nil, fmt.Errorf("player not found")
	}

	cell := s.Definition.Board.getCellByID(s.State.PendingAction.CellID)

	s.State.PendingAction = nil

	events := s.executeActions(chosen.Then, player, cell)

	s.advanceTurn()
	return events, nil
}

func (s *GameSession) advanceTurn() {
	activePlayers := 0
	var lastActive string
	for _, p := range s.State.Players {
		if !p.Bankrupt {
			activePlayers++
			lastActive = p.ID
		}
	}
	if activePlayers <= 1 {
		s.State.Status = "finished"
		s.State.WinnerPlayerID = lastActive
		s.State.Log = append(s.State.Log, NewGameEvent("game_over",
			fmt.Sprintf("%s wins the game!", s.getPlayerByID(lastActive).Name), nil))
		return
	}

	playerCount := len(s.State.Players)
	nextIdx := (s.State.CurrentPlayerIndex + 1) % playerCount
	for s.State.Players[nextIdx].Bankrupt {
		nextIdx = (nextIdx + 1) % playerCount
	}

	if nextIdx <= s.State.CurrentPlayerIndex {
		s.State.RoundNumber++
	}
	s.State.CurrentPlayerIndex = nextIdx
	s.State.TurnNumber++
}

func (s *GameSession) CurrentPlayer() *PlayerState {
	return &s.State.Players[s.State.CurrentPlayerIndex]
}

// getPlayerByID finds a player by ID.
func (s *GameSession) getPlayerByID(id string) *PlayerState {
	for i := range s.State.Players {
		if s.State.Players[i].ID == id {
			return &s.State.Players[i]
		}
	}
	return nil
}

func resolveAmount(a ActionDefinition, fields map[string]any) int {
	if a.Amount != nil {
		return *a.Amount
	}
	if a.AmountField != "" && fields != nil {
		return getIntField(fields, a.AmountField, 0)
	}
	return 0
}

func getIntField(fields map[string]any, key string, defaultVal int) int {
	if fields == nil {
		return defaultVal
	}
	v, ok := fields[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return defaultVal
	}
}

func joinInts(vals []int, sep string) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, sep)
}

type EdgeMap = map[string][]EdgeDefinition

func (b *Board) getCellByID(id string) *CellDefinition {
	for i := range b.Cells {
		if b.Cells[i].ID == id {
			return &b.Cells[i]
		}
	}
	return nil
}

func (b *Board) buildEdgeMap() map[string][]EdgeDefinition {
	m := make(map[string][]EdgeDefinition)
	for _, e := range b.Edges {
		m[e.From] = append(m[e.From], e)
	}
	for k := range m {
		sort.Slice(m[k], func(i, j int) bool {
			return m[k][i].ID < m[k][j].ID
		})
	}
	return m
}
