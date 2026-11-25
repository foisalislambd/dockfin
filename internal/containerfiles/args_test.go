package containerfiles

import "testing"

func TestListArgsRejectsTraversal(t *testing.T) {
	args, err := ListArgs("web", "/app")
	if err != nil || args[0] != "docker" || args[2] != "web" {
		t.Fatalf("%v %v", args, err)
	}
	for _, dir := range []string{"../etc", "/tmp/../etc", "relative", "/tmp/a\nb", "/tmp/\x00"} {
		if _, err := ListArgs("web", dir); err == nil {
			t.Fatalf("list accepted %q", dir)
		}
		if _, err := ReadArgs("web", dir); err == nil {
			t.Fatalf("read accepted %q", dir)
		}
	}
	if _, err := ListArgs("web;id", "/tmp"); err == nil {
		t.Fatal("accepted container")
	}
}
