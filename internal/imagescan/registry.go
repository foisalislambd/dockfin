package imagescan

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

// dockerHubBase is replaced in tests.
var dockerHubBase = "https://hub.docker.com"

// ListTags reads public tags for a repository. Docker Hub uses its JSON API.
// Other registries use the OCI tags list, with an anonymous bearer token when the registry asks for one.
func ListTags(ctx context.Context, client *http.Client, imageRef string) ([]string, error) {
	repo, _, ok := ImageName(imageRef)
	if !ok {
		return nil, fmt.Errorf("image reference is not a registry image")
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if strings.HasPrefix(repo, "docker.io/") {
		return ociTags(ctx, client, "registry-1.docker.io", strings.TrimPrefix(repo, "docker.io/"))
	}
	host, path, _ := strings.Cut(repo, "/")
	return ociTags(ctx, client, host, path)
}

// registryHostForTest replaces the registry origin in tests.
var registryHostForTest string

func registryRoot(host string) string {
	if registryHostForTest != "" {
		return strings.TrimRight(registryHostForTest, "/")
	}
	return "https://" + host
}

func ociTags(ctx context.Context, client *http.Client, host, repo string) ([]string, error) {
	endpoint := registryRoot(host) + "/v2/" + repo + "/tags/list"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusUnauthorized {
		challenge := res.Header.Get("Www-Authenticate")
		res.Body.Close()
		token, err := anonymousToken(ctx, client, challenge, repo)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res, err = client.Do(req)
		if err != nil {
			return nil, err
		}
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned %d", res.StatusCode)
	}
	var doc struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	out := append([]string{}, doc.Tags...)
	for pages := 0; pages < 10; pages++ {
		next := nextLink(res.Header.Get("Link"))
		if next == "" {
			break
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		if auth := res.Request.Header.Get("Authorization"); auth != "" {
			req.Header.Set("Authorization", auth)
		}
		res.Body.Close()
		res, err = client.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ = io.ReadAll(io.LimitReader(res.Body, 4<<20))
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			return nil, fmt.Errorf("registry returned %d", res.StatusCode)
		}
		doc = struct {
			Tags []string `json:"tags"`
		}{}
		if err := json.Unmarshal(body, &doc); err != nil {
			res.Body.Close()
			return nil, err
		}
		out = append(out, doc.Tags...)
	}
	res.Body.Close()
	if len(out) == 0 {
		return nil, fmt.Errorf("no tags listed")
	}
	return out, nil
}

func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}

func anonymousToken(ctx context.Context, client *http.Client, challenge, repo string) (string, error) {
	realm, service, scope := parseChallenge(challenge)
	if realm == "" {
		return "", fmt.Errorf("registry did not offer an anonymous token")
	}
	if scope == "" {
		scope = "repository:" + repo + ":pull"
	}
	u, err := url.Parse(realm)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if service != "" {
		q.Set("service", service)
	}
	q.Set("scope", scope)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request returned %d", res.StatusCode)
	}
	var doc struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	if doc.Token != "" {
		return doc.Token, nil
	}
	if doc.AccessToken == "" {
		return "", fmt.Errorf("token response was empty")
	}
	return doc.AccessToken, nil
}

func parseChallenge(h string) (realm, service, scope string) {
	h = strings.TrimSpace(strings.TrimPrefix(h, "Bearer"))
	for _, part := range strings.Split(h, ",") {
		part = strings.TrimSpace(part)
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		switch strings.ToLower(k) {
		case "realm":
			realm = v
		case "service":
			service = v
		case "scope":
			scope = v
		}
	}
	return realm, service, scope
}

// TagDigests looks up registry digests for a few tags. Missing or private tags are omitted.
func TagDigests(ctx context.Context, client *http.Client, imageRef string, tags []string) map[string]string {
	repo, _, ok := ImageName(imageRef)
	if !ok || len(tags) == 0 {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	out := map[string]string{}
	limit := len(tags)
	if limit > 4 {
		limit = 4
	}
	for _, tag := range tags[:limit] {
		if tag == "" {
			continue
		}
		digest := ""
		var err error
		if strings.HasPrefix(repo, "docker.io/") {
			digest, err = dockerHubDigest(ctx, client, strings.TrimPrefix(repo, "docker.io/"), tag)
		} else {
			host, path, _ := strings.Cut(repo, "/")
			digest, err = ociDigest(ctx, client, host, path, tag)
		}
		if err == nil && strings.HasPrefix(digest, "sha256:") {
			out[tag] = digest
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func dockerHubDigest(ctx context.Context, client *http.Client, repo, tag string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dockerHubBase+"/v2/repositories/"+repo+"/tags/"+url.PathEscape(tag), nil)
	if err != nil {
		return "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("docker hub returned %d", res.StatusCode)
	}
	var doc struct {
		Digest string `json:"digest"`
		Images []struct {
			Digest string `json:"digest"`
		} `json:"images"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	if strings.HasPrefix(doc.Digest, "sha256:") {
		return doc.Digest, nil
	}
	for _, img := range doc.Images {
		if strings.HasPrefix(img.Digest, "sha256:") {
			return img.Digest, nil
		}
	}
	return "", fmt.Errorf("no digest")
}

func ociDigest(ctx context.Context, client *http.Client, host, repo, tag string) (string, error) {
	endpoint := "https://" + host + "/v2/" + repo + "/manifests/" + tag
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json")
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if res.StatusCode == http.StatusUnauthorized {
		challenge := res.Header.Get("Www-Authenticate")
		res.Body.Close()
		token, err := anonymousToken(ctx, client, challenge, repo)
		if err != nil {
			return "", err
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json")
		req.Header.Set("Authorization", "Bearer "+token)
		res, err = client.Do(req)
		if err != nil {
			return "", err
		}
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry returned %d", res.StatusCode)
	}
	digest := res.Header.Get("Docker-Content-Digest")
	if !strings.HasPrefix(digest, "sha256:") {
		return "", fmt.Errorf("no digest")
	}
	return digest, nil
}
