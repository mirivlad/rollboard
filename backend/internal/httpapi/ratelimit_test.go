package httpapi

import (
	"fmt"
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

func requestFrom(remote string, forwarded string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	request.RemoteAddr = remote
	if forwarded != "" {
		request.Header.Set("X-Forwarded-For", forwarded)
	}
	return request
}

func TestForwardedHeadersAreIgnoredFromAnUntrustedClient(t *testing.T) {
	// Otherwise anybody could reset their own budget on every request by
	// writing a different address in a header they control.
	trusted, err := parseTrustedProxies("10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	key := clientKey(requestFrom("203.0.113.9:44321", "1.2.3.4"), trusted)
	if key != "203.0.113.9" {
		t.Fatalf("key = %q, want the address the request actually came from", key)
	}
}

func TestForwardedHeadersAreBelievedFromATrustedProxy(t *testing.T) {
	trusted, err := parseTrustedProxies("10.0.0.0/8, 192.168.1.5")
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct{ remote, forwarded, want string }{
		{"10.0.0.7:5000", "203.0.113.9", "203.0.113.9"},
		{"192.168.1.5:5000", "203.0.113.9", "203.0.113.9"},
		// Two proxies in the chain: the client is the hop before the ones we
		// vouched for, not the leftmost value.
		{"10.0.0.7:5000", "203.0.113.9, 10.0.0.8", "203.0.113.9"},
	} {
		if key := clientKey(requestFrom(testCase.remote, testCase.forwarded), trusted); key != testCase.want {
			t.Fatalf("from %s with %q: key = %q, want %q", testCase.remote, testCase.forwarded, key, testCase.want)
		}
	}
}

func TestTwoVisitorsBehindOneProxyGetTheirOwnBudgets(t *testing.T) {
	// The bug this fixes: behind nginx every visitor keyed to the proxy, so a
	// few failed sign-ins from one person locked out everybody else.
	trusted, _ := parseTrustedProxies("10.0.0.1")
	limiter := newRateLimiter(2, time.Minute)

	for i := 0; i < 2; i++ {
		if allowed, _ := limiter.allow(clientKey(requestFrom("10.0.0.1:1", "203.0.113.9"), trusted)); !allowed {
			t.Fatalf("attempt %d from the first visitor was refused", i)
		}
	}
	if allowed, _ := limiter.allow(clientKey(requestFrom("10.0.0.1:1", "203.0.113.9"), trusted)); allowed {
		t.Fatal("the first visitor was not limited")
	}
	if allowed, _ := limiter.allow(clientKey(requestFrom("10.0.0.1:1", "198.51.100.4"), trusted)); !allowed {
		t.Fatal("a second visitor was locked out by the first")
	}
}

func TestAForgedChainCannotOutrunTheLimit(t *testing.T) {
	trusted, _ := parseTrustedProxies("10.0.0.1")
	limiter := newRateLimiter(2, time.Minute)

	// The client writes a fresh fake hop each time; nginx appends the real
	// address, so the rightmost untrusted hop is still the same person.
	keys := map[string]bool{}
	for i := 0; i < 5; i++ {
		forwarded := fmt.Sprintf("9.9.9.%d, 203.0.113.9", i)
		keys[clientKey(requestFrom("10.0.0.1:1", forwarded), trusted)] = true
	}
	if len(keys) != 1 {
		t.Fatalf("forging the chain produced %d different keys: %v", len(keys), keys)
	}
	allowed := 0
	for i := 0; i < 5; i++ {
		if ok, _ := limiter.allow(clientKey(requestFrom("10.0.0.1:1", fmt.Sprintf("9.9.9.%d, 203.0.113.9", i)), trusted)); ok {
			allowed++
		}
	}
	if allowed != 2 {
		t.Fatalf("%d attempts got through a limit of 2", allowed)
	}
}

func TestAMalformedProxyListTrustsNobody(t *testing.T) {
	if _, err := parseTrustedProxies("not-an-address"); err == nil {
		t.Fatal("a nonsense entry was accepted")
	}
	key := clientKey(requestFrom("10.0.0.1:1", "203.0.113.9"), nil)
	if key != "10.0.0.1" {
		t.Fatalf("with no trusted proxies the key was %q", key)
	}
}
