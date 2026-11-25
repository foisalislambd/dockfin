package imagescan

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestImageRefsAndDigests(t *testing.T) {
	if ApplicationImage(nil) != "" {
		t.Fatal("nil app")
	}
	id := uuid.New()
	app := &store.Application{ID: id, BuildPack: "dockerfile"}
	if got := ApplicationImage(app); got != "dockfin/"+id.String()+":latest" {
		t.Fatal(got)
	}
	app.BuildPack = "dockerimage"
	app.DockerRegistryImageName = "nginx"
	app.DockerRegistryImageTag = "alpine"
	if ApplicationImage(app) != "nginx:alpine" {
		t.Fatal(ApplicationImage(app))
	}
	svc := &store.Service{DockerComposeRaw: "services:\n  web:\n    image: redis:7\n"}
	if imgs := ServiceImages(svc); len(imgs) != 1 || !strings.Contains(imgs[0], "redis") {
		t.Fatal(imgs)
	}

	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"digest":"sha256:abc","images":[{"digest":"sha256:def"}]}`))
	}))
	defer hub.Close()
	prev := dockerHubBase
	dockerHubBase = hub.URL
	t.Cleanup(func() { dockerHubBase = prev })
	got := TagDigests(t.Context(), hub.Client(), "docker.io/library/nginx:latest", []string{"alpine", ""})
	if got["alpine"] != "sha256:abc" {
		t.Fatal(got)
	}
}
