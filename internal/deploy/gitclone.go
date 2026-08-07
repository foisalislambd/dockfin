package deploy

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/dockfin/dockfin/internal/git/githubapp"
	"github.com/dockfin/dockfin/internal/redact"
	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

func (p *Pipeline) cloneURL(ctx context.Context, req Request) (string, error) {
	repo := req.App.GitRepository
	// Deploy key takes precedence (Coolify deploymentType).
	if req.App.PrivateKeyID != nil {
		return repo, nil
	}
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
	repo = normalizeHTTPSRepo(repo, sec.HTMLURL)
	return githubapp.CloneURL(repo, tok), nil
}

func normalizeHTTPSRepo(repo, htmlURL string) string {
	repo = strings.TrimSpace(repo)
	if strings.HasPrefix(repo, "http://") || strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "git@") {
		return repo
	}
	base := strings.TrimRight(htmlURL, "/")
	if base == "" {
		base = "https://github.com"
	}
	repo = strings.TrimPrefix(repo, "/")
	repo = strings.TrimSuffix(repo, ".git")
	return base + "/" + repo + ".git"
}

func (p *Pipeline) gitClone(ctx context.Context, client *ssh.Client, req Request, destDir string) error {
	branch := req.GitBranch
	if branch == "" {
		branch = req.App.GitBranch
	}
	if branch == "" {
		branch = "main"
	}
	commit := strings.TrimSpace(req.CommitSHA)
	if commit == "HEAD" {
		commit = ""
	}
	p.log("fetch", "Cloning repository")
	_, _, _ = sshx.RunArgs(client, "rm", "-rf", destDir)

	runClone := func(extraEnv []string, repo string) error {
		args := append([]string{}, extraEnv...)
		args = append(args, "git", "clone", "--branch", branch, "--depth", "1", repo, destDir)
		_, errOut, err := sshx.RunArgs(client, args...)
		if err != nil {
			return fmt.Errorf("git clone: %s", redact.Join(err.Error(), errOut))
		}
		if commit != "" {
			p.log("fetch", "Checking out commit "+commit)
			_, errOut, err = sshx.RunArgs(client, "git", "-C", destDir, "fetch", "--depth", "1", "origin", commit)
			if err != nil {
				// Fall back to unshallow fetch of the SHA (some remotes reject shallow fetch by SHA).
				_, errOut2, err2 := sshx.RunArgs(client, "git", "-C", destDir, "fetch", "origin", commit)
				if err2 != nil {
					return fmt.Errorf("git fetch commit: %s", redact.Join(err.Error(), errOut, err2.Error(), errOut2))
				}
			}
			_, errOut, err = sshx.RunArgs(client, "git", "-C", destDir, "checkout", commit)
			if err != nil {
				return fmt.Errorf("git checkout: %s", redact.Join(err.Error(), errOut))
			}
		}
		if req.App.IsGitSubmodulesEnabled {
			p.log("fetch", "Updating git submodules")
			_, errOut, err = sshx.RunArgs(client, "git", "-C", destDir, "submodule", "update", "--init", "--recursive")
			if err != nil {
				return fmt.Errorf("git submodule: %s", redact.Join(err.Error(), errOut))
			}
		}
		return nil
	}

	// Private deploy key path (SSH).
	if req.App.PrivateKeyID != nil && p.Store != nil {
		enc, err := p.Store.GetPrivateKeyMaterial(ctx, req.TeamID, *req.App.PrivateKeyID)
		if err != nil {
			return fmt.Errorf("deploy key: %w", err)
		}
		plain, err := p.Store.Box.DecryptString(enc)
		if err != nil {
			return fmt.Errorf("decrypt deploy key: %w", err)
		}
		keyPath := "/tmp/dockfin-deploy-" + req.App.PrivateKeyID.String()
		b64 := base64.StdEncoding.EncodeToString([]byte(plain))
		writeCmd := fmt.Sprintf("printf '%%s' %q | base64 -d > %s && chmod 600 %s", b64, keyPath, keyPath)
		if _, errOut, err := sshx.Run(client, writeCmd); err != nil {
			return fmt.Errorf("write deploy key: %v %s", err, errOut)
		}
		defer func() { _, _, _ = sshx.RunArgs(client, "rm", "-f", keyPath) }()

		user := "git"
		if req.App.GitSourceID != nil {
			if sec, err := p.Store.GetGitSourceSecrets(ctx, req.TeamID, *req.App.GitSourceID); err == nil && sec.CustomUser != "" {
				user = sec.CustomUser
			}
		}
		repo := githubapp.ToSSHURL(req.App.GitRepository, user)
		_, _, _ = sshx.RunArgs(client, "mkdir", "-p", "/data/dockfin/.ssh")
		sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/data/dockfin/.ssh/known_hosts", keyPath)
		if err := runClone([]string{"env", "GIT_SSH_COMMAND=" + sshCmd}, repo); err != nil {
			return fmt.Errorf("git clone (deploy key): %w", err)
		}
		return nil
	}

	repo, err := p.cloneURL(ctx, req)
	if err != nil {
		return err
	}
	return runClone(nil, repo)
}
