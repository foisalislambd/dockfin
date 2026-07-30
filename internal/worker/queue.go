package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/deploy"
	"github.com/goolify/goolify/internal/notify"
	"github.com/goolify/goolify/internal/sshx"
	"github.com/goolify/goolify/internal/store"
	"github.com/goolify/goolify/internal/ws"
)

type DeployJob struct {
	DeploymentID uuid.UUID
	TeamID       uuid.UUID
	ForceRebuild bool
}

// Queue is an in-process job queue backed by Postgres deployment rows.
type Queue struct {
	Store  *store.Store
	SSH    *sshx.Pool
	Hub    *ws.Hub
	jobs   chan DeployJob
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewQueue(st *store.Store, sshPool *sshx.Pool, hub *ws.Hub, workers int) *Queue {
	if workers < 1 {
		workers = 2
	}
	return &Queue{
		Store: st,
		SSH:   sshPool,
		Hub:   hub,
		jobs:  make(chan DeployJob, 256),
	}
}

func (q *Queue) Start(ctx context.Context, workers int) {
	ctx, q.cancel = context.WithCancel(ctx)
	if workers < 1 {
		workers = 2
	}
	q.reclaim(ctx)
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go func() {
			defer q.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-q.jobs:
					q.handle(ctx, job)
				}
			}
		}()
	}
}

func (q *Queue) reclaim(ctx context.Context) {
	rows, err := q.Store.ListQueuedDeployments(ctx, 200)
	if err != nil {
		slog.Error("reclaim queued deployments", "err", err)
		return
	}
	for _, row := range rows {
		// Stale in_progress from a crashed process → re-queue
		if row.Status == "in_progress" {
			_ = q.Store.SetDeploymentStatus(ctx, row.ID, "queued", "reclaimed after restart")
		}
		job := DeployJob{DeploymentID: row.ID, TeamID: row.TeamID}
		select {
		case q.jobs <- job:
			slog.Info("reclaimed queued deployment", "id", row.ID)
		default:
			slog.Warn("queue full during reclaim", "id", row.ID)
			return
		}
	}
}

func (q *Queue) Stop() {
	if q.cancel != nil {
		q.cancel()
	}
	q.wg.Wait()
}

func (q *Queue) Enqueue(job DeployJob) error {
	select {
	case q.jobs <- job:
		return nil
	default:
		_ = q.Store.SetDeploymentStatus(context.Background(), job.DeploymentID, "failed", "deploy queue full")
		return errQueueFull
	}
}

var errQueueFull = errString("deploy queue full")

type errString string

func (e errString) Error() string { return string(e) }

func (q *Queue) handle(ctx context.Context, job DeployJob) {
	dep, err := q.Store.GetDeployment(ctx, job.TeamID, job.DeploymentID)
	if err != nil {
		slog.Error("deployment not found", "id", job.DeploymentID, "err", err)
		_ = q.Store.SetDeploymentStatus(ctx, job.DeploymentID, "failed", "deployment not found")
		return
	}
	if dep.Status == "cancelled" || dep.Status == "finished" || dep.Status == "failed" {
		return
	}

	_ = q.Store.SetDeploymentStatus(ctx, job.DeploymentID, "in_progress", "")
	q.publish(job.DeploymentID, "status", "in_progress")

	app, err := q.Store.GetApplication(ctx, job.TeamID, dep.ApplicationID)
	if err != nil {
		_ = q.Store.SetDeploymentStatus(ctx, job.DeploymentID, "failed", err.Error())
		q.publish(job.DeploymentID, "status", "failed")
		return
	}
	if app.DestinationID == nil {
		_ = q.Store.SetDeploymentStatus(ctx, job.DeploymentID, "failed", "application has no destination")
		q.publish(job.DeploymentID, "status", "failed")
		return
	}
	dest, err := q.Store.GetDestination(ctx, job.TeamID, *app.DestinationID)
	if err != nil {
		_ = q.Store.SetDeploymentStatus(ctx, job.DeploymentID, "failed", err.Error())
		q.publish(job.DeploymentID, "status", "failed")
		return
	}
	srv, err := q.Store.GetServer(ctx, job.TeamID, dest.ServerID)
	if err != nil {
		_ = q.Store.SetDeploymentStatus(ctx, job.DeploymentID, "failed", err.Error())
		q.publish(job.DeploymentID, "status", "failed")
		return
	}
	if srv.PrivateKeyID == nil {
		_ = q.Store.SetDeploymentStatus(ctx, job.DeploymentID, "failed", "server has no private key")
		q.publish(job.DeploymentID, "status", "failed")
		return
	}
	enc, err := q.Store.GetPrivateKeyMaterial(ctx, job.TeamID, *srv.PrivateKeyID)
	if err != nil {
		_ = q.Store.SetDeploymentStatus(ctx, job.DeploymentID, "failed", err.Error())
		q.publish(job.DeploymentID, "status", "failed")
		return
	}
	priv, err := q.Store.Box.DecryptString(enc)
	if err != nil {
		_ = q.Store.SetDeploymentStatus(ctx, job.DeploymentID, "failed", "decrypt key: "+err.Error())
		q.publish(job.DeploymentID, "status", "failed")
		return
	}

	pipe := &deploy.Pipeline{
		Store: q.Store,
		SSH:   q.SSH,
		Log: func(stage, line string) {
			_ = q.Store.AppendDeploymentLog(context.Background(), job.DeploymentID, stage, line)
			q.publish(job.DeploymentID, stage, line)
		},
	}

	err = pipe.Run(ctx, deploy.Request{
		DeploymentID: job.DeploymentID,
		TeamID:       job.TeamID,
		App:          app,
		Server:       srv,
		Destination:  dest,
		PrivateKey:   []byte(priv),
		ForceRebuild: job.ForceRebuild,
	})
	if err != nil {
		slog.Error("deployment failed", "id", job.DeploymentID, "err", err)
		_ = q.Store.SetDeploymentStatus(ctx, job.DeploymentID, "failed", err.Error())
		_ = q.Store.UpdateApplicationStatus(ctx, app.ID, "exited")
		q.publish(job.DeploymentID, "status", "failed")
		q.notifyDeploy(ctx, job.TeamID, app.Name, "failed", err.Error())
		return
	}
	_ = q.Store.SetDeploymentStatus(ctx, job.DeploymentID, "finished", "")
	q.publish(job.DeploymentID, "status", "finished")
	q.notifyDeploy(ctx, job.TeamID, app.Name, "finished", "")
}

func (q *Queue) notifyDeploy(ctx context.Context, teamID uuid.UUID, appName, status, errMsg string) {
	settings, err := q.Store.ListEnabledNotifications(ctx, teamID)
	if err != nil || len(settings) == 0 {
		return
	}
	eventType := "deployment_success"
	title := "Deployment succeeded"
	msg := appName + " finished successfully"
	if status == "failed" {
		eventType = "deployment_failed"
		title = "Deployment failed"
		msg = appName + ": " + errMsg
	}
	ev := notify.Event{
		TeamID:  teamID.String(),
		Type:    eventType,
		Title:   title,
		Message: msg,
	}
	for _, n := range settings {
		if !eventAllowed(n.Events, eventType) {
			continue
		}
		cfgJSON, decErr := q.Store.Box.DecryptString(n.ConfigEnc)
		if decErr != nil || cfgJSON == "" {
			continue
		}
		switch n.Channel {
		case "webhook":
			var cfg notify.WebhookConfig
			if json.Unmarshal([]byte(cfgJSON), &cfg) == nil && cfg.URL != "" {
				_ = notify.SendWebhook(ctx, cfg.URL, ev)
			}
		case "discord":
			var cfg notify.DiscordConfig
			if json.Unmarshal([]byte(cfgJSON), &cfg) == nil && cfg.WebhookURL != "" {
				_ = notify.SendDiscord(ctx, cfg.WebhookURL, ev)
			}
		case "slack":
			var cfg notify.SlackConfig
			if json.Unmarshal([]byte(cfgJSON), &cfg) == nil && cfg.WebhookURL != "" {
				_ = notify.SendSlack(ctx, cfg.WebhookURL, ev)
			}
		}
	}
}

func eventAllowed(events []string, want string) bool {
	if len(events) == 0 {
		return true
	}
	for _, e := range events {
		if strings.EqualFold(e, want) {
			return true
		}
	}
	return false
}

func (q *Queue) publish(id uuid.UUID, stage, line string) {
	if q.Hub == nil {
		return
	}
	q.Hub.Publish(id, map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"stage": stage,
		"line":  line,
	})
}
