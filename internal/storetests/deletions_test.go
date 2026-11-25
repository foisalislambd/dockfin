package storetests

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestDeletions(t *testing.T) {
	w := newWorld(t, "delete@example.com", "deletes")
	ctx := t.Context()
	st := w.st
	teamID := w.team.ID
	app := w.app(t, "web", "dockerfile")
	db := w.database(t, "pg")
	svc := w.service(t, "n8n")
	preview, err := st.CreatePreview(ctx, teamID, app.ID, 12, "Add preview", "feature", "pr.example.com")
	if err != nil {
		t.Fatal(err)
	}

	previewPayload := store.MarshalDeletionPayload(store.DeletionPayload{
		ResourceType: "preview", ResourceID: preview.ID.String(), ApplicationID: app.ID.String(), PullRequestID: 12,
	})
	previewJob, err := st.MarkDeleting(ctx, teamID, "preview", preview.ID, previewPayload)
	if err != nil || previewJob.Kind != "resource_delete" || previewJob.Status != "queued" {
		t.Fatalf("preview job %+v %v", previewJob, err)
	}
	visible, err := st.ListPreviews(ctx, teamID, app.ID)
	if err != nil || len(visible) != 0 {
		t.Fatalf("visible previews %+v %v", visible, err)
	}
	hidden, err := st.ListPreviewsIncludingDeleted(ctx, teamID, app.ID)
	if err != nil || len(hidden) != 1 || hidden[0].ID != preview.ID || hidden[0].Status != "deleting" || hidden[0].PullRequestID != 12 {
		t.Fatalf("hidden previews %+v %v", hidden, err)
	}
	byID, err := st.GetPreviewByID(ctx, teamID, preview.ID)
	if err != nil || byID.PullRequestTitle != "Add preview" || byID.GitBranch != "feature" {
		t.Fatalf("preview by id %+v %v", byID, err)
	}
	if err := st.SetDeleteError(ctx, teamID, "preview", preview.ID, "proxy leftover"); err != nil {
		t.Fatal(err)
	}
	pending, err := st.ListPendingDeletions(ctx, teamID)
	if err != nil {
		t.Fatal(err)
	}
	row := mustFind(t, pending, func(p store.PendingDeletion) bool { return p.ResourceID == preview.ID })
	if row.ResourceType != "preview" || row.Error != "proxy leftover" || row.Name == "" {
		t.Fatalf("pending preview %+v", row)
	}

	dbPayload := store.MarshalDeletionPayload(store.DeletionPayload{
		ResourceType: "database", ResourceID: db.ID.String(), DeleteVolumes: true,
	})
	dbJob, err := st.MarkDeleting(ctx, teamID, "database", db.ID, dbPayload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetDatabase(ctx, teamID, db.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted database still visible: %v", err)
	}
	anyDB, err := st.GetDatabaseAny(ctx, teamID, db.ID)
	if err != nil || anyDB.Name != "pg" || anyDB.Engine != "postgresql" || anyDB.Status != "deleting" {
		t.Fatalf("database any %+v %v", anyDB, err)
	}
	active, err := st.DeleteJobActive(ctx, teamID, db.ID)
	if err != nil || !active {
		t.Fatalf("db job active %v %v", active, err)
	}
	if err := st.CancelQueuedDeleteJobs(ctx, teamID, db.ID); err != nil {
		t.Fatal(err)
	}
	cancelled, err := st.GetJob(ctx, teamID, dbJob.ID)
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("cancelled job %+v %v", cancelled, err)
	}
	active, err = st.DeleteJobActive(ctx, teamID, db.ID)
	if err != nil || active {
		t.Fatalf("db job still active %v %v", active, err)
	}
	latest, err := st.LatestDeletePayload(ctx, teamID, db.ID)
	if err != nil {
		t.Fatal(err)
	}
	var decoded store.DeletionPayload
	if err := json.Unmarshal(latest, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ResourceType != "database" || decoded.ResourceID != db.ID.String() || !decoded.DeleteVolumes {
		t.Fatalf("payload %+v", decoded)
	}
	if payload, err := st.LatestDeletePayload(ctx, teamID, uuid.New()); err != nil || payload != nil {
		t.Fatalf("missing payload %s %v", payload, err)
	}

	if _, err := st.MarkDeleting(ctx, teamID, "service", svc.ID, store.MarshalDeletionPayload(store.DeletionPayload{
		ResourceType: "service", ResourceID: svc.ID.String(),
	})); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetService(ctx, teamID, svc.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted service still visible: %v", err)
	}
	anySvc, err := st.GetServiceAny(ctx, teamID, svc.ID)
	if err != nil || anySvc.Name != "n8n" || anySvc.Status != "deleting" || !strings.Contains(anySvc.DockerComposeRaw, "n8nio/n8n") {
		t.Fatalf("service any %+v %v", anySvc, err)
	}
	if err := st.SetDeleteError(ctx, teamID, "service", svc.ID, "volume busy"); err != nil {
		t.Fatal(err)
	}

	appJob, err := st.MarkDeleting(ctx, teamID, "application", app.ID, store.MarshalDeletionPayload(store.DeletionPayload{
		ResourceType: "application", ResourceID: app.ID.String(), DockerCleanup: true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetApplication(ctx, teamID, app.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted app still visible: %v", err)
	}
	anyApp, err := st.GetApplicationAny(ctx, teamID, app.ID)
	if err != nil || anyApp.Name != "web" || anyApp.Status != "deleting" {
		t.Fatalf("app any %+v %v", anyApp, err)
	}
	if err := st.SetDeleteError(ctx, teamID, "application", app.ID, "network left"); err != nil {
		t.Fatal(err)
	}
	// A recorded error allows a second delete job.
	again, err := st.MarkDeleting(ctx, teamID, "application", app.ID, []byte(`{"resource_type":"application","resource_id":"`+app.ID.String()+`"}`))
	if err != nil || again.ID == appJob.ID {
		t.Fatalf("second delete %+v %v", again, err)
	}
	pending, err = st.ListPendingDeletions(ctx, teamID)
	if err != nil {
		t.Fatal(err)
	}
	var sawApp, sawDB, sawSvc bool
	for _, item := range pending {
		switch item.ResourceID {
		case app.ID:
			sawApp = item.ResourceType == "application" && item.Error == ""
		case db.ID:
			sawDB = item.ResourceType == "database"
		case svc.ID:
			sawSvc = item.Error == "volume busy"
		}
	}
	if !sawApp || !sawDB || !sawSvc {
		t.Fatalf("pending %+v", pending)
	}
	if _, err := st.MarkDeleting(ctx, teamID, "volume", uuid.New(), nil); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestJobs(t *testing.T) {
	w := newWorld(t, "jobs@example.com", "jobs")
	ctx := t.Context()
	st := w.st
	teamID := w.team.ID
	app := w.app(t, "web", "dockerfile")

	delJob, err := st.MarkDeleting(ctx, teamID, "application", app.ID, store.MarshalDeletionPayload(store.DeletionPayload{
		ResourceType: "application", ResourceID: app.ID.String(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimJob(ctx)
	if err != nil || claimed == nil || claimed.ID != delJob.ID || claimed.Status != "running" || claimed.Attempts != 1 {
		t.Fatalf("claim %+v %v", claimed, err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE background_jobs SET attempts = max_attempts, locked_until = NOW() - INTERVAL '1 minute' WHERE id=$1`, claimed.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.RequeueExpiredJobs(ctx); err != nil {
		t.Fatal(err)
	}
	expired, err := st.GetJob(ctx, teamID, claimed.ID)
	if err != nil || expired.Status != "failed" || expired.Error != "worker stopped before the job finished" {
		t.Fatalf("expired %+v %v", expired, err)
	}
	pending, err := st.ListPendingDeletions(ctx, teamID)
	if err != nil {
		t.Fatal(err)
	}
	row := mustFind(t, pending, func(p store.PendingDeletion) bool { return p.ResourceID == app.ID })
	if row.Error != "worker stopped before the job finished" {
		t.Fatalf("delete error %q", row.Error)
	}

	retry, err := st.EnqueueJob(ctx, teamID, "echo", []byte(`{"n":1}`))
	if err != nil || retry.Status != "queued" || retry.MaxAttempts != 3 {
		t.Fatalf("enqueue %+v %v", retry, err)
	}
	claimed, err = st.ClaimJob(ctx)
	if err != nil || claimed == nil || claimed.ID != retry.ID {
		t.Fatalf("claim echo %+v %v", claimed, err)
	}
	if err := st.FailJob(ctx, claimed.ID, claimed.Attempts, claimed.MaxAttempts, "boom"); err != nil {
		t.Fatal(err)
	}
	retried, err := st.GetJob(ctx, teamID, retry.ID)
	if err != nil || retried.Status != "queued" || retried.Error != "boom" || !retried.RunAfter.After(claimed.RunAfter) {
		t.Fatalf("requeued %+v %v", retried, err)
	}

	dead, err := st.EnqueueJob(ctx, teamID, "work", nil)
	if err != nil || string(dead.Payload) != "{}" {
		t.Fatalf("empty payload %+v %v", dead, err)
	}
	claimed, err = st.ClaimJob(ctx)
	if err != nil || claimed == nil || claimed.ID != dead.ID {
		t.Fatalf("claim work %+v %v", claimed, err)
	}
	if err := st.FailJob(ctx, claimed.ID, claimed.MaxAttempts, claimed.MaxAttempts, "dead"); err != nil {
		t.Fatal(err)
	}
	failed, err := st.GetJob(ctx, teamID, dead.ID)
	if err != nil || failed.Status != "failed" || failed.Error != "dead" {
		t.Fatalf("failed %+v %v", failed, err)
	}

	done, err := st.EnqueueJob(ctx, teamID, "done", []byte(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	claimed, err = st.ClaimJob(ctx)
	if err != nil || claimed == nil || claimed.ID != done.ID {
		t.Fatalf("claim done %+v %v", claimed, err)
	}
	if err := st.FinishJob(ctx, claimed.ID, []byte(`{"result":"ok"}`)); err != nil {
		t.Fatal(err)
	}
	finished, err := st.GetJob(ctx, teamID, done.ID)
	if err != nil || finished.Status != "succeeded" || finished.Error != "" {
		t.Fatalf("finished %+v %v", finished, err)
	}
	var result map[string]string
	if err := json.Unmarshal(finished.Result, &result); err != nil || result["result"] != "ok" {
		t.Fatalf("result %s %v", finished.Result, err)
	}

	skip, err := st.EnqueueJob(ctx, teamID, "skip", []byte(`{"skip":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CancelJob(ctx, teamID, skip.ID); err != nil {
		t.Fatal(err)
	}
	cancelled, err := st.GetJob(ctx, teamID, skip.ID)
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("cancel %+v %v", cancelled, err)
	}
	if err := st.CancelJob(ctx, teamID, skip.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second cancel %v", err)
	}
	if _, err := st.GetJob(ctx, teamID, uuid.New()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing job %v", err)
	}
	// The retried echo job is scheduled in the future, so nothing is claimable.
	if next, err := st.ClaimJob(ctx); err != nil || next != nil {
		t.Fatalf("unexpected claim %+v %v", next, err)
	}
	jobs, err := st.ListJobs(ctx, teamID, 10)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, job := range jobs {
		kinds[job.Kind] = job.Status
	}
	if kinds["echo"] != "queued" || kinds["work"] != "failed" || kinds["done"] != "succeeded" || kinds["skip"] != "cancelled" || kinds["resource_delete"] != "failed" {
		t.Fatalf("jobs %#v", kinds)
	}
}
