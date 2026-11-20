package scheduler

import (
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestResolveOwnedContainerRejectsOtherWorkloads(t *testing.T) {
	id := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	other := "dockfin-" + uuid.MustParse("11111111-2222-3333-4444-555555555555").String()
	fallback := "dockfin-" + id.String()

	if name, ok := resolveOwnedContainer("application", id, "dockfin-proxy", fallback); name != "" || ok {
		t.Fatalf("proxy accepted: %q %v", name, ok)
	}
	if name, ok := resolveOwnedContainer("application", id, other, fallback); ok || name != other {
		t.Fatalf("other app statically owned: %q %v", name, ok)
	}
	if name, ok := resolveOwnedContainer("application", id, fallback, fallback); name != fallback || !ok {
		t.Fatalf("own app: %q %v", name, ok)
	}
	preview := fallback + "-pr-3"
	if name, ok := resolveOwnedContainer("application", id, preview, fallback); name != preview || !ok {
		t.Fatalf("preview: %q %v", name, ok)
	}
	// A custom compose container_name is not statically owned; the runner
	// confirms the compose project label before exec.
	if name, ok := resolveOwnedContainer("application", id, "healthchecks", fallback); name != "healthchecks" || ok {
		t.Fatalf("custom compose name: %q %v", name, ok)
	}

	svc := "dockfin-svc-" + id.String()[:8] + "-web-1"
	if name, ok := resolveOwnedContainer("service", id, "web", ""); name != svc || !ok {
		t.Fatalf("service unit: %q %v", name, ok)
	}
	if name, ok := resolveOwnedContainer("service", id, "dockfin-proxy", ""); name != "" || ok {
		t.Fatalf("service proxy: %q %v", name, ok)
	}
	foreign := "dockfin-svc-deadbeef-web-1"
	if _, ok := resolveOwnedContainer("service", id, foreign, ""); ok {
		t.Fatal("foreign service prefix treated as owned")
	}

	db := "dockfin-db-" + id.String()
	if name, ok := resolveOwnedContainer("database", id, db, db); name != db || !ok {
		t.Fatalf("database: %q %v", name, ok)
	}
	if name, ok := resolveOwnedContainer("database", id, "dockfin-db-"+uuid.NewString(), db); name != "" || ok {
		t.Fatalf("other database: %q %v", name, ok)
	}
}

func TestApplicationTaskContainerCustomName(t *testing.T) {
	id := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	app := &store.Application{ID: id, IsConsistentContainerNameEnabled: true, CustomInternalName: "Web App"}
	if got := applicationTaskContainer(app); got != "dockfin-web-app" {
		t.Fatalf("custom: %s", got)
	}
	app.CustomInternalName = "proxy"
	if got := applicationTaskContainer(app); got != "dockfin-"+id.String() {
		t.Fatalf("reserved custom fell through to %s", got)
	}
}
