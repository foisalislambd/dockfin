package git

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type PushEvent struct {
	Provider string
	Branch   string
	Commit   string
	Message  string
	PRNumber int
}

// ParseWebhook extracts a normalized push/PR event from GitHub/GitLab/Gitea payloads.
func ParseWebhook(provider string, r *http.Request, body []byte) (*PushEvent, error) {
	switch strings.ToLower(provider) {
	case "github":
		return parseGitHub(r, body)
	case "gitlab":
		return parseGitLab(body)
	default:
		return parseGeneric(body)
	}
}

func parseGitHub(r *http.Request, body []byte) (*PushEvent, error) {
	event := r.Header.Get("X-GitHub-Event")
	switch event {
	case "push":
		var p struct {
			Ref        string `json:"ref"`
			After      string `json:"after"`
			HeadCommit struct {
				Message string `json:"message"`
			} `json:"head_commit"`
		}
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, err
		}
		return &PushEvent{
			Provider: "github",
			Branch:   strings.TrimPrefix(p.Ref, "refs/heads/"),
			Commit:   p.After,
			Message:  p.HeadCommit.Message,
		}, nil
	case "pull_request":
		var p struct {
			Action      string `json:"action"`
			Number      int    `json:"number"`
			PullRequest struct {
				Head struct {
					Ref string `json:"ref"`
					Sha string `json:"sha"`
				} `json:"head"`
				Title string `json:"title"`
			} `json:"pull_request"`
		}
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, err
		}
		if p.Action != "opened" && p.Action != "synchronize" && p.Action != "reopened" {
			return nil, fmt.Errorf("ignored pr action %s", p.Action)
		}
		return &PushEvent{
			Provider: "github",
			Branch:   p.PullRequest.Head.Ref,
			Commit:   p.PullRequest.Head.Sha,
			Message:  p.PullRequest.Title,
			PRNumber: p.Number,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported github event %s", event)
	}
}

func parseGitLab(body []byte) (*PushEvent, error) {
	var p struct {
		Ref     string `json:"ref"`
		After   string `json:"after"`
		Commits []struct {
			Message string `json:"message"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	msg := ""
	if len(p.Commits) > 0 {
		msg = p.Commits[0].Message
	}
	return &PushEvent{
		Provider: "gitlab",
		Branch:   strings.TrimPrefix(p.Ref, "refs/heads/"),
		Commit:   p.After,
		Message:  msg,
	}, nil
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
	return &PushEvent{Provider: "generic", Branch: p.Branch, Commit: p.Commit, Message: p.Message}, nil
}

func VerifyGitHubSignature(secret string, body []byte, signatureHeader string) bool {
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}

func ReadBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(io.LimitReader(r.Body, 2<<20))
}
