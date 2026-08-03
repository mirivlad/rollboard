package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rollboard/internal/catalog"
	"rollboard/internal/game"
	"rollboard/internal/identity"
)

// spyStore records the owner the handler scopes each session lookup to.
type spyStore struct {
	session         *game.GameSession
	getOwnerUserID  string
	saveOwnerUserID string
	getCalled       bool
	saveCalled      bool
}

func (s *spyStore) Close()                     {}
func (s *spyStore) Ping(context.Context) error { return nil }

func (s *spyStore) SaveSession(_ context.Context, ownerUserID string, _ *game.GameSession) error {
	s.saveCalled = true
	s.saveOwnerUserID = ownerUserID
	return nil
}

func (s *spyStore) GetSession(_ context.Context, _, ownerUserID string) (*game.GameSession, error) {
	s.getCalled = true
	s.getOwnerUserID = ownerUserID
	return s.session, nil
}

func newAuthzMux(api *API) *http.ServeMux {
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	return mux
}

// TestAnonymousCannotReachAuthoringRoutes locks down the routes that used to be
// served with no authentication whatsoever. Removed routes answer 404 and
// surviving routes answer 401.
func TestAnonymousCannotReachAuthoringRoutes(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"read any game", http.MethodGet, "/api/games/victim-game", "", http.StatusNotFound},
		{"overwrite any game", http.MethodPut, "/api/games/victim-game", `{"title":"HIJACKED"}`, http.StatusNotFound},
		{"delete any game", http.MethodDelete, "/api/games/victim-game", "", http.StatusNotFound},
		{"validate any game", http.MethodPost, "/api/games/victim-game/validate", "", http.StatusUnauthorized},
		{"playtest any game", http.MethodPost, "/api/games/victim-game/playtest", `{"players":[{"name":"A"},{"name":"B"}]}`, http.StatusUnauthorized},
		{"read any session", http.MethodGet, "/api/sessions/victim-session", "", http.StatusUnauthorized},
		{"roll any session", http.MethodPost, "/api/sessions/victim-session/roll", "", http.StatusUnauthorized},
		{"act on any session", http.MethodPost, "/api/sessions/victim-session/actions", `{"actionId":"buy"}`, http.StatusUnauthorized},
		{"advance any session", http.MethodPost, "/api/sessions/victim-session/next-turn", "", http.StatusUnauthorized},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			api := New(&spyStore{}).WithIdentity(fakeIdentity{}).WithCatalog(&fakeCatalog{})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))

			newAuthzMux(api).ServeHTTP(recorder, request)

			if recorder.Code != testCase.want {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, testCase.want, recorder.Body.String())
			}
		})
	}
}

// TestAnonymousPutCannotOverwriteAnotherAuthorsGame is the regression for the
// exploit that was reproduced by hand: an unauthenticated PUT replaced a
// registered author's game definition and title.
func TestAnonymousPutCannotOverwriteAnotherAuthorsGame(t *testing.T) {
	store := &spyStore{}
	catalogService := &fakeCatalog{}
	api := New(store).WithIdentity(fakeIdentity{}).WithCatalog(catalogService)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/games/victim-game", strings.NewReader(`{
		"title":"HIJACKED","version":1,
		"board":{"width":96,"height":96,"cellSize":96,"cells":[],"edges":[]},
		"rules":{"dice":{"count":1,"sides":6},"resources":{},"cellTypes":{}}
	}`))

	newAuthzMux(api).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
	if catalogService.saveCalled || catalogService.createCalled {
		t.Fatal("an anonymous request reached the catalog write path")
	}
}

func TestGuestCannotPlaytestOrReadSessions(t *testing.T) {
	guest := identity.Guest{ID: "22222222-2222-2222-2222-222222222222", DisplayName: "Guest"}
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"playtest", http.MethodPost, "/api/games/game-id/playtest", `{"players":[{"name":"A"},{"name":"B"}]}`},
		{"read session", http.MethodGet, "/api/sessions/session-id", ""},
		{"roll", http.MethodPost, "/api/sessions/session-id/roll", ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			api := New(&spyStore{}).
				WithIdentity(fakeIdentity{actor: &identity.Actor{Guest: &guest}}).
				WithCatalog(&fakeCatalog{})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
			request.Header.Set(csrfHeaderName, "csrf-token")

			newAuthzMux(api).ServeHTTP(recorder, request)

			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
			}
		})
	}
}

func TestPlaytestRequiresCSRFAndOwnedDraft(t *testing.T) {
	user := identity.User{ID: "11111111-1111-1111-1111-111111111111", Email: "author@example.com", DisplayName: "Author"}

	t.Run("missing CSRF is rejected", func(t *testing.T) {
		api := New(&spyStore{}).
			WithIdentity(fakeIdentity{actor: &identity.Actor{User: &user}}).
			WithCatalog(&fakeCatalog{draft: &catalog.Draft{GameID: "game-id"}})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/games/game-id/playtest", strings.NewReader(`{"players":[{"name":"A"},{"name":"B"}]}`))
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})

		newAuthzMux(api).ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
		}
	})

	t.Run("draft owned by somebody else reads as missing", func(t *testing.T) {
		// fakeCatalog returns a nil draft, mirroring the owner-scoped query
		// missing for a game the caller does not own.
		store := &spyStore{}
		api := New(store).
			WithIdentity(fakeIdentity{actor: &identity.Actor{User: &user}}).
			WithCatalog(&fakeCatalog{draft: nil})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/games/someone-elses-game/playtest", strings.NewReader(`{"players":[{"name":"A"},{"name":"B"}]}`))
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})
		request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
		request.Header.Set(csrfHeaderName, "csrf-token")

		newAuthzMux(api).ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
		}
		if store.saveCalled {
			t.Fatal("a session was created from a draft the caller does not own")
		}
	})
}

// TestSessionReadsAreScopedToTheCallingAccount proves the owner reaches the
// storage layer, so a leaked session ID alone is not enough to read a session.
func TestSessionReadsAreScopedToTheCallingAccount(t *testing.T) {
	user := identity.User{ID: "11111111-1111-1111-1111-111111111111", Email: "author@example.com", DisplayName: "Author"}
	store := &spyStore{}
	api := New(store).
		WithIdentity(fakeIdentity{actor: &identity.Actor{User: &user}}).
		WithCatalog(&fakeCatalog{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/sessions/session-id", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})

	newAuthzMux(api).ServeHTTP(recorder, request)

	if !store.getCalled {
		t.Fatal("the session lookup never reached the store")
	}
	if store.getOwnerUserID != user.ID {
		t.Fatalf("session lookup owner = %q, want %q", store.getOwnerUserID, user.ID)
	}
	// The spy returns no session, which is how the store reports both
	// "missing" and "not yours".
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
}

func TestSessionCommandsRequireCSRF(t *testing.T) {
	user := identity.User{ID: "11111111-1111-1111-1111-111111111111", Email: "author@example.com", DisplayName: "Author"}
	store := &spyStore{}
	api := New(store).
		WithIdentity(fakeIdentity{actor: &identity.Actor{User: &user}}).
		WithCatalog(&fakeCatalog{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/sessions/session-id/roll", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})

	newAuthzMux(api).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if store.getCalled {
		t.Fatal("a request without CSRF reached the session store")
	}
}
