package scheduler

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/google/uuid"
)

func TestTickRecordsAFailedTaskWithoutSSH(t *testing.T) {
	st := testkit.OpenStore(t)
	ctx := t.Context()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "sched@example.com", "Sched", "hash")
	if err != nil {
		t.Fatal(err)
	}
	task, err := st.CreateScheduledTask(ctx, team.ID, "application", uuid.New(), "nightly", "echo ok", "* * * * *", "")
	if err != nil {
		t.Fatal(err)
	}
	r := New(st, sshx.NewPool(), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "")
	r.tick(ctx)
	deadline := time.Now().Add(2 * time.Second)
	var status, message string
	for time.Now().Before(deadline) {
		err = st.Pool.QueryRow(ctx, `SELECT status, COALESCE(output,'') FROM scheduled_task_executions WHERE scheduled_task_id=$1`, task.ID).Scan(&status, &message)
		if err == nil && status != "" && status != "running" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status != "failed" || message == "" {
		t.Fatalf("status %q message %q err %v", status, message, err)
	}
	// A second tick in the same minute must not start another execution.
	r.tick(ctx)
	var n int
	if err := st.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM scheduled_task_executions WHERE scheduled_task_id=$1`, task.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("executions %d", n)
	}
}
