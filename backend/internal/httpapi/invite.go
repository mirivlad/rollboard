package httpapi

import "net/http"

// handleInvite serves /api/rooms/invite/{token} and
// /api/rooms/invite/{token}/join.
//
// Resolving an invite deliberately needs no session: somebody following a link
// has to see what they are joining before deciding to identify themselves.
// Joining, of course, does.
func (a *API) handleInvite(w http.ResponseWriter, r *http.Request, parts []string) {
	if a.rooms == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "room service not ready", "try again later")
		return
	}
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "invite not found", "check the invite link")
		return
	}
	token := parts[0]

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET")
			return
		}
		invite, err := a.rooms.ResolveInvite(r.Context(), token)
		if err != nil {
			writeRoomError(w, err, "resolve invite for")
			return
		}
		writeJSON(w, http.StatusOK, invite)
		return
	}

	if len(parts) == 2 && parts[1] == "join" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use POST")
			return
		}
		actor, ok := a.currentActor(w, r)
		if !ok {
			return
		}
		if !requireCSRF(w, r) {
			return
		}
		roomID, err := a.rooms.JoinByInvite(r.Context(), *actor, token)
		if err != nil {
			writeRoomError(w, err, "join by invite")
			return
		}
		a.refreshRoom(r.Context(), roomID)
		writeJSON(w, http.StatusOK, map[string]string{"roomId": roomID})
		return
	}

	writeError(w, http.StatusNotFound, "NOT_FOUND", "not found", "unknown invite endpoint")
}

// handleRoomInvite serves /api/rooms/{id}/invite: the host reads the current
// token with GET and replaces it with POST.
func (a *API) handleRoomInvite(w http.ResponseWriter, r *http.Request, roomID string) {
	if a.rooms == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "room service not ready", "try again later")
		return
	}
	actor, ok := a.currentActor(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		token, err := a.rooms.InviteToken(r.Context(), *actor, roomID)
		if err != nil {
			writeRoomError(w, err, "read the invite for")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"token": token})

	case http.MethodPost:
		if !requireCSRF(w, r) {
			return
		}
		token, err := a.rooms.RotateInvite(r.Context(), *actor, roomID)
		if err != nil {
			writeRoomError(w, err, "rotate the invite for")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"token": token})

	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET or POST")
	}
}
