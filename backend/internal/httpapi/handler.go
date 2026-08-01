package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"rollboard/internal/game"
	"rollboard/internal/identity"
	"rollboard/internal/storage"
)

type API struct {
	store    storage.Store
	identity *identity.Repository
}

func (a *API) WithIdentity(repository *identity.Repository) *API {
	a.identity = repository
	return a
}

type apiError struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
	Code    string `json:"code"`
}

func New(store storage.Store) *API {
	return &API{store: store}
}

func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", a.Healthz)
	mux.HandleFunc("/readyz", a.Readyz)
	mux.HandleFunc("/api/health", a.handleHealth)
	mux.HandleFunc("/api/auth/register", a.handleRegister)
	mux.HandleFunc("/api/games", a.handleGames)
	mux.HandleFunc("/api/games/", a.handleGameByID)
	mux.HandleFunc("/api/sessions/", a.handleSessions)
}

func (a *API) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	if a.identity == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "identity service not ready", "try again later")
		return
	}
	var input identity.RegistrationInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON", "request body must be valid JSON")
		return
	}
	user, err := a.identity.Register(r.Context(), input)
	if errors.Is(err, identity.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "EMAIL_TAKEN", "email is already registered", "sign in instead")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REGISTRATION", "registration failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, user.Public())
}

func (a *API) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) Readyz(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "service not ready", "database is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	a.Healthz(w, r)
}

func (a *API) handleGames(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := a.store.ListGames(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list games", "try again later")
			return
		}
		writeJSON(w, http.StatusOK, list)

	case http.MethodPost:
		var definition game.GameDefinition
		if err := json.NewDecoder(r.Body).Decode(&definition); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON", "request body must be valid JSON")
			return
		}
		if strings.TrimSpace(definition.ID) == "" {
			definition.ID = generateSlug(definition.Title)
		}
		if definition.Version == 0 {
			definition.Version = 1
		}
		if err := a.store.CreateGame(r.Context(), &definition); err != nil {
			if strings.Contains(err.Error(), "duplicate key") {
				writeError(w, http.StatusConflict, "CONFLICT", "game already exists", "choose a different game ID")
				return
			}
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not create game", "try again later")
			return
		}
		writeJSON(w, http.StatusCreated, definition)

	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET or POST")
	}
}

func (a *API) handleGameByID(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/games/"), "/")
	if id == "" {
		a.handleGames(w, r)
		return
	}
	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		switch parts[1] {
		case "validate":
			a.handleValidate(w, r, parts[0])
		case "playtest":
			a.handlePlaytest(w, r, parts[0])
		default:
			writeError(w, http.StatusNotFound, "NOT_FOUND", "not found", "unknown game endpoint")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		definition, err := a.store.GetGame(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load game", "try again later")
			return
		}
		if definition == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "game not found", "check the game ID")
			return
		}
		writeJSON(w, http.StatusOK, definition)

	case http.MethodPut:
		var definition game.GameDefinition
		if err := json.NewDecoder(r.Body).Decode(&definition); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON", "request body must be valid JSON")
			return
		}
		definition.ID = id
		existing, err := a.store.GetGame(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load game", "try again later")
			return
		}
		if existing == nil {
			if definition.Version == 0 {
				definition.Version = 1
			}
			err = a.store.CreateGame(r.Context(), &definition)
		} else {
			definition.Version = existing.Version
			err = a.store.UpdateGame(r.Context(), &definition)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not save game", "try again later")
			return
		}
		writeJSON(w, http.StatusOK, definition)

	case http.MethodDelete:
		writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "game deletion is not available", "use a new draft instead")

	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET, PUT, or DELETE")
	}
}

func (a *API) handleValidate(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	definition, err := a.store.GetGame(r.Context(), gameID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load game", "try again later")
		return
	}
	if definition == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "game not found", "check the game ID")
		return
	}
	if err := game.ValidateDefinition(definition); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "errors": err.Errors})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"valid": true})
}

func (a *API) handlePlaytest(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	definition, err := a.store.GetGame(r.Context(), gameID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load game", "try again later")
		return
	}
	if definition == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "game not found", "check the game ID")
		return
	}

	var body struct {
		Mode    string              `json:"mode"`
		Players []game.PlayerConfig `json:"players"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON", "request body must be valid JSON")
		return
	}
	if len(body.Players) < 2 {
		writeError(w, http.StatusBadRequest, "INVALID_PLAYERS", "at least two players are required", "add another player")
		return
	}
	if len(body.Players) > 6 {
		body.Players = body.Players[:6]
	}
	if body.Mode == "" {
		body.Mode = "hotseat"
	}

	session := game.StartSession(definition, body.Players)
	session.Mode = body.Mode
	if err := a.store.SaveSession(r.Context(), session); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not start playtest", "try again later")
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (a *API) handleSessions(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "INVALID_SESSION", "session ID is required", "provide a session ID")
		return
	}
	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		a.handleSessionCommand(w, r, parts[0], parts[1])
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET")
		return
	}
	session, err := a.store.GetSession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load session", "try again later")
		return
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "session not found", "check the session ID")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (a *API) handleSessionCommand(w http.ResponseWriter, r *http.Request, sessionID, command string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	session, err := a.store.GetSession(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load session", "try again later")
		return
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "session not found", "check the session ID")
		return
	}

	switch command {
	case "roll":
		if session.State.Status != "active" {
			writeError(w, http.StatusBadRequest, "INVALID_STATE", "game is not active", "start a new game")
			return
		}
		if session.State.PendingAction != nil {
			writeError(w, http.StatusBadRequest, "PENDING_ACTION", "pending action must be resolved first", "choose an available action")
			return
		}
		roll, diceEvent := session.RollDice()
		session.State.Log = append(session.State.Log, *diceEvent)
		session.State.Log = append(session.State.Log, session.MoveCurrentPlayer(roll.Total, roll.Rolls, roll.Total)...)

	case "actions":
		var body struct {
			ActionID string `json:"actionId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON", "request body must be valid JSON")
			return
		}
		events, err := session.ResolvePendingAction(body.ActionID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ACTION", "action cannot be resolved", err.Error())
			return
		}
		session.State.Log = append(session.State.Log, events...)

	case "next-turn":
		if session.State.PendingAction != nil {
			writeError(w, http.StatusBadRequest, "PENDING_ACTION", "pending action must be resolved first", "choose an available action")
			return
		}

	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "unknown session command", "use roll, actions, or next-turn")
		return
	}

	if err := a.store.SaveSession(r.Context(), session); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not save session", "try again later")
		return
	}
	writeJSON(w, http.StatusOK, session)
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message, details string) {
	writeJSON(w, status, apiError{Error: message, Details: details, Code: code})
}
