package worker

import (
	"testing"

	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/google/uuid"
)

func TestHandleServiceFailsClosed(t *testing.T) {
	st := testkit.OpenStore(t)
	ctx := t.Context()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "svc-worker@example.com", "Svc", "hash")
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := st.CreateProject(ctx, team.ID, "svc", "")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := st.CreateService(ctx, &store.Service{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "empty", ServiceType: "custom",
	})
	if err != nil {
		t.Fatal(err)
	}
	dep, err := st.CreateServiceDeployment(ctx, team.ID, empty.ID, nil, false, false, true)
	if err != nil {
		t.Fatal(err)
	}
	q := NewQueue(st, sshx.NewPool(), nil, 1)
	q.handle(ctx, DeployJob{DeploymentID: dep.ID, TeamID: team.ID})
	got, err := st.GetDeployment(ctx, team.ID, dep.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "failed" || got.ErrorMessage == "" {
		t.Fatalf("empty compose %s %q", got.Status, got.ErrorMessage)
	}

	composed, err := st.CreateService(ctx, &store.Service{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "redis", ServiceType: "custom",
		DockerComposeRaw: "services:\n  redis:\n    image: redis:7\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	dep, err = st.CreateServiceDeployment(ctx, team.ID, composed.ID, nil, false, false, true)
	if err != nil {
		t.Fatal(err)
	}
	q.handle(ctx, DeployJob{DeploymentID: dep.ID, TeamID: team.ID})
	got, err = st.GetDeployment(ctx, team.ID, dep.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "failed" {
		t.Fatalf("no destination %s %q", got.Status, got.ErrorMessage)
	}
	if got.ErrorMessage == "" {
		t.Fatal("missing error")
	}

	missing := uuid.New()
	q.handle(ctx, DeployJob{DeploymentID: missing, TeamID: team.ID})
}
