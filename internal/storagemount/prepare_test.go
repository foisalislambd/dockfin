package storagemount

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultHostStaysOutsideDatabaseData(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	host := DefaultHost("database", id, "my.cnf", true)
	data := "/data/dockfin/databases/" + id
	if host == data || strings.HasPrefix(host, data+"/") {
		t.Fatalf("file mount %s is inside the database data directory", host)
	}
	if !strings.Contains(host, "/database-files/") || !strings.HasSuffix(host, "/files/my.cnf") {
		t.Fatalf("host %s", host)
	}
	if DefaultHost("database", id, "my.cnf", true) == DefaultHost("database", id, "my-cnf", true) {
		t.Fatal("different names collapsed to one host path")
	}
	spaced := DefaultHost("database", id, "my vol", true)
	dashed := DefaultHost("database", id, "my-vol", true)
	if spaced == dashed {
		t.Fatal("unsafe name collapsed onto a safe name")
	}
	dir := DefaultHost("database", id, "extra", false)
	if strings.HasPrefix(dir, data+"/") {
		t.Fatalf("directory mount %s is inside the database data directory", dir)
	}
}

func TestAcceptCanonicalRejectsSymlinkRewrite(t *testing.T) {
	if err := AcceptCanonical("/data/dockfin/applications/a/files/c", "/data/dockfin/applications/a/files/c"); err != nil {
		t.Fatal(err)
	}
	if err := AcceptCanonical("/data/dockfin/applications/a/files/c", "/etc/c"); err == nil {
		t.Fatal("rewritten path was accepted")
	}
	if err := AcceptCanonical("/", "/"); err == nil {
		t.Fatal("root was accepted")
	}
}

func TestValidateResourceHost(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	other := "22222222-2222-2222-2222-222222222222"
	own := DefaultHost("application", id, "nginx.conf", true)
	if err := ValidateResourceHost("application", id, own); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResourceHost("application", id, "/data/dockfin/applications/"+other+"/files/a"); err == nil {
		t.Fatal("another application's files were accepted")
	}
	if err := ValidateResourceHost("database", id, "/data/dockfin/databases/"+id+"/secret"); err == nil {
		t.Fatal("database data directory was accepted")
	}
	if err := ValidateResourceHost("database", id, "/data/dockfin/database-ssl/"+id+"/server.key"); err == nil {
		t.Fatal("certificate directory was accepted")
	}
	if _, err := ResolveHost("application", id, "nginx.conf", "", true); err != nil {
		t.Fatal(err)
	}
}

func TestValidateContent(t *testing.T) {
	if err := ValidateContent(strings.Repeat("a", MaxContentBytes+1)); err == nil {
		t.Fatal("oversized content")
	}
	if err := ValidateContent("ok\x00no"); err == nil {
		t.Fatal("binary content")
	}
	if err := ValidateMountPath("/etc/nginx/nginx.conf"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMountPath("/var/lib/postgresql/data"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMountPath("/"); err == nil {
		t.Fatal("root mount")
	}
}

func TestInspectScriptFollowsNoSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sh", "-c", inspectScript, "sh", filepath.Join(link, "missing")).CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "LINK" {
		t.Fatalf("symlink ancestor should be refused, got %s", out)
	}
	out, err = exec.Command("sh", "-c", inspectScript, "sh", filepath.Join(target, "missing")).CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "KIND missing") || !strings.Contains(text, "RESOLVED ") {
		t.Fatalf("output %s", text)
	}
}
