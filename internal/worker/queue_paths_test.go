package worker

import (
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/google/uuid"
)

func TestHandleFailsClosedWithoutSSH(t *testing.T) {
	st := testkit.OpenStore(t)
	ctx := t.Context()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "worker@example.com", "Worker", "hash")
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := st.CreateProject(ctx, team.ID, "w", "")
	if err != nil {
		t.Fatal(err)
	}
	app, err := st.CreateApplication(ctx, &store.Application{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "web", BuildPack: "dockerfile", Dockerfile: "FROM scratch\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	q := NewQueue(st, nil, nil, 1)

	missing := DeployJob{DeploymentID: uuid.New(), TeamID: team.ID}
	q.handle(ctx, missing)
	got, err := st.GetDeployment(ctx, team.ID, missing.DeploymentID)
	if err == nil && got != nil && got.Status != "failed" {
		t.Fatalf("missing deployment status %s", got.Status)
	}

	dep, err := st.CreateDeployment(ctx, team.ID, app.ID, nil, "", "", false, false, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetDeploymentStatus(ctx, dep.ID, "finished", ""); err != nil {
		t.Fatal(err)
	}
	q.handle(ctx, DeployJob{DeploymentID: dep.ID, TeamID: team.ID})
	got, err = st.GetDeployment(ctx, team.ID, dep.ID)
	if err != nil || got.Status != "finished" {
		t.Fatalf("terminal deployment was mutated: %+v %v", got, err)
	}

	queued, err := st.CreateDeployment(ctx, team.ID, app.ID, nil, "", "", false, false, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	q.handle(ctx, DeployJob{DeploymentID: queued.ID, TeamID: team.ID})
	got, err = st.GetDeployment(ctx, team.ID, queued.ID)
	if err != nil || got.Status != "failed" {
		t.Fatalf("no-destination status %s %v", got.Status, err)
	}
	if got.ErrorMessage == "" {
		t.Fatal("expected a failure reason")
	}
}

func TestHandleServiceFailsWithoutComposeOrKey(t *testing.T) {
	st := testkit.OpenStore(t)
	ctx := t.Context()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "svc-worker@example.com", "Worker", "hash")
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := st.CreateProject(ctx, team.ID, "w", "")
	if err != nil {
		t.Fatal(err)
	}
	svc, err := st.CreateService(ctx, &store.Service{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "empty", ServiceType: "n8n",
	})
	if err != nil {
		t.Fatal(err)
	}
	dep, err := st.CreateServiceDeployment(ctx, team.ID, svc.ID, nil, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	q := NewQueue(st, nil, nil, 1)
	q.handle(ctx, DeployJob{DeploymentID: dep.ID, TeamID: team.ID})
	got, err := st.GetDeployment(ctx, team.ID, dep.ID)
	if err != nil || got.Status != "failed" || got.ErrorMessage == "" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestEnqueueQueueFull(t *testing.T) {
	st := testkit.OpenStore(t)
	ctx := t.Context()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "full@example.com", "Full", "hash")
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := st.CreateProject(ctx, team.ID, "f", "")
	if err != nil {
		t.Fatal(err)
	}
	app, err := st.CreateApplication(ctx, &store.Application{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "web", BuildPack: "dockerfile", Dockerfile: "FROM scratch\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	dep, err := st.CreateDeployment(ctx, team.ID, app.ID, nil, "", "", false, false, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	q := &Queue{Store: st, jobs: make(chan DeployJob)}
	if err := q.Enqueue(DeployJob{DeploymentID: dep.ID, TeamID: team.ID}); err == nil {
		t.Fatal("expected queue full")
	}
	got, err := st.GetDeployment(ctx, team.ID, dep.ID)
	if err != nil || got.Status != "failed" {
		t.Fatalf("status %v %v", got, err)
	}
}
