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
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/google/uuid"
)

func TestDeleteRetryAndForget(t *testing.T) {
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
	do := func(method, path string, body any) (int, map[string]any) {
		t.Helper()
		var rdr io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rdr = bytes.NewReader(b)
		}
		req, err := http.NewRequest(method, srv.URL+path, rdr)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var parsed map[string]any
		_ = json.Unmarshal(raw, &parsed)
		return res.StatusCode, parsed
	}
	code, me := do(http.MethodGet, "/api/v1/auth/me", nil)
	if code != http.StatusOK {
		t.Fatalf("me %d %v", code, me)
	}
	teamID := uuid.MustParse(me["team"].(map[string]any)["id"].(string))
	code, created := do(http.MethodPost, "/api/v1/projects", map[string]string{"name": "del"})
	if code != http.StatusCreated {
		t.Fatalf("project %d %v", code, created)
	}
	envID := uuid.MustParse(created["environment"].(map[string]any)["id"].(string))
	ctx := t.Context()
	app, err := st.CreateApplication(ctx, &store.Application{
		TeamID: teamID, EnvironmentID: envID, Name: "gone", BuildPack: "dockerfile",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := st.CreateService(ctx, &store.Service{TeamID: teamID, EnvironmentID: envID, Name: "gone-svc"})
	if err != nil {
		t.Fatal(err)
	}
	db, err := st.CreateDatabase(ctx, &store.Database{
		TeamID: teamID, EnvironmentID: envID, Name: "gone-db", Engine: "postgresql",
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	refuseWhileBusy(rec, "deleting")
	if rec.Code != http.StatusConflict {
		t.Fatal(rec.Code)
	}
	if refuseWhileBusy(httptest.NewRecorder(), "running") {
		t.Fatal("running was treated as busy")
	}

	for _, kind := range []struct {
		kind string
		id   uuid.UUID
	}{
		{"application", app.ID},
		{"service", svc.ID},
		{"database", db.ID},
	} {
		job, err := api.scheduleDelete(ctx, teamID, kind.kind, kind.id, DeleteOptions{}, uuid.Nil, 0)
		if err != nil {
			t.Fatalf("schedule %s: %v", kind.kind, err)
		}
		out, err := api.jobResourceDelete(ctx, job)
		if err != nil {
			t.Fatalf("job %s: %v", kind.kind, err)
		}
		if !bytes.Contains(out, []byte("deleted")) {
			t.Fatal(string(out))
		}
		if _, err := st.GetApplicationAny(ctx, teamID, app.ID); kind.kind == "application" && err == nil {
			t.Fatal("application still present")
		}
	}
	if _, err := api.jobResourceDelete(ctx, &store.BackgroundJob{Payload: []byte("{")}); err == nil {
		t.Fatal("bad payload")
	}
	if _, err := api.jobResourceDelete(ctx, &store.BackgroundJob{Payload: []byte(`{"resource_type":"nope","resource_id":"` + uuid.NewString() + `"}`)}); err == nil {
		t.Fatal("unknown type")
	}

	again, err := st.CreateApplication(ctx, &store.Application{
		TeamID: teamID, EnvironmentID: envID, Name: "retry", BuildPack: "dockerfile",
	})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	if _, err := api.scheduleDelete(ctx, teamID, "application", again.ID, DeleteOptions{DeleteVolumes: &yes, DeleteConfigurations: &yes, DeleteNetworks: &yes, DockerCleanup: &yes}, uuid.Nil, 0); err != nil {
		t.Fatal(err)
	}
	code, listed := do(http.MethodGet, "/api/v1/deletions", nil)
	if code != http.StatusOK {
		t.Fatalf("list %d %v", code, listed)
	}
	body := map[string]string{"resource_type": "application", "resource_id": again.ID.String()}
	code, retried := do(http.MethodPost, "/api/v1/deletions/retry", body)
	if code != http.StatusAccepted && code != http.StatusConflict {
		t.Fatalf("retry %d %v", code, retried)
	}
	code, forgotten := do(http.MethodPost, "/api/v1/deletions/forget", body)
	if code != http.StatusOK && code != http.StatusConflict && code != http.StatusNotFound {
		t.Fatalf("forget %d %v", code, forgotten)
	}
	code, bad := do(http.MethodPost, "/api/v1/deletions/forget", map[string]string{"resource_type": "nope", "resource_id": uuid.NewString()})
	if code != http.StatusBadRequest {
		t.Fatalf("bad kind %d %v", code, bad)
	}
}
