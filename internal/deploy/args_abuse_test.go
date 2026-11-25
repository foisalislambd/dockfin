package deploy

import (
	"strings"
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

func TestDockerArgsStripShellAbuse(t *testing.T) {
	opts := splitDockerRunOptions(`--name web --label "a b" --flag 'c; rm -rf /'`)
	if len(opts) < 6 || opts[0] != "--name" || opts[1] != "web" || opts[3] != "a b" || opts[5] != "c; rm -rf /" {
		t.Fatalf("%#v", opts)
	}
	if got := sanitizeProxyRouter("App Name; $(id)"); strings.ContainsAny(got, " $;()") {
		t.Fatalf("router %q", got)
	}
	if shellQuotePath("/tmp/a'b") != `'/tmp/a'\''b'` {
		t.Fatal(shellQuotePath("/tmp/a'b"))
	}
	app := &store.Application{
		CustomDockerRestartPolicy:   "on-failure",
		CustomDockerMaxRestartCount: 3,
		IsGPUEnabled:                true,
		GPUDeviceIDs:                "0;touch /tmp/x",
		PortsMappings:               "80:80; rm -rf /, 443:443",
		FQDN:                        "https://app.example.com/api/v1, https://other.example.com",
	}
	if restartPolicy(app) != "on-failure:3" {
		t.Fatal(restartPolicy(app))
	}
	if restartPolicy(nil) != "unless-stopped" {
		t.Fatal(restartPolicy(nil))
	}
	gpus := (&Pipeline{}).gpuArgs(app)
	if len(gpus) != 2 || !strings.HasPrefix(gpus[1], "device=") {
		t.Fatal(gpus)
	}
	ports := portsMappingArgs(app)
	for _, p := range ports {
		if strings.Contains(p, "rm") || strings.Contains(p, ";") {
			t.Fatal(ports)
		}
	}
	if !strings.Contains(strings.Join(ports, " "), "80:80") {
		t.Fatal(ports)
	}
	if pathPrefixFromFQDN(app.FQDN) != "/api/v1" {
		t.Fatal(pathPrefixFromFQDN(app.FQDN))
	}
	if pathPrefixFromFQDN("https://app.example.com") != "" || pathPrefixFromFQDN("") != "" {
		t.Fatal("empty prefix")
	}
	id := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	req := Request{CommitSHA: "HEAD", App: &store.Application{ID: id, GitCommitSHA: "abc123"}}
	if deployCommit(req) != "abc123" {
		t.Fatal(deployCommit(req))
	}
	req.CommitSHA = "deadbeef"
	if deployCommit(req) != "deadbeef" {
		t.Fatal(deployCommit(req))
	}
	if injectBuildArgsEnabled(nil) != true {
		t.Fatal("nil inject")
	}
	off := &store.Application{InjectBuildArgsToDockerfile: false}
	if injectBuildArgsEnabled(off) {
		t.Fatal("inject off")
	}
	labels := appendTraefikMiddleware([]string{"traefik.http.routers.app.middlewares=auth"}, "app", "compress")
	if !strings.Contains(labels[0], "auth,compress") {
		t.Fatal(labels)
	}
}
