package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/google/uuid"
)

func TestJobClaimIsExclusiveAndReclaimsCrashes(t *testing.T) {
	st := testkit.OpenStore(t)
	ctx := context.Background()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "jobs@example.com", "Jobs", "hash")
	if err != nil {
		t.Fatal(err)
	}

	const n = 8
	want := map[uuid.UUID]struct{}{}
	for i := 0; i < n; i++ {
		job, err := st.EnqueueJob(ctx, team.ID, "docker_cleanup", []byte(`{"i":1}`))
		if err != nil {
			t.Fatal(err)
		}
		if job.Status != "queued" || job.Attempts != 0 || job.MaxAttempts != 3 {
			t.Fatalf("enqueued job: %+v", job)
		}
		want[job.ID] = struct{}{}
	}

	var mu sync.Mutex
	got := map[uuid.UUID]int{}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			job, err := st.ClaimJob(ctx)
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			if job == nil {
				t.Error("claim returned nil while a job was queued")
				return
			}
			mu.Lock()
			got[job.ID]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(got) != n {
		t.Fatalf("claimed %d unique jobs, want %d", len(got), n)
	}
	for id, count := range got {
		if count != 1 {
			t.Fatalf("job %s claimed %d times", id, count)
		}
		if _, ok := want[id]; !ok {
			t.Fatalf("claimed unexpected job %s", id)
		}
	}
	empty, err := st.ClaimJob(ctx)
	if err != nil || empty != nil {
		t.Fatalf("queue should be empty, got %+v %v", empty, err)
	}

	// A crashed worker leaves the job running with an expired lease.
	job, err := st.EnqueueJob(ctx, team.ID, "docker_cleanup", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimJob(ctx)
	if err != nil || claimed == nil || claimed.ID != job.ID || claimed.Attempts != 1 {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE background_jobs SET locked_until=$2 WHERE id=$1`, job.ID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := st.RequeueExpiredJobs(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := st.GetJob(ctx, team.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Status != "queued" {
		t.Fatalf("expired lease status %q", again.Status)
	}
	if again.Error != "worker stopped before the job finished" {
		t.Fatalf("reclaim error %q", again.Error)
	}

	// Last attempt that dies is a failure, not another retry.
	if _, err := st.Pool.Exec(ctx, `UPDATE background_jobs SET attempts=3, status='running', locked_until=$2 WHERE id=$1`, job.ID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := st.RequeueExpiredJobs(ctx); err != nil {
		t.Fatal(err)
	}
	final, err := st.GetJob(ctx, team.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != "failed" {
		t.Fatalf("final status %q", final.Status)
	}

	if err := st.FinishJob(ctx, job.ID, []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	still, err := st.GetJob(ctx, team.ID, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if still.Status != "failed" {
		t.Fatal("finish must not revive a failed job")
	}
}

func TestDeletionTombstoneRollbackAndCrashSurface(t *testing.T) {
	st := testkit.OpenStore(t)
	ctx := context.Background()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "del@example.com", "Del", "hash")
	if err != nil {
		t.Fatal(err)
	}
	project, env, err := st.CreateProject(ctx, team.ID, "'; DROP TABLE users; --", "quote")
	if err != nil {
		t.Fatal(err)
	}
	if project.Name != "'; DROP TABLE users; --" {
		t.Fatalf("name stored incorrectly: %q", project.Name)
	}
	var users int
	if err := st.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&users); err != nil || users != 1 {
		t.Fatalf("users table after hostile name: %d %v", users, err)
	}

	app, err := st.CreateApplication(ctx, &store.Application{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "web", BuildPack: "dockerfile",
		GitBranch: "main", DockerfileLocation: "/Dockerfile", PortsExposes: "80", BaseDirectory: "/",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := st.MarkDeleting(ctx, team.ID, "application", app.ID, []byte("not-json")); err == nil {
		t.Fatal("invalid payload must roll the tombstone back")
	}
	fresh, err := st.GetApplication(ctx, team.ID, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Status == "deleting" {
		t.Fatal("failed mark left the application deleting")
	}

	payload := store.MarshalDeletionPayload(store.DeletionPayload{
		ResourceType: "application", ResourceID: app.ID.String(),
		DeleteVolumes: true, DeleteConfigurations: true, DeleteNetworks: true, DockerCleanup: true,
	})
	job, err := st.MarkDeleting(ctx, team.ID, "application", app.ID, payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.MarkDeleting(ctx, team.ID, "application", app.ID, payload); err == nil {
		t.Fatal("second delete while a job is queued must conflict")
	}
	listed, err := st.ListApplications(ctx, team.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("tombstone still listed: %+v", listed)
	}
	pending, err := st.ListPendingDeletions(ctx, team.ID)
	if err != nil || len(pending) != 1 || pending[0].ResourceID != app.ID || pending[0].Error != "" {
		t.Fatalf("pending: %+v %v", pending, err)
	}

	claimed, err := st.ClaimJob(ctx)
	if err != nil || claimed == nil || claimed.ID != job.ID {
		t.Fatalf("claim delete job: %+v %v", claimed, err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE background_jobs SET attempts=max_attempts, locked_until=$2 WHERE id=$1`, job.ID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := st.RequeueExpiredJobs(ctx); err != nil {
		t.Fatal(err)
	}
	pending, err = st.ListPendingDeletions(ctx, team.ID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending after crash: %+v %v", pending, err)
	}
	if pending[0].Error == "" {
		t.Fatal("crashed cleanup left delete_error empty, so the dashboard cannot offer retry")
	}
	active, err := st.DeleteJobActive(ctx, team.ID, app.ID)
	if err != nil || active {
		t.Fatalf("failed job should not look active: %v %v", active, err)
	}

	// Power actions that die on the last attempt must not stay "stopping".
	app2, err := st.CreateApplication(ctx, &store.Application{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "api", BuildPack: "dockerfile",
		GitBranch: "main", DockerfileLocation: "/Dockerfile", PortsExposes: "80", BaseDirectory: "/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateApplicationStatus(ctx, app2.ID, "stopping"); err != nil {
		t.Fatal(err)
	}
	power, err := st.EnqueueJob(ctx, team.ID, "resource_power", []byte(`{"resource_type":"application","resource_id":"`+app2.ID.String()+`","action":"stop"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimJob(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE background_jobs SET attempts=max_attempts, locked_until=$2 WHERE id=$1`, power.ID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := st.RequeueExpiredJobs(ctx); err != nil {
		t.Fatal(err)
	}
	stopped, err := st.GetApplication(ctx, team.ID, app2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Status != "error" {
		t.Fatalf("crashed stop left status %q", stopped.Status)
	}

	_ = project
}
