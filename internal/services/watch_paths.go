package services

import (
	"path"
	"strings"
)

// WatchPathsMatch implements Coolify-style order-based watch path filtering.
// Patterns support *, **, ? and leading ! for negation. Last matching pattern wins.
// Empty patterns → always match. Empty changed files → match (insufficient data).
func WatchPathsMatch(patterns string, changedFiles []string) bool {
	patterns = strings.TrimSpace(patterns)
	if patterns == "" {
		return true
	}
	if len(changedFiles) == 0 {
		return true
	}
	lines := strings.Split(patterns, "\n")
	matched := false
	anyRule := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		anyRule = true
		neg := false
		if strings.HasPrefix(line, "!") {
			neg = true
			line = strings.TrimSpace(strings.TrimPrefix(line, "!"))
		}
		if line == "" {
			continue
		}
		for _, f := range changedFiles {
			f = strings.TrimPrefix(strings.ReplaceAll(f, "\\", "/"), "./")
			if watchGlobMatch(line, f) {
				matched = !neg
			}
		}
	}
	if !anyRule {
		return true
	}
	return matched
}

func watchGlobMatch(pattern, name string) bool {
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	name = strings.ReplaceAll(name, "\\", "/")
	// ** → * for path.Match after collapsing path segments loosely.
	if strings.Contains(pattern, "**") {
		// Match if any path suffix/prefix fits the simplified pattern.
		simplified := strings.ReplaceAll(pattern, "**", "*")
		if ok, _ := path.Match(simplified, name); ok {
			return true
		}
		// Also try matching against each path suffix (a/b/c ↔ **/c).
		parts := strings.Split(name, "/")
		for i := range parts {
			suffix := strings.Join(parts[i:], "/")
			if ok, _ := path.Match(simplified, suffix); ok {
				return true
			}
		}
		return false
	}
	ok, err := path.Match(pattern, name)
	return err == nil && ok
}
