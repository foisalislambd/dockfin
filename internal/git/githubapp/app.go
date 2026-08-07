package githubapp

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type App struct {
	AppID         string
	ClientID      string
	PrivateKeyPEM string
	HTMLURL       string // e.g. https://github.com
	APIURL        string // e.g. https://api.github.com
	Name          string // GitHub App slug for install URL
	Organization  string
}

func (a *App) apiBase() string {
	if a.APIURL != "" {
		return strings.TrimRight(a.APIURL, "/")
	}
	return "https://api.github.com"
}

func (a *App) htmlBase() string {
	if a.HTMLURL != "" {
		return strings.TrimRight(a.HTMLURL, "/")
	}
	return "https://github.com"
}

// APIURLFromHTML derives the GitHub API base from an HTML host (GHES / GHE Cloud aware).
func APIURLFromHTML(htmlURL string) string {
	htmlURL = strings.TrimRight(strings.TrimSpace(htmlURL), "/")
	if htmlURL == "" || htmlURL == "https://github.com" {
		return "https://api.github.com"
	}
	u, err := url.Parse(htmlURL)
	if err != nil || u.Host == "" {
		return "https://api.github.com"
	}
	host := strings.ToLower(u.Host)
	if host == "github.com" || host == "www.github.com" {
		return "https://api.github.com"
	}
	// GitHub Enterprise Cloud: api.<host>
	if strings.HasSuffix(host, ".ghe.com") || strings.Contains(host, "github.") {
		return u.Scheme + "://api." + host
	}
	// Self-hosted GHES
	return htmlURL + "/api/v3"
}

func (a *App) signedJWT() (string, error) {
	block, _ := pem.Decode([]byte(a.PrivateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("invalid private key PEM")
	}
	var signer any
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		signer = k
	} else if k8, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		signer = k8
	} else {
		return "", fmt.Errorf("unsupported private key")
	}
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": a.AppID,
	})
	return tok.SignedString(signer)
}

func (a *App) InstallationToken(installationID string) (string, error) {
	jwtStr, err := a.signedJWT()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, a.apiBase()+"/app/installations/"+url.PathEscape(installationID)+"/access_tokens", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtStr)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("installation token: %s %s", resp.Status, string(body))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.Token, nil
}

func (a *App) InstallURL(state string) string {
	name := a.Name
	if name == "" {
		name = "app"
	}
	u := a.htmlBase() + "/apps/" + url.PathEscape(name) + "/installations/new"
	if state != "" {
		u += "?state=" + url.QueryEscape(state)
	}
	return u
}

// AppSlug returns the GitHub App slug via the authenticated /app endpoint.
func (a *App) AppSlug() (string, error) {
	jwtStr, err := a.signedJWT()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, a.apiBase()+"/app", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtStr)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("app slug: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.Slug != "" {
		return out.Slug, nil
	}
	return out.Name, nil
}

// ManifestFormAction is the GitHub URL that accepts a POST with a "manifest" field.
func (a *App) ManifestFormAction(state string) string {
	org := strings.Trim(strings.TrimSpace(a.Organization), "/")
	var path string
	if org != "" {
		path = "organizations/" + url.PathEscape(org) + "/settings/apps/new"
	} else {
		path = "settings/apps/new"
	}
	u := a.htmlBase() + "/" + path
	if state != "" {
		u += "?state=" + url.QueryEscape(state)
	}
	return u
}

// BuildManifest returns the GitHub App Manifest JSON (Coolify-compatible).
// setupSourceID is embedded in setup_url so GitHub's post-install redirect works
// even after the one-time manifest state was consumed.
func BuildManifest(name, publicBase, webhookPath, redirectPath, setupPath, setupSourceID string, previewDeployments bool) map[string]any {
	publicBase = strings.TrimRight(publicBase, "/")
	webhookBase := publicBase + "/api/v1/webhooks"
	perms := map[string]string{
		"contents":       "read",
		"metadata":       "read",
		"emails":         "read",
		"administration": "read",
	}
	events := []string{"push"}
	if previewDeployments {
		perms["pull_requests"] = "write"
		events = append(events, "pull_request")
	}
	setupURL := webhookBase + setupPath
	if setupSourceID != "" {
		setupURL += "?source_id=" + url.QueryEscape(setupSourceID)
	}
	return map[string]any{
		"name": name,
		"url":  publicBase,
		"hook_attributes": map[string]any{
			"url":    webhookBase + webhookPath,
			"active": true,
		},
		"redirect_url":             webhookBase + redirectPath,
		"callback_urls":            []string{publicBase + "/login/github/app"},
		"public":                   false,
		"request_oauth_on_install": false,
		"setup_url":                setupURL,
		"setup_on_update":          true,
		"default_permissions":      perms,
		"default_events":           events,
	}
}

type ManifestConversion struct {
	ID            int64  `json:"id"`
	Slug          string `json:"slug"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	WebhookSecret string `json:"webhook_secret"`
	PEM           string `json:"pem"`
	HTMLURL       string `json:"html_url"`
}

// ConvertManifest exchanges a manifest code for GitHub App credentials.
func ConvertManifest(apiURL, code string) (*ManifestConversion, error) {
	apiURL = strings.TrimRight(apiURL, "/")
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}
	req, err := http.NewRequest(http.MethodPost, apiURL+"/app-manifests/"+url.PathEscape(code)+"/conversions", bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("manifest conversion: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out ManifestConversion
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *App) ListRepos(installationID string, page int) ([]map[string]any, error) {
	repos, _, err := a.listRepositoriesPage(installationID, page)
	return repos, err
}

// ListAllRepositories pages through every installation repo (50 per page, max 2000).
func (a *App) ListAllRepositories(installationID string) ([]map[string]any, error) {
	var all []map[string]any
	for page := 1; page <= 40; page++ {
		repos, total, err := a.listRepositoriesPage(installationID, page)
		if err != nil {
			return nil, err
		}
		all = append(all, repos...)
		if len(repos) < 50 || (total > 0 && len(all) >= total) {
			break
		}
	}
	return all, nil
}

func (a *App) listRepositoriesPage(installationID string, page int) ([]map[string]any, int, error) {
	token, err := a.InstallationToken(installationID)
	if err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/installation/repositories?per_page=50&page=%d", a.apiBase(), page), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("list repos: %s %s", resp.Status, string(body))
	}
	var out struct {
		TotalCount   int              `json:"total_count"`
		Repositories []map[string]any `json:"repositories"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, 0, err
	}
	return out.Repositories, out.TotalCount, nil
}

func (a *App) ListBranches(installationID, owner, repo string) ([]string, error) {
	token, err := a.InstallationToken(installationID)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/branches?per_page=100",
		a.apiBase(), url.PathEscape(owner), url.PathEscape(repo)), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list branches: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var rows []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Name != "" {
			out = append(out, r.Name)
		}
	}
	return out, nil
}

func CloneURL(repoHTTPS, token string) string {
	repoHTTPS = strings.TrimSpace(repoHTTPS)
	if token == "" {
		return repoHTTPS
	}
	u, err := url.Parse(repoHTTPS)
	if err != nil {
		return repoHTTPS
	}
	u.User = url.UserPassword("x-access-token", token)
	return u.String()
}

// ToSSHURL converts https://host/owner/repo(.git) to git@host:owner/repo.git
func ToSSHURL(repo string, user string) string {
	repo = strings.TrimSpace(repo)
	if user == "" {
		user = "git"
	}
	if strings.HasPrefix(repo, "git@") || strings.HasPrefix(repo, "ssh://") {
		return repo
	}
	u, err := url.Parse(repo)
	if err != nil || u.Host == "" {
		return repo
	}
	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	return fmt.Sprintf("%s@%s:%s.git", user, u.Host, path)
}
