package httpapi

import (
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestContainerBelongsToServer(t *testing.T) {
	server := uuid.New()
	other := uuid.New()
	app := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	svc := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	db := uuid.MustParse("99999999-8888-7777-6666-555555555555")
	resources := []containerResource{
		{Kind: "application", ID: app, ServerID: server, CustomName: "dockfin-web"},
		{Kind: "service", ID: svc, ServerID: server},
		{Kind: "database", ID: db, ServerID: server},
	}

	allow := []string{
		"dockfin-" + app.String(),
		"dockfin-" + app.String() + "-pr-12",
		"dockfin-aaaaaaaa-web-1",
		"dockfin-web",
		"dockfin-svc-11111111-api-1",
		"dockfin-db-" + db.String(),
	}
	for _, name := range allow {
		if !containerBelongsToServer(name, server, resources) {
			t.Fatalf("expected allow %s", name)
		}
	}
	deny := []string{
		"dockfin-proxy",
		"dockfin-sentinel",
		"dockfin-db",
		"postgres",
		"dockfin-" + uuid.New().String(),
		"dockfin-svc-aaaaaaaa-api-1",
		"dockfin-db-" + uuid.New().String(),
	}
	for _, name := range deny {
		if containerBelongsToServer(name, server, resources) {
			t.Fatalf("expected deny %s", name)
		}
	}
	if containerBelongsToServer("dockfin-"+app.String(), other, resources) {
		t.Fatal("container on a different server must be denied")
	}
}

func TestApplicationLogContainerAllowed(t *testing.T) {
	id := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	app := &store.Application{ID: id, BuildPack: "dockerfile"}
	if !applicationLogContainerAllowed(app, "dockfin-"+id.String(), nil) {
		t.Fatal("canonical name")
	}
	if !applicationLogContainerAllowed(app, "dockfin-"+id.String()+"-pr-3", nil) {
		t.Fatal("preview name")
	}
	if applicationLogContainerAllowed(app, "dockfin-proxy", nil) {
		t.Fatal("proxy logs")
	}
	if applicationLogContainerAllowed(app, "postgres", nil) {
		t.Fatal("arbitrary container")
	}

	compose := &store.Application{ID: id, BuildPack: "dockercompose"}
	if applicationLogContainerAllowed(compose, "dockfin-other", nil) {
		t.Fatal("compose container outside the project list")
	}
	if !applicationLogContainerAllowed(compose, "custom-web", []string{"custom-web"}) {
		t.Fatal("live compose container")
	}
	if applicationLogContainerAllowed(compose, "dockfin-proxy", []string{"dockfin-proxy"}) {
		t.Fatal("reserved name even if listed")
	}
}
