package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dockfin/dockfin/internal/config"
	"github.com/dockfin/dockfin/internal/store"
)

// RatePolicy is the per-client-IP budget. A zero limit disables that bucket.
type RatePolicy struct {
	GlobalPerMinute   int
	AuthPer15Min      int
	WebhookPerMinute  int
	SentinelPerMinute int
}

func defaultRatePolicy() RatePolicy {
	return RatePolicy{
		GlobalPerMinute:   600,
		AuthPer15Min:      30,
		WebhookPerMinute:  120,
		SentinelPerMinute: 600,
	}
}

func ratePolicyFromConfig(cfg *config.Config) RatePolicy {
	p := defaultRatePolicy()
	if cfg == nil {
		return p
	}
	p.GlobalPerMinute = cfg.RateLimitPerMinute
	p.AuthPer15Min = cfg.AuthRateLimitPer15Min
	p.WebhookPerMinute = cfg.WebhookRateLimitPerMinute
	return p
}

type limitBucket struct {
	name   string
	limit  int
	window time.Duration
}

type requestLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
	now  func() time.Time
}

func newRequestLimiter() *requestLimiter {
	return &requestLimiter{hits: map[string][]time.Time{}, now: time.Now}
}

// rateGate admits one request into a sliding window. ok is false when the
// budget is spent. retry is how long the client should wait.
type rateGate interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (ok bool, retry time.Duration)
}

func (l *requestLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, time.Duration) {
	return l.allow(key, limit, window)
}

// pgGate shares a budget in Postgres and falls back to the in-process window
// when the database cannot answer. A fallback keeps one replica protected
// during an outage instead of failing open.
type pgGate struct {
	store    *store.Store
	log      *slog.Logger
	fallback *requestLimiter
}

func (g *pgGate) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration) {
	if g == nil || g.store == nil {
		if g != nil && g.fallback != nil {
			return g.fallback.allow(key, limit, window)
		}
		return true, 0
	}
	ok, retry, err := g.store.AllowRateLimit(ctx, key, limit, window)
	if err != nil {
		if g.log != nil {
			g.log.Warn("shared rate limit unavailable", "error", err.Error())
		}
		if g.fallback != nil {
			return g.fallback.allow(key, limit, window)
		}
		return true, 0
	}
	return ok, retry
}

func (a *API) rateGate() rateGate {
	if a != nil && a.Cfg != nil && a.Cfg.RateLimitStore == "postgres" && a.Store != nil {
		return &pgGate{store: a.Store, log: a.Logger, fallback: a.requestLimiter()}
	}
	return a.requestLimiter()
}

func (l *requestLimiter) allow(key string, limit int, window time.Duration) (bool, time.Duration) {
	if limit <= 0 || window <= 0 {
		return true, 0
	}
	now := l.now()
	cut := now.Add(-window)
	l.mu.Lock()
	defer l.mu.Unlock()
	prev := l.hits[key]
	kept := prev[:0]
	for _, t := range prev {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		retry := window - now.Sub(kept[0])
		if retry < time.Second {
			retry = time.Second
		}
		l.hits[key] = kept
		return false, retry
	}
	kept = append(kept, now)
	l.hits[key] = kept
	if len(l.hits) > 20000 {
		l.gcLocked(cut)
	}
	return true, 0
}

func (l *requestLimiter) gcLocked(cut time.Time) {
	for key, times := range l.hits {
		alive := times[:0]
		for _, t := range times {
			if t.After(cut) {
				alive = append(alive, t)
			}
		}
		if len(alive) == 0 {
			delete(l.hits, key)
		} else {
			l.hits[key] = alive
		}
	}
}

func (a *API) requestLimiter() *requestLimiter {
	if a.limiter == nil {
		a.limiter = newRequestLimiter()
	}
	return a.limiter
}

func (a *API) ratePolicy() RatePolicy {
	if a != nil && a.policySet {
		return a.policy
	}
	if a != nil && a.Cfg != nil {
		return ratePolicyFromConfig(a.Cfg)
	}
	return defaultRatePolicy()
}

// classifyLimit returns an extra bucket for sensitive routes.
// The global bucket still applies unless the path is exempt.
func classifyLimit(method, path string, p RatePolicy) (bucket limitBucket, exemptGlobal bool) {
	if method == http.MethodOptions || path == "/health" || path == "/metrics" || isStaticAsset(path) {
		return limitBucket{}, true
	}
	switch {
	case isAuthLimited(method, path):
		return limitBucket{name: "auth", limit: p.AuthPer15Min, window: 15 * time.Minute}, false
	case isSentinelPath(path):
		return limitBucket{name: "sentinel", limit: p.SentinelPerMinute, window: time.Minute}, false
	case isWebhookPath(method, path):
		return limitBucket{name: "webhook", limit: p.WebhookPerMinute, window: time.Minute}, false
	default:
		return limitBucket{}, false
	}
}

func isAuthLimited(method, path string) bool {
	if method != http.MethodPost && method != http.MethodGet {
		return false
	}
	switch path {
	case "/api/v1/auth/login",
		"/api/v1/auth/login/2fa",
		"/api/v1/auth/register",
		"/api/v1/auth/forgot-password",
		"/api/v1/auth/reset-password":
		return method == http.MethodPost
	default:
		return method == http.MethodGet && strings.HasPrefix(path, "/api/v1/auth/oauth/") && strings.HasSuffix(path, "/start")
	}
}

func isWebhookPath(method, path string) bool {
	if method != http.MethodPost && method != http.MethodGet {
		return false
	}
	if path == "/api/v1/deploy" || strings.HasPrefix(path, "/api/v1/webhooks/") {
		return true
	}
	return false
}

func isSentinelPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/sentinel/")
}

func isStaticAsset(path string) bool {
	if strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/svgs/") || strings.HasPrefix(path, "/brand/") {
		return true
	}
	switch {
	case strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".css"), strings.HasSuffix(path, ".map"),
		strings.HasSuffix(path, ".png"), strings.HasSuffix(path, ".svg"), strings.HasSuffix(path, ".ico"),
		strings.HasSuffix(path, ".woff2"), strings.HasSuffix(path, ".webp"):
		return true
	default:
		return false
	}
}

func (a *API) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		policy := a.ratePolicy()
		extra, exempt := classifyLimit(r.Method, path, policy)
		ip := a.rateLimitIP(r)
		lim := a.rateGate()
		if extra.limit > 0 {
			if ok, retry := lim.Allow(r.Context(), extra.name+":"+ip, extra.limit, extra.window); !ok {
				writeRateLimited(w, retry)
				return
			}
		}
		if !exempt && policy.GlobalPerMinute > 0 {
			if ok, retry := lim.Allow(r.Context(), "global:"+ip, policy.GlobalPerMinute, time.Minute); !ok {
				writeRateLimited(w, retry)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeRateLimited(w http.ResponseWriter, retry time.Duration) {
	sec := int(retry.Seconds())
	if sec < 1 {
		sec = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(sec))
	writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
}
