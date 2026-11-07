package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/config"
	"gopkg.in/yaml.v3"
)

func TestRateLimiterWindow(t *testing.T) {
	lim := newRequestLimiter()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lim.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		ok, _ := lim.allow("auth:1.1.1.1", 3, time.Minute)
		if !ok {
			t.Fatalf("allow %d", i)
		}
	}
	ok, retry := lim.allow("auth:1.1.1.1", 3, time.Minute)
	if ok || retry < time.Second {
		t.Fatalf("ok %v retry %s", ok, retry)
	}
	ok, _ = lim.allow("auth:2.2.2.2", 3, time.Minute)
	if !ok {
		t.Fatal("other key")
	}
	now = now.Add(time.Minute + time.Second)
	ok, _ = lim.allow("auth:1.1.1.1", 3, time.Minute)
	if !ok {
		t.Fatal("window should reset")
	}
}

func TestClassifyAuthAndWebhook(t *testing.T) {
	p := defaultRatePolicy()
	bucket, exempt := classifyLimit(http.MethodPost, "/api/v1/auth/login", p)
	if exempt || bucket.name != "auth" || bucket.limit != p.AuthPer15Min {
		t.Fatalf("%+v exempt %v", bucket, exempt)
	}
	bucket, _ = classifyLimit(http.MethodPost, "/api/v1/webhooks/git/abc", p)
	if bucket.name != "webhook" {
		t.Fatal(bucket.name)
	}
	bucket, _ = classifyLimit(http.MethodPost, "/api/v1/deploy", p)
	if bucket.name != "webhook" {
		t.Fatal(bucket.name)
	}
	_, exempt = classifyLimit(http.MethodGet, "/health", p)
	if !exempt {
		t.Fatal("health")
	}
	_, exempt = classifyLimit(http.MethodGet, "/assets/app.js", p)
	if !exempt {
		t.Fatal("asset")
	}
}

func TestRouterRateLimitAuth(t *testing.T) {
	a := &API{Cfg: &config.Config{}}
	a.policy = RatePolicy{GlobalPerMinute: 1000, AuthPer15Min: 2, WebhookPerMinute: 1000, SentinelPerMinute: 1000}
	a.policySet = true
	h := a.Router()
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
		req.RemoteAddr = "198.51.100.10:1234"
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("early 429 on %d", i)
		}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	req.RemoteAddr = "198.51.100.10:1234"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	if !strings.Contains(rec.Body.String(), "rate limit exceeded") {
		t.Fatal(rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "198.51.100.10:1234"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health %d", rec.Code)
	}
}

func TestOpenAPIServedAndComplete(t *testing.T) {
	a := &API{Cfg: &config.Config{Env: "development"}}
	h := a.Router()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatal(ct)
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	paths, _ := doc["paths"].(map[string]any)
	if _, ok := paths["/api/v1/overview"]; !ok {
		t.Fatal("missing overview")
	}
	freeze, _ := paths["/api/v1/projects/{projectID}/environments/{envID}/freeze"].(map[string]any)
	if freeze["put"] == nil {
		t.Fatal("missing freeze")
	}
	if _, ok := paths["/health"]; !ok {
		t.Fatal("missing health")
	}
	for _, op := range a.openAPIOps {
		if strings.Contains(op.Path, "*") {
			t.Fatal(op.Path)
		}
	}
	if len(a.openAPIOps) < 100 {
		t.Fatalf("expected a full route list, got %d", len(a.openAPIOps))
	}

	docs := httptest.NewRecorder()
	h.ServeHTTP(docs, httptest.NewRequest(http.MethodGet, "/api/docs", nil))
	if docs.Code != http.StatusOK || !strings.Contains(docs.Body.String(), "/api/v1/overview") {
		t.Fatalf("docs %d", docs.Code)
	}

	metrics := httptest.NewRecorder()
	h.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK || !strings.Contains(metrics.Body.String(), "dockfin_http_requests_total") {
		t.Fatalf("metrics %d %s", metrics.Code, metrics.Body.String())
	}
}

func TestRouterGlobalLimit(t *testing.T) {
	a := &API{Cfg: &config.Config{}}
	a.policy = RatePolicy{GlobalPerMinute: 2, AuthPer15Min: 100, WebhookPerMinute: 100, SentinelPerMinute: 100}
	a.policySet = true
	h := a.Router()
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
		req.RemoteAddr = "198.51.100.20:9"
		h.ServeHTTP(rec, req)
		if i < 2 && rec.Code == http.StatusTooManyRequests {
			t.Fatalf("early 429 on %d", i)
		}
		if i == 2 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("code %d", rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/index.js", nil)
	req.RemoteAddr = "198.51.100.20:9"
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatal("static asset should not consume the API budget")
	}
}

func TestOpenAPIFileMatchesRouter(t *testing.T) {
	a := &API{Cfg: &config.Config{
		RateLimitPerMinute:        600,
		AuthRateLimitPer15Min:     30,
		WebhookRateLimitPerMinute: 120,
	}}
	_ = a.Router()
	got := a.openAPIYAML
	path := filepath.Join("..", "..", "packages", "openapi", "openapi.yaml")
	if os.Getenv("DOCKFIN_UPDATE_OPENAPI") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got)) {
		t.Fatalf("packages/openapi/openapi.yaml is out of date with the router (%d vs %d bytes). Regenerate with DOCKFIN_UPDATE_OPENAPI=1 go test ./internal/httpapi/ -run TestOpenAPIFileMatchesRouter", len(want), len(got))
	}
	var doc map[string]any
	if err := yaml.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Fatal(doc["openapi"])
	}
	info, _ := doc["info"].(map[string]any)
	desc, _ := info["description"].(string)
	if !strings.Contains(desc, "600 requests/minute") || !strings.Contains(desc, "30 auth") || !strings.Contains(desc, "120 webhook") {
		t.Fatal(desc)
	}
	if info["version"] != "dev" {
		t.Fatal(info["version"])
	}
}
