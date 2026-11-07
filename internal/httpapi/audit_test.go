package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/dockfin/dockfin/internal/config"
)

func TestAuditFromPath(t *testing.T) {
	action, rtype, rid := auditFromPath(http.MethodPost, "/api/v1/services/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/deploy")
	if rtype != "services" || rid != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" || action != "deploy" {
		t.Fatalf("got %q %q %q", action, rtype, rid)
	}
	cat, summary := classifyAudit(http.MethodPost, "/api/v1/services/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/deploy", action, rtype)
	if cat != "deploy" || !strings.Contains(summary, "Queued") {
		t.Fatalf("%s %s", cat, summary)
	}
	cat, _ = classifyAudit(http.MethodPatch, "/api/v1/team/members/x", "members", "team")
	if cat != "access" {
		t.Fatal(cat)
	}
}

func TestMetricsToken(t *testing.T) {
	open := &API{Cfg: &config.Config{Env: "development"}}
	h := open.Router()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "dockfin_http_requests_total") {
		t.Fatalf("open metrics %d %s", rec.Code, rec.Body.String())
	}

	prod := &API{Cfg: &config.Config{Env: "production"}}
	h = prod.Router()
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("production without a token %d", rec.Code)
	}

	locked := &API{Cfg: &config.Config{MetricsToken: "metrics-secret"}}
	h = locked.Router()
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("locked %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer metrics-secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bearer %d %s", rec.Code, rec.Body.String())
	}
}

func TestBuiltDashboardHasNoInlineScript(t *testing.T) {
	b, err := os.ReadFile("../../apps/web/dist/index.html")
	if err != nil {
		t.Skip("apps/web/dist is not built")
	}
	html := string(b)
	if strings.Contains(html, "<script>") || strings.Contains(html, "javascript:") {
		t.Fatalf("inline script would violate script-src 'self': %s", html)
	}
	if !strings.Contains(html, "src=\"/assets/") {
		t.Fatal(html)
	}
}

func TestDeploymentLogPayloads(t *testing.T) {
	raw := []byte(`[{"stage":"build","line":"ok"},{"stage":"run","line":"up"}]`)
	got := deploymentLogPayloads(raw)
	if len(got) != 2 || !strings.Contains(string(got[0]), "build") {
		t.Fatalf("%s", got)
	}
	if deploymentLogPayloads(nil) != nil {
		t.Fatal("nil")
	}
}
