package services

import "testing"

func TestWatchPathsMatch(t *testing.T) {
	files := []string{"services/api/main.go", "README.md"}
	if !WatchPathsMatch("services/api/**", files) {
		t.Fatal("expected match")
	}
	if WatchPathsMatch("docs/**", files) {
		t.Fatal("expected no match")
	}
	// Negation: match all then exclude readme
	if WatchPathsMatch("**\n!README.md", []string{"README.md"}) {
		t.Fatal("expected negation to win")
	}
	if !WatchPathsMatch("**\n!README.md", []string{"services/api/main.go"}) {
		t.Fatal("expected match after negation of other file")
	}
	if !WatchPathsMatch("", files) {
		t.Fatal("empty patterns always match")
	}
}
