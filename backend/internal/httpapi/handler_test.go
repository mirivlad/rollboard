package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"rollboard/internal/game"
	"rollboard/internal/storage"
)

func TestReadyzReturns503WhenStorePingFails(t *testing.T) {
	api := New(fakeStore{pingErr: errors.New("database unavailable")})
	recorder := httptest.NewRecorder()

	api.Readyz(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	var body apiError
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "NOT_READY" || body.Error != "service not ready" {
		t.Fatalf("response = %#v, want NOT_READY service error", body)
	}
}

func TestWriteErrorHasStableShape(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeError(recorder, http.StatusBadRequest, "INVALID_JSON", "invalid JSON", "body is malformed")

	var body apiError
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error != "invalid JSON" || body.Details != "body is malformed" || body.Code != "INVALID_JSON" {
		t.Fatalf("response = %#v, want stable error fields", body)
	}
}

type fakeStore struct {
	pingErr error
}

func (s fakeStore) Close() {}

func (s fakeStore) Ping(context.Context) error { return s.pingErr }

func (fakeStore) ListGames(context.Context) ([]storage.GameSummary, error) { return nil, nil }

func (fakeStore) GetGame(context.Context, string) (*game.GameDefinition, error) { return nil, nil }

func (fakeStore) CreateGame(context.Context, *game.GameDefinition) error { return nil }

func (fakeStore) UpdateGame(context.Context, *game.GameDefinition) error { return nil }

func (fakeStore) SaveSession(context.Context, *game.GameSession) error { return nil }

func (fakeStore) GetSession(context.Context, string) (*game.GameSession, error) { return nil, nil }

var _ storage.Store = fakeStore{}
