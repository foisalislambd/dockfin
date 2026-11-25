package services

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestShippedTemplatesAndSwarmPrepare(t *testing.T) {
	t.Setenv("DOCKFIN_TEMPLATES_DIR", filepath.Join("..", "..", "templates", "compose"))
	mu.Lock()
	loaded = false
	mu.Unlock()

	list := ListTemplates()
	if len(list) < 3 {
		t.Fatalf("templates %d", len(list))
	}
	for _, item := range list {
		if item.Name == "" || item.Type == "" {
			t.Fatalf("blank template %+v", item)
		}
		got, err := GetTemplate(item.Type)
		if err != nil || !strings.Contains(got.Compose, "services:") && !strings.Contains(got.Compose, "image:") {
			t.Fatalf("%s: %v compose %q", item.Type, err, got.Compose)
		}
	}
	if _, err := GetTemplate("not-a-real-template"); err == nil {
		t.Fatal("missing template accepted")
	}

	raw := "services:\n  web:\n    image: nginx:alpine\n    ports:\n      - 8080:80\n    volumes:\n      - data:/var/lib\nvolumes:\n  data: {}\n"
	out, env, err := PrepareCompose(raw, PrepareOpts{
		FQDN: "web.example.com", ServiceID: "web", Network: "dockfin",
		SwarmReplicas: 2, SwarmWorkersOnly: true,
		SwarmPlacementConstraints: []string{"node.role==worker", " "},
		GPUEnabled:                true, GPUCount: 1, RestartPolicy: "unless-stopped", StopGracePeriodSec: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "replicas") || !strings.Contains(out, "worker") {
		t.Fatalf("swarm block missing:\n%s", out)
	}
	if !strings.Contains(out, "nginx") {
		t.Fatal(out)
	}
	if !hasTopLevelVolumes(out) {
		t.Fatalf("volumes dropped:\n%s", out)
	}
	_ = env
}
