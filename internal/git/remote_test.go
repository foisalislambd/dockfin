package git

import "testing"

func TestValidateRemote(t *testing.T) {
	ok := []string{
		"https://github.com/acme/app.git",
		"https://x-access-token:ghs_abc@github.com/acme/app.git",
		"http://gitlab.internal.example/group/app.git",
		"ssh://git@github.com/acme/app.git",
		"git@github.com:acme/app.git",
		"acme/app",
	}
	for _, raw := range ok {
		if err := ValidateRemote(raw); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	bad := []string{
		"",
		"--upload-pack=id",
		"ext::sh -c id",
		"file:///etc/passwd",
		"/etc/passwd",
		"../secret",
		"http://169.254.169.254/latest/meta-data",
		"https://metadata.google.internal/computeMetadata/v1/",
		"git@169.254.169.254:repo.git",
		"https://github.com/acme/app.git\n--upload-pack=id",
		"-c core.sshCommand=id",
	}
	for _, raw := range bad {
		if err := ValidateRemote(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestValidateRef(t *testing.T) {
	if err := ValidateRef("feature/login"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRef("--upload-pack=id"); err == nil {
		t.Fatal("option-like ref accepted")
	}
	if err := ValidateRef("foo..bar"); err == nil {
		t.Fatal("dotdot ref accepted")
	}
}
