package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type attemptWindow struct {
	count int
	until time.Time
}

// A bounded, single-process limiter. Proxy headers are deliberately not trusted.
type authLimiter struct {
	mu      sync.Mutex
	clients map[string]attemptWindow
	now     func() time.Time
}

func newAuthLimiter() *authLimiter {
	return &authLimiter{clients: make(map[string]attemptWindow), now: time.Now}
}

func (l *authLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for key, window := range l.clients {
		if !now.Before(window.until) {
			delete(l.clients, key)
		}
	}
	w, exists := l.clients[ip]
	if !exists {
		if len(l.clients) >= 10000 {
			return false
		}
		w.until = now.Add(time.Minute)
	}
	if w.count >= 10 {
		return false
	}
	w.count++
	l.clients[ip] = w
	return true
}

func (l *authLimiter) wrap(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !l.allow(ip) {
			w.Header().Set("Retry-After", "60")
			writeError(w, 429, "rate_limited", "Too many authentication attempts")
			return
		}
		next(w, r)
	})
}
