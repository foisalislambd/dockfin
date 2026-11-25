package storetests

import (
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
)

type world struct {
	st      *store.Store
	user    *store.User
	team    *store.Team
	project *store.Project
	env     *store.Environment
}

func newWorld(t *testing.T, email, projectName string) *world {
	t.Helper()
	st := testkit.OpenStore(t)
	user, team, err := st.CreateUserWithPersonalTeam(t.Context(), email, "Tester", "hash")
	if err != nil {
		t.Fatal(err)
	}
	project, env, err := st.CreateProject(t.Context(), team.ID, projectName, "desc")
	if err != nil {
		t.Fatal(err)
	}
	return &world{st: st, user: user, team: team, project: project, env: env}
}

func (w *world) app(t *testing.T, name, pack string) *store.Application {
	t.Helper()
	app, err := w.st.CreateApplication(t.Context(), &store.Application{
		TeamID: w.team.ID, EnvironmentID: w.env.ID, Name: name, BuildPack: pack,
		Dockerfile: "FROM nginx:alpine\n", GitRepository: "https://github.com/acme/web.git", GitBranch: "main",
		PortsExposes: "80", BaseDirectory: "/",
	})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func (w *world) database(t *testing.T, name string) *store.Database {
	t.Helper()
	cred, err := w.st.Box.EncryptString("db-secret")
	if err != nil {
		t.Fatal(err)
	}
	db, err := w.st.CreateDatabase(t.Context(), &store.Database{
		TeamID: w.team.ID, EnvironmentID: w.env.ID, Name: name, Engine: "postgresql",
	}, cred)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func (w *world) service(t *testing.T, name string) *store.Service {
	t.Helper()
	svc, err := w.st.CreateService(t.Context(), &store.Service{
		TeamID: w.team.ID, EnvironmentID: w.env.ID, Name: name, ServiceType: "n8n",
		DockerComposeRaw: "services:\n  n8n:\n    image: n8nio/n8n\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func encrypt(t *testing.T, st *store.Store, plain string) string {
	t.Helper()
	out, err := st.Box.EncryptString(plain)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustFind[T any](t *testing.T, items []T, ok func(T) bool) T {
	t.Helper()
	for _, item := range items {
		if ok(item) {
			return item
		}
	}
	t.Fatalf("item not found in %d rows", len(items))
	var zero T
	return zero
}
