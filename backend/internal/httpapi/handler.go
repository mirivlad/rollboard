package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rollboard/internal/catalog"
	"rollboard/internal/game"
	"rollboard/internal/identity"
	"rollboard/internal/realtime"
	"rollboard/internal/room"
	"rollboard/internal/storage"
)

const (
	sessionCookieName = "rollboard_session"
	csrfCookieName    = "rollboard_csrf"
	csrfHeaderName    = "X-CSRF-Token"
	defaultSessionTTL = 30 * 24 * time.Hour
)

type API struct {
	store    storage.Store
	identity identity.Service
	catalog  CatalogService
	rooms    RoomService
	hub      *realtime.Hub
	auth     AuthOptions
}

type CatalogService interface {
	CreateGame(context.Context, string, game.GameDefinition) (catalog.Game, error)
	GetDraft(context.Context, string, string) (*catalog.Draft, error)
	SaveDraft(context.Context, string, string, game.GameDefinition) error
	Publish(context.Context, string, string) (catalog.Version, error)
	GetVersion(context.Context, string, int) (*catalog.Version, error)
}

type RoomService interface {
	Create(context.Context, identity.Actor, string, room.CreateInput) (room.Room, error)
	Get(context.Context, string) (*room.Room, error)
	Join(context.Context, identity.Actor, string) (room.RoomMember, error)
	Mute(context.Context, identity.Actor, string, string, bool) error
	Remove(context.Context, identity.Actor, string, string) error
	ListMessages(context.Context, identity.Actor, string, int) ([]room.RoomMessage, error)
}

type guestClaimer interface {
	ClaimGuest(context.Context, string, string) (identity.Guest, error)
}

type AuthOptions struct {
	CookieSecure            bool
	SessionTTL              time.Duration
	WebSocketOriginPatterns []string
}

func (a *API) WithIdentity(service identity.Service) *API {
	a.identity = service
	return a
}

func (a *API) WithCatalog(service CatalogService) *API {
	a.catalog = service
	return a
}

func (a *API) WithRooms(service RoomService) *API {
	a.rooms = service
	return a
}

func (a *API) WithRealtimeHub(hub *realtime.Hub) *API {
	a.hub = hub
	return a
}

func (a *API) WithAuthOptions(options AuthOptions) *API {
	if options.SessionTTL > 0 {
		a.auth.SessionTTL = options.SessionTTL
	}
	a.auth.CookieSecure = options.CookieSecure
	a.auth.WebSocketOriginPatterns = append([]string(nil), options.WebSocketOriginPatterns...)
	return a
}

type apiError struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
	Code    string `json:"code"`
}

func New(store storage.Store) *API {
	return &API{store: store, auth: AuthOptions{SessionTTL: defaultSessionTTL}}
}

func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", a.Healthz)
	mux.HandleFunc("/readyz", a.Readyz)
	mux.HandleFunc("/api/health", a.handleHealth)
	mux.HandleFunc("/api/auth/register", a.handleRegister)
	mux.HandleFunc("/api/auth/guest", a.handleGuestEntry)
	mux.HandleFunc("/api/auth/login", a.handleLogin)
	mux.HandleFunc("/api/auth/me", a.handleMe)
	mux.HandleFunc("/api/auth/logout", a.handleLogout)
	mux.HandleFunc("/api/games", a.handleGames)
	mux.HandleFunc("/api/games/", a.handleGameByID)
	mux.HandleFunc("/api/rooms", a.handleRooms)
	mux.HandleFunc("/api/rooms/", a.handleRoomByID)
	mux.HandleFunc("/api/sessions/", a.handleSessions)
}

func (a *API) handleGuestEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	if a.identity == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "identity service not ready", "try again later")
		return
	}
	var body struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON", "request body must be valid JSON")
		return
	}
	guest, err := a.identity.CreateGuest(r.Context(), body.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_GUEST", "guest entry failed", "display name must contain 1 to 64 characters")
		return
	}
	_, token, err := a.identity.CreateGuestSession(r.Context(), guest.ID, time.Now().Add(a.auth.SessionTTL))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not start guest session", "try again later")
		return
	}
	if err := a.setSessionCookies(w, token); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not start guest session", "try again later")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"kind":  "guest",
		"guest": map[string]string{"id": guest.ID, "displayName": guest.DisplayName},
	})
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	if a.identity == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "identity service not ready", "try again later")
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON", "request body must be valid JSON")
		return
	}
	user, err := a.identity.Authenticate(r.Context(), body.Email, body.Password)
	if errors.Is(err, identity.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password", "check your credentials")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not sign in", "try again later")
		return
	}
	_, token, err := a.identity.CreateUserSession(r.Context(), user.ID, time.Now().Add(a.auth.SessionTTL))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not start account session", "try again later")
		return
	}
	if err := a.setSessionCookies(w, token); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not start account session", "try again later")
		return
	}
	writeJSON(w, http.StatusOK, user.Public())
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET")
		return
	}
	actor, ok := a.currentActor(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, publicActor(actor))
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	if a.identity == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "identity service not ready", "try again later")
		return
	}
	if !requireCSRF(w, r) {
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if err := a.identity.DeleteSession(r.Context(), cookie.Value); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not end session", "try again later")
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		Secure:   a.auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func requireCSRF(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(r.Header.Get(csrfHeaderName))) != 1 {
		writeError(w, http.StatusForbidden, "CSRF_FAILED", "request verification failed", "refresh the page and try again")
		return false
	}
	return true
}

func (a *API) currentActor(w http.ResponseWriter, r *http.Request) (*identity.Actor, bool) {
	if a.identity == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "identity service not ready", "try again later")
		return nil, false
	}
	cookie, err := r.Cookie(sessionCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "sign in required", "create a guest session or sign in")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_SESSION", "invalid session", "sign in again")
		return nil, false
	}
	actor, err := a.identity.LookupSession(r.Context(), cookie.Value, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load session", "try again later")
		return nil, false
	}
	if actor == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "sign in required", "create a guest session or sign in")
		return nil, false
	}
	return actor, true
}

func publicActor(actor *identity.Actor) map[string]any {
	if actor.User != nil {
		return map[string]any{"kind": "user", "user": actor.User.Public()}
	}
	return map[string]any{
		"kind":  "guest",
		"guest": map[string]string{"id": actor.Guest.ID, "displayName": actor.Guest.DisplayName},
	}
}

func (a *API) setSessionCookies(w http.ResponseWriter, token string) error {
	csrfToken, _, err := identity.NewToken()
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(a.auth.SessionTTL.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/",
		Secure:   a.auth.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(a.auth.SessionTTL.Seconds()),
	})
	return nil
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
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REGISTRATION", "registration failed", err.Error())
		return
	}
	user, err := a.identity.Register(r.Context(), input)
	if errors.Is(err, identity.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "EMAIL_TAKEN", "email is already registered", "sign in instead")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not register account", "try again later")
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		actor, err := a.identity.LookupSession(r.Context(), cookie.Value, time.Now())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not claim guest profile", "try again later")
			return
		}
		if actor != nil && actor.Guest != nil {
			claimer, ok := a.identity.(guestClaimer)
			if !ok {
				writeError(w, http.StatusServiceUnavailable, "NOT_READY", "identity service not ready", "try again later")
				return
			}
			if _, err := claimer.ClaimGuest(r.Context(), actor.Guest.ID, user.ID); err != nil {
				writeError(w, http.StatusConflict, "GUEST_CLAIM_FAILED", "could not claim guest profile", "sign in and try again")
				return
			}
		}
	}
	_, token, err := a.identity.CreateUserSession(r.Context(), user.ID, time.Now().Add(a.auth.SessionTTL))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not start account session", "try again later")
		return
	}
	if err := a.setSessionCookies(w, token); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not start account session", "try again later")
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
		if a.catalog != nil {
			actor, ok := a.currentActor(w, r)
			if !ok {
				return
			}
			if actor.User == nil {
				writeError(w, http.StatusForbidden, "ACCOUNT_REQUIRED", "account required", "claim your guest profile or sign in")
				return
			}
			if !requireCSRF(w, r) {
				return
			}
			created, err := a.catalog.CreateGame(r.Context(), actor.User.ID, definition)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not create game draft", "try again later")
				return
			}
			writeJSON(w, http.StatusCreated, created)
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

func (a *API) handleRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	if a.rooms == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "room service not ready", "try again later")
		return
	}
	actor, ok := a.currentActor(w, r)
	if !ok {
		return
	}
	if actor.User == nil {
		writeError(w, http.StatusForbidden, "ACCOUNT_REQUIRED", "account required", "claim your guest profile or sign in")
		return
	}
	if !requireCSRF(w, r) {
		return
	}
	var body struct {
		GameVersionID string `json:"gameVersionId"`
		Title         string `json:"title"`
		MaxPlayers    int16  `json:"maxPlayers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON", "request body must be valid JSON")
		return
	}
	created, err := a.rooms.Create(r.Context(), *actor, body.GameVersionID, room.CreateInput{Title: body.Title, MaxPlayers: body.MaxPlayers})
	if err != nil {
		writeRoomError(w, err, "create")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) handleRoomByID(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/rooms/"), "/")
	if path == "" {
		a.handleRooms(w, r)
		return
	}
	parts := strings.Split(path, "/")
	roomID := parts[0]
	if len(parts) == 1 {
		a.handleRoom(w, r, roomID)
		return
	}
	if len(parts) == 2 && parts[1] == "join" {
		a.handleRoomJoin(w, r, roomID)
		return
	}
	if len(parts) == 2 && parts[1] == "ws" {
		a.handleRoomWebSocket(w, r, roomID)
		return
	}
	if len(parts) == 2 && parts[1] == "messages" {
		a.handleRoomMessages(w, r, roomID)
		return
	}
	if len(parts) == 4 && parts[1] == "members" && parts[3] == "mute" {
		a.handleRoomMute(w, r, roomID, parts[2])
		return
	}
	if len(parts) == 3 && parts[1] == "members" {
		a.handleRoomRemoval(w, r, roomID, parts[2])
		return
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "not found", "unknown room endpoint")
}

func (a *API) handleRoom(w http.ResponseWriter, r *http.Request, roomID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET")
		return
	}
	if a.rooms == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "room service not ready", "try again later")
		return
	}
	actor, ok := a.currentActor(w, r)
	if !ok {
		return
	}
	stored, err := a.rooms.Get(r.Context(), roomID)
	if err != nil {
		writeRoomError(w, err, "load")
		return
	}
	if !actorIsRoomMember(*actor, stored.Members) {
		writeError(w, http.StatusForbidden, "ROOM_MEMBERSHIP_REQUIRED", "room membership required", "join the room first")
		return
	}
	writeJSON(w, http.StatusOK, stored)
}

func (a *API) handleRoomJoin(w http.ResponseWriter, r *http.Request, roomID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	if a.rooms == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "room service not ready", "try again later")
		return
	}
	actor, ok := a.currentActor(w, r)
	if !ok {
		return
	}
	if !requireCSRF(w, r) {
		return
	}
	member, err := a.rooms.Join(r.Context(), *actor, roomID)
	if err != nil {
		writeRoomError(w, err, "join")
		return
	}
	a.refreshRoom(r.Context(), roomID)
	writeJSON(w, http.StatusCreated, member)
}

func (a *API) handleRoomMessages(w http.ResponseWriter, r *http.Request, roomID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET")
		return
	}
	if a.rooms == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "room service not ready", "try again later")
		return
	}
	actor, ok := a.currentActor(w, r)
	if !ok {
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "INVALID_LIMIT", "invalid message limit", "use a value from 1 to 100")
			return
		}
		limit = parsed
	}
	messages, err := a.rooms.ListMessages(r.Context(), *actor, roomID, limit)
	if err != nil {
		writeRoomError(w, err, "load messages for")
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (a *API) handleRoomMute(w http.ResponseWriter, r *http.Request, roomID, memberID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	if a.rooms == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "room service not ready", "try again later")
		return
	}
	actor, ok := a.currentActor(w, r)
	if !ok {
		return
	}
	if !requireCSRF(w, r) {
		return
	}
	var body struct {
		Muted *bool `json:"muted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Muted == nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON", "muted must be a boolean")
		return
	}
	if err := a.rooms.Mute(r.Context(), *actor, roomID, memberID, *body.Muted); err != nil {
		writeRoomError(w, err, "moderate")
		return
	}
	a.refreshRoom(r.Context(), roomID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleRoomRemoval(w http.ResponseWriter, r *http.Request, roomID, memberID string) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use DELETE")
		return
	}
	if a.rooms == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "room service not ready", "try again later")
		return
	}
	actor, ok := a.currentActor(w, r)
	if !ok {
		return
	}
	if !requireCSRF(w, r) {
		return
	}
	if err := a.rooms.Remove(r.Context(), *actor, roomID, memberID); err != nil {
		writeRoomError(w, err, "moderate")
		return
	}
	a.refreshRoom(r.Context(), roomID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) refreshRoom(ctx context.Context, roomID string) {
	if a.hub == nil {
		return
	}
	if err := a.hub.Refresh(ctx, roomID); err != nil {
		log.Printf("refresh room %s after membership change: %v", roomID, err)
	}
}

func actorIsRoomMember(actor identity.Actor, members []room.RoomMember) bool {
	for _, member := range members {
		if actor.User != nil && member.ActorKind == "user" && member.ActorID == actor.User.ID {
			return true
		}
		if actor.Guest != nil && member.ActorKind == "guest" && member.ActorID == actor.Guest.ID {
			return true
		}
	}
	return false
}

func writeRoomError(w http.ResponseWriter, err error, operation string) {
	switch {
	case errors.Is(err, room.ErrNotFound), errors.Is(err, room.ErrGameVersionNotFound), errors.Is(err, room.ErrMemberNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "room resource not found", "check the room, member, or published game version")
	case errors.Is(err, room.ErrAccountRequired), errors.Is(err, room.ErrNotHost):
		writeError(w, http.StatusForbidden, "FORBIDDEN", "room operation is not permitted", "check your account and room permissions")
	case errors.Is(err, room.ErrAlreadyMember), errors.Is(err, room.ErrRoomFull), errors.Is(err, room.ErrRoomNotJoinable), errors.Is(err, room.ErrCannotRemoveHost):
		writeError(w, http.StatusConflict, "ROOM_CONFLICT", "room operation cannot be completed", "refresh the room and try again")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not "+operation+" room", "try again later")
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
		case "draft":
			a.handleDraft(w, r, parts[0])
		case "publish":
			a.handlePublish(w, r, parts[0])
		default:
			if strings.HasPrefix(parts[1], "versions/") {
				a.handleVersion(w, r, parts[0], strings.TrimPrefix(parts[1], "versions/"))
				return
			}
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

func (a *API) handleVersion(w http.ResponseWriter, r *http.Request, gameID, rawNumber string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET")
		return
	}
	if a.catalog == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "catalog service not ready", "try again later")
		return
	}
	number, err := strconv.Atoi(rawNumber)
	if err != nil || number < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_VERSION", "invalid version number", "use a positive integer")
		return
	}
	version, err := a.catalog.GetVersion(r.Context(), gameID, number)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load published version", "try again later")
		return
	}
	if version == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "published version not found", "check the game ID and version number")
		return
	}
	writeJSON(w, http.StatusOK, version)
}

func (a *API) handlePublish(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
		return
	}
	if a.catalog == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "catalog service not ready", "try again later")
		return
	}
	actor, ok := a.currentActor(w, r)
	if !ok {
		return
	}
	if actor.User == nil {
		writeError(w, http.StatusForbidden, "ACCOUNT_REQUIRED", "account required", "claim your guest profile or sign in")
		return
	}
	if !requireCSRF(w, r) {
		return
	}
	version, err := a.catalog.Publish(r.Context(), actor.User.ID, gameID)
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "game draft not found", "check the game ID")
		return
	}
	if errors.Is(err, catalog.ErrInvalidDefinition) {
		writeError(w, http.StatusBadRequest, "INVALID_DEFINITION", "game definition is invalid", "fix validation errors before publishing")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not publish game", "try again later")
		return
	}
	writeJSON(w, http.StatusCreated, version)
}

func (a *API) handleDraft(w http.ResponseWriter, r *http.Request, gameID string) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET")
		return
	}
	if a.catalog == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "catalog service not ready", "try again later")
		return
	}
	actor, ok := a.currentActor(w, r)
	if !ok {
		return
	}
	if actor.User == nil {
		writeError(w, http.StatusForbidden, "ACCOUNT_REQUIRED", "account required", "claim your guest profile or sign in")
		return
	}
	if r.Method == http.MethodPut {
		if !requireCSRF(w, r) {
			return
		}
		var definition game.GameDefinition
		if err := json.NewDecoder(r.Body).Decode(&definition); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON", "request body must be valid JSON")
			return
		}
		if err := a.catalog.SaveDraft(r.Context(), actor.User.ID, gameID, definition); err != nil {
			if errors.Is(err, catalog.ErrNotFound) {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "game draft not found", "check the game ID")
				return
			}
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not save game draft", "try again later")
			return
		}
		definition.ID = gameID
		writeJSON(w, http.StatusOK, definition)
		return
	}
	draft, err := a.catalog.GetDraft(r.Context(), actor.User.ID, gameID)
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "game draft not found", "check the game ID")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load game draft", "try again later")
		return
	}
	writeJSON(w, http.StatusOK, draft.Definition)
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
