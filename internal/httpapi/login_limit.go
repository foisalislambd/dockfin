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
	if len(kept) == 0 {
		delete(l.attempts, ip)
		return true
	}
	l.attempts[ip] = kept
	if len(l.attempts) > 10000 {
		l.gcLocked(cut)
	}
	return len(kept) < loginMaxFails
}

func (l *loginLimiter) gcLocked(cut time.Time) {
	for ip, times := range l.attempts {
		alive := times[:0]
		for _, t := range times {
			if t.After(cut) {
				alive = append(alive, t)
			}
		}
		if len(alive) == 0 {
			delete(l.attempts, ip)
		} else {
			l.attempts[ip] = alive
		}
	}
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
	return requestClientIP(r)
}

func (a *API) rateLimitIP(r *http.Request) string {
	if a != nil && a.Cfg != nil && a.Cfg.TrustProxy {
		for _, h := range []string{"CF-Connecting-IP", "True-Client-IP"} {
			raw := strings.TrimSpace(r.Header.Get(h))
			if ip := net.ParseIP(raw); ip != nil {
				return ip.String()
			}
		}
	}
	return loginClientIP(r)
}
