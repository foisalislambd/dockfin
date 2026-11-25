package scheduler

import (
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestTaskNamesAndCronRejectAbuse(t *testing.T) {
	if got := sanitizeTaskContainer("Web; rm -rf /"); got != "web-rm-rf" && strings.ContainsAny(got, ";/ ") {
		t.Fatalf("container %q", got)
	}
	if sanitizeTaskContainer("dockfin-proxy") == "" {
		t.Fatal("sanitize emptied a reserved-looking name")
	}
	if !reservedTaskContainer("dockfin-proxy") || reservedTaskContainer("dockfin-web") {
		t.Fatal("reserved set")
	}
	if shellQuote("a'b") != `'a'"'"'b'` {
		t.Fatal(shellQuote("a'b"))
	}
	id := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	name := defaultServiceContainer(id, "services:\n  redis:\n    image: redis:7\n")
	if !strings.HasPrefix(name, "dockfin-svc-aaaaaaaa-redis") {
		t.Fatal(name)
	}
	if defaultServiceContainer(id, "not compose") != "" {
		t.Fatal("empty compose produced a container")
	}
	app := &store.Application{ID: id, IsConsistentContainerNameEnabled: true, CustomInternalName: "proxy"}
	if got := applicationTaskContainer(app); got == "dockfin-proxy" || strings.Contains(got, ";") {
		t.Fatalf("custom %q", got)
	}
	if applicationTaskContainer(nil) != "" {
		t.Fatal("nil app")
	}

	now := time.Date(2026, 7, 30, 12, 5, 0, 0, time.UTC)
	for _, expr := range []string{"* * * * *; id", "5 12 * * *\nrm -rf", "$(id) * * * *", ""} {
		if Matches(expr, now) {
			t.Fatalf("matched %q", expr)
		}
	}
	if !Matches("5 12 30 7 4", now) {
		t.Fatal("exact fields")
	}
	if Matches("5 12 31 2 *", now) {
		t.Fatal("impossible day matched")
	}
}
