package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/git"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/worker"
	"golang.org/x/crypto/ssh"
)

type webhookActionResult struct {
	ApplicationID   uuid.UUID  `json:"application_id"`
	ApplicationName string     `json:"application_name"`
	Status          string     `json:"status"` // success | skipped | failed
	Message         string     `json:"message,omitempty"`
	DeploymentID    *uuid.UUID `json:"deployment_id,omitempty"`
	PreviewID       *uuid.UUID `json:"preview_id,omitempty"`
	PreviewFQDN     string     `json:"preview_fqdn,omitempty"`
	Commit          string     `json:"commit,omitempty"`
}

func (a *API) resolveAppServerID(ctx context.Context, app *store.Application) *uuid.UUID {
	if app == nil || app.DestinationID == nil {
		return nil
	}
	dest, err := a.Store.GetDestination(ctx, app.TeamID, *app.DestinationID)
	if err != nil {
		return nil
	}
	return &dest.ServerID
}

func (a *API) enqueueAutoDeploy(ctx context.Context, app *store.Application, event *git.PushEvent) webhookActionResult {
	res := webhookActionResult{
		ApplicationID:   app.ID,
		ApplicationName: app.Name,
		Commit:          event.Commit,
	}
	if app.GitBranch != "" && event.Branch != "" && event.Branch != app.GitBranch {
		res.Status = "skipped"
		res.Message = "branch mismatch"
		return res
	}
	if git.IsNullCommit(event.Commit) {
		res.Status = "skipped"
		res.Message = "branch deleted or null commit"
		return res
	}
	if !app.IsAutoDeployEnabled {
		res.Status = "skipped"
		res.Message = "auto deploy disabled"
		return res
	}
	if git.ShouldSkipDeploy(event.CommitMessages) || (len(event.CommitMessages) == 0 && git.ShouldSkipDeploy([]string{event.Message})) {
		res.Status = "skipped"
		res.Message = "all commits contain [skip ci] or [skip cd]"
		return res
	}
	if paths := strings.TrimSpace(app.WatchPaths); paths != "" && len(event.ChangedFiles) > 0 {
		if !services.WatchPathsMatch(paths, event.ChangedFiles) {
			res.Status = "skipped"
			res.Message = "watch paths mismatch"
			return res
		}
	}
	serverID := a.resolveAppServerID(ctx, app)
	dep, err := a.Store.CreateDeployment(ctx, app.TeamID, app.ID, serverID, event.Commit, event.Message, false, true, false, 0)
	if err != nil {
		res.Status = "failed"
		res.Message = err.Error()
		return res
	}
	if err := a.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: app.TeamID}); err != nil {
		res.Status = "failed"
		res.Message = err.Error()
		return res
	}
	res.Status = "success"
	res.Message = "deployment queued"
	res.DeploymentID = &dep.ID
	return res
}

func (a *API) enqueuePreviewDeploy(ctx context.Context, app *store.Application, event *git.PushEvent) webhookActionResult {
	res := webhookActionResult{
		ApplicationID:   app.ID,
		ApplicationName: app.Name,
		Commit:          event.Commit,
	}
	if event.PRNumber <= 0 {
		res.Status = "skipped"
		res.Message = "not a pull request"
		return res
	}
	if !app.IsPreviewEnabled {
		res.Status = "skipped"
		res.Message = "preview deployments disabled"
		return res
	}
	// PR/MR must target the app's configured branch.
	base := event.BaseBranch
	if base != "" && app.GitBranch != "" && base != app.GitBranch {
		res.Status = "skipped"
		res.Message = "base branch mismatch"
		return res
	}
	if git.ShouldSkipDeployAny(event.CommitMessages) || git.ShouldSkipDeployAny([]string{event.Message}) {
		res.Status = "skipped"
		res.Message = "commit or title contains [skip ci] or [skip cd]"
		return res
	}
	fqdn := ""
	if app.FQDN != "" {
		host := strings.TrimSpace(strings.Split(app.FQDN, ",")[0])
		// strip scheme if stored as URL
		host = strings.TrimPrefix(host, "https://")
		host = strings.TrimPrefix(host, "http://")
		host = strings.Split(host, "/")[0]
		fqdn = fmt.Sprintf("pr-%d.%s", event.PRNumber, host)
	}
	preview, err := a.Store.CreatePreview(ctx, app.TeamID, app.ID, event.PRNumber, event.Message, event.Branch, fqdn)
	if err != nil {
		res.Status = "failed"
		res.Message = err.Error()
		return res
	}
	serverID := a.resolveAppServerID(ctx, app)
	dep, err := a.Store.CreateDeployment(ctx, app.TeamID, app.ID, serverID, event.Commit, event.Message, false, true, false, event.PRNumber)
	if err != nil {
		res.Status = "failed"
		res.Message = err.Error()
		return res
	}
	if err := a.Queue.Enqueue(worker.DeployJob{DeploymentID: dep.ID, TeamID: app.TeamID}); err != nil {
		res.Status = "failed"
		res.Message = err.Error()
		return res
	}
	res.Status = "success"
	res.Message = "preview deployment queued"
	res.DeploymentID = &dep.ID
	res.PreviewID = &preview.ID
	res.PreviewFQDN = preview.FQDN
	return res
}

func (a *API) cleanupPreview(ctx context.Context, app *store.Application, prID int) webhookActionResult {
	res := webhookActionResult{
		ApplicationID:   app.ID,
		ApplicationName: app.Name,
	}
	if prID <= 0 {
		res.Status = "skipped"
		res.Message = "invalid pr id"
		return res
	}
	a.cleanupPreviewRemote(ctx, app, prID)
	if err := a.Store.DeletePreview(ctx, app.TeamID, app.ID, prID); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			res.Status = "failed"
			res.Message = err.Error()
			return res
		}
	}
	res.Status = "success"
	res.Message = "preview cleaned up"
	return res
}

func (a *API) cleanupPreviewRemote(ctx context.Context, app *store.Application, prID int) {
	if app.DestinationID == nil {
		return
	}
	dest, err := a.Store.GetDestination(ctx, app.TeamID, *app.DestinationID)
	if err != nil {
		return
	}
	client, err := a.dialServerForTeam(ctx, app.TeamID, dest.ServerID)
	if err != nil {
		return
	}
	appID := app.ID
	cname := fmt.Sprintf("dockfin-%s-pr-%d", appID.String(), prID)
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", cname)
	project := fmt.Sprintf("dockfin-%s-pr-%d", appID.String()[:8], prID)
	workdir := fmt.Sprintf("/data/dockfin/applications/%s-pr-%d", appID.String(), prID)
	for _, f := range []string{
		workdir + "/src/docker-compose.yaml",
		workdir + "/src/docker-compose.yml",
		workdir + "/src/compose.yaml",
	} {
		_, _, _ = sshx.RunArgs(client, "docker", "compose", "-p", project, "-f", f, "down", "--remove-orphans", "-v")
	}
	_, _, _ = sshx.RunArgs(client, "rm", "-rf", workdir)
}

func (a *API) dialServerForTeam(ctx context.Context, teamID, serverID uuid.UUID) (*ssh.Client, error) {
	srv, err := a.Store.GetServer(ctx, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if srv.PrivateKeyID == nil {
		return nil, fmt.Errorf("server has no private key")
	}
	enc, err := a.Store.GetPrivateKeyMaterial(ctx, teamID, *srv.PrivateKeyID)
	if err != nil {
		return nil, err
	}
	priv, err := a.Store.Box.DecryptString(enc)
	if err != nil {
		return nil, err
	}
	if a.Queue == nil || a.Queue.SSH == nil {
		return nil, fmt.Errorf("ssh pool unavailable")
	}
	res, err := a.Queue.SSH.Dial(sshx.Target{
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
	if res.IsNewHost {
		_ = a.Store.UpdateServerHostKey(ctx, serverID, res.Fingerprint, res.KeyType)
	}
	return res.Client, nil
}

// processWebhookEvent applies push / preview / cleanup for one application.
func (a *API) processWebhookEvent(ctx context.Context, app *store.Application, event *git.PushEvent) webhookActionResult {
	if event == nil {
		return webhookActionResult{Status: "skipped", Message: "empty event"}
	}
	switch strings.ToLower(event.Action) {
	case "ping", "ignored":
		return webhookActionResult{
			ApplicationID:   app.ID,
			ApplicationName: app.Name,
			Status:          "skipped",
			Message:         event.Action,
		}
	}
	if event.IsClosed() && event.PRNumber > 0 {
		return a.cleanupPreview(ctx, app, event.PRNumber)
	}
	if event.IsPreviewOpen() {
		return a.enqueuePreviewDeploy(ctx, app, event)
	}
	return a.enqueueAutoDeploy(ctx, app, event)
}

func detectWebhookProvider(r *http.Request, queryProvider string) string {
	if q := strings.TrimSpace(queryProvider); q != "" {
		return strings.ToLower(q)
	}
	p := git.DetectProvider(r)
	if p == "generic" {
		// Backward compatible default for clients that omit provider headers.
		return "github"
	}
	return p
}
