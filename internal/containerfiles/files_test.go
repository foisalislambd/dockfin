package containerfiles

import "testing"

func TestCleanPath(t *testing.T) {
	p, err := CleanPath("/var/www/html")
	if err != nil || p != "/var/www/html" {
		t.Fatal(p, err)
	}
	if _, err := CleanPath("/tmp/../etc/passwd"); err == nil {
		t.Fatal("traversal")
	}
	if _, err := CleanPath("relative"); err == nil {
		t.Fatal("relative")
	}
	p, err = CleanPath("")
	if err != nil || p != "/" {
		t.Fatal(p, err)
	}
}

func TestListArgsRejectsBadContainer(t *testing.T) {
	if _, err := ListArgs("ok name", "/tmp"); err == nil {
		t.Fatal("space")
	}
	args, err := ListArgs("app_1", "/app")
	if err != nil || args[0] != "docker" || args[2] != "app_1" {
		t.Fatal(args, err)
	}
}

func TestParseStatLines(t *testing.T) {
	out := "directory|4096|1700000000|/app/src\nregular file|12|1700000001|/app/readme\n"
	got := ParseStatLines(out)
	if len(got) != 2 || !got[0].IsDir || got[0].Name != "src" || got[1].Size != 12 {
		t.Fatalf("%+v", got)
	}
	n, ok := ParseByteCount("42 /app/readme")
	if !ok || n != 42 {
		t.Fatal(n, ok)
	}
}
