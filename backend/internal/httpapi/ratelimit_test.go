package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRateLimiterAllowsUpToLimitThenBlocks(t *testing.T) {
	limiter := newRateLimiter(3, time.Minute)
	for attempt := 1; attempt <= 3; attempt++ {
		if allowed, _ := limiter.allow("10.0.0.1"); !allowed {
			t.Fatalf("attempt %d was blocked, want allowed", attempt)
		}
	}
	allowed, retryAfter := limiter.allow("10.0.0.1")
	if allowed {
		t.Fatal("attempt over the limit was allowed")
	}
	if retryAfter <= 0 {
		t.Fatalf("retryAfter = %v, want a positive duration", retryAfter)
	}
}

func TestRateLimiterIsolatesKeys(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)
	if allowed, _ := limiter.allow("10.0.0.1"); !allowed {
		t.Fatal("first key was blocked on its first attempt")
	}
	if allowed, _ := limiter.allow("10.0.0.2"); !allowed {
		t.Fatal("a different source IP was blocked by another IP's budget")
	}
	if allowed, _ := limiter.allow("10.0.0.1"); allowed {
		t.Fatal("first key exceeded its own budget")
	}
}

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	now := time.Now()
	limiter := newRateLimiter(1, time.Minute)
	limiter.nowFunc = func() time.Time { return now }

	if allowed, _ := limiter.allow("10.0.0.1"); !allowed {
		t.Fatal("first attempt was blocked")
	}
	if allowed, _ := limiter.allow("10.0.0.1"); allowed {
		t.Fatal("second attempt inside the window was allowed")
	}

	now = now.Add(2 * time.Minute)
	if allowed, _ := limiter.allow("10.0.0.1"); !allowed {
		t.Fatal("attempt after the window elapsed was still blocked")
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	api := New(&spyStore{}).WithIdentity(fakeIdentity{})
	api.authLimiter = newRateLimiter(2, time.Minute)
	mux := newAuthzMux(api)

	for attempt := 1; attempt <= 2; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@example.com","password":"guessing"}`))
		mux.ServeHTTP(recorder, request)
		if recorder.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d was rate limited too early", attempt)
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@example.com","password":"guessing"}`))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusTooManyRequests, recorder.Body.String())
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header is missing from the rate limited response")
	}
}

func TestRegisterIsRateLimited(t *testing.T) {
	api := New(&spyStore{}).WithIdentity(fakeIdentity{})
	api.authLimiter = newRateLimiter(1, time.Minute)
	mux := newAuthzMux(api)

	body := `{"email":"a@example.com","displayName":"A","password":"correct-horse-battery"}`
	first := httptest.NewRecorder()
	mux.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body)))
	if first.Code == http.StatusTooManyRequests {
		t.Fatal("the first registration was rate limited")
	}

	second := httptest.NewRecorder()
	mux.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body)))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d; body=%s", second.Code, http.StatusTooManyRequests, second.Body.String())
	}
}
