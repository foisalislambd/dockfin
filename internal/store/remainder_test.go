package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/db"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestScopedAccessTransferAndFleet(t *testing.T) {
	dsn := os.Getenv("DOCKFIN_DATABASE_URL")
	if dsn == "" {
		t.Skip("DOCKFIN_DATABASE_URL not set")
	}
	ctx := context.Background()
	if err := db.Migrate(dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	box, err := crypto.NewBox("test-master-key-for-integration-32b!")
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(pool, box)
	suffix := uuid.NewString()[:8]
	owner, team, err := st.CreateUserWithPersonalTeam(ctx, "acl-"+suffix+"@example.com", "Owner", "hash")
	if err != nil {
		t.Fatal(err)
	}
	member, _, err := st.CreateUserWithPersonalTeam(ctx, "member-"+suffix+"@example.com", "Member", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO team_members (team_id, user_id, role) VALUES ($1,$2,'member')`, team.ID, member.ID); err != nil {
		t.Fatal(err)
	}
	project, _, err := st.CreateProject(ctx, team.ID, "app-"+suffix, "")
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := st.CreateProject(ctx, team.ID, "other-"+suffix, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetProjectAccess(ctx, team.ID, member.ID, "projects", []store.ProjectGrant{{ProjectID: project.ID, Permission: "view"}}); err != nil {
		t.Fatal(err)
	}
	scope, err := st.MemberAccess(ctx, team.ID, member.ID, "member")
	if err != nil {
		t.Fatal(err)
	}
	if scope.Unrestricted || !scope.Allows(project.ID, false, false) || scope.Allows(project.ID, true, false) || scope.Allows(other.ID, false, false) {
		t.Fatalf("scope = %+v", scope)
	}
	members, err := st.ListTeamMembers(ctx, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, m := range members {
		if m.UserID == member.ID && m.AccessScope == "projects" {
			found = true
		}
	}
	if !found {
		t.Fatal("member access_scope was not stored")
	}
	srv, err := st.CreateServer(ctx, team.ID, nil, "srv-"+suffix, "", "203.0.113.10", "root", 22, "caddy")
	if err != nil {
		t.Fatal(err)
	}
	target, _, err := st.CreateUserWithPersonalTeam(ctx, "dest-"+suffix+"@example.com", "Dest", "hash")
	if err != nil {
		t.Fatal(err)
	}
	_ = owner
	_ = target
	rows, err := st.FleetMetrics(ctx, team.ID)
	if err != nil || len(rows) == 0 || rows[0].Name == "" {
		t.Fatalf("fleet: %v %+v", err, rows)
	}
	destTeams, err := st.ListTeamsForUser(ctx, target.ID)
	if err != nil || len(destTeams) == 0 {
		t.Fatal(err)
	}
	res, err := st.TransferServer(ctx, team.ID, destTeams[0].ID, srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Applications != 0 || res.Projects != 0 {
		t.Fatalf("empty transfer: %+v", res)
	}
	busy, err := st.CreateServer(ctx, team.ID, nil, "busy-"+suffix, "", "203.0.113.11", "root", 22, "caddy")
	if err != nil {
		t.Fatal(err)
	}
	otherHost, err := st.CreateServer(ctx, team.ID, nil, "otherhost-"+suffix, "", "203.0.113.12", "root", 22, "caddy")
	if err != nil {
		t.Fatal(err)
	}
	dests, err := st.ListDestinations(ctx, team.ID, &busy.ID)
	if err != nil || len(dests) == 0 {
		t.Fatal(err)
	}
	others, err := st.ListDestinations(ctx, team.ID, &otherHost.ID)
	if err != nil || len(others) == 0 {
		t.Fatal(err)
	}
	shared, sharedEnv, err := st.CreateProject(ctx, team.ID, "shared-"+suffix, "")
	if err != nil {
		t.Fatal(err)
	}
	movedApp, err := st.CreateApplication(ctx, &store.Application{
		TeamID: team.ID, EnvironmentID: sharedEnv.ID, DestinationID: &dests[0].ID,
		Name: "web-" + suffix, BuildPack: "dockerimage", GitBranch: "main",
		DockerRegistryImageName: "nginx", DockerRegistryImageTag: "1.27.1", PortsExposes: "80",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateApplication(ctx, &store.Application{
		TeamID: team.ID, EnvironmentID: sharedEnv.ID, DestinationID: &others[0].ID,
		Name: "stay-" + suffix, BuildPack: "dockerfile", GitBranch: "main",
		DockerfileLocation: "/Dockerfile", PortsExposes: "3000",
	}); err != nil {
		t.Fatal(err)
	}
	splitRes, err := st.TransferServer(ctx, team.ID, destTeams[0].ID, busy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if splitRes.Applications != 1 || splitRes.Split != 1 || len(splitRes.Warnings) == 0 {
		t.Fatalf("split transfer: %+v", splitRes)
	}
	if _, err := st.GetApplication(ctx, destTeams[0].ID, movedApp.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetApplication(ctx, team.ID, movedApp.ID); err != store.ErrNotFound {
		t.Fatalf("application still on source: %v", err)
	}
	projects, err := st.ListProjects(ctx, destTeams[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	var split bool
	for _, p := range projects {
		if p.Name == shared.Name+" (moved)" {
			split = true
		}
	}
	if !split {
		t.Fatalf("target projects: %+v", projects)
	}
	if _, err := st.GetProject(ctx, team.ID, shared.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetServer(ctx, team.ID, srv.ID); err != store.ErrNotFound {
		t.Fatalf("server still on source team: %v", err)
	}
	moved, err := st.GetServer(ctx, destTeams[0].ID, srv.ID)
	if err != nil || moved.ProxyType != "caddy" {
		t.Fatalf("moved server: %v %+v", err, moved)
	}
	exp, err := st.ExportServer(ctx, destTeams[0].ID, srv.ID)
	if err != nil || exp.IP != "203.0.113.10" {
		t.Fatalf("export: %v %+v", err, exp)
	}
}
