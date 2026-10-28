package githubrunner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/git/githubapp"
	"github.com/dockfin/dockfin/internal/sshdial"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

// Provision claims one queued execution and starts its container.
// A nil result means another worker took it or no server had capacity.
func Provision(ctx context.Context, st *store.Store, pool *sshx.Pool, executionID uuid.UUID) error {
	reserved, err := st.ReserveRunner(ctx, executionID, RunnerName(executionID), Matches)
	if err != nil || reserved == nil {
		return err
	}
	if err := startReserved(ctx, st, pool, reserved); err != nil {
		return failStart(ctx, st, pool, reserved, err)
	}
	return nil
}

func startReserved(ctx context.Context, st *store.Store, pool *sshx.Pool, reserved *store.ReservedRunner) error {
	client, err := sshdial.DialClient(ctx, st, pool, reserved.TeamID, reserved.ServerID)
	if err != nil {
		return err
	}
	cfg := reserved.Config
	if _, _, err := sshx.RunArgs(client, "mkdir", "-p", SecretsDir); err != nil {
		return err
	}
	if _, _, err := sshx.RunArgs(client, "chmod", "700", SecretsDir); err != nil {
		return err
	}
	for _, argv := range PrepareArgs(reserved.ExecutionID, cfg.RunnerImage, cfg.DockerMode, cfg.CPULimit, cfg.MemoryLimit) {
		if err := runDocker(client, argv); err != nil {
			return err
		}
	}
	if cfg.DockerMode == "dind" {
		if err := waitForDocker(client, reserved.ExecutionID); err != nil {
			return err
		}
	}
	app, err := appFor(ctx, st, reserved.TeamID, reserved.GitSourceID)
	if err != nil {
		return err
	}
	sec, err := st.GetGitSourceSecrets(ctx, reserved.TeamID, reserved.GitSourceID)
	if err != nil {
		return err
	}
	groupID, err := st.GitSourceRunnerGroup(ctx, reserved.GitSourceID)
	if err != nil {
		return err
	}
	group, err := app.EnsureRunnerGroup(sec.InstallationID, "Dockfin "+reserved.GitSourceID.String()[:8], groupID)
	if err != nil {
		return err
	}
	if group.ID != groupID {
		_ = st.SetGitSourceRunnerGroup(ctx, reserved.GitSourceID, group.ID)
	}
	jit, err := app.GenerateJITConfig(sec.InstallationID, reserved.RunnerName, group.ID, RegisteredLabels(cfg.Labels))
	if err != nil {
		return err
	}
	if err := st.SetRunnerResult(ctx, reserved.ExecutionID, jit.ID, "provisioning", ""); err != nil {
		_ = app.DeleteRunner(sec.InstallationID, jit.ID)
		return err
	}
	envPath := EnvPath(reserved.ExecutionID)
	if err := sshx.WriteFile(client, envPath, []byte("ACTIONS_RUNNER_INPUT_JITCONFIG="+jit.EncodedConfig+"\n")); err != nil {
		_ = app.DeleteRunner(sec.InstallationID, jit.ID)
		return err
	}
	if _, stderr, err := sshx.RunArgs(client, "chmod", "600", envPath); err != nil {
		_ = app.DeleteRunner(sec.InstallationID, jit.ID)
		return fmt.Errorf("protect runner secret: %s", strings.TrimSpace(stderr))
	}
	hook := ""
	if !cfg.AllowPullRequests {
		hook = HookFile(reserved.ExecutionID)
		if err := sshx.WriteFile(client, hook, []byte(HookScript())); err != nil {
			_ = app.DeleteRunner(sec.InstallationID, jit.ID)
			return err
		}
		if _, stderr, err := sshx.RunArgs(client, "chmod", "644", hook); err != nil {
			_ = app.DeleteRunner(sec.InstallationID, jit.ID)
			return fmt.Errorf("install job hook: %s", strings.TrimSpace(stderr))
		}
	}
	if cfg.DockerMode == "dind" {
		if err := runDocker(client, ChownWorkArgs(reserved.ExecutionID)); err != nil {
			_ = app.DeleteRunner(sec.InstallationID, jit.ID)
			return err
		}
	}
	if err := runDocker(client, StartArgs(reserved.ExecutionID, cfg.RunnerImage, cfg.DockerMode, envPath, hook, cfg.CPULimit, cfg.MemoryLimit)); err != nil {
		_ = app.DeleteRunner(sec.InstallationID, jit.ID)
		return err
	}
	_, _, _ = sshx.RunArgs(client, "rm", "-f", envPath)
	return st.SetRunnerResult(ctx, reserved.ExecutionID, jit.ID, "idle", "")
}

func failStart(ctx context.Context, st *store.Store, pool *sshx.Pool, reserved *store.ReservedRunner, cause error) error {
	cleanupExecution(ctx, st, pool, reserved.TeamID, reserved.ServerID, reserved.GitSourceID, reserved.ExecutionID)
	msg := cause.Error()
	if reserved.Attempts >= MaxAttempts {
		return st.SetRunnerResult(ctx, reserved.ExecutionID, 0, "failed", fmt.Sprintf("could not start a runner after %d attempts: %s", reserved.Attempts, msg))
	}
	return st.RequeueRunner(ctx, reserved.ExecutionID, fmt.Sprintf("attempt %d of %d failed: %s", reserved.Attempts, MaxAttempts, msg))
}

// Cleanup removes one runner's containers and its GitHub registration.
func Cleanup(ctx context.Context, st *store.Store, pool *sshx.Pool, teamID, serverID, sourceID, executionID uuid.UUID) {
	cleanupExecution(ctx, st, pool, teamID, serverID, sourceID, executionID)
}

func cleanupExecution(ctx context.Context, st *store.Store, pool *sshx.Pool, teamID, serverID, sourceID, executionID uuid.UUID) {
	if serverID != uuid.Nil && pool != nil {
		if client, err := sshdial.DialClient(ctx, st, pool, teamID, serverID); err == nil {
			for _, argv := range CleanupArgs(executionID) {
				_, _, _ = sshx.RunArgs(client, argv...)
			}
		}
	}
	var runnerID *int64
	_ = st.Pool.QueryRow(ctx, `SELECT runner_id FROM github_runner_executions WHERE id=$1`, executionID).Scan(&runnerID)
	if runnerID == nil || *runnerID == 0 {
		return
	}
	app, sec, err := appAndSecrets(ctx, st, teamID, sourceID)
	if err != nil {
		return
	}
	_ = app.DeleteRunner(sec.InstallationID, *runnerID)
}

// Reconcile starts queued jobs and removes runners that waited or ran too long.
func Reconcile(ctx context.Context, st *store.Store, pool *sshx.Pool, logger *slog.Logger) {
	items, err := st.ListRunnerMaintenance(ctx)
	if err != nil {
		logger.Error("list github runners", "err", err)
		return
	}
	now := time.Now()
	started := 0
	for _, item := range items {
		exec := item.Execution
		switch exec.Status {
		case "queued":
			if exec.QueuedAt != nil && now.Sub(*exec.QueuedAt) > time.Duration(exec.QueueWaitMinutes)*time.Minute {
				_ = st.SetRunnerResult(ctx, exec.ID, 0, "timed_out", "the job waited too long for a free runner")
				continue
			}
			if started >= 5 {
				continue
			}
			started++
			if err := Provision(ctx, st, pool, exec.ID); err != nil {
				logger.Error("provision github runner", "execution", exec.ID, "err", err)
			}
		case "provisioning":
			if now.Sub(exec.UpdatedAt) > 20*time.Minute {
				_ = st.SetRunnerResult(ctx, exec.ID, 0, "failed", "the runner did not become ready")
				finishCleanup(ctx, st, pool, exec)
			}
		case "idle":
			if exec.ReadyAt != nil && now.Sub(*exec.ReadyAt) > time.Duration(item.IdleTimeoutMinutes)*time.Minute {
				_ = st.SetRunnerResult(ctx, exec.ID, 0, "timed_out", "the runner received no job")
				finishCleanup(ctx, st, pool, exec)
			}
		case "running":
			if exec.StartedAt != nil && now.Sub(*exec.StartedAt) > time.Duration(item.JobTimeoutMinutes)*time.Minute {
				_ = st.SetRunnerResult(ctx, exec.ID, 0, "timed_out", "the job ran longer than the limit")
				finishCleanup(ctx, st, pool, exec)
			}
		}
	}
}

func finishCleanup(ctx context.Context, st *store.Store, pool *sshx.Pool, exec store.GithubRunnerExecution) {
	serverID := uuid.Nil
	if exec.ServerID != nil {
		serverID = *exec.ServerID
	}
	cleanupExecution(ctx, st, pool, exec.TeamID, serverID, exec.GitSourceID, exec.ID)
}

func appFor(ctx context.Context, st *store.Store, teamID, sourceID uuid.UUID) (*githubapp.App, error) {
	app, _, err := appAndSecrets(ctx, st, teamID, sourceID)
	return app, err
}

func appAndSecrets(ctx context.Context, st *store.Store, teamID, sourceID uuid.UUID) (*githubapp.App, *store.GitSourceSecrets, error) {
	sec, err := st.GetGitSourceSecrets(ctx, teamID, sourceID)
	if err != nil {
		return nil, nil, err
	}
	key, err := st.Box.DecryptString(sec.PrivateKeyEnc)
	if err != nil {
		return nil, nil, err
	}
	return &githubapp.App{
		AppID:         sec.AppID,
		ClientID:      sec.ClientID,
		PrivateKeyPEM: key,
		HTMLURL:       sec.HTMLURL,
		APIURL:        sec.APIURL,
		Name:          sec.Name,
		Organization:  sec.Organization,
	}, sec, nil
}

func runDocker(client *ssh.Client, argv []string) error {
	_, stderr, err := sshx.RunArgs(client, argv...)
	if err == nil {
		return nil
	}
	if len(argv) >= 3 && (argv[1] == "network" || argv[1] == "volume") && argv[2] == "create" && strings.Contains(stderr, "already exists") {
		return nil
	}
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		msg = err.Error()
	}
	return fmt.Errorf("%s", msg)
}

func waitForDocker(client *ssh.Client, id uuid.UUID) error {
	argv := DockerReadyArgs(id)
	for i := 0; i < 60; i++ {
		out, _, err := sshx.RunArgs(client, argv...)
		if err == nil && strings.TrimSpace(out) != "" {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("the Docker sidecar did not start within 60 seconds")
}
