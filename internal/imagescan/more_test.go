package imagescan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDockerHubTagDigests(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/v2/repositories/library/nginx/tags/1.27.4":
			_ = json.NewEncoder(w).Encode(map[string]string{"digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
		case "/v2/repositories/library/nginx/tags/1.28.0":
			_ = json.NewEncoder(w).Encode(map[string]any{"images": []map[string]string{{"digest": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}})
		case "/v2/repositories/library/nginx/tags/1.29.0":
			http.NotFound(w, r)
		case "/v2/repositories/library/nginx/tags/2.0.0":
			_ = json.NewEncoder(w).Encode(map[string]string{"digest": "md5:not-a-digest"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	old := dockerHubBase
	dockerHubBase = srv.URL
	t.Cleanup(func() { dockerHubBase = old })

	got := TagDigests(context.Background(), srv.Client(), "nginx:1.27.1", []string{"1.27.4", "1.28.0", "1.29.0", "2.0.0", "9.9.9"})
	if len(paths) != 4 || strings.Contains(strings.Join(paths, " "), "9.9.9") {
		t.Fatalf("paths %v", paths)
	}
	if got["1.27.4"] != "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		got["1.28.0"] != "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" ||
		got["1.29.0"] != "" || got["2.0.0"] != "" {
		t.Fatalf("%v", got)
	}
	if TagDigests(context.Background(), srv.Client(), "nginx@sha256:abc", []string{"1"}) != nil {
		t.Fatal("digest reference should not be looked up")
	}
	if TagDigests(context.Background(), srv.Client(), "nginx:1", nil) != nil {
		t.Fatal("empty tag list should return nil")
	}
}

func TestListTagsChallengeAndNextPage(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			if r.URL.Query().Get("service") != "registry" || r.URL.Query().Get("scope") != "repository:org/app:pull" {
				t.Fatalf("token query %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "secret-token"})
			return
		}
		if !strings.Contains(r.URL.Path, "/v2/org/app/tags/list") {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("Www-Authenticate", `Bearer realm="`+srv.URL+`/token",service="registry",scope="repository:org/app:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatalf("auth %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`{"tags":["1.3.0"]}`))
			return
		}
		w.Header().Set("Link", `<`+srv.URL+`/v2/org/app/tags/list?page=2>; rel="next"`)
		_, _ = w.Write([]byte(`{"tags":["1.2.0","1.2.3"]}`))
	}))
	t.Cleanup(srv.Close)
	old := registryHostForTest
	registryHostForTest = srv.URL
	t.Cleanup(func() { registryHostForTest = old })

	tags, err := ListTags(context.Background(), srv.Client(), "ghcr.io/org/app:1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 3 || tags[0] != "1.2.0" || tags[1] != "1.2.3" || tags[2] != "1.3.0" {
		t.Fatalf("%v", tags)
	}
	choice := SelectTags("1.2.0", tags)
	if choice.Patch != "1.2.3" || choice.Line != "1.3.0" {
		t.Fatalf("%+v", choice)
	}
}

func TestListTagsRejectsBadResponses(t *testing.T) {
	if _, err := ListTags(context.Background(), nil, "not an image"); err == nil {
		t.Fatal("invalid reference accepted")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	old := registryHostForTest
	registryHostForTest = srv.URL
	t.Cleanup(func() { registryHostForTest = old })
	if _, err := ListTags(context.Background(), srv.Client(), "ghcr.io/org/app:1"); err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatal(err)
	}
}
