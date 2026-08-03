package httpapi

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// rateLimiter is a fixed-window counter keyed by client address.
//
// It is deliberately per-process: a replica that is being flooded stops serving
// the flood without needing Redis on the authentication path. With N replicas
// behind a load balancer the effective ceiling is N*limit, which is still a
// hard bound on offline password guessing. Move this to the Redis backplane if
// a deployment ever needs an exact global budget.
type rateLimiter struct {
	mu       sync.Mutex
	windows  map[string]*rateWindow
	limit    int
	window   time.Duration
	lastGC   time.Time
	nowFunc  func() time.Time
	disabled bool
}

type rateWindow struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		windows:  make(map[string]*rateWindow),
		limit:    limit,
		window:   window,
		nowFunc:  time.Now,
		disabled: limit <= 0,
	}
}

// allow reports whether the key may perform another attempt in the current
// window, and how long the caller should wait when it may not.
func (l *rateLimiter) allow(key string) (bool, time.Duration) {
	if l == nil || l.disabled {
		return true, 0
	}
	now := l.nowFunc()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.collectExpired(now)

	entry, ok := l.windows[key]
	if !ok || now.After(entry.resetAt) {
		l.windows[key] = &rateWindow{count: 1, resetAt: now.Add(l.window)}
		return true, 0
	}
	if entry.count >= l.limit {
		return false, entry.resetAt.Sub(now)
	}
	entry.count++
	return true, 0
}

// collectExpired keeps the map from growing without bound under a spray of
// distinct source addresses. Callers must hold the mutex.
func (l *rateLimiter) collectExpired(now time.Time) {
	if now.Sub(l.lastGC) < l.window {
		return
	}
	l.lastGC = now
	for key, entry := range l.windows {
		if now.After(entry.resetAt) {
			delete(l.windows, key)
		}
	}
}

// clientKey identifies the caller by source IP.
//
// Proxy headers are intentionally ignored: trusting a client-supplied
// X-Forwarded-For would let an attacker reset their own budget on every
// request. Deployments behind a reverse proxy should rate limit at the proxy
// as well.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// enforceRateLimit writes a 429 and reports false when the caller is over budget.
func enforceRateLimit(w http.ResponseWriter, r *http.Request, limiter *rateLimiter) bool {
	allowed, retryAfter := limiter.allow(clientKey(r))
	if allowed {
		return true
	}
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many attempts", "wait a moment and try again")
	return false
}
