package git

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dockfin/dockfin/internal/netsafe"
)

// ValidateRemote accepts http(s), ssh, git, and scp-style git@host:path URLs.
// It rejects option injection, local filesystem paths, ext:: helpers, and
// cloud metadata hosts. Those URLs are passed to git on the control plane
// during compose detection, where a hostile remote would execute as the API.
func ValidateRemote(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, " \t\r\n'\\") {
		return fmt.Errorf("invalid git repository")
	}
	if strings.HasPrefix(raw, "-") || strings.Contains(raw, "..") || strings.ContainsAny(raw, ";|&$`(){}<>") {
		return fmt.Errorf("invalid git repository")
	}
	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "https://"), strings.HasPrefix(lower, "http://"),
		strings.HasPrefix(lower, "ssh://"), strings.HasPrefix(lower, "git://"):
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			return fmt.Errorf("invalid git repository")
		}
		if u.User != nil {
			user := u.User.Username()
			if strings.ContainsAny(user, " \t\r\n") {
				return fmt.Errorf("invalid git repository")
			}
		}
		if BlockedFetchHost(u.Hostname()) {
			return fmt.Errorf("git host is not allowed")
		}
		return nil
	case strings.HasPrefix(raw, "git@"):
		rest := strings.TrimPrefix(raw, "git@")
		host, path, ok := strings.Cut(rest, ":")
		if !ok || host == "" || path == "" || strings.HasPrefix(path, "-") {
			return fmt.Errorf("invalid git repository")
		}
		if BlockedFetchHost(host) {
			return fmt.Errorf("git host is not allowed")
		}
		return nil
	default:
		// owner/name shorthand. Anything else is a local path or a git helper.
		if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, ".") || strings.Count(raw, "/") != 1 {
			return fmt.Errorf("invalid git repository")
		}
		owner, name, _ := strings.Cut(raw, "/")
		if owner == "" || name == "" || strings.Contains(name, "/") || strings.Contains(raw, ":") {
			return fmt.Errorf("invalid git repository")
		}
		if ip := netsafe.ParseIPLoose(owner); ip != nil {
			return fmt.Errorf("invalid git repository")
		}
		if strings.Contains(strings.ToLower(owner), "169.254.169.254") || strings.Contains(strings.ToLower(name), "169.254.169.254") {
			return fmt.Errorf("invalid git repository")
		}
		if !gitShorthandPart(owner) || !gitShorthandPart(name) {
			return fmt.Errorf("invalid git repository")
		}
		return nil
	}
}

// ValidateRef rejects branch and commit values git would treat as options.
func ValidateRef(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	if strings.HasPrefix(ref, "-") || strings.Contains(ref, "..") || strings.ContainsAny(ref, " \t\r\n'\"\\") {
		return fmt.Errorf("invalid git ref")
	}
	return nil
}

// gitShorthandPart is a GitHub-style owner or repository name.
// A letter is required so numeric hosts and metadata addresses cannot hide in owner/name form.
func gitShorthandPart(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	letter := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
			letter = true
		case r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-':
		default:
			return false
		}
	}
	return letter
}

// ValidSSHUser is the remote user placed in a git SSH URL.
func ValidSSHUser(user string) bool {
	if user == "" {
		return false
	}
	if len(user) > 32 {
		return false
	}
	for _, r := range user {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !ok {
			return false
		}
	}
	return true
}

// BlockedFetchHost reports cloud metadata names and link-local addresses.
func BlockedFetchHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	h = strings.Trim(h, "[]")
	if strings.Contains(h, "169.254.169.254") {
		return true
	}
	switch h {
	case "169.254.169.254", "metadata", "metadata.google.internal", "metadata.goog", "metadata.internal":
		return true
	}
	if ip := netsafe.ParseIPLoose(h); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			ip = v4
		}
		return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
	}
	return false
}
