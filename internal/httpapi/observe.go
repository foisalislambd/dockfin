package httpapi

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// httpMetrics is a small Prometheus text exposition of request traffic.
// Route labels use the chi pattern, so UUIDs never become series.
type httpMetrics struct {
	mu       sync.Mutex
	counts   map[string]uint64
	sumSecs  map[string]float64
	inFlight atomic.Int64
}

func newHTTPMetrics() *httpMetrics {
	return &httpMetrics{
		counts:  map[string]uint64{},
		sumSecs: map[string]float64{},
	}
}

func metricKey(method, route, status string) string {
	return method + "\x00" + route + "\x00" + status
}

func (m *httpMetrics) observe(method, route string, status int, d time.Duration) {
	if m == nil {
		return
	}
	if route == "" {
		route = "unmatched"
	}
	key := metricKey(method, route, strconv.Itoa(status))
	m.mu.Lock()
	m.counts[key]++
	m.sumSecs[key] += d.Seconds()
	m.mu.Unlock()
}

func (m *httpMetrics) render() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var b strings.Builder
	b.WriteString("# HELP dockfin_http_requests_total HTTP requests handled by method, route pattern, and status.\n")
	b.WriteString("# TYPE dockfin_http_requests_total counter\n")
	b.WriteString("# HELP dockfin_http_request_duration_seconds_sum Total request time in seconds.\n")
	b.WriteString("# TYPE dockfin_http_request_duration_seconds_sum counter\n")
	for key, n := range m.counts {
		method, route, status := splitMetricKey(key)
		labels := fmt.Sprintf(`method=%q,route=%q,status=%q`, method, route, status)
		fmt.Fprintf(&b, "dockfin_http_requests_total{%s} %d\n", labels, n)
		fmt.Fprintf(&b, "dockfin_http_request_duration_seconds_sum{%s} %g\n", labels, m.sumSecs[key])
	}
	b.WriteString("# HELP dockfin_http_requests_in_flight Requests currently being served.\n")
	b.WriteString("# TYPE dockfin_http_requests_in_flight gauge\n")
	fmt.Fprintf(&b, "dockfin_http_requests_in_flight %d\n", m.inFlight.Load())
	return b.String()
}

func splitMetricKey(key string) (method, route, status string) {
	parts := strings.SplitN(key, "\x00", 3)
	if len(parts) != 3 {
		return "GET", "unmatched", "0"
	}
	return parts[0], parts[1], parts[2]
}

func (a *API) metrics() *httpMetrics {
	if a.httpMetrics == nil {
		a.httpMetrics = newHTTPMetrics()
	}
	return a.httpMetrics
}

func (a *API) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if !metricsAuthorized(a, r) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "metrics token required")
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(a.metrics().render()))
}

// metricsAuthorized allows /metrics when DOCKFIN_METRICS_TOKEN is unset.
// A configured token must be sent as Authorization: Bearer.
func metricsAuthorized(a *API, r *http.Request) bool {
	token := ""
	if a != nil && a.Cfg != nil {
		token = a.Cfg.MetricsToken
	}
	if token == "" {
		return true
	}
	got := ""
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		got = strings.TrimSpace(h[7:])
	}
	if len(got) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

func (a *API) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		metrics := a.metrics()
		metrics.inFlight.Add(1)
		defer metrics.inFlight.Add(-1)
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(sw, r)
		pattern := chiRoutePattern(r)
		elapsed := time.Since(start)
		status := sw.code
		if status == 0 {
			status = http.StatusOK
		}
		metrics.observe(r.Method, pattern, status, elapsed)
		a.logRequest(r, pattern, status, elapsed)
	})
}

func chiRoutePattern(r *http.Request) string {
	rc := chi.RouteContext(r.Context())
	if rc == nil {
		return ""
	}
	return rc.RoutePattern()
}

func (a *API) logRequest(r *http.Request, pattern string, status int, elapsed time.Duration) {
	if a == nil || a.Logger == nil || r == nil {
		return
	}
	path := r.URL.Path
	if path == "/health" || path == "/metrics" {
		return
	}
	level := slog.LevelInfo
	if status >= 500 {
		level = slog.LevelError
	} else if status >= 400 {
		level = slog.LevelWarn
	}
	a.Logger.Log(r.Context(), level, "request",
		"method", r.Method,
		"path", path,
		"route", pattern,
		"status", status,
		"duration_ms", elapsed.Milliseconds(),
		"request_id", chimw.GetReqID(r.Context()),
		"ip", a.rateLimitIP(r),
	)
}
