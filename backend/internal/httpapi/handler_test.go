package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rollboard/internal/game"
	"rollboard/internal/identity"
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

func TestRegisterReturnsPublicUser(t *testing.T) {
	createdAt := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)
	api := New(fakeStore{}).WithIdentity(fakeIdentity{
		registerUser: identity.User{
			ID:           "11111111-1111-1111-1111-111111111111",
			Email:        "author@example.com",
			DisplayName:  "Author",
			PasswordHash: "must-not-be-exposed",
			CreatedAt:    createdAt,
		},
	})
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{
		"email":"author@example.com",
		"displayName":"Author",
		"password":"correct horse battery staple"
	}`))

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["email"] != "author@example.com" || response["displayName"] != "Author" {
		t.Fatalf("response = %#v, want public user", response)
	}
	if _, ok := response["PasswordHash"]; ok {
		t.Fatalf("response exposed PasswordHash: %#v", response)
	}
}

func TestRegisterDoesNotExposeInternalIdentityErrors(t *testing.T) {
	api := New(fakeStore{}).WithIdentity(fakeIdentity{registerErr: errors.New("insert user: database connection password leaked")})
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{
		"email":"author@example.com",
		"displayName":"Author",
		"password":"correct horse battery staple"
	}`))

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	var body apiError
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "INTERNAL_ERROR" || body.Error != "could not register account" || body.Details != "try again later" {
		t.Fatalf("response = %#v, want safe internal error", body)
	}
	if strings.Contains(recorder.Body.String(), "database connection password leaked") {
		t.Fatalf("response exposed internal error: %s", recorder.Body.String())
	}
}

func TestGuestEntrySetsOpaqueHTTPOnlySessionCookie(t *testing.T) {
	api := New(fakeStore{}).WithIdentity(fakeIdentity{
		guest: identity.Guest{ID: "22222222-2222-2222-2222-222222222222", DisplayName: "Guest player"},
	})
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/guest", strings.NewReader(`{"displayName":"Guest player"}`))

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	cookie, ok := cookieNamed(cookies, sessionCookieName)
	if !ok {
		t.Fatalf("cookies = %#v, want session cookie", cookies)
	}
	if cookie.Name != sessionCookieName || cookie.Value != "opaque-session-token" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie = %#v, want opaque HttpOnly Lax session cookie", cookie)
	}
	csrf, ok := cookieNamed(cookies, csrfCookieName)
	if !ok || csrf.HttpOnly || csrf.Value == "" || csrf.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookies = %#v, want readable CSRF cookie", cookies)
	}
	if strings.Contains(recorder.Body.String(), "opaque-session-token") {
		t.Fatalf("response exposed raw session token: %s", recorder.Body.String())
	}
}

func TestSessionCookiesHonorConfiguredSecurity(t *testing.T) {
	api := New(fakeStore{}).WithIdentity(fakeIdentity{
		guest: identity.Guest{ID: "22222222-2222-2222-2222-222222222222", DisplayName: "Guest player"},
	}).WithAuthOptions(AuthOptions{CookieSecure: true, SessionTTL: 2 * time.Hour})
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/guest", strings.NewReader(`{"displayName":"Guest player"}`))

	mux.ServeHTTP(recorder, request)

	for _, name := range []string{sessionCookieName, csrfCookieName} {
		cookie, ok := cookieNamed(recorder.Result().Cookies(), name)
		if !ok || !cookie.Secure || cookie.MaxAge != int((2*time.Hour).Seconds()) {
			t.Fatalf("cookie %q = %#v, want secure configured lifetime", name, cookie)
		}
	}
}

func TestLoginSetsOpaqueHTTPOnlySessionCookie(t *testing.T) {
	api := New(fakeStore{}).WithIdentity(fakeIdentity{
		loginUser: identity.User{ID: "11111111-1111-1111-1111-111111111111", Email: "author@example.com", DisplayName: "Author"},
	})
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{
		"email":"author@example.com",
		"password":"correct horse battery staple"
	}`))

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	cookie, ok := cookieNamed(cookies, sessionCookieName)
	if !ok || cookie.Value != "opaque-user-session-token" || !cookie.HttpOnly {
		t.Fatalf("cookies = %#v, want opaque HttpOnly session cookie", cookies)
	}
	if csrf, ok := cookieNamed(cookies, csrfCookieName); !ok || csrf.HttpOnly || csrf.Value == "" {
		t.Fatalf("cookies = %#v, want readable CSRF cookie", cookies)
	}
	if strings.Contains(recorder.Body.String(), "opaque-user-session-token") {
		t.Fatalf("response exposed raw session token: %s", recorder.Body.String())
	}
}

func TestMeReturnsAuthenticatedUserFromSessionCookie(t *testing.T) {
	user := identity.User{ID: "11111111-1111-1111-1111-111111111111", Email: "author@example.com", DisplayName: "Author"}
	api := New(fakeStore{}).WithIdentity(fakeIdentity{actor: &identity.Actor{SessionID: "session-id", User: &user}})
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "opaque-user-session-token"})

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["kind"] != "user" || body["user"].(map[string]any)["email"] != "author@example.com" {
		t.Fatalf("response = %#v, want authenticated public user", body)
	}
}

func TestLogoutRevokesSessionAndClearsCookie(t *testing.T) {
	api := New(fakeStore{}).WithIdentity(fakeIdentity{})
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "opaque-user-session-token"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	request.Header.Set(csrfHeaderName, "csrf-token")

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	sessionCookie, sessionOK := cookieNamed(cookies, sessionCookieName)
	csrfCookie, csrfOK := cookieNamed(cookies, csrfCookieName)
	if !sessionOK || !csrfOK || sessionCookie.MaxAge >= 0 || csrfCookie.MaxAge >= 0 {
		t.Fatalf("cookies = %#v, want expired session cookie", cookies)
	}
}

func TestLogoutRejectsMissingCSRFHeader(t *testing.T) {
	api := New(fakeStore{}).WithIdentity(fakeIdentity{})
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "opaque-user-session-token"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})

	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func cookieNamed(cookies []*http.Cookie, name string) (*http.Cookie, bool) {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie, true
		}
	}
	return nil, false
}

type fakeStore struct {
	pingErr error
}

type fakeIdentity struct {
	registerUser identity.User
	registerErr  error
	guest        identity.Guest
	loginUser    identity.User
	actor        *identity.Actor
}

func (f fakeIdentity) Register(context.Context, identity.RegistrationInput) (identity.User, error) {
	return f.registerUser, f.registerErr
}

func (f fakeIdentity) CreateGuest(context.Context, string) (identity.Guest, error) {
	return f.guest, nil
}

func (fakeIdentity) CreateGuestSession(context.Context, string, time.Time) (identity.Session, string, error) {
	return identity.Session{}, "opaque-session-token", nil
}

func (f fakeIdentity) Authenticate(context.Context, string, string) (identity.User, error) {
	return f.loginUser, nil
}

func (fakeIdentity) CreateUserSession(context.Context, string, time.Time) (identity.Session, string, error) {
	return identity.Session{}, "opaque-user-session-token", nil
}

func (f fakeIdentity) LookupSession(context.Context, string, time.Time) (*identity.Actor, error) {
	return f.actor, nil
}

func (fakeIdentity) DeleteSession(context.Context, string) error { return nil }

func (s fakeStore) Close() {}

func (s fakeStore) Ping(context.Context) error { return s.pingErr }

func (fakeStore) ListGames(context.Context) ([]storage.GameSummary, error) { return nil, nil }

func (fakeStore) GetGame(context.Context, string) (*game.GameDefinition, error) { return nil, nil }

func (fakeStore) CreateGame(context.Context, *game.GameDefinition) error { return nil }

func (fakeStore) UpdateGame(context.Context, *game.GameDefinition) error { return nil }

func (fakeStore) SaveSession(context.Context, *game.GameSession) error { return nil }

func (fakeStore) GetSession(context.Context, string) (*game.GameSession, error) { return nil, nil }

var _ storage.Store = fakeStore{}
