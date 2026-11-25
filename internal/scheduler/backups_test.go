package scheduler

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/google/uuid"
)

func TestTickBackupsFailWithoutSSH(t *testing.T) {
	st := testkit.OpenStore(t)
	ctx := t.Context()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "backups@example.com", "Backups", "hash")
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := st.CreateProject(ctx, team.ID, "bk", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateServer(ctx, team.ID, nil, "edge", "", "10.9.9.9", "root", 22, "traefik"); err != nil {
		t.Fatal(err)
	}
	app, err := st.CreateApplication(ctx, &store.Application{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "web", BuildPack: "dockerfile",
	})
	if err != nil {
		t.Fatal(err)
	}
	db, err := st.CreateDatabase(ctx, &store.Database{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "pg", Engine: "postgresql",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	svc, err := st.CreateService(ctx, &store.Service{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "redis",
		DockerComposeRaw: "services:\n  redis:\n    image: redis:7\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		kind string
		id   uuid.UUID
	}{
		{"database", db.ID},
		{"application", app.ID},
		{"service", svc.ID},
	} {
		if _, err := st.CreateScheduledBackup(ctx, team.ID, item.kind, item.id, nil, nil, "* * * * *", 3); err != nil {
			t.Fatal(err)
		}
	}
	r := New(st, sshx.NewPool(), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "")
	r.tick(ctx)
	deadline := time.Now().Add(3 * time.Second)
	var failed int
	for time.Now().Before(deadline) {
		if err := st.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM backup_executions WHERE team_id=$1 AND status='failed'`, team.ID).Scan(&failed); err != nil {
			t.Fatal(err)
		}
		if failed >= 3 {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if failed < 3 {
		t.Fatalf("failed backups %d", failed)
	}
}
