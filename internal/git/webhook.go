package git

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type PushEvent struct {
	Provider       string
	Action         string // push, opened, synchronize, reopened, closed, merged, …
	Branch         string // head / push branch
	BaseBranch     string // PR/MR target branch
	Commit         string
	Message        string
	PRNumber       int
	ChangedFiles   []string // optional; used for watch_paths filtering
	RepoFullName   string   // normalized owner/repo
	CommitMessages []string // push commit messages for skip-ci
}

// IsClosed reports whether this event should tear down a preview.
func (e *PushEvent) IsClosed() bool {
	if e == nil {
		return false
	}
	switch strings.ToLower(e.Action) {
	case "closed", "merged", "close", "rejected", "fulfilled":
		return true
	default:
		return false
	}
}

// IsPreviewOpen reports whether this is a PR/MR that should create/update a preview.
func (e *PushEvent) IsPreviewOpen() bool {
	if e == nil || e.PRNumber <= 0 || e.IsClosed() {
		return false
	}
	return true
}

// DetectProvider infers the git host from request headers.
func DetectProvider(r *http.Request) string {
	if r == nil {
		return "generic"
	}
	switch {
	// Gitea often also sends X-GitHub-Event for compatibility — prefer Gitea.
	case r.Header.Get("X-Gitea-Event") != "":
		return "gitea"
	case r.Header.Get("X-GitHub-Event") != "":
		return "github"
	case r.Header.Get("X-Gitlab-Event") != "" || r.Header.Get("X-Gitlab-Token") != "":
		return "gitlab"
	case r.Header.Get("X-Event-Key") != "":
		return "bitbucket"
	default:
		return "generic"
	}
}

// IsNullCommit reports a branch-deletion / empty SHA (GitHub sends 40 zeros).
func IsNullCommit(sha string) bool {
	s := strings.TrimSpace(sha)
	if s == "" || strings.EqualFold(s, "HEAD") {
		return false
	}
	for _, c := range s {
		if c != '0' {
			return false
		}
	}
	return len(s) > 0
}

// ParseWebhook extracts a normalized push/PR event from GitHub/GitLab/Gitea/Bitbucket payloads.
func ParseWebhook(provider string, r *http.Request, body []byte) (*PushEvent, error) {
	switch strings.ToLower(provider) {
	case "github":
		return parseGitHub(r, body)
	case "gitea":
		return parseGitea(r, body)
	case "gitlab":
		return parseGitLab(r, body)
	case "bitbucket":
		return parseBitbucket(r, body)
	default:
		return parseGeneric(body)
	}
}

func parseGitHub(r *http.Request, body []byte) (*PushEvent, error) {
	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		event = r.Header.Get("X-Gitea-Event")
	}
	switch event {
	case "push":
		return parseGitHubPush("github", body)
	case "pull_request":
		return parseGitHubPullRequest("github", body)
	case "ping":
		return &PushEvent{Provider: "github", Action: "ping"}, nil
	default:
		return nil, fmt.Errorf("unsupported github event %s", event)
	}
}

func parseGitea(r *http.Request, body []byte) (*PushEvent, error) {
	event := r.Header.Get("X-Gitea-Event")
	switch event {
	case "push":
		ev, err := parseGitHubPush("gitea", body)
		return ev, err
	case "pull_request":
		ev, err := parseGitHubPullRequest("gitea", body)
		return ev, err
	case "ping":
		return &PushEvent{Provider: "gitea", Action: "ping"}, nil
	default:
		return nil, fmt.Errorf("unsupported gitea event %s", event)
	}
}

func parseGitHubPush(provider string, body []byte) (*PushEvent, error) {
	var p struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		HeadCommit struct {
			Message  string   `json:"message"`
			Added    []string `json:"added"`
			Removed  []string `json:"removed"`
			Modified []string `json:"modified"`
		} `json:"head_commit"`
		Commits []struct {
			Message  string   `json:"message"`
			Added    []string `json:"added"`
			Removed  []string `json:"removed"`
			Modified []string `json:"modified"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	files := map[string]struct{}{}
	add := func(list []string) {
		for _, f := range list {
			f = strings.TrimSpace(f)
			if f != "" {
				files[f] = struct{}{}
			}
		}
	}
	add(p.HeadCommit.Added)
	add(p.HeadCommit.Removed)
	add(p.HeadCommit.Modified)
	msgs := make([]string, 0, len(p.Commits)+1)
	if strings.TrimSpace(p.HeadCommit.Message) != "" {
		msgs = append(msgs, p.HeadCommit.Message)
	}
	for _, c := range p.Commits {
		add(c.Added)
		add(c.Removed)
		add(c.Modified)
		if strings.TrimSpace(c.Message) != "" {
			msgs = append(msgs, c.Message)
		}
	}
	changed := make([]string, 0, len(files))
	for f := range files {
		changed = append(changed, f)
	}
	msg := p.HeadCommit.Message
	if msg == "" && len(msgs) > 0 {
		msg = msgs[0]
	}
	return &PushEvent{
		Provider:       provider,
		Action:         "push",
		Branch:         strings.TrimPrefix(p.Ref, "refs/heads/"),
		Commit:         p.After,
		Message:        msg,
		ChangedFiles:   changed,
		RepoFullName:   NormalizeRepoFullName(p.Repository.FullName),
		CommitMessages: msgs,
	}, nil
}

func parseGitHubPullRequest(provider string, body []byte) (*PushEvent, error) {
	var p struct {
		Action      string `json:"action"`
		Number      int    `json:"number"`
		Repository  struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		PullRequest struct {
			Head struct {
				Ref string `json:"ref"`
				Sha string `json:"sha"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
			Title string `json:"title"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	action := strings.ToLower(p.Action)
	switch action {
	case "opened", "synchronize", "reopened", "closed":
		// deploy / cleanup actions
	default:
		// labeled, edited, assigned, … — acknowledge without error so hosts do not retry
		action = "ignored"
	}
	return &PushEvent{
		Provider:       provider,
		Action:         action,
		Branch:         p.PullRequest.Head.Ref,
		BaseBranch:     p.PullRequest.Base.Ref,
		Commit:         p.PullRequest.Head.Sha,
		Message:        p.PullRequest.Title,
		PRNumber:       p.Number,
		RepoFullName:   NormalizeRepoFullName(p.Repository.FullName),
		CommitMessages: []string{p.PullRequest.Title},
	}, nil
}

func parseGitLab(r *http.Request, body []byte) (*PushEvent, error) {
	var kind struct {
		ObjectKind string `json:"object_kind"`
		EventName  string `json:"event_name"`
	}
	if err := json.Unmarshal(body, &kind); err != nil {
		return nil, err
	}
	objectKind := kind.ObjectKind
	if objectKind == "" {
		objectKind = kind.EventName
	}
	// Header may also say Push Hook / Merge Request Hook
	if objectKind == "" {
		h := strings.ToLower(r.Header.Get("X-Gitlab-Event"))
		switch {
		case strings.Contains(h, "merge"):
			objectKind = "merge_request"
		case strings.Contains(h, "push"):
			objectKind = "push"
		}
	}
	switch objectKind {
	case "push":
		var p struct {
			Ref     string `json:"ref"`
			After   string `json:"after"`
			Project struct {
				PathWithNamespace string `json:"path_with_namespace"`
			} `json:"project"`
			Commits []struct {
				Message  string   `json:"message"`
				Added    []string `json:"added"`
				Removed  []string `json:"removed"`
				Modified []string `json:"modified"`
			} `json:"commits"`
		}
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, err
		}
		files := map[string]struct{}{}
		msgs := make([]string, 0, len(p.Commits))
		for _, c := range p.Commits {
			for _, f := range append(append(c.Added, c.Removed...), c.Modified...) {
				f = strings.TrimSpace(f)
				if f != "" {
					files[f] = struct{}{}
				}
			}
			if strings.TrimSpace(c.Message) != "" {
				msgs = append(msgs, c.Message)
			}
		}
		changed := make([]string, 0, len(files))
		for f := range files {
			changed = append(changed, f)
		}
		msg := ""
		if len(msgs) > 0 {
			msg = msgs[0]
		}
		return &PushEvent{
			Provider:       "gitlab",
			Action:         "push",
			Branch:         strings.TrimPrefix(p.Ref, "refs/heads/"),
			Commit:         p.After,
			Message:        msg,
			ChangedFiles:   changed,
			RepoFullName:   NormalizeRepoFullName(p.Project.PathWithNamespace),
			CommitMessages: msgs,
		}, nil
	case "merge_request":
		var p struct {
			Project struct {
				PathWithNamespace string `json:"path_with_namespace"`
			} `json:"project"`
			ObjectAttributes struct {
				Action       string `json:"action"`
				IID          int    `json:"iid"`
				SourceBranch string `json:"source_branch"`
				TargetBranch string `json:"target_branch"`
				Title        string `json:"title"`
				LastCommit   struct {
					ID      string `json:"id"`
					Message string `json:"message"`
				} `json:"last_commit"`
			} `json:"object_attributes"`
		}
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, err
		}
		action := strings.ToLower(p.ObjectAttributes.Action)
		switch action {
		case "open", "opened":
			action = "opened"
		case "update":
			action = "synchronize"
		case "reopen", "reopened":
			action = "reopened"
		case "close", "closed":
			action = "closed"
		case "merge", "merged":
			action = "merged"
		default:
			action = "ignored"
		}
		commit := p.ObjectAttributes.LastCommit.ID
		msgs := []string{p.ObjectAttributes.Title}
		if strings.TrimSpace(p.ObjectAttributes.LastCommit.Message) != "" {
			msgs = append(msgs, p.ObjectAttributes.LastCommit.Message)
		}
		return &PushEvent{
			Provider:       "gitlab",
			Action:         action,
			Branch:         p.ObjectAttributes.SourceBranch,
			BaseBranch:     p.ObjectAttributes.TargetBranch,
			Commit:         commit,
			Message:        p.ObjectAttributes.Title,
			PRNumber:       p.ObjectAttributes.IID,
			RepoFullName:   NormalizeRepoFullName(p.Project.PathWithNamespace),
			CommitMessages: msgs,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported gitlab event %s", objectKind)
	}
}

func parseBitbucket(r *http.Request, body []byte) (*PushEvent, error) {
	eventKey := strings.ToLower(r.Header.Get("X-Event-Key"))
	switch eventKey {
	case "repo:push":
		var p struct {
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
			Push struct {
				Changes []struct {
					New *struct {
						Name   string `json:"name"`
						Target struct {
							Hash    string `json:"hash"`
							Message string `json:"message"`
						} `json:"target"`
					} `json:"new"`
					Commits []struct {
						Message string `json:"message"`
					} `json:"commits"`
				} `json:"changes"`
			} `json:"push"`
		}
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, err
		}
		if len(p.Push.Changes) == 0 || p.Push.Changes[0].New == nil {
			return nil, fmt.Errorf("nothing to do: no branch in bitbucket push")
		}
		ch := p.Push.Changes[0]
		msgs := make([]string, 0, len(ch.Commits)+1)
		for _, c := range ch.Commits {
			if strings.TrimSpace(c.Message) != "" {
				msgs = append(msgs, c.Message)
			}
		}
		if strings.TrimSpace(ch.New.Target.Message) != "" {
			msgs = append(msgs, ch.New.Target.Message)
		}
		msg := ""
		if len(msgs) > 0 {
			msg = msgs[0]
		}
		return &PushEvent{
			Provider:       "bitbucket",
			Action:         "push",
			Branch:         ch.New.Name,
			Commit:         ch.New.Target.Hash,
			Message:        msg,
			RepoFullName:   NormalizeRepoFullName(p.Repository.FullName),
			CommitMessages: msgs,
		}, nil
	case "pullrequest:created", "pullrequest:updated", "pullrequest:rejected", "pullrequest:fulfilled":
		var p struct {
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
			PullRequest struct {
				ID    int    `json:"id"`
				Title string `json:"title"`
				Source struct {
					Branch struct {
						Name string `json:"name"`
					} `json:"branch"`
					Commit struct {
						Hash string `json:"hash"`
					} `json:"commit"`
				} `json:"source"`
				Destination struct {
					Branch struct {
						Name string `json:"name"`
					} `json:"branch"`
				} `json:"destination"`
			} `json:"pullrequest"`
		}
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, err
		}
		action := "synchronize"
		switch eventKey {
		case "pullrequest:created":
			action = "opened"
		case "pullrequest:updated":
			action = "synchronize"
		case "pullrequest:rejected", "pullrequest:fulfilled":
			action = "closed"
		}
		return &PushEvent{
			Provider:       "bitbucket",
			Action:         action,
			Branch:         p.PullRequest.Source.Branch.Name,
			BaseBranch:     p.PullRequest.Destination.Branch.Name,
			Commit:         p.PullRequest.Source.Commit.Hash,
			Message:        p.PullRequest.Title,
			PRNumber:       p.PullRequest.ID,
			RepoFullName:   NormalizeRepoFullName(p.Repository.FullName),
			CommitMessages: []string{p.PullRequest.Title},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported bitbucket event %s", eventKey)
	}
}

func parseGeneric(body []byte) (*PushEvent, error) {
	var p struct {
		Branch  string `json:"branch"`
		Commit  string `json:"commit"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	return &PushEvent{
		Provider:       "generic",
		Action:         "push",
		Branch:         p.Branch,
		Commit:         p.Commit,
		Message:        p.Message,
		CommitMessages: []string{p.Message},
	}, nil
}

// NormalizeRepoFullName strips URLs/SSH forms down to owner/repo (lowercase).
func NormalizeRepoFullName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSpace(s)

	// git@host:owner/repo
	if strings.HasPrefix(s, "git@") {
		if i := strings.Index(s, ":"); i >= 0 {
			s = s[i+1:]
		}
	} else if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil {
			s = strings.TrimPrefix(u.Path, "/")
		}
	}
	// Drop accidental leading host path leftovers like github.com/owner/repo
	parts := strings.Split(s, "/")
	if len(parts) >= 3 && (strings.Contains(parts[0], ".") || parts[0] == "github.com" || parts[0] == "gitlab.com" || parts[0] == "bitbucket.org") {
		s = strings.Join(parts[len(parts)-2:], "/")
	} else if len(parts) > 2 {
		// keep last two segments for nested groups: group/sub/repo → sub/repo is wrong;
		// prefer full path without host — already owner/.../repo
		s = strings.Join(parts, "/")
	}
	return strings.ToLower(strings.Trim(s, "/"))
}

// RepoNamesMatch compares two repository identifiers after normalization.
func RepoNamesMatch(a, b string) bool {
	na, nb := NormalizeRepoFullName(a), NormalizeRepoFullName(b)
	if na == "" || nb == "" {
		return false
	}
	return na == nb
}

// ShouldSkipDeploy returns true if every non-empty message contains [skip ci] or [skip cd].
func ShouldSkipDeploy(messages []string) bool {
	var nonEmpty []string
	for _, m := range messages {
		m = strings.TrimSpace(m)
		if m != "" {
			nonEmpty = append(nonEmpty, m)
		}
	}
	if len(nonEmpty) == 0 {
		return false
	}
	for _, m := range nonEmpty {
		lower := strings.ToLower(m)
		if !strings.Contains(lower, "[skip ci]") && !strings.Contains(lower, "[skip cd]") {
			return false
		}
	}
	return true
}

// ShouldSkipDeployAny returns true if any non-empty message contains [skip ci] or [skip cd].
func ShouldSkipDeployAny(messages []string) bool {
	for _, m := range messages {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		lower := strings.ToLower(m)
		if strings.Contains(lower, "[skip ci]") || strings.Contains(lower, "[skip cd]") {
			return true
		}
	}
	return false
}

func VerifyGitHubSignature(secret string, body []byte, signatureHeader string) bool {
	sig := strings.TrimSpace(signatureHeader)
	if sig == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	sum := hex.EncodeToString(mac.Sum(nil))
	got := strings.TrimPrefix(strings.ToLower(sig), "sha256=")
	return hmac.Equal([]byte(sum), []byte(got))
}

func ReadBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(io.LimitReader(r.Body, 2<<20))
}
