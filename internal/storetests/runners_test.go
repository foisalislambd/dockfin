package storetests

import (
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestGithubRunners(t *testing.T) {
	w := newWorld(t, "runners@example.com", "runners")
	ctx := t.Context()
	st := w.st
	teamID := w.team.ID

	src, err := st.CreateGitSource(ctx, teamID, "github", "actions", "acme", "9001", "client", "csec", "pkey", "wh", "", "", "git", 22)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := st.CreateServer(ctx, teamID, nil, "runner-a", "", "203.0.113.20", "root", 22, "traefik")
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateServer(ctx, teamID, nil, "runner-b", "", "203.0.113.21", "root", 22, "traefik")
	if err != nil {
		t.Fatal(err)
	}

	saved, err := st.SaveGithubRunnerConfig(ctx, store.GithubRunnerConfig{
		TeamID: teamID, ServerID: srv.ID, GitSourceID: src.ID,
		Labels: []string{"linux", "x64"}, Enabled: true, AllowPullRequests: false, MaxRunners: 2,
		DockerMode: "none", RunnerImage: "ghcr.io/actions/runner:latest",
		QueueWaitMinutes: 30, IdleTimeoutMinutes: 11, JobTimeoutMinutes: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.GitSourceName != "actions" || saved.Organization != "acme" || !saved.Enabled || saved.MaxRunners != 2 || saved.RunnerImage == "" {
		t.Fatalf("saved config %+v", saved)
	}
	if _, err := st.SaveGithubRunnerConfig(ctx, store.GithubRunnerConfig{
		TeamID: teamID, ServerID: other.ID, GitSourceID: src.ID,
		Labels: []string{"linux"}, Enabled: false, MaxRunners: 1, DockerMode: "dind",
		QueueWaitMinutes: 60, IdleTimeoutMinutes: 10, JobTimeoutMinutes: 360,
	}); err != nil {
		t.Fatal(err)
	}
	enabled, err := st.EnabledRunnerConfigsForSource(ctx, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(enabled) != 1 || enabled[0].ServerID != srv.ID || enabled[0].DockerMode != "none" {
		t.Fatalf("enabled %+v", enabled)
	}

	empty, err := st.ListGithubRunnerExecutions(ctx, teamID, uuid.New(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected no executions, got %+v", empty)
	}

	prID, created, err := st.CreateRunnerExecution(ctx, store.RunnerExecutionInput{
		TeamID: teamID, GitSourceID: src.ID, TriggerJobID: 1, HTMLURL: "https://github.com/acme/web/actions/runs/1",
		WorkflowName: "ci", JobName: "test", Repository: "acme/web", Labels: []string{"linux"}, IsPullRequest: true,
	})
	if err != nil || !created || prID == uuid.Nil {
		t.Fatalf("create pr runner %v %v %v", prID, created, err)
	}
	if reserved, err := st.ReserveRunner(ctx, prID, "runner-pr", labelsMatch); err != nil || reserved != nil {
		t.Fatalf("pull requests are disabled, reserved=%+v err=%v", reserved, err)
	}
	updated, err := st.SaveGithubRunnerConfig(ctx, store.GithubRunnerConfig{
		TeamID: teamID, ServerID: srv.ID, GitSourceID: src.ID,
		Labels: []string{"linux", "x64"}, Enabled: true, AllowPullRequests: true, MaxRunners: 2,
		DockerMode: "none", RunnerImage: "ghcr.io/actions/runner:2",
		QueueWaitMinutes: 30, IdleTimeoutMinutes: 11, JobTimeoutMinutes: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.RunnerImage != "ghcr.io/actions/runner:2" || !updated.AllowPullRequests {
		t.Fatalf("updated config %+v", updated)
	}

	reserved, err := st.ReserveRunner(ctx, prID, "runner-pr", labelsMatch)
	if err != nil || reserved == nil {
		t.Fatalf("reserve pr %v %v", reserved, err)
	}
	if reserved.ServerID != srv.ID || reserved.RunnerName != "runner-pr" || reserved.Attempts != 1 || reserved.Config.ID != saved.ID {
		t.Fatalf("reserved %+v", reserved)
	}
	if again, created, err := st.CreateRunnerExecution(ctx, store.RunnerExecutionInput{
		TeamID: teamID, GitSourceID: src.ID, TriggerJobID: 1, Repository: "acme/web", Labels: []string{"linux"},
	}); err != nil || created || again != uuid.Nil {
		t.Fatalf("duplicate create %v %v %v", again, created, err)
	}

	job2, created, err := st.CreateRunnerExecution(ctx, store.RunnerExecutionInput{
		TeamID: teamID, GitSourceID: src.ID, TriggerJobID: 2, Repository: "acme/web",
		WorkflowName: "build", JobName: "package", Labels: []string{"linux"}, HTMLURL: "https://github.com/acme/web/actions/runs/2",
	})
	if err != nil || !created {
		t.Fatal(err)
	}
	second, err := st.ReserveRunner(ctx, job2, "runner-build", labelsMatch)
	if err != nil || second == nil || second.Attempts != 1 {
		t.Fatalf("reserve build %+v %v", second, err)
	}
	if taken, err := st.ReserveRunner(ctx, job2, "runner-again", labelsMatch); err != nil || taken != nil {
		t.Fatalf("second reserve of same job %+v %v", taken, err)
	}
	requeued, err := st.MarkRunnerRunning(ctx, src.ID, "runner-build", 99, "https://github.com/acme/web/actions/runs/99", "build", "package", "acme/web")
	if err != nil {
		t.Fatal(err)
	}
	if requeued == uuid.Nil {
		t.Fatal("expected original job to be queued again")
	}
	if status, err := st.RunnerStatus(ctx, job2); err != nil || status != "running" {
		t.Fatalf("running status %q %v", status, err)
	}
	if status, err := st.RunnerStatus(ctx, requeued); err != nil || status != "queued" {
		t.Fatalf("requeued status %q %v", status, err)
	}
	finished, err := st.FinishRunnerByName(ctx, src.ID, "runner-build", "success")
	if err != nil || finished == nil || finished.Status != "completed" || finished.Conclusion != "success" || finished.ID != job2 {
		t.Fatalf("finish %+v %v", finished, err)
	}
	if err := st.CancelQueuedRunner(ctx, src.ID, 2, "superseded"); err != nil {
		t.Fatal(err)
	}
	if status, err := st.RunnerStatus(ctx, requeued); err != nil || status != "cancelled" {
		t.Fatalf("cancelled requeue %q %v", status, err)
	}

	idleOK, err := st.MarkRunnerIdle(ctx, prID, 55)
	if err != nil || !idleOK {
		t.Fatalf("idle %v %v", idleOK, err)
	}
	if status, err := st.RunnerStatus(ctx, prID); err != nil || status != "idle" {
		t.Fatalf("idle status %q %v", status, err)
	}
	if again, err := st.MarkRunnerIdle(ctx, prID, 55); err != nil || again {
		t.Fatalf("idle twice %v %v", again, err)
	}
	same, err := st.MarkRunnerRunning(ctx, src.ID, "runner-pr", 1, "", "", "", "")
	if err != nil || same != uuid.Nil {
		t.Fatalf("same job requeue %v %v", same, err)
	}
	if status, err := st.RunnerStatus(ctx, prID); err != nil || status != "running" {
		t.Fatalf("pr running %q %v", status, err)
	}

	job3, _, err := st.CreateRunnerExecution(ctx, store.RunnerExecutionInput{
		TeamID: teamID, GitSourceID: src.ID, TriggerJobID: 3, Repository: "acme/web", Labels: []string{"linux"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The finished runner frees a slot; max is 2 and the PR runner is still running.
	third, err := st.ReserveRunner(ctx, job3, "runner-retry", labelsMatch)
	if err != nil || third == nil {
		t.Fatalf("reserve retry %+v %v", third, err)
	}
	if err := st.RequeueRunner(ctx, job3, "image pull failed"); err != nil {
		t.Fatal(err)
	}
	if status, err := st.RunnerStatus(ctx, job3); err != nil || status != "queued" {
		t.Fatalf("requeue status %q %v", status, err)
	}
	if err := st.CancelQueuedRunner(ctx, src.ID, 3, "cancelled by user"); err != nil {
		t.Fatal(err)
	}
	if status, err := st.RunnerStatus(ctx, job3); err != nil || status != "cancelled" {
		t.Fatalf("cancel queued %q %v", status, err)
	}

	job4, _, err := st.CreateRunnerExecution(ctx, store.RunnerExecutionInput{
		TeamID: teamID, GitSourceID: src.ID, TriggerJobID: 4, Repository: "acme/web", Labels: []string{"linux"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReserveRunner(ctx, job4, "runner-fail", labelsMatch); err != nil {
		t.Fatal(err)
	}
	if err := st.SetRunnerResult(ctx, job4, 77, "failed", "disk full"); err != nil {
		t.Fatal(err)
	}
	if status, err := st.RunnerStatus(ctx, job4); err != nil || status != "failed" {
		t.Fatalf("failed status %q %v", status, err)
	}

	job5, _, err := st.CreateRunnerExecution(ctx, store.RunnerExecutionInput{
		TeamID: teamID, GitSourceID: src.ID, TriggerJobID: 5, Repository: "acme/web", Labels: []string{"linux"},
	})
	if err != nil {
		t.Fatal(err)
	}
	maint, err := st.ListRunnerMaintenance(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var sawQueued, sawRunning bool
	for _, item := range maint {
		if item.Execution.ID == job5 && item.Execution.Status == "queued" {
			sawQueued = true
			if item.IdleTimeoutMinutes != 10 || item.JobTimeoutMinutes != 360 {
				t.Fatalf("queued row timeouts %+v", item)
			}
		}
		if item.Execution.ID == prID && item.Execution.Status == "running" {
			sawRunning = true
			if item.IdleTimeoutMinutes != 11 || item.JobTimeoutMinutes != 120 {
				t.Fatalf("timeouts %+v", item)
			}
		}
	}
	if !sawQueued || !sawRunning {
		t.Fatalf("maintenance missing rows queued=%v running=%v %+v", sawQueued, sawRunning, maint)
	}

	listed, err := st.ListGithubRunnerExecutions(ctx, teamID, srv.ID, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) < 4 {
		t.Fatalf("listed %d executions", len(listed))
	}
	found := false
	for _, row := range listed {
		if row.ID == job4 && row.ErrorMessage == "disk full" && row.RunnerID != nil && *row.RunnerID == 77 {
			found = true
		}
	}
	if !found {
		t.Fatalf("failed execution not listed: %+v", listed)
	}

	if group, err := st.GitSourceRunnerGroup(ctx, src.ID); err != nil || group != 0 {
		t.Fatalf("group before set %d %v", group, err)
	}
	if err := st.SetGitSourceRunnerGroup(ctx, src.ID, 77); err != nil {
		t.Fatal(err)
	}
	if group, err := st.GitSourceRunnerGroup(ctx, src.ID); err != nil || group != 77 {
		t.Fatalf("group %d %v", group, err)
	}
	if _, err := st.GitSourceRunnerGroup(ctx, uuid.New()); err == nil {
		t.Fatal("missing git source returned a group")
	}
}

func labelsMatch(custom, requested []string) bool {
	have := map[string]struct{}{}
	for _, label := range custom {
		have[label] = struct{}{}
	}
	for _, label := range requested {
		if _, ok := have[label]; !ok {
			return false
		}
	}
	return true
}
