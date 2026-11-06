package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Repo is a GitHub or GitLab repository Dockfin can list without cloning.
type Repo struct {
	Host  string
	Owner string
	Name  string
}

// ParseRepo accepts owner/name and https git URLs.
func ParseRepo(raw string) (Repo, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, ".git")
	if strings.Contains(raw, "://") || strings.HasPrefix(raw, "git@") {
		raw = strings.TrimPrefix(raw, "git@")
		raw = strings.Replace(raw, ":", "/", 1)
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			if err == nil && !strings.Contains(raw, "://") {
				u, err = url.Parse("https://" + raw)
			}
			if err != nil || u == nil || u.Host == "" {
				return Repo{}, fmt.Errorf("could not parse repository")
			}
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 {
			return Repo{}, fmt.Errorf("repository needs an owner and a name")
		}
		return Repo{Host: u.Host, Owner: parts[0], Name: parts[1]}, nil
	}
	parts := strings.Split(strings.Trim(raw, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Repo{}, fmt.Errorf("repository must look like owner/name")
	}
	return Repo{Host: "github.com", Owner: parts[0], Name: parts[1]}, nil
}

// ListPaths returns file paths at ref. GitHub uses the trees API. GitLab uses the repository tree API.
func ListPaths(ctx context.Context, repo Repo, ref, token string) ([]string, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = "HEAD"
	}
	switch strings.ToLower(repo.Host) {
	case "github.com":
		return githubTree(ctx, client, repo, ref, token)
	default:
		return gitlabTree(ctx, client, repo, ref, token)
	}
}

func githubTree(ctx context.Context, client *http.Client, repo Repo, ref, token string) ([]string, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), url.PathEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "dockfin")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("github tree returned HTTP %d", res.StatusCode)
	}
	var body struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	var paths []string
	for _, item := range body.Tree {
		if item.Type == "blob" || item.Type == "" {
			paths = append(paths, item.Path)
		}
	}
	return paths, nil
}

func gitlabTree(ctx context.Context, client *http.Client, repo Repo, ref, token string) ([]string, error) {
	project := url.PathEscape(repo.Owner + "/" + repo.Name)
	base := "https://" + repo.Host
	var paths []string
	page := 1
	for page <= 20 {
		u := fmt.Sprintf("%s/api/v4/projects/%s/repository/tree?recursive=true&per_page=100&page=%d&ref=%s", base, project, page, url.QueryEscape(ref))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		if token != "" {
			req.Header.Set("PRIVATE-TOKEN", token)
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, fmt.Errorf("gitlab tree returned HTTP %d", res.StatusCode)
		}
		var items []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			if item.Type == "blob" || item.Type == "" {
				paths = append(paths, item.Path)
			}
		}
		if len(items) < 100 {
			break
		}
		page++
	}
	return paths, nil
}
