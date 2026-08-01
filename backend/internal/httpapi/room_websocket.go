package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/coder/websocket"

	"rollboard/internal/realtime"
	"rollboard/internal/room"
)

func (a *API) handleRoomWebSocket(w http.ResponseWriter, r *http.Request, roomID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", "use GET")
		return
	}
	if a.hub == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "realtime service not ready", "try again later")
		return
	}
	actor, ok := a.currentActor(w, r)
	if !ok {
		return
	}
	since, err := parseRoomSequence(r.URL.Query().Get("since"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_SEQUENCE", "invalid room sequence", "use a non-negative integer")
		return
	}
	client, err := a.hub.Connect(r.Context(), roomID, *actor, since)
	if errors.Is(err, realtime.ErrNotMember) {
		writeError(w, http.StatusForbidden, "ROOM_MEMBERSHIP_REQUIRED", "room membership required", "join the room first")
		return
	}
	if err != nil {
		writeRoomError(w, err, "connect to")
		return
	}
	defer client.Close()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: a.auth.WebSocketOriginPatterns})
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())

	outbound := make(chan []byte, 32)
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for {
			select {
			case <-ctx.Done():
				return
			case event, open := <-client.Events:
				if !open {
					return
				}
				payload, err := json.Marshal(event)
				if err != nil || conn.Write(ctx, websocket.MessageText, payload) != nil {
					return
				}
			case payload := <-outbound:
				if conn.Write(ctx, websocket.MessageText, payload) != nil {
					return
				}
			}
		}
	}()
	defer func() {
		cancel()
		_ = conn.Close(websocket.StatusNormalClosure, "bye")
		<-writerDone
	}()

	send := func(value any) bool {
		payload, err := json.Marshal(value)
		if err != nil {
			return false
		}
		select {
		case outbound <- payload:
			return true
		case <-ctx.Done():
			return false
		default:
			return false
		}
	}
	for {
		messageType, payload, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if messageType != websocket.MessageText {
			if !send(realtimeError("INVALID_MESSAGE", "messages must contain JSON")) {
				return
			}
			continue
		}
		var intent realtime.Intent
		if err := json.Unmarshal(payload, &intent); err != nil {
			if !send(realtimeError("INVALID_JSON", "messages must contain valid JSON")) {
				return
			}
			continue
		}
		if _, err := a.hub.Submit(ctx, roomID, *actor, intent); err != nil {
			if !send(realtimeError(realtimeErrorCode(err), "room intention was not accepted")) {
				return
			}
		}
	}
}

func parseRoomSequence(raw string) (uint64, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseUint(raw, 10, 64)
}

func realtimeError(code, details string) map[string]string {
	return map[string]string{"type": "room_error", "code": code, "details": details}
}

func realtimeErrorCode(err error) string {
	switch {
	case errors.Is(err, room.ErrNotYourTurn):
		return "NOT_YOUR_TURN"
	case errors.Is(err, room.ErrPendingAction):
		return "PENDING_ACTION"
	case errors.Is(err, room.ErrNoPendingAction):
		return "NO_PENDING_ACTION"
	case errors.Is(err, room.ErrGameNotActive):
		return "GAME_NOT_ACTIVE"
	case errors.Is(err, room.ErrNotHost):
		return "NOT_HOST"
	case errors.Is(err, realtime.ErrUnsupportedIntent):
		return "INVALID_INTENT"
	default:
		return "ROOM_ACTION_REJECTED"
	}
}
