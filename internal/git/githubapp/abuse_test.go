package githubapp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIBaseAndCloneRejectAbuse(t *testing.T) {
	for _, base := range []string{
		"http://169.254.169.254",
		"http://0169.254.169.254",
		"file:///etc/passwd",
		"http://metadata.google.internal",
		"",
	} {
		app := &App{APIURL: base}
		if base == "" {
			app.APIURL = "http://169.254.169.254"
		}
		if _, err := app.checkedAPIBase(); err == nil {
			t.Fatalf("accepted %q", base)
		}
	}
	ok, err := (&App{APIURL: "https://api.github.com"}).checkedAPIBase()
	if err != nil || ok != "https://api.github.com" {
		t.Fatal(ok, err)
	}
	if got := APIURLFromHTML("https://acme.ghe.com"); !strings.Contains(got, "api.acme.ghe.com") {
		t.Fatal(got)
	}
	if CloneURL("https://github.com/acme/app.git", "") != "https://github.com/acme/app.git" {
		t.Fatal("empty token rewrote the URL")
	}
	if strings.Contains(CloneURL("not a url", "tok"), "169.254.169.254") {
		t.Fatal("clone rewrote a relative path into metadata")
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("missing jwt")
		}
		if r.URL.Path != "/app/installations/99/access_tokens" {
			http.Error(w, "no", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"ghs_test"}`))
	}))
	defer srv.Close()
	app := &App{AppID: "1", PrivateKeyPEM: string(pemBytes), APIURL: srv.URL, HTTP: srv.Client()}
	tok, err := app.InstallationToken("99")
	if err != nil || tok != "ghs_test" {
		t.Fatal(err, tok)
	}
	if _, err := app.InstallationToken("../admin"); err == nil {
		t.Fatal("path traversal installation id was accepted")
	}
	bad := &App{PrivateKeyPEM: "not a key", APIURL: srv.URL}
	if _, err := bad.InstallationToken("1"); err == nil {
		t.Fatal("bad pem")
	}
}
