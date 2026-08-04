package game

import "fmt"

// applyProgression promotes a player whose experience has crossed the next
// threshold, possibly several times at once after a large reward.
//
// It runs after every executed action list rather than being something a
// definition must remember to call, because an experience gain that silently
// fails to level somebody up is the kind of bug nobody notices until much
// later.
func (s *GameSession) applyProgression(player *PlayerState) []GameEvent {
	if s.Definition == nil || s.Definition.Rules.Progression == nil {
		return nil
	}
	rule := s.Definition.Rules.Progression
	if rule.ExperienceResource == "" || rule.LevelResource == "" {
		return nil
	}

	var events []GameEvent
	for {
		level := player.Resources[rule.LevelResource]
		if level < 1 {
			level = 1
		}
		// Thresholds[0] is the experience needed to reach level 2.
		index := level - 1
		if index < 0 || index >= len(rule.Thresholds) {
			break
		}
		if player.Resources[rule.ExperienceResource] < rule.Thresholds[index] {
			break
		}
		player.Resources[rule.LevelResource] = level + 1
		message := fmt.Sprintf("%s reached level %d", player.Name, level+1)
		if rule.PointsResource != "" && rule.PointsPerLevel > 0 {
			player.Resources[rule.PointsResource] += rule.PointsPerLevel
			message = fmt.Sprintf("%s reached level %d and gained %d %s",
				player.Name, level+1, rule.PointsPerLevel, rule.PointsResource)
		}
		events = append(events, NewGameEvent("level_up", message, nil))
	}
	return events
}
