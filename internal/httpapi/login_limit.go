package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Simple per-IP login throttle to slow credential stuffing.
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

var globalLoginLimiter = &loginLimiter{attempts: map[string][]time.Time{}}

const (
	loginWindow   = 15 * time.Minute
	loginMaxFails = 20
)

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cut := now.Add(-loginWindow)
	kept := l.attempts[ip][:0]
	for _, t := range l.attempts[ip] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	l.attempts[ip] = kept
	return len(kept) < loginMaxFails
}

func (l *loginLimiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts[ip] = append(l.attempts[ip], time.Now())
}

func (l *loginLimiter) success(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

func loginClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
