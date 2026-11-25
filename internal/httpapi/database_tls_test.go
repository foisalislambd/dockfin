package httpapi

import (
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
)

func TestDatabaseTLSMaterialRoundTrip(t *testing.T) {
	st := testkit.OpenStore(t)
	ctx := t.Context()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "tls@example.com", "TLS", "hash")
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := st.CreateProject(ctx, team.ID, "tls", "")
	if err != nil {
		t.Fatal(err)
	}
	server, err := st.CreateServer(ctx, team.ID, nil, "db-host", "", "10.1.0.5", "root", 22, "traefik")
	if err != nil {
		t.Fatal(err)
	}
	db, err := st.CreateDatabase(ctx, &store.Database{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "pg", Engine: "postgresql",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	api := &API{Store: st}
	first, err := api.prepareDatabaseTLS(ctx, team.ID, server.ID, db)
	if err != nil || len(first.Cert) == 0 || len(first.Key) == 0 || len(first.CACert) == 0 {
		t.Fatalf("%v %+v", err, first)
	}
	second, err := api.prepareDatabaseTLS(ctx, team.ID, server.ID, db)
	if err != nil || string(second.Cert) != string(first.Cert) {
		t.Fatal(err)
	}
	db.Engine = "sqlite"
	if _, err := api.prepareDatabaseTLS(ctx, team.ID, server.ID, db); err == nil {
		t.Fatal("sqlite ssl")
	}
}
