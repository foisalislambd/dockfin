package imagescan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryTagList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/v2/library/nginx/tags/list") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tags":["1.27.1","1.27.4","1.28.0"]}`))
	}))
	defer srv.Close()
	old := registryHostForTest
	registryHostForTest = srv.URL
	t.Cleanup(func() { registryHostForTest = old })

	tags, err := ListTags(context.Background(), srv.Client(), "nginx:1.27.1")
	if err != nil {
		t.Fatal(err)
	}
	got := SelectTags("1.27.1", tags)
	if got.Patch != "1.27.4" || got.Line != "1.28.0" {
		t.Fatalf("%+v tags %v", got, tags)
	}
}
