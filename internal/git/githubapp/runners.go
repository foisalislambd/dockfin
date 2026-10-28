package githubapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RunnerGroup is the organization runner group Dockfin registers runners in.
type RunnerGroup struct {
	ID int64
}

// JITRunner is a one-job runner registration. EncodedConfig is a secret.
type JITRunner struct {
	ID            int64
	EncodedConfig string
}

// EnsureRunnerGroup returns the saved group when it still exists, or creates one.
// An existing group's visibility is left as the organization set it.
func (a *App) EnsureRunnerGroup(installationID, name string, savedID int64) (RunnerGroup, error) {
	org, err := a.organization()
	if err != nil {
		return RunnerGroup{}, err
	}
	if savedID > 0 {
		body, status, err := a.orgJSON(http.MethodGet, installationID, "/orgs/"+org+"/actions/runner-groups/"+fmt.Sprint(savedID), nil)
		if err != nil {
			return RunnerGroup{}, err
		}
		if status == http.StatusOK {
			return RunnerGroup{ID: savedID}, nil
		}
		if status != http.StatusNotFound {
			return RunnerGroup{}, fmt.Errorf("read runner group: %s", githubMessage(body, status))
		}
	}
	payload, _ := json.Marshal(map[string]any{
		"name":                       name,
		"visibility":                 "all",
		"allows_public_repositories": true,
	})
	body, status, err := a.orgJSON(http.MethodPost, installationID, "/orgs/"+org+"/actions/runner-groups", payload)
	if err != nil {
		return RunnerGroup{}, err
	}
	var out struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(body, &out)
	if status >= 300 || out.ID == 0 {
		return RunnerGroup{}, fmt.Errorf("create runner group: %s", githubMessage(body, status))
	}
	return RunnerGroup{ID: out.ID}, nil
}

// GenerateJITConfig registers one ephemeral runner. The encoded config must not be logged.
func (a *App) GenerateJITConfig(installationID, name string, groupID int64, labels []string) (JITRunner, error) {
	org, err := a.organization()
	if err != nil {
		return JITRunner{}, err
	}
	payload, _ := json.Marshal(map[string]any{
		"name":            name,
		"runner_group_id": groupID,
		"labels":          labels,
		"work_folder":     "_work",
	})
	body, status, err := a.orgJSON(http.MethodPost, installationID, "/orgs/"+org+"/actions/runners/generate-jitconfig", payload)
	if err != nil {
		return JITRunner{}, err
	}
	var out struct {
		Runner struct {
			ID int64 `json:"id"`
		} `json:"runner"`
		Encoded string `json:"encoded_jit_config"`
	}
	_ = json.Unmarshal(body, &out)
	if status >= 300 || out.Encoded == "" || out.Runner.ID == 0 {
		return JITRunner{}, fmt.Errorf("register runner: %s", githubMessage(body, status))
	}
	return JITRunner{ID: out.Runner.ID, EncodedConfig: out.Encoded}, nil
}

// DeleteRunner removes a runner. GitHub already removes a finished JIT runner, so 404 is success.
func (a *App) DeleteRunner(installationID string, runnerID int64) error {
	if runnerID <= 0 {
		return nil
	}
	org, err := a.organization()
	if err != nil {
		return err
	}
	body, status, err := a.orgJSON(http.MethodDelete, installationID, "/orgs/"+org+"/actions/runners/"+fmt.Sprint(runnerID), nil)
	if err != nil {
		return err
	}
	if status >= 300 && status != http.StatusNotFound {
		return fmt.Errorf("delete runner: %s", githubMessage(body, status))
	}
	return nil
}

// WorkflowRunEvent returns the event that started a workflow run, such as push or pull_request.
func (a *App) WorkflowRunEvent(installationID, repository string, runID int64) (string, error) {
	parts := strings.SplitN(repository, "/", 2)
	if len(parts) != 2 || runID <= 0 {
		return "", fmt.Errorf("workflow run is missing a repository")
	}
	path := "/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/actions/runs/" + fmt.Sprint(runID)
	body, status, err := a.orgJSON(http.MethodGet, installationID, path, nil)
	if err != nil {
		return "", err
	}
	var out struct {
		Event string `json:"event"`
	}
	_ = json.Unmarshal(body, &out)
	if status >= 300 || out.Event == "" {
		return "", fmt.Errorf("read workflow run: %s", githubMessage(body, status))
	}
	return out.Event, nil
}

func (a *App) organization() (string, error) {
	org := strings.Trim(strings.TrimSpace(a.Organization), "/")
	if org == "" || strings.Contains(org, "/") {
		return "", fmt.Errorf("GitHub Actions runners need a GitHub App that belongs to an organization")
	}
	return url.PathEscape(org), nil
}

func (a *App) orgJSON(method, installationID, path string, payload []byte) ([]byte, int, error) {
	token, err := a.InstallationToken(installationID)
	if err != nil {
		return nil, 0, err
	}
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, a.apiBase()+path, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := a.httpClient()
	client = &http.Client{Timeout: 30 * time.Second, Transport: client.Transport}
	if a.HTTP != nil && a.HTTP.Timeout > 0 {
		client.Timeout = a.HTTP.Timeout
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return raw, resp.StatusCode, nil
}

func githubMessage(body []byte, status int) string {
	var out struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &out)
	if out.Message != "" {
		return out.Message
	}
	return fmt.Sprintf("HTTP %d", status)
}
