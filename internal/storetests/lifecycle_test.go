package storetests

import (
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/google/uuid"
)

func TestResourceLifecycle(t *testing.T) {
	st := testkit.OpenStore(t)
	ctx := t.Context()
	user, team, err := st.CreateUserWithPersonalTeam(ctx, "life@example.com", "Life", "hash")
	if err != nil {
		t.Fatal(err)
	}
	teamID := team.ID
	project, env, err := st.CreateProject(ctx, teamID, "life", "desc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateProject(ctx, teamID, project.ID, "life-renamed", "more"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListProjects(ctx, teamID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetProject(ctx, teamID, project.ID); err != nil {
		t.Fatal(err)
	}
	env2, err := st.CreateEnvironment(ctx, teamID, project.ID, "staging", "stage")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateEnvironment(ctx, teamID, project.ID, env.ID, "production", "prod"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListEnvironments(ctx, teamID, project.ID); err != nil {
		t.Fatal(err)
	}
	app, err := st.CreateApplication(ctx, &store.Application{
		TeamID: teamID, EnvironmentID: env.ID, Name: "web", BuildPack: "dockerfile",
		Dockerfile: "FROM nginx:alpine\n", GitRepository: "https://github.com/acme/web.git", GitBranch: "main",
		PortsExposes: "80", BaseDirectory: "/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateApplication(ctx, app); err != nil {
		t.Fatal(err)
	}
	for _, call := range []func() error{
		func() error { return st.SetApplicationBuildServerEnabled(ctx, teamID, app.ID, true) },
		func() error { return st.SetApplicationPreviewEnabled(ctx, teamID, app.ID, true) },
		func() error { return st.SetApplicationAutoDeployEnabled(ctx, teamID, app.ID, true) },
		func() error { return st.SetApplicationGitSubmodulesEnabled(ctx, teamID, app.ID, false) },
		func() error { return st.SetApplicationPreserveRepositoryEnabled(ctx, teamID, app.ID, true) },
		func() error { return st.SetApplicationWatchPaths(ctx, teamID, app.ID, "apps/web/**") },
		func() error { return st.SetApplicationForceHTTPS(ctx, teamID, app.ID, true) },
		func() error {
			return st.SetApplicationAdvancedSettings(ctx, teamID, app.ID, store.ApplicationAdvancedSettings{
				IsSPA: true, CustomDockerRestartPolicy: "unless-stopped", DockerImagesToKeep: 2, LogsLineLimit: 100,
			})
		},
		func() error { return st.UpdateApplicationStatus(ctx, app.ID, "running") },
		func() error { return st.UpdateApplicationGitCommitSHA(ctx, app.ID, "abc123") },
		func() error {
			return st.UpdateApplicationComposePreview(ctx, teamID, app.ID, "services: {}\n", "services: {}\n")
		},
	} {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.GetApplication(ctx, teamID, app.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetApplicationByID(ctx, app.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListApplications(ctx, teamID, &env.ID); err != nil {
		t.Fatal(err)
	}
	if empty, err := st.ProjectIsEmpty(ctx, teamID, project.ID); err != nil || empty {
		t.Fatalf("project empty %v %v", empty, err)
	}
	cloned, err := st.CloneApplication(ctx, teamID, app.ID, "web-copy", &env2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MoveResource(ctx, teamID, "application", cloned.ID, env.ID); err != nil {
		t.Fatal(err)
	}

	db, err := st.CreateDatabase(ctx, &store.Database{
		TeamID: teamID, EnvironmentID: env.ID, Name: "pg", Engine: "postgresql",
	}, "enc")
	if err != nil {
		t.Fatal(err)
	}
	name := "pg2"
	if _, err := st.UpdateDatabase(ctx, teamID, db.ID, store.UpdateDatabaseInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetDatabaseCredentials(ctx, teamID, db.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateDatabaseStatus(ctx, db.ID, "running"); err != nil {
		t.Fatal(err)
	}

	svc, err := st.CreateService(ctx, &store.Service{
		TeamID: teamID, EnvironmentID: env.ID, Name: "n8n", ServiceType: "n8n",
		DockerComposeRaw: "services:\n  n8n:\n    image: n8nio/n8n\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateServiceMeta(ctx, teamID, svc.ID, "n8n", "automation"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateServiceComposeRaw(ctx, teamID, svc.ID, "services: {}\n"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateServiceCompose(ctx, svc.ID, "services: {}\n"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateServiceFQDN(ctx, svc.ID, "n8n.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetServiceForceHTTPS(ctx, teamID, svc.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateServiceStatus(ctx, svc.ID, "running"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListServices(ctx, teamID, &env.ID); err != nil {
		t.Fatal(err)
	}

	dep, err := st.CreateDeployment(ctx, teamID, app.ID, nil, "abc", "msg", false, false, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListDeployments(ctx, teamID, app.ID, 10); err != nil {
		t.Fatal(err)
	}
	if err := st.AppendDeploymentLog(ctx, dep.ID, "build", "hello"); err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimDeployment(ctx, teamID, dep.ID)
	if err != nil || !claimed {
		t.Fatalf("claim %v %v", claimed, err)
	}
	if err := st.SetDeploymentStatus(ctx, dep.ID, "finished", ""); err != nil {
		t.Fatal(err)
	}
	cancelled, err := st.IsDeploymentCancelled(ctx, dep.ID)
	if err != nil || cancelled {
		t.Fatalf("cancelled %v %v", cancelled, err)
	}
	sdep, err := st.CreateServiceDeployment(ctx, teamID, svc.ID, nil, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListServiceDeployments(ctx, teamID, svc.ID, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListQueuedDeployments(ctx, 10); err != nil {
		t.Fatal(err)
	}
	_ = sdep

	preview, err := st.CreatePreview(ctx, teamID, app.ID, 7, "pr", "feature", "pr.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetPreviewByPR(ctx, teamID, app.ID, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListPreviews(ctx, teamID, app.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdatePreviewStatus(ctx, teamID, app.ID, 7, "running"); err != nil {
		t.Fatal(err)
	}
	_ = preview

	ev, err := st.UpsertEnvVar(ctx, teamID, "application", app.ID, store.UpsertEnvVarInput{Key: "PORT", Value: "80", Runtime: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListEnvVars(ctx, teamID, "application", app.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetEnvVarLocked(ctx, teamID, ev.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertEnvVar(ctx, teamID, "application", app.ID, store.UpsertEnvVarInput{Key: "PORT", Value: "81", Runtime: true}); err == nil {
		t.Fatal("locked env var was overwritten")
	}
	if _, err := st.UpsertSharedEnv(ctx, teamID, "team", nil, "SHARED", "1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListSharedEnv(ctx, teamID, "team", nil, true); err != nil {
		t.Fatal(err)
	}

	s3, err := st.CreateS3Storage(ctx, teamID, "b2", "https://s3.example.com", "bucket", "us-east-1", "ak", "sk", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.GetS3StorageSecrets(ctx, teamID, s3.ID); err != nil {
		t.Fatal(err)
	}
	sched, err := st.CreateScheduledBackup(ctx, teamID, "database", db.ID, []uuid.UUID{s3.ID}, nil, "0 3 * * *", 7)
	if err != nil {
		t.Fatal(err)
	}
	freq := "0 4 * * *"
	if _, err := st.UpdateScheduledBackup(ctx, teamID, sched.ID, store.UpdateScheduledBackupInput{Frequency: &freq}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListScheduledBackups(ctx, teamID); err != nil {
		t.Fatal(err)
	}
	exec, err := st.CreateBackupExecution(ctx, teamID, "database", db.ID, "pg-backup.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MarkBackupS3Uploaded(ctx, exec.ID, "pg-backup.sql"); err != nil {
		t.Fatal(err)
	}
	if err := st.FinishBackupExecution(ctx, exec.ID, "finished", 10, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ListBackupExecutions(ctx, teamID, "database", db.ID); err != nil {
		t.Fatal(err)
	}

	task, err := st.CreateScheduledTask(ctx, teamID, "application", app.ID, "tick", "true", "0 * * * *", "")
	if err != nil {
		t.Fatal(err)
	}
	tn := "tick2"
	if _, err := st.UpdateScheduledTask(ctx, teamID, task.ID, store.UpdateScheduledTaskInput{Name: &tn}); err != nil {
		t.Fatal(err)
	}
	execID, err := st.CreateTaskExecution(ctx, teamID, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishTaskExecution(ctx, execID, "finished", "ok"); err != nil {
		t.Fatal(err)
	}
	ran, err := st.TaskRanThisMinute(ctx, task.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	_ = ran
	if _, err := st.ListScheduledTaskExecutions(ctx, teamID, task.ID, 10); err != nil {
		t.Fatal(err)
	}

	uid := user.ID
	if _, err := st.SetEnvironmentDeployFreeze(ctx, teamID, project.ID, env.ID, &uid, true, "maintenance"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.TeamOverview(ctx, teamID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePreview(ctx, teamID, app.ID, 7); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteEnvVar(ctx, teamID, ev.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteScheduledTask(ctx, teamID, task.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteScheduledBackup(ctx, teamID, sched.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteS3Storage(ctx, teamID, s3.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteService(ctx, teamID, svc.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteDatabase(ctx, teamID, db.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteApplication(ctx, teamID, cloned.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnvironmentIsEmpty(ctx, teamID, env2.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteEnvironment(ctx, teamID, project.ID, env2.ID); err != nil {
		t.Fatal(err)
	}
}
