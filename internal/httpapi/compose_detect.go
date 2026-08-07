package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/git/githubapp"
	"github.com/dockfin/dockfin/internal/redact"
	"github.com/dockfin/dockfin/internal/services"
)

type detectComposeBody struct {
	GitRepository string `json:"git_repository"`
	GitBranch     string `json:"git_branch"`
	GitSourceID   string `json:"git_source_id"`
	PrivateKeyID  string `json:"private_key_id"`
	Save          bool   `json:"save"` // when used with appID, persist preferred path
}

// handleDetectCompose shallow-clones a repo (public / GitHub App / deploy key) and
// returns compose file candidates. Used from Create Application before the app exists.
func (a *API) handleDetectCompose(w http.ResponseWriter, r *http.Request) {
	var body detectComposeBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(body.GitRepository) == "" {
		writeError(w, http.StatusBadRequest, "git_repository required")
		return
	}
	teamID := currentTeamID(r)
	result, err := a.detectComposeInRepo(r, teamID, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleDetectComposeForApp uses the application's git settings (overrides allowed in body).
func (a *API) handleDetectComposeForApp(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	var body detectComposeBody
	body.Save = true // default persist when detecting for an existing app
	if err := decodeJSON(r, &body); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.GitRepository == "" {
		body.GitRepository = app.GitRepository
	}
	if body.GitBranch == "" {
		body.GitBranch = app.GitBranch
	}
	if body.GitSourceID == "" && app.GitSourceID != nil {
		body.GitSourceID = app.GitSourceID.String()
	}
	if body.PrivateKeyID == "" && app.PrivateKeyID != nil {
		body.PrivateKeyID = app.PrivateKeyID.String()
	}
	if body.GitRepository == "" {
		writeError(w, http.StatusBadRequest, "application has no git repository")
		return
	}
	result, err := a.detectComposeInRepo(r, teamID, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Save && result.Location != "" {
		app.DockerComposeLocation = result.Location
		if err := a.Store.UpdateApplication(r.Context(), app); err != nil {
			mapStoreErr(w, err)
			return
		}
		result.Saved = true
	}
	writeJSON(w, http.StatusOK, result)
}

type detectComposeResult struct {
	Location   string   `json:"location"`
	Candidates []string `json:"candidates"`
	Saved      bool     `json:"saved,omitempty"`
}

func (a *API) detectComposeInRepo(r *http.Request, teamID uuid.UUID, body detectComposeBody) (*detectComposeResult, error) {
	branch := strings.TrimSpace(body.GitBranch)
	if branch == "" {
		branch = "main"
	}
	repoURL, env, cleanup, err := a.composeDetectCloneAuth(r, teamID, body)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	tmp, err := os.MkdirTemp("", "dockfin-compose-detect-*")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	cleanupDir := tmp
	defer func() { _ = os.RemoveAll(cleanupDir) }()

	ctx := r.Context()
	args := []string{"clone", "--depth", "1", "--branch", branch, repoURL, tmp}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		// Retry without --branch (default branch) when named branch missing.
		_ = os.RemoveAll(tmp)
		tmp2, err2 := os.MkdirTemp("", "dockfin-compose-detect-*")
		if err2 != nil {
			return nil, fmt.Errorf("git clone: %s", redact.Join(err.Error(), truncateOut(out)))
		}
		cleanupDir = tmp2
		tmp = tmp2
		cmd = exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, tmp)
		cmd.Env = append(os.Environ(), env...)
		cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			return nil, fmt.Errorf("git clone: %s", redact.Join(err2.Error(), truncateOut(out2)))
		}
	}

	found, err := services.FindComposeFiles(tmp)
	if err != nil {
		return nil, err
	}
	best := services.PreferComposeFile(found)
	if best == "" {
		return nil, fmt.Errorf("no compose file found (looked for docker-compose.yaml/yml and compose.yaml/yml, max depth 3)")
	}
	return &detectComposeResult{Location: best, Candidates: found}, nil
}

func (a *API) composeDetectCloneAuth(r *http.Request, teamID uuid.UUID, body detectComposeBody) (repoURL string, env []string, cleanup func(), err error) {
	repo := strings.TrimSpace(body.GitRepository)
	ctx := r.Context()

	// Deploy key (SSH).
	if body.PrivateKeyID != "" {
		id, err := uuid.Parse(body.PrivateKeyID)
		if err != nil {
			return "", nil, nil, fmt.Errorf("invalid private_key_id")
		}
		enc, err := a.Store.GetPrivateKeyMaterial(ctx, teamID, id)
		if err != nil {
			return "", nil, nil, fmt.Errorf("deploy key: %w", err)
		}
		plain, err := a.Store.Box.DecryptString(enc)
		if err != nil {
			return "", nil, nil, fmt.Errorf("decrypt deploy key: %w", err)
		}
		keyFile, err := os.CreateTemp("", "dockfin-detect-key-*")
		if err != nil {
			return "", nil, nil, err
		}
		keyPath := keyFile.Name()
		_, _ = keyFile.WriteString(plain)
		_ = keyFile.Close()
		_ = os.Chmod(keyPath, 0o600)
		user := "git"
		if body.GitSourceID != "" {
			if sid, e := uuid.Parse(body.GitSourceID); e == nil {
				if sec, e := a.Store.GetGitSourceSecrets(ctx, teamID, sid); e == nil && sec.CustomUser != "" {
					user = sec.CustomUser
				}
			}
		}
		repo = normalizeDetectRepo(repo, "https://github.com")
		sshURL := githubapp.ToSSHURL(repo, user)
		if !strings.HasPrefix(sshURL, "git@") && !strings.HasPrefix(sshURL, "ssh://") {
			return "", nil, nil, fmt.Errorf("could not build SSH clone URL from %q", body.GitRepository)
		}
		knownHosts := filepath.Join(a.Cfg.DataDir, "ssh", "known_hosts")
		_ = os.MkdirAll(filepath.Dir(knownHosts), 0o700)
		sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=%s", keyPath, knownHosts)
		return sshURL, []string{"GIT_SSH_COMMAND=" + sshCmd}, func() { _ = os.Remove(keyPath) }, nil
	}

	// GitHub App installation token.
	if body.GitSourceID != "" {
		id, err := uuid.Parse(body.GitSourceID)
		if err != nil {
			return "", nil, nil, fmt.Errorf("invalid git_source_id")
		}
		sec, err := a.Store.GetGitSourceSecrets(ctx, teamID, id)
		if err != nil {
			return "", nil, nil, fmt.Errorf("git source: %w", err)
		}
		if sec.InstallationID != "" {
			plain, err := a.Store.Box.DecryptString(sec.PrivateKeyEnc)
			if err != nil {
				return "", nil, nil, fmt.Errorf("decrypt git source key: %w", err)
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
				return "", nil, nil, fmt.Errorf("github installation token: %w", err)
			}
			repo = normalizeDetectRepo(repo, sec.HTMLURL)
			return githubapp.CloneURL(repo, tok), nil, nil, nil
		}
	}

	// Public HTTPS.
	if !strings.HasPrefix(repo, "http://") && !strings.HasPrefix(repo, "https://") && !strings.HasPrefix(repo, "git@") {
		repo = "https://github.com/" + strings.TrimPrefix(repo, "/")
		if !strings.HasSuffix(repo, ".git") {
			repo += ".git"
		}
	}
	return repo, nil, nil, nil
}

func normalizeDetectRepo(repo, htmlURL string) string {
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

func truncateOut(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}