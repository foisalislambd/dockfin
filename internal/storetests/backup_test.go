package storetests

import (
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestBackupS3AndInstance(t *testing.T) {
	w := newWorld(t, "backup@example.com", "backups")
	ctx := t.Context()
	st := w.st
	teamID := w.team.ID
	db := w.database(t, "pg")

	a, err := st.CreateS3Storage(ctx, teamID, "a-store", "https://s3.example.com", "bucket-a", "us-east-1", "ak", "sk", true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.CreateS3Storage(ctx, teamID, "b-store", "https://s3.example.com", "bucket-b", "us-east-1", "ak", "sk", true)
	if err != nil {
		t.Fatal(err)
	}
	sched, err := st.CreateScheduledBackup(ctx, teamID, "database", db.ID, []uuid.UUID{a.ID, b.ID}, nil, "0 3 * * *", 7)
	if err != nil {
		t.Fatal(err)
	}
	if sched.S3StorageID == nil || len(sched.S3StorageIDs) != 2 {
		t.Fatalf("schedule %+v", sched)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE scheduled_backups SET s3_storage_id=NULL WHERE id=$1`, sched.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.PromoteBackupPrimaries(ctx, teamID); err != nil {
		t.Fatal(err)
	}
	promoted, err := st.GetScheduledBackup(ctx, teamID, sched.ID)
	if err != nil || promoted.S3StorageID == nil || *promoted.S3StorageID != a.ID {
		t.Fatalf("promoted %+v %v", promoted, err)
	}

	exec, err := st.CreateBackupExecution(ctx, teamID, "database", db.ID, "pg.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RecordBackupS3Replica(ctx, exec.ID, a.ID, "a-store", true, "backups/pg.sql", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordBackupS3Replica(ctx, exec.ID, b.ID, "b-store", true, "backups/pg.sql", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.RefreshBackupS3Summary(ctx, exec.ID); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListBackupExecutions(ctx, teamID, "database", db.ID)
	if err != nil || len(rows) != 1 || !rows[0].S3Uploaded || rows[0].S3Key != "backups/pg.sql" || len(rows[0].S3Replicas) != 2 {
		t.Fatalf("replicas %+v %v", rows, err)
	}
	if err := st.RecordBackupS3Replica(ctx, exec.ID, b.ID, "b-store", false, "", "timeout"); err != nil {
		t.Fatal(err)
	}
	if err := st.RefreshBackupS3Summary(ctx, exec.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkBackupS3Incomplete(ctx, exec.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.FinishBackupExecution(ctx, exec.ID, "finished", 42, ""); err != nil {
		t.Fatal(err)
	}
	if err := st.NoteBackupS3Failure(ctx, exec.ID, "b-store timed out"); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetBackupExecution(ctx, teamID, exec.ID)
	if err != nil || got.Status != "finished" || got.SizeBytes != 42 || got.S3Uploaded || got.ErrorMessage != "b-store timed out" {
		t.Fatalf("execution %+v %v", got, err)
	}
	// A second note must not replace the first message.
	if err := st.NoteBackupS3Failure(ctx, exec.ID, "another"); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetBackupExecution(ctx, teamID, exec.ID)
	if err != nil || got.ErrorMessage != "b-store timed out" {
		t.Fatalf("error overwritten: %+v %v", got, err)
	}
	rows, err = st.ListBackupExecutions(ctx, teamID, "database", db.ID)
	if err != nil {
		t.Fatal(err)
	}
	failed := mustFind(t, rows[0].S3Replicas, func(r store.BackupS3Replica) bool {
		return r.S3StorageID != nil && *r.S3StorageID == b.ID
	})
	if failed.Uploaded || failed.Message != "timeout" || failed.StorageName != "b-store" {
		t.Fatalf("failed replica %+v", failed)
	}

	cfg, err := st.GetInstanceBackupConfig(ctx)
	if err != nil || cfg.Configured {
		t.Fatalf("unconfigured %+v %v", cfg, err)
	}
	disabled := false
	if _, err := st.UpdateInstanceBackupConfig(ctx, store.InstanceBackupPatch{Enabled: &disabled}); err == nil {
		t.Fatal("updated instance backup before it was configured")
	}
	cfg, err = st.ConfigureInstanceBackup(ctx, "dockfin-db", "dockfin", "dockfin", "nightly")
	if err != nil || !cfg.Configured || !cfg.Enabled || cfg.Container != "dockfin-db" || cfg.DBName != "dockfin" || cfg.Frequency != "0 0 * * *" || cfg.Retention != 7 {
		t.Fatalf("configured %+v %v", cfg, err)
	}
	freq, retention := "15 2 * * *", 3
	off := false
	cfg, err = st.UpdateInstanceBackupConfig(ctx, store.InstanceBackupPatch{Frequency: &freq, Retention: &retention, Enabled: &off, Description: strPtr("weekly")})
	if err != nil || cfg.Enabled || cfg.Frequency != freq || cfg.Retention != 3 || cfg.Description != "weekly" {
		t.Fatalf("patched %+v %v", cfg, err)
	}
	inst, err := st.CreateInstanceBackupExecution(ctx, teamID, "dockfin.sql")
	if err != nil || inst.ResourceType != store.InstanceBackupResourceType || inst.ResourceID != store.InstanceBackupResourceID || inst.Filename != "dockfin.sql" || inst.Status != "running" {
		t.Fatalf("instance exec %+v %v", inst, err)
	}
	ran, err := st.InstanceBackupRanThisMinute(ctx, inst.StartedAt.UTC().Truncate(time.Minute))
	if err != nil || !ran {
		t.Fatalf("ran this minute %v %v", ran, err)
	}
	listed, err := st.ListInstanceBackupExecutions(ctx, 10)
	if err != nil || len(listed) != 1 || listed[0].ID != inst.ID {
		t.Fatalf("list %+v %v", listed, err)
	}
	one, err := st.GetInstanceBackupExecution(ctx, inst.ID)
	if err != nil || one.Filename != "dockfin.sql" {
		t.Fatalf("get %+v %v", one, err)
	}
	first, err := st.FirstTeamID(ctx)
	if err != nil || first != teamID {
		t.Fatalf("first team %s %v", first, err)
	}
}

func strPtr(s string) *string { return &s }
