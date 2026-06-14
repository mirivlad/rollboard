package httpapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"rollboard/internal/game"
	"rollboard/internal/storage/sqlite"
)

type API struct {
	store *sqlite.Store
}

func New(store *sqlite.Store) *API {
	return &API{store: store}
}

func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/health", a.handleHealth)
	mux.HandleFunc("/api/games", a.handleGames)
	mux.HandleFunc("/api/games/", a.handleGameByID)
	mux.HandleFunc("/api/sessions/", a.handleSessions)
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (a *API) handleGames(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := a.store.ListGames()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if list == nil {
			list = []sqlite.GameSummary{}
		}
		writeJSON(w, 200, list)

	case http.MethodPost:
		var g game.GameDefinition
		if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
			writeError(w, 400, "invalid JSON: "+err.Error())
			return
		}
		if strings.TrimSpace(g.ID) == "" {
			g.ID = generateSlug(g.Title)
		}
		if g.Version == 0 {
			g.Version = 1
		}
		if err := a.store.CreateGame(&g); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, g)

	default:
		writeError(w, 405, "method not allowed")
	}
}

func (a *API) handleGameByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/games/")
	id = strings.TrimSuffix(id, "/")

	if id == "" {
		a.handleGames(w, r)
		return
	}

	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		gameID := parts[0]
		sub := parts[1]
		switch sub {
		case "validate":
			a.handleValidate(w, r, gameID)
			return
		case "playtest":
			a.handlePlaytest(w, r, gameID)
			return
		default:
			writeError(w, 404, "not found")
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		g, err := a.store.GetGame(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if g == nil {
			writeError(w, 404, "game not found")
			return
		}
		writeJSON(w, 200, g)

	case http.MethodPut:
		var g game.GameDefinition
		if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
			writeError(w, 400, "invalid JSON: "+err.Error())
			return
		}
		g.ID = id
		existing, err := a.store.GetGame(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if existing == nil {
			if g.Version == 0 {
				g.Version = 1
			}
			if err := a.store.CreateGame(&g); err != nil {
				writeError(w, 500, err.Error())
				return
			}
		} else {
			g.Version = existing.Version
			if err := a.store.UpdateGame(&g); err != nil {
				writeError(w, 500, err.Error())
				return
			}
		}
		writeJSON(w, 200, g)

	case http.MethodDelete:
		writeError(w, 501, "not implemented")

	default:
		writeError(w, 405, "method not allowed")
	}
}

func (a *API) handleValidate(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method not allowed")
		return
	}
	g, err := a.store.GetGame(gameID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if g == nil {
		writeError(w, 404, "game not found")
		return
	}
	if err := game.ValidateDefinition(g); err != nil {
		writeJSON(w, 200, map[string]any{"valid": false, "errors": err.Errors})
		return
	}
	writeJSON(w, 200, map[string]bool{"valid": true})
}

func (a *API) handlePlaytest(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method not allowed")
		return
	}

	g, err := a.store.GetGame(gameID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if g == nil {
		writeError(w, 404, "game not found")
		return
	}

	var body struct {
		Mode    string              `json:"mode"`
		Players []game.PlayerConfig `json:"players"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON: "+err.Error())
		return
	}

	if len(body.Players) < 2 {
		writeError(w, 400, "at least 2 players required")
		return
	}
	if len(body.Players) > 6 {
		body.Players = body.Players[:6]
	}
	if body.Mode == "" {
		body.Mode = "hotseat"
	}

	session := game.StartSession(g, body.Players)
	session.Mode = body.Mode

	if err := a.store.SaveSession(session); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, session)
}

func (a *API) handleSessions(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	id = strings.TrimSuffix(id, "/")

	if id == "" {
		writeError(w, 400, "session id required")
		return
	}

	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		sessionID := parts[0]
		action := parts[1]

		session, err := a.store.GetSession(sessionID)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if session == nil {
			writeError(w, 404, "session not found")
			return
		}

		switch action {
		case "roll":
			if r.Method != http.MethodPost {
				writeError(w, 405, "method not allowed")
				return
			}
			if session.State.Status != "active" {
				writeError(w, 400, "game is not active")
				return
			}
			if session.State.PendingAction != nil {
				writeError(w, 400, "pending action must be resolved first")
				return
			}

			roll, diceEvt := session.RollDice()
			session.State.Log = append(session.State.Log, *diceEvt)
			moveEvents := session.MoveCurrentPlayer(roll.Total, roll.Rolls, roll.Total)
			session.State.Log = append(session.State.Log, moveEvents...)

			// In hotseat mode, always show turn-pass screen after roll + move
			// (turn was already advanced by MoveCurrentPlayer if no pending action)

			if err := a.store.SaveSession(session); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, session)

		case "actions":
			if r.Method != http.MethodPost {
				writeError(w, 405, "method not allowed")
				return
			}
			var body struct {
				ActionID string `json:"actionId"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeError(w, 400, "invalid JSON")
				return
			}

			events, actionErr := session.ResolvePendingAction(body.ActionID)
			if actionErr != nil {
				writeError(w, 400, actionErr.Error())
				return
			}
			session.State.Log = append(session.State.Log, events...)

			if err := a.store.SaveSession(session); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, session)

		case "next-turn":
			// In hotseat mode, signal that the player acknowledges the turn end
			if r.Method != http.MethodPost {
				writeError(w, 405, "method not allowed")
				return
			}
			if session.State.PendingAction != nil {
				writeError(w, 400, "pending action must be resolved first")
				return
			}
			// Turn is already advanced by roll+move, this is just a
			// hotseat acknowledgement. We can re-save or just return.
			if err := a.store.SaveSession(session); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, session)

		default:
			writeError(w, 404, "unknown action")
		}
		return
	}

	if r.Method == http.MethodGet {
		session, err := a.store.GetSession(id)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		if session == nil {
			writeError(w, 404, "session not found")
			return
		}
		writeJSON(w, 200, session)
		return
	}

	writeError(w, 405, "method not allowed")
}

func generateSlug(title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	for _, ch := range []string{".", ",", "!", "?", ":", ";", "'", "\"", "(", ")"} {
		slug = strings.ReplaceAll(slug, ch, "")
	}
	if slug == "" {
		return fmt.Sprintf("game_%d", len(title))
	}
	return slug
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
