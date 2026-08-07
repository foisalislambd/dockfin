package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/backup"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
)

type Runner struct {
	Store        *store.Store
	SSH          *sshx.Pool
	Logger       *slog.Logger
	DataDir      string
	DatabaseURL  string
	cancel       context.CancelFunc
}

func New(st *store.Store, pool *sshx.Pool, logger *slog.Logger, dataDir, databaseURL string) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{Store: st, SSH: pool, Logger: logger, DataDir: dataDir, DatabaseURL: databaseURL}
}

func (r *Runner) Start(ctx context.Context) {
	ctx, r.cancel = context.WithCancel(ctx)
	go r.loop(ctx)
}

func (r *Runner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}

func (r *Runner) loop(ctx context.Context) {
	// Align to next local-minute boundary, then tick every minute.
	now := time.Now()
	wait := time.Until(now.Truncate(time.Minute).Add(time.Minute))
	if wait <= 0 {
		wait = time.Minute
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			r.tick(ctx)
			timer.Reset(time.Minute)
		}
	}
}

func (r *Runner) tick(ctx context.Context) {
	// Use local wall clock so cron expressions match the host timezone
	// (same clock used to align the ticker).
	minute := time.Now().Truncate(time.Minute)
	r.runTasks(ctx, minute)
	r.runBackups(ctx, minute)
	r.runInstanceBackup(ctx, minute)
	r.runDockerCleanups(ctx, minute)
}

func (r *Runner) runTasks(ctx context.Context, minute time.Time) {
	tasks, err := r.Store.ListEnabledScheduledTasks(ctx)
	if err != nil {
		r.Logger.Error("list scheduled tasks", "err", err)
		return
	}
	for _, t := range tasks {
		if !Matches(t.Frequency, minute) {
			continue
		}
		ran, err := r.Store.TaskRanThisMinute(ctx, t.ID, minute)
		if err != nil {
			r.Logger.Error("task ran check", "task", t.ID, "err", err)
			continue
		}
		if ran {
			continue
		}
		// Claim the minute synchronously so concurrent ticks cannot double-run.
		execID, err := r.Store.CreateTaskExecution(ctx, t.TeamID, t.ID)
		if err != nil {
			r.Logger.Error("create task execution", "task", t.ID, "err", err)
			continue
		}
		go r.executeTask(context.Background(), t, execID)
	}
}

// ExecuteTaskNow runs a scheduled task immediately (manual trigger from the UI).
func (r *Runner) ExecuteTaskNow(ctx context.Context, t store.ScheduledTaskRow, execID uuid.UUID) {
	r.executeTask(ctx, t, execID)
}

func (r *Runner) executeTask(ctx context.Context, t store.ScheduledTaskRow, execID uuid.UUID) {
	done := false
	defer func() {
		if !done {
			_ = r.Store.FinishTaskExecution(context.Background(), execID, "failed", "interrupted")
		}
	}()

	client, container, err := r.dialForResource(ctx, t.TeamID, t.ResourceType, t.ResourceID)
	if err != nil {
		_ = r.Store.FinishTaskExecution(ctx, execID, "failed", err.Error())
		done = true
		return
	}
	if t.Container != "" {
		container = resolveServiceContainer(t.ResourceType, t.ResourceID, t.Container, container)
	}
	// Always run inside a container — never fall through to a raw host shell
	// (that would execute arbitrary cron commands on the VPS).
	if container == "" {
		_ = r.Store.FinishTaskExecution(ctx, execID, "failed", "no container resolved for scheduled task")
		done = true
		return
	}
	cmd := fmt.Sprintf("docker exec %s sh -lc %s", shellQuote(container), shellQuote(t.Command))
	stdout, stderr, err := sshx.Run(client, cmd)
	out := strings.TrimSpace(stdout + "\n" + stderr)
	if len(out) > 8000 {
		out = out[:8000] + "…"
	}
	status := "finished"
	if err != nil {
		status = "failed"
		if out == "" {
			out = err.Error()
		} else {
			out = out + "\n" + err.Error()
		}
	}
	_ = r.Store.FinishTaskExecution(ctx, execID, status, out)
	done = true
	r.Logger.Info("scheduled task ran", "task", t.ID, "name", t.Name, "status", status)
}

func (r *Runner) runInstanceBackup(ctx context.Context, minute time.Time) {
	cfg, err := r.Store.GetInstanceBackupConfig(ctx)
	if err != nil || !cfg.Configured || !cfg.Enabled {
		return
	}
	if !Matches(cfg.Frequency, minute) {
		return
	}
	ran, err := r.Store.InstanceBackupRanThisMinute(ctx, minute)
	if err != nil {
		r.Logger.Error("instance backup ran check", "err", err)
		return
	}
	if ran {
		return
	}
	teamID, err := r.Store.FirstTeamID(ctx)
	if err != nil {
		r.Logger.Error("instance backup team", "err", err)
		return
	}
	user, password, dbName, err := backup.ParseDatabaseURL(r.DatabaseURL)
	if err != nil {
		r.Logger.Error("instance backup db url", "err", err)
		return
	}
	if cfg.DBUser != "" {
		user = cfg.DBUser
	}
	if cfg.DBName != "" {
		dbName = cfg.DBName
	}
	container, err := backup.DetectPostgresContainer(cfg.Container)
	if err != nil {
		r.Logger.Error("instance backup container", "err", err)
		return
	}
	filename := backup.InstanceDumpFilename()
	execRow, err := r.Store.CreateInstanceBackupExecution(ctx, teamID, filename)
	if err != nil {
		r.Logger.Error("instance backup execution", "err", err)
		return
	}
	_, size, err := backup.DumpInstanceLocal(r.DataDir, container, user, password, dbName, filename)
	if err != nil {
		_ = r.Store.FinishBackupExecution(ctx, execRow.ID, "failed", 0, err.Error())
		r.Logger.Error("instance backup dump", "err", err)
		return
	}
	_ = r.Store.FinishBackupExecution(ctx, execRow.ID, "finished", size, "")
	_ = backup.EnforceLocalRetention(r.DataDir, cfg.Retention)
	r.Logger.Info("instance backup finished", "file", filename, "bytes", size)
}

func (r *Runner) runBackups(ctx context.Context, minute time.Time) {
	list, err := r.Store.ListEnabledScheduledBackups(ctx)
	if err != nil {
		r.Logger.Error("list scheduled backups", "err", err)
		return
	}
	for _, b := range list {
		if !Matches(b.Frequency, minute) {
			continue
		}
		ran, err := r.Store.BackupRanThisMinute(ctx, b.ID, minute)
		if err != nil {
			r.Logger.Error("backup ran check", "backup", b.ID, "err", err)
			continue
		}
		if ran {
			continue
		}
		sid := b.ID
		if b.ResourceType == "application" {
			filename := backup.DefaultAppBackupFilename(b.ResourceID.String())
			exec, err := r.Store.CreateBackupExecutionScheduled(ctx, b.TeamID, &sid, "application", b.ResourceID, filename)
			if err != nil {
				r.Logger.Error("create app backup execution", "err", err)
				continue
			}
			go r.executeApplicationBackup(context.Background(), b, exec.ID, filename)
			continue
		}
		if b.ResourceType != "database" {
			exec, err := r.Store.CreateBackupExecutionScheduled(ctx, b.TeamID, &sid, b.ResourceType, b.ResourceID, "skipped")
			if err == nil {
				_ = r.Store.FinishBackupExecution(ctx, exec.ID, "failed", 0, "unsupported resource_type")
			}
			continue
		}
		db, err := r.Store.GetDatabase(ctx, b.TeamID, b.ResourceID)
		if err != nil {
			exec, cerr := r.Store.CreateBackupExecutionScheduled(ctx, b.TeamID, &sid, "database", b.ResourceID, "skipped")
			if cerr == nil {
				_ = r.Store.FinishBackupExecution(ctx, exec.ID, "failed", 0, err.Error())
			}
			continue
		}
		if db.Engine != "postgresql" && db.Engine != "mysql" && db.Engine != "mariadb" && db.Engine != "redis" && db.Engine != "keydb" {
			exec, err := r.Store.CreateBackupExecutionScheduled(ctx, b.TeamID, &sid, "database", db.ID, "skipped")
			if err == nil {
				_ = r.Store.FinishBackupExecution(ctx, exec.ID, "failed", 0, fmt.Sprintf("scheduled backup does not support engine %q", db.Engine))
			}
			continue
		}
		filename := backup.DefaultFilename(db.Engine, db.ID.String())
		exec, err := r.Store.CreateBackupExecutionScheduled(ctx, b.TeamID, &sid, "database", db.ID, filename)
		if err != nil {
			r.Logger.Error("create backup execution", "err", err)
			continue
		}
		go r.executeBackup(context.Background(), db, exec.ID, filename, b.S3StorageID, b.Retention)
	}
}

func (r *Runner) executeBackup(ctx context.Context, db *store.Database, execID uuid.UUID, filename string, s3StorageID *uuid.UUID, retention int) {
	done := false
	defer func() {
		if !done {
			_ = r.Store.FinishBackupExecution(context.Background(), execID, "failed", 0, "interrupted")
		}
	}()

	client, password, err := r.dialDatabase(ctx, db)
	if err != nil {
		_ = r.Store.FinishBackupExecution(ctx, execID, "failed", 0, err.Error())
		done = true
		return
	}
	path := backup.DumpPath(filename)
	container := "dockfin-db-" + db.ID.String()
	if err := backup.DumpDatabase(client, db.Engine, container, password, path); err != nil {
		_ = r.Store.FinishBackupExecution(ctx, execID, "failed", 0, err.Error())
		done = true
		return
	}
	size := backup.FileSize(client, path)
	_ = r.Store.FinishBackupExecution(ctx, execID, "finished", size, "")
	done = true
	r.Logger.Info("scheduled backup finished", "db", db.ID, "file", filename, "bytes", size)
	_ = backup.EnforceRemoteRetention(client, db.Engine, db.ID.String(), retention)

	if s3StorageID != nil {
		if err := r.uploadBackupToS3(ctx, client, db, execID, filename, path, *s3StorageID); err != nil {
			r.Logger.Error("s3 upload after backup", "db", db.ID, "err", err)
		}
	}
}

func (r *Runner) executeApplicationBackup(ctx context.Context, b store.ScheduledBackupRow, execID uuid.UUID, filename string) {
	done := false
	defer func() {
		if !done {
			_ = r.Store.FinishBackupExecution(context.Background(), execID, "failed", 0, "interrupted")
		}
	}()

	app, err := r.Store.GetApplication(ctx, b.TeamID, b.ResourceID)
	if err != nil {
		_ = r.Store.FinishBackupExecution(ctx, execID, "failed", 0, err.Error())
		done = true
		return
	}
	if app.DestinationID == nil {
		_ = r.Store.FinishBackupExecution(ctx, execID, "failed", 0, "application has no destination")
		done = true
		return
	}
	client, err := r.dialDestination(ctx, b.TeamID, *app.DestinationID)
	if err != nil {
		_ = r.Store.FinishBackupExecution(ctx, execID, "failed", 0, err.Error())
		done = true
		return
	}

	path := backup.AppVolumeArchivePath(app.ID.String(), filename)
	var paths []string
	vols, _ := r.Store.ListVolumes(ctx, b.TeamID, "application", app.ID)
	if b.VolumeID != nil {
		for _, v := range vols {
			if v.ID == *b.VolumeID {
				host := strings.TrimSpace(v.HostPath)
				if host == "" {
					host = "/data/dockfin/applications/" + app.ID.String() + "/volumes/" + v.Name
				}
				paths = append(paths, host)
				break
			}
		}
		if len(paths) == 0 {
			_ = r.Store.FinishBackupExecution(ctx, execID, "failed", 0, "volume not found")
			done = true
			return
		}
	} else if len(vols) > 0 {
		// Prefer the parent volumes dir when present to avoid packing parent+children twice.
		parent := "/data/dockfin/applications/" + app.ID.String() + "/volumes"
		allUnderParent := true
		var leafs []string
		for _, v := range vols {
			host := strings.TrimSpace(v.HostPath)
			if host == "" {
				host = parent + "/" + v.Name
			}
			leafs = append(leafs, host)
			if !strings.HasPrefix(host, parent+"/") && host != parent {
				allUnderParent = false
			}
		}
		if allUnderParent {
			paths = []string{parent}
		} else {
			paths = leafs
		}
	} else {
		paths = []string{"/data/dockfin/applications/" + app.ID.String() + "/volumes"}
	}
	paths = backup.FilterExistingPaths(client, paths)
	if len(paths) == 0 {
		_ = r.Store.FinishBackupExecution(ctx, execID, "failed", 0, "no volume paths found on server — deploy first or add volumes")
		done = true
		return
	}
	if err := backup.TarHostPaths(client, path, paths); err != nil {
		_ = r.Store.FinishBackupExecution(ctx, execID, "failed", 0, err.Error())
		done = true
		return
	}
	size := backup.FileSize(client, path)
	_ = r.Store.FinishBackupExecution(ctx, execID, "finished", size, "")
	done = true
	r.Logger.Info("application backup finished", "app", app.ID, "file", filename, "bytes", size)
	_ = backup.EnforceAppBackupRetention(client, app.ID.String(), b.Retention)

	if b.S3StorageID != nil {
		if err := r.uploadAppBackupToS3(ctx, client, app.TeamID, app.ID, execID, filename, path, *b.S3StorageID); err != nil {
			r.Logger.Error("s3 upload after app backup", "app", app.ID, "err", err)
		}
	}
}

func (r *Runner) uploadAppBackupToS3(ctx context.Context, client *ssh.Client, teamID, appID, execID uuid.UUID, filename, remotePath string, s3ID uuid.UUID) error {
	st, err := r.Store.GetS3Storage(ctx, teamID, s3ID)
	if err != nil {
		return err
	}
	akEnc, skEnc, err := r.Store.GetS3StorageSecrets(ctx, teamID, s3ID)
	if err != nil {
		return err
	}
	accessKey, err := r.Store.Box.DecryptString(akEnc)
	if err != nil {
		return fmt.Errorf("decrypt access key: %w", err)
	}
	secretKey, err := r.Store.Box.DecryptString(skEnc)
	if err != nil {
		return fmt.Errorf("decrypt secret key: %w", err)
	}
	objectKey := "backups/applications/" + appID.String() + "/" + filename
	if err := backup.UploadRemoteToS3(client, remotePath, objectKey, backup.S3Creds{
		Endpoint:  st.Endpoint,
		Bucket:    st.Bucket,
		Region:    st.Region,
		AccessKey: accessKey,
		SecretKey: secretKey,
		PathStyle: st.PathStyle,
	}); err != nil {
		return err
	}
	return r.Store.MarkBackupS3Uploaded(ctx, execID, objectKey)
}

func (r *Runner) uploadBackupToS3(ctx context.Context, client *ssh.Client, db *store.Database, execID uuid.UUID, filename, remotePath string, s3ID uuid.UUID) error {
	st, err := r.Store.GetS3Storage(ctx, db.TeamID, s3ID)
	if err != nil {
		return err
	}
	akEnc, skEnc, err := r.Store.GetS3StorageSecrets(ctx, db.TeamID, s3ID)
	if err != nil {
		return err
	}
	accessKey, err := r.Store.Box.DecryptString(akEnc)
	if err != nil {
		return fmt.Errorf("decrypt access key: %w", err)
	}
	secretKey, err := r.Store.Box.DecryptString(skEnc)
	if err != nil {
		return fmt.Errorf("decrypt secret key: %w", err)
	}
	objectKey := "backups/" + db.ID.String() + "/" + filename
	if err := backup.UploadRemoteToS3(client, remotePath, objectKey, backup.S3Creds{
		Endpoint:  st.Endpoint,
		Bucket:    st.Bucket,
		Region:    st.Region,
		AccessKey: accessKey,
		SecretKey: secretKey,
		PathStyle: st.PathStyle,
	}); err != nil {
		return err
	}
	return r.Store.MarkBackupS3Uploaded(ctx, execID, objectKey)
}

func (r *Runner) dialForResource(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) (*ssh.Client, string, error) {
	switch resourceType {
	case "application":
		app, err := r.Store.GetApplication(ctx, teamID, resourceID)
		if err != nil {
			return nil, "", err
		}
		if app.DestinationID == nil {
			return nil, "", fmt.Errorf("application has no destination")
		}
		client, err := r.dialDestination(ctx, teamID, *app.DestinationID)
		return client, "dockfin-" + app.ID.String(), err
	case "database":
		db, err := r.Store.GetDatabase(ctx, teamID, resourceID)
		if err != nil {
			return nil, "", err
		}
		if db.DestinationID == nil {
			return nil, "", fmt.Errorf("database has no destination")
		}
		client, err := r.dialDestination(ctx, teamID, *db.DestinationID)
		return client, "dockfin-db-" + db.ID.String(), err
	case "service":
		svc, err := r.Store.GetService(ctx, teamID, resourceID)
		if err != nil {
			return nil, "", err
		}
		var client *ssh.Client
		switch {
		case svc.ServerID != nil:
			client, err = r.dialServer(ctx, teamID, *svc.ServerID)
		case svc.DestinationID != nil:
			client, err = r.dialDestination(ctx, teamID, *svc.DestinationID)
		default:
			return nil, "", fmt.Errorf("service has no server or destination")
		}
		if err != nil {
			return nil, "", err
		}
		compose := svc.DockerCompose
		if compose == "" {
			compose = svc.DockerComposeRaw
		}
		return client, defaultServiceContainer(svc.ID, compose), nil
	default:
		return nil, "", fmt.Errorf("unsupported resource_type %s", resourceType)
	}
}

// defaultServiceContainer picks the first compose unit container name for a stack.
func defaultServiceContainer(serviceID uuid.UUID, composeYAML string) string {
	units := services.ParseComposeUnits(composeYAML)
	if len(units) == 0 {
		return ""
	}
	return fmt.Sprintf("dockfin-svc-%s-%s-1", serviceID.String()[:8], units[0].Name)
}

// resolveServiceContainer maps a compose service name (or full container name) to a docker name.
func resolveServiceContainer(resourceType string, resourceID uuid.UUID, named, fallback string) string {
	named = strings.TrimSpace(named)
	if named == "" {
		return fallback
	}
	if strings.HasPrefix(named, "dockfin-svc-") || strings.HasPrefix(named, "dockfin-db-") || strings.HasPrefix(named, "dockfin-") {
		return named
	}
	if resourceType == "service" {
		return fmt.Sprintf("dockfin-svc-%s-%s-1", resourceID.String()[:8], named)
	}
	return named
}

func (r *Runner) dialDatabase(ctx context.Context, db *store.Database) (*ssh.Client, string, error) {
	if db.DestinationID == nil {
		return nil, "", fmt.Errorf("database has no destination")
	}
	client, err := r.dialDestination(ctx, db.TeamID, *db.DestinationID)
	if err != nil {
		return nil, "", err
	}
	enc, err := r.Store.GetDatabaseCredentials(ctx, db.TeamID, db.ID)
	if err != nil {
		return nil, "", err
	}
	password := ""
	if enc != "" {
		plain, err := r.Store.Box.Decrypt(enc)
		if err != nil {
			return nil, "", fmt.Errorf("decrypt credentials")
		}
		var creds map[string]string
		if json.Unmarshal(plain, &creds) == nil {
			password = creds["password"]
		}
	}
	return client, password, nil
}

func (r *Runner) dialDestination(ctx context.Context, teamID, destID uuid.UUID) (*ssh.Client, error) {
	dest, err := r.Store.GetDestination(ctx, teamID, destID)
	if err != nil {
		return nil, err
	}
	return r.dialServer(ctx, teamID, dest.ServerID)
}

func (r *Runner) dialServer(ctx context.Context, teamID, serverID uuid.UUID) (*ssh.Client, error) {
	srv, err := r.Store.GetServer(ctx, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if srv.PrivateKeyID == nil {
		return nil, fmt.Errorf("server has no private key")
	}
	enc, err := r.Store.GetPrivateKeyMaterial(ctx, teamID, *srv.PrivateKeyID)
	if err != nil {
		return nil, err
	}
	priv, err := r.Store.Box.DecryptString(enc)
	if err != nil {
		return nil, err
	}
	res, err := r.SSH.Dial(sshx.Target{
		Host:                srv.IP,
		Port:                srv.Port,
		User:                srv.UserName,
		PrivateKey:          []byte(priv),
		ExpectedFingerprint: srv.HostKeyFingerprint,
		ExpectedKeyType:     srv.HostKeyType,
	})
	if err != nil {
		return nil, err
	}
	return res.Client, nil
}

func (r *Runner) runDockerCleanups(ctx context.Context, minute time.Time) {
	servers, err := r.Store.ListServersForDockerCleanup(ctx)
	if err != nil {
		r.Logger.Error("list servers for docker cleanup", "err", err)
		return
	}
	for _, s := range servers {
		if !Matches(s.Frequency, minute) {
			continue
		}
		// Skip if already started this minute.
		recent, err := r.Store.ListDockerCleanupExecutions(ctx, s.TeamID, s.ServerID, 1)
		if err == nil && len(recent) > 0 && recent[0].StartedAt.Truncate(time.Minute).Equal(minute) {
			continue
		}
		ops, err := r.Store.GetServerOpsSettings(ctx, s.TeamID, s.ServerID)
		if err != nil {
			continue
		}
		exec, err := r.Store.CreateDockerCleanupExecution(ctx, s.TeamID, s.ServerID)
		if err != nil {
			r.Logger.Error("create docker cleanup execution", "server", s.ServerID, "err", err)
			continue
		}
		go func(teamID, serverID, execID uuid.UUID, force bool, threshold int) {
			client, err := r.dialServer(context.Background(), teamID, serverID)
			if err != nil {
				_ = r.Store.FinishDockerCleanupExecution(context.Background(), execID, "failed", err.Error())
				return
			}
			msg, runErr := scheduledDockerCleanup(client, force, threshold)
			status := "finished"
			if runErr != nil {
				status = "failed"
				if msg == "" {
					msg = runErr.Error()
				} else {
					msg += "; " + runErr.Error()
				}
			}
			_ = r.Store.FinishDockerCleanupExecution(context.Background(), execID, status, msg)
		}(s.TeamID, s.ServerID, exec.ID, ops.ForceDockerCleanup, ops.DockerCleanupThreshold)
	}
}

func scheduledDockerCleanup(client *ssh.Client, force bool, threshold int) (string, error) {
	var parts []string
	if !force && threshold > 0 {
		out, _, err := sshx.Run(client, `df -P / | awk 'NR==2 {gsub(/%/,"",$5); print $5}'`)
		if err == nil {
			used := 0
			fmt.Sscanf(strings.TrimSpace(out), "%d", &used)
			if used > 0 && used < threshold {
				return fmt.Sprintf("skipped: disk usage %d%% below threshold %d%%", used, threshold), nil
			}
			parts = append(parts, fmt.Sprintf("disk usage %d%%", used))
		}
	}
	if _, errOut, err := sshx.RunArgs(client, "docker", "image", "prune", "-af"); err != nil {
		return strings.Join(parts, "; "), fmt.Errorf("image prune: %v %s", err, errOut)
	}
	parts = append(parts, "image prune ok")
	if _, errOut, err := sshx.RunArgs(client, "docker", "builder", "prune", "-af"); err != nil {
		return strings.Join(parts, "; "), fmt.Errorf("builder prune: %v %s", err, errOut)
	}
	parts = append(parts, "builder prune ok")
	_, _, _ = sshx.RunArgs(client, "docker", "container", "prune", "-f")
	parts = append(parts, "container prune ok")
	return strings.Join(parts, "; "), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
