package deploy

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/store"
)

func TestContainerNameCandidateSuffix(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	app := &store.Application{ID: id, Name: "demo"}
	name := containerNameFor(Request{App: app})
	candidate := name + "-new"
	if !strings.HasSuffix(candidate, "-new") {
		t.Fatalf("candidate = %q", candidate)
	}
	if candidate == name {
		t.Fatal("candidate must differ from production name")
	}
}
