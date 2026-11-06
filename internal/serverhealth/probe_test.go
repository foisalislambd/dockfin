package serverhealth

import (
	"errors"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestProbeUnreachableThenRecoverFlag(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	prev := store.ServerHealth{ServerID: uuid.New(), TeamID: uuid.New(), DiskMount: "/", PatchNotifiedCount: -1}
	next, notes := Probe(nil, prev, "web-1", 80, "traefik", false, errors.New("timeout"), now)
	if next.ConsecutiveFailures != 1 || len(notes) != 0 || next.Reachable {
		t.Fatalf("%+v %v", next, notes)
	}
	next, notes = Probe(nil, next, "web-1", 80, "traefik", false, errors.New("timeout"), now)
	if !next.UnreachableNotified || len(notes) != 1 || notes[0].Type != "server_unreachable" {
		t.Fatalf("%+v %v", next, notes)
	}
	next, notes = Probe(nil, next, "web-1", 80, "traefik", false, errors.New("timeout"), now)
	if len(notes) != 0 || next.ConsecutiveFailures != 3 {
		t.Fatal("should not notify again")
	}
}
