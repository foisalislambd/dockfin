package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/config"
	"github.com/dockfin/dockfin/internal/testkit"
)

func TestCreateApplicationRejectsTraversalAndBadPacks(t *testing.T) {
	t.Chdir(t.TempDir())
	st := testkit.OpenStore(t)
	api := &API{
		Cfg: &config.Config{
			Env: "development", PublicURL: "http://127.0.0.1:8000",
			MasterKey: "test-master-key-for-integration-32b!", SessionTTL: time.Hour,
		},
		Store:  st,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	srv := httptest.NewServer(api.Router())
	t.Cleanup(srv.Close)
	token := registerOwner(t, srv.Client(), srv.URL)
	team := mustTeam(t, st, "owner@example.com")
	_, env, err := st.CreateProject(t.Context(), team, "create", "")
	if err != nil {
		t.Fatal(err)
	}
	do := func(body map[string]any) (int, []byte) {
		t.Helper()
		raw, _ := json.Marshal(body)
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/applications", bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		out, _ := io.ReadAll(res.Body)
		return res.StatusCode, out
	}
	base := map[string]any{
		"environment_id": env.ID.String(), "name": "web", "build_pack": "dockerfile",
		"dockerfile": "FROM nginx:alpine\nEXPOSE 80\n",
	}
	bad := map[string]any{}
	for k, v := range base {
		bad[k] = v
	}
	bad["base_directory"] = "/../../etc"
	code, raw := do(bad)
	if code != http.StatusBadRequest || !bytes.Contains(raw, []byte("base_directory")) {
		t.Fatalf("traversal %d %s", code, raw)
	}
	bad["base_directory"] = "/"
	bad["fqdn"] = "http://169.254.169.254/latest"
	code, raw = do(bad)
	if code != http.StatusBadRequest || !bytes.Contains(bytes.ToLower(raw), []byte("fqdn")) {
		t.Fatalf("fqdn %d %s", code, raw)
	}
	bad["fqdn"] = ""
	bad["name"] = ""
	code, raw = do(bad)
	if code < 400 {
		t.Fatalf("blank name %d %s", code, raw)
	}
	code, raw = do(base)
	if code != http.StatusCreated && code != http.StatusOK {
		t.Fatalf("create %d %s", code, raw)
	}
	if bytes.Contains(raw, []byte("169.254.169.254")) {
		t.Fatal(string(raw))
	}
}
