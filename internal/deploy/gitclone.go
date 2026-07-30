package deploy

import (
	"context"
	"fmt"

	"github.com/goolify/goolify/internal/git/githubapp"
	"github.com/goolify/goolify/internal/sshx"
	"golang.org/x/crypto/ssh"
)

func (p *Pipeline) cloneURL(ctx context.Context, req Request) (string, error) {
	repo := req.App.GitRepository
	if req.App.GitSourceID == nil || p.Store == nil {
		return repo, nil
	}
	sec, err := p.Store.GetGitSourceSecrets(ctx, req.TeamID, *req.App.GitSourceID)
	if err != nil {
		return "", fmt.Errorf("git source: %w", err)
	}
	if sec.InstallationID == "" {
		return repo, nil
	}
	plain, err := p.Store.Box.DecryptString(sec.PrivateKeyEnc)
	if err != nil {
		return "", fmt.Errorf("decrypt git source key: %w", err)
	}
	app := githubapp.App{
		AppID:         sec.AppID,
		ClientID:      sec.ClientID,
		PrivateKeyPEM: plain,
		HTMLURL:       sec.HTMLURL,
		APIURL:        sec.APIURL,
		Name:          sec.Name,
	}
	tok, err := app.InstallationToken(sec.InstallationID)
	if err != nil {
		return "", fmt.Errorf("github installation token: %w", err)
	}
	return githubapp.CloneURL(repo, tok), nil
}

func (p *Pipeline) gitClone(ctx context.Context, client *ssh.Client, req Request, destDir string) error {
	repo, err := p.cloneURL(ctx, req)
	if err != nil {
		return err
	}
	branch := req.App.GitBranch
	if branch == "" {
		branch = "main"
	}
	p.log("fetch", "Cloning repository")
	_, _, _ = sshx.RunArgs(client, "rm", "-rf", destDir)
	_, errOut, err := sshx.RunArgs(client, "git", "clone", "--branch", branch, "--depth", "1", repo, destDir)
	if err != nil {
		return fmt.Errorf("git clone: %v %s", err, errOut)
	}
	return nil
}
