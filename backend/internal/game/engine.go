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

	playerName := s.State.Players[s.State.CurrentPlayerIndex].Name
	msg := fmt.Sprintf("%s rolled %d", playerName, total)
	if len(rolls) > 1 {
		msg = fmt.Sprintf("%s rolled %s = %d", playerName, joinInts(rolls, " + "), total)
	}
	evt := NewGameEvent("dice_roll", msg,
		map[string]any{"rolls": rolls, "total": total, "playerId": s.State.Players[s.State.CurrentPlayerIndex].ID})

	return &RollResult{Rolls: rolls, Total: total}, &evt
}

// MoveContext contains information available for evaluating edge conditions
// during a movement step.
type MoveContext struct {
	PlayerID       string
	Dice           []int
	Total          int
	Step           int
	RemainingSteps int
}

// evaluateCondition checks whether an edge is available given the current context.
func (s *GameSession) evaluateCondition(e EdgeDefinition, ctx MoveContext) bool {
	cond := e.Condition
	switch cond.Type {
	case "always", "":
		return true
	case "dice_total_even":
		return ctx.Total%2 == 0
	case "dice_total_odd":
		return ctx.Total%2 != 0
	case "dice_total_in":
		for _, v := range cond.Values {
			if ctx.Total == v {
				return true
			}
		}
		return false
	case "player_resource_at_least":
		player := s.getPlayerByID(ctx.PlayerID)
		if player == nil {
			return false
		}
		amt := 0
		if cond.Amount != nil {
			amt = *cond.Amount
		}
		return player.Resources[cond.Resource] >= amt
	case "manual_choice":
		return true
	case "pay_resource":
		player := s.getPlayerByID(ctx.PlayerID)
		if player == nil {
			return false
		}
		amt := 0
		if cond.Amount != nil {
			amt = *cond.Amount
		}
		return player.Resources[cond.Resource] >= amt
	default:
		return true
	}
}

// MoveCurrentPlayer moves the current player step by step, evaluating edge
// conditions at each step. Returns movement events.
func (s *GameSession) MoveCurrentPlayer(steps int, diceRolls []int, diceTotal int) []GameEvent {
	return s.moveSteps(steps, diceRolls, diceTotal, s.State.Players[s.State.CurrentPlayerIndex].PositionCellID, nil)
}

// moveSteps is the core step-by-step movement loop. It evaluates edge conditions
// and pauses for route_choice when multiple manual-choice edges are available.
func (s *GameSession) moveSteps(steps int, diceRolls []int, diceTotal int, fromCellID string, existingEvents []GameEvent) []GameEvent {
	events := existingEvents
	if events == nil {
		events = []GameEvent{}
	}
	player := &s.State.Players[s.State.CurrentPlayerIndex]

	if player.Bankrupt {
		s.advanceTurn()
		return nil
	}

	if steps <= 0 {
		events = append(events, NewGameEvent("move",
			fmt.Sprintf("%s stayed in place (rolled %d)", player.Name, diceTotal),
			map[string]any{
				"from":     player.PositionCellID,
				"to":       player.PositionCellID,
				"path":     []string{},
				"playerId": player.ID,
				"dice":     diceRolls,
				"total":    diceTotal,
			}))
		if s.State.PendingAction == nil {
			s.advanceTurn()
		}
		return events
	}

	edgeMap := s.Definition.Board.buildEdgeMap()
	passedStart := false
	pathCells := []*CellDefinition{}

	currentCell := s.Definition.Board.getCellByID(fromCellID)
	if currentCell == nil {
		events = append(events, NewGameEvent("error",
			fmt.Sprintf("player %s position cell '%s' not found in board definition",
				player.Name, fromCellID), nil))
		return events
	}

	for step := 0; step < steps; step++ {
		edges := edgeMap[currentCell.ID]
		if len(edges) == 0 {
			events = append(events, NewGameEvent("move_blocked",
				fmt.Sprintf("%s has no path from %s (rolled %d, %d steps taken, %d remaining)",
					player.Name, currentCell.ID, diceTotal, step, steps-step), nil))
			break
		}

		ctx := MoveContext{
			PlayerID:       player.ID,
			Dice:           diceRolls,
			Total:          diceTotal,
			Step:           step,
			RemainingSteps: steps - step,
		}

		var available []EdgeDefinition
		for _, e := range edges {
			if s.evaluateCondition(e, ctx) {
				available = append(available, e)
			}
		}

		if len(available) == 0 {
			events = append(events, NewGameEvent("move_blocked",
				fmt.Sprintf("%s has no available path at %s (rolled %d, %d steps taken)",
					player.Name, currentCell.ID, diceTotal, step), nil))
			break
		}

		if len(available) == 1 {
			nextEdge := available[0]
			if nextEdge.Condition.Type == "pay_resource" {
				if nextEdge.Condition.Resource != "" && nextEdge.Condition.Amount != nil {
					player.Resources[nextEdge.Condition.Resource] -= *nextEdge.Condition.Amount
				}
			}
			nextCell := s.Definition.Board.getCellByID(nextEdge.To)
			if nextCell == nil {
				events = append(events, NewGameEvent("error",
					fmt.Sprintf("edge '%s' leads to unknown cell '%s'", nextEdge.ID, nextEdge.To), nil))
				break
			}
			pathCells = append(pathCells, nextCell)
			currentCell = nextCell
			continue
		}

		hasManualChoice := false
		for _, e := range available {
			if e.Condition.Type == "manual_choice" || e.Condition.Label != "" {
				hasManualChoice = true
				break
			}
		}

		if hasManualChoice {
			player.PositionCellID = currentCell.ID

			pathIDs := make([]string, len(pathCells))
			for i, pc := range pathCells {
				pathIDs[i] = pc.ID
			}

			options := make([]ActionOption, 0, len(available))
			for _, e := range available {
				label := e.Condition.Label
				if label == "" {
					label = fmt.Sprintf("%s → %s", e.From, e.To)
				}
				options = append(options, ActionOption{
					ID:    e.ID,
					Title: label,
				})
			}

			s.State.PendingAction = &PendingAction{
				Type:     "route_choice",
				PlayerID: player.ID,
				Title:    "Choose path",
				CellID:   currentCell.ID,
				Options:  options,
			}

			s.State.PendingMovement = &PendingMovement{
				PlayerID:       player.ID,
				CurrentCellID:  currentCell.ID,
				RemainingSteps: steps - step,
				Dice:           diceRolls,
				Total:          diceTotal,
				PathSoFar:      pathIDs,
			}

			return events
		}

		nextEdge := available[0]
		if nextEdge.Condition.Type == "pay_resource" {
			if nextEdge.Condition.Resource != "" && nextEdge.Condition.Amount != nil {
				player.Resources[nextEdge.Condition.Resource] -= *nextEdge.Condition.Amount
			}
		}
		events = append(events, NewGameEvent("move_ambiguous",
			fmt.Sprintf("Multiple paths from %s, selected %s → %s",
				currentCell.ID, nextEdge.From, nextEdge.To), nil))
		nextCell := s.Definition.Board.getCellByID(nextEdge.To)
		if nextCell == nil {
			events = append(events, NewGameEvent("error",
				fmt.Sprintf("edge '%s' leads to unknown cell '%s'", nextEdge.ID, nextEdge.To), nil))
			break
		}
		pathCells = append(pathCells, nextCell)
		currentCell = nextCell
	}

	finalCell := currentCell

	intermediateCells := pathCells
	if len(intermediateCells) > 1 {
		intermediateCells = pathCells[:len(pathCells)-1]
	}
	for _, pc := range intermediateCells {
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
		fmt.Sprintf("%s moved from %s to %s", player.Name, oldPos, finalCell.ID),
		map[string]any{
			"from":     oldPos,
			"to":       finalCell.ID,
			"path":     pathIDs,
			"playerId": player.ID,
			"dice":     diceRolls,
			"total":    diceTotal,
		}))

	if passedStart {
		bonus := s.Definition.Rules.StartBonus
		res := s.Definition.Rules.StartBonusResource
		if res != "" && bonus != 0 {
			player.Resources[res] += bonus
			events = append(events, NewGameEvent("start_bonus",
				fmt.Sprintf("%s passed START and received %d %s", player.Name, bonus, res), nil))
		}
	}

	passedCells := pathCells
	if len(passedCells) > 1 {
		passedCells = pathCells[:len(pathCells)-1]
	}
	for _, pc := range passedCells {
		if pc != nil && len(pc.OnPass) > 0 {
			passEvents := s.executeActions(pc.OnPass, player, pc)
			events = append(events, passEvents...)
		}
	}

	if finalCell != nil && len(finalCell.OnLand) > 0 {
		landEvents := s.executeActions(finalCell.OnLand, player, finalCell)
		events = append(events, landEvents...)
	}

	if s.State.PendingAction == nil {
		s.advanceTurn()
	}

	return events
}

// executeActions runs a list of action definitions and returns events.
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
		// Assigning an owner resets buildings and mortgage state, because a
		// property changing hands must not carry the previous owner's
		// investment with it.
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

	case "set_cell_level":
		// Building levels are generic: houses and hotels in a property game,
		// fortification in a war game, growth stages in a farming game.
		state := s.State.CellStates[cell.ID]
		level := resolveAmount(a, cell.Fields)
		if level < 0 {
			level = 0
		}
		state.Level = level
		s.State.CellStates[cell.ID] = state
		return []GameEvent{NewGameEvent("set_cell_level",
			fmt.Sprintf("%s set %s to level %d", player.Name, cell.Title, level), nil)}

	case "if_cell_level_ge":
		if s.State.CellStates[cell.ID].Level >= resolveAmount(a, cell.Fields) {
			return s.executeActions(a.Then, player, cell)
		}
		return s.executeActions(a.Else, player, cell)

	case "set_cell_mortgaged":
		state := s.State.CellStates[cell.ID]
		state.Mortgaged = a.Target != "false"
		s.State.CellStates[cell.ID] = state
		verb := "mortgaged"
		if !state.Mortgaged {
			verb = "redeemed"
		}
		return []GameEvent{NewGameEvent("set_cell_mortgaged",
			fmt.Sprintf("%s %s %s", player.Name, verb, cell.Title), nil)}

	case "if_cell_mortgaged":
		if s.State.CellStates[cell.ID].Mortgaged {
			return s.executeActions(a.Then, player, cell)
		}
		return s.executeActions(a.Else, player, cell)

	case "move_player_to":
		target := s.Definition.Board.getCellByID(a.To)
		if target == nil {
			return []GameEvent{NewGameEvent("invalid_action",
				fmt.Sprintf("Cannot move %s: no cell %q", player.Name, a.To), nil)}
		}
		player.PositionCellID = target.ID
		events := []GameEvent{NewGameEvent("move",
			fmt.Sprintf("%s moved to %s", player.Name, target.Title), map[string]any{
				"playerId": player.ID,
				"path":     []string{target.ID},
			})}
		// Actions on the destination run, otherwise "go to jail" would land a
		// player on a cell without its consequences. Recursion is bounded
		// because a teleport chain that returns to its own start would need the
		// definition to be written that way, and validation rejects a cell
		// whose own onLand teleports to itself.
		return append(events, s.executeActions(target.OnLand, player, target)...)

	case "skip_turns":
		turns := resolveAmount(a, cell.Fields)
		if turns < 0 {
			turns = 0
		}
		player.SkipTurns += turns
		return []GameEvent{NewGameEvent("skip_turns",
			fmt.Sprintf("%s loses %d turn(s)", player.Name, turns), nil)}

	case "random_branch":
		// Server-side randomness, like the dice: the client never decides an
		// outcome. This is what makes chance and event cards possible.
		if len(a.Options) == 0 {
			return nil
		}
		picked := a.Options[rand.Intn(len(a.Options))]
		events := []GameEvent{NewGameEvent("random_branch",
			fmt.Sprintf("%s: %s", player.Name, picked.Title), nil)}
		return append(events, s.executeActions(picked.Then, player, cell)...)

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

func (s *GameSession) ResolvePendingAction(actionID string) ([]GameEvent, error) {
	if s.State.PendingAction == nil {
		return nil, fmt.Errorf("no pending action")
	}

	switch s.State.PendingAction.Type {
	case "route_choice":
		return s.resolveRouteChoice(actionID)
	default:
		return s.resolveStandardChoice(actionID)
	}
}

func (s *GameSession) resolveStandardChoice(actionID string) ([]GameEvent, error) {
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

func (s *GameSession) resolveRouteChoice(edgeID string) ([]GameEvent, error) {
	pm := s.State.PendingMovement
	if pm == nil {
		return nil, fmt.Errorf("no pending movement context")
	}

	edgeMap := s.Definition.Board.buildEdgeMap()
	currentEdges := edgeMap[pm.CurrentCellID]
	var chosenEdge *EdgeDefinition
	for _, e := range currentEdges {
		if e.ID == edgeID {
			chosenEdge = &e
			break
		}
	}
	if chosenEdge == nil {
		return nil, fmt.Errorf("edge '%s' not found from cell '%s'", edgeID, pm.CurrentCellID)
	}

	ctx := MoveContext{
		PlayerID:       pm.PlayerID,
		Dice:           pm.Dice,
		Total:          pm.Total,
		Step:           len(pm.PathSoFar),
		RemainingSteps: pm.RemainingSteps,
	}
	if !s.evaluateCondition(*chosenEdge, ctx) {
		return nil, fmt.Errorf("edge condition no longer met")
	}

	player := s.getPlayerByID(pm.PlayerID)
	if player == nil {
		return nil, fmt.Errorf("player not found")
	}

	if chosenEdge.Condition.Type == "pay_resource" {
		amt := 0
		if chosenEdge.Condition.Amount != nil {
			amt = *chosenEdge.Condition.Amount
		}
		if chosenEdge.Condition.Resource != "" {
			if player.Resources[chosenEdge.Condition.Resource] < amt {
				return nil, fmt.Errorf("insufficient %s for pay_resource", chosenEdge.Condition.Resource)
			}
			player.Resources[chosenEdge.Condition.Resource] -= amt
		}
	}

	nextCell := s.Definition.Board.getCellByID(chosenEdge.To)
	if nextCell == nil {
		return nil, fmt.Errorf("edge '%s' leads to unknown cell '%s'", chosenEdge.ID, chosenEdge.To)
	}

	var events []GameEvent
	label := chosenEdge.Condition.Label
	if label == "" {
		label = fmt.Sprintf("%s → %s", chosenEdge.From, chosenEdge.To)
	}
	events = append(events, NewGameEvent("route_chosen",
		fmt.Sprintf("%s chose path: %s", player.Name, label), nil))

	player.PositionCellID = nextCell.ID

	pathIDs := append(pm.PathSoFar, nextCell.ID)
	pm.PathSoFar = pathIDs
	pm.CurrentCellID = nextCell.ID
	pm.RemainingSteps--

	s.State.PendingAction = nil

	if pm.RemainingSteps > 0 {
		moveEvents := s.moveSteps(pm.RemainingSteps, pm.Dice, pm.Total, nextCell.ID, events)
		if s.State.PendingMovement != nil && s.State.PendingAction != nil {
			return moveEvents, nil
		}
		events = moveEvents
		s.State.PendingMovement = nil
		return events, nil
	}

	if nextCell != nil && len(nextCell.OnLand) > 0 {
		landEvents := s.executeActions(nextCell.OnLand, player, nextCell)
		events = append(events, landEvents...)
	}

	events = append(events, NewGameEvent("move",
		fmt.Sprintf("%s moved from %s to %s", player.Name, pm.PathSoFar[0], nextCell.ID),
		map[string]any{
			"from":     pm.PathSoFar[0],
			"to":       nextCell.ID,
			"path":     pathIDs,
			"playerId": player.ID,
			"dice":     pm.Dice,
			"total":    pm.Total,
		}))

	if s.State.PendingAction == nil {
		s.advanceTurn()
	}

	s.State.PendingMovement = nil
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
	rounds := 0
	if nextIdx <= s.State.CurrentPlayerIndex {
		rounds++
	}

	// Walk forward past bankrupt players, and past anyone still serving a
	// forfeited turn. Each skipped player consumes one of their pending turns,
	// so a jail sentence measured in turns actually elapses. The loop is
	// bounded by the number of skips outstanding plus the player count, and
	// activePlayers > 1 above guarantees at least one player can act.
	for {
		next := &s.State.Players[nextIdx]
		if next.Bankrupt {
			nextIdx = (nextIdx + 1) % playerCount
			if nextIdx == 0 {
				rounds++
			}
			continue
		}
		if next.SkipTurns > 0 {
			next.SkipTurns--
			s.State.Log = append(s.State.Log, NewGameEvent("turn_skipped",
				fmt.Sprintf("%s sits this turn out", next.Name), nil))
			nextIdx = (nextIdx + 1) % playerCount
			if nextIdx == 0 {
				rounds++
			}
			continue
		}
		break
	}

	s.State.RoundNumber += rounds
	s.State.CurrentPlayerIndex = nextIdx
	s.State.TurnNumber++
}

func (s *GameSession) CurrentPlayer() *PlayerState {
	return &s.State.Players[s.State.CurrentPlayerIndex]
}

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
