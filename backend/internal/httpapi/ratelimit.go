package httpapi

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
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

// trustedProxies decides whose forwarded headers may be believed.
//
// A header from a direct client is worthless — anybody can write
// X-Forwarded-For and reset their own budget on every request — so it is read
// only when the connection itself comes from an address the operator listed.
// Without this the limiter keyed every visitor behind nginx to the proxy's own
// address, and one person's failed sign-ins locked out everybody else.
type trustedProxies struct {
	nets []*net.IPNet
	ips  []net.IP
}

// parseTrustedProxies accepts addresses and CIDR blocks, comma separated.
func parseTrustedProxies(list string) (*trustedProxies, error) {
	trusted := &trustedProxies{}
	for _, entry := range strings.Split(list, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(entry); err == nil {
			trusted.nets = append(trusted.nets, network)
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			return nil, fmt.Errorf("%q is not an IP address or CIDR block", entry)
		}
		trusted.ips = append(trusted.ips, ip)
	}
	return trusted, nil
}

func (t *trustedProxies) has(ip net.IP) bool {
	if t == nil || ip == nil {
		return false
	}
	for _, known := range t.ips {
		if known.Equal(ip) {
			return true
		}
	}
	for _, network := range t.nets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// clientKey identifies the caller by source IP.
//
// Behind trusted proxies the chain is walked from the right, skipping hops the
// operator vouched for, and the first address that is not one of theirs is the
// client. Walking from the left would take whatever the client typed.
func clientKey(r *http.Request, trusted *trustedProxies) string {
	remote := remoteIP(r)
	if trusted == nil || !trusted.has(remote) {
		if remote == nil {
			return r.RemoteAddr
		}
		return remote.String()
	}

	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
			if ip := net.ParseIP(real); ip != nil {
				return ip.String()
			}
		}
		return remote.String()
	}
	hops := strings.Split(forwarded, ",")
	for i := len(hops) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(hops[i]))
		if ip == nil {
			// A malformed hop ends the walk: everything to its left was
			// written by somebody we have no reason to believe.
			break
		}
		if !trusted.has(ip) {
			return ip.String()
		}
	}
	// Every hop was a trusted proxy, so the nearest one is as far as we get.
	return remote.String()
}

func remoteIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(host)
}

// enforceRateLimit writes a 429 and reports false when the caller is over budget.
func enforceRateLimit(w http.ResponseWriter, r *http.Request, limiter *rateLimiter, trusted *trustedProxies) bool {
	allowed, retryAfter := limiter.allow(clientKey(r, trusted))
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
