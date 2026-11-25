package dns

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudflareClientUsesTheConfiguredBaseAndHidesTheToken(t *testing.T) {
	var sawAuth, sawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		sawPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":[],"result_info":{"page":1,"total_pages":1}}`))
	}))
	defer srv.Close()
	client := &CloudflareClient{Token: "cf-secret-token", Base: srv.URL + "/", HTTP: srv.Client()}
	zones, err := client.ListZones()
	if err != nil || len(zones) != 0 {
		t.Fatal(err, zones)
	}
	if sawAuth != "Bearer cf-secret-token" || !strings.Contains(sawPath, "/zones") {
		t.Fatalf("auth %q path %q", sawAuth, sawPath)
	}
	if _, err := (&CloudflareClient{Base: srv.URL, HTTP: srv.Client()}).ListZones(); err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatal(err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer bad.Close()
	if _, err := (&CloudflareClient{Token: "t", Base: bad.URL, HTTP: bad.Client()}).ListZones(); err == nil {
		t.Fatal("accepted a 403")
	}
	if (&CloudflareClient{}).base() != "https://api.cloudflare.com/client/v4" {
		t.Fatal("default base")
	}
}
