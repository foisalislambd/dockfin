package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/version"
)

func TestMapStoreErrDeployFrozen(t *testing.T) {
	rec := httptest.NewRecorder()
	mapStoreErr(rec, store.FrozenDeployError("holiday"))
	if rec.Code != http.StatusLocked {
		t.Fatalf("code %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "holiday") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestSecurityHeadersAndRecover(t *testing.T) {
	h := securityHeaders(recoverJSON(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "internal error") {
		t.Fatalf("body %s", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type %q", ct)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal(rec.Header())
	}
	if rec.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatal(rec.Header())
	}
}

func TestHealthIncludesVersion(t *testing.T) {
	code, body := handleHealthStatus(&API{}, nil)
	if code != http.StatusOK {
		t.Fatalf("code %d", code)
	}
	if body["status"] != "ok" {
		t.Fatalf("%v", body)
	}
	if body["version"] != version.Version {
		t.Fatalf("version %#v", body["version"])
	}
}
