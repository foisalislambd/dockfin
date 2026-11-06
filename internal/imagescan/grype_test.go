package imagescan

import "testing"

func TestParseGrypeIgnoresAndCounts(t *testing.T) {
	raw := []byte(`{"matches":[
		{"vulnerability":{"id":"CVE-1","severity":"Critical","description":"bad","fix":{"versions":["2.0"]}},"artifact":{"name":"openssl","version":"1.1"}},
		{"vulnerability":{"id":"CVE-2","severity":"Low","description":"meh"},"artifact":{"name":"libc","version":"1"}}
	]}`)
	sum, err := ParseGrype(raw, map[string]struct{}{"CVE-2": {}})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Critical != 1 || sum.Low != 0 || len(sum.Findings) != 1 {
		t.Fatalf("%+v", sum)
	}
	if sum.Findings[0].FixedVersion != "2.0" || sum.Findings[0].Package != "openssl" {
		t.Fatalf("%+v", sum.Findings[0])
	}
}

func TestValidImageRef(t *testing.T) {
	if !ValidImageRef("ghcr.io/org/app:1.2") || !ValidImageRef("dockfin/app:latest") {
		t.Fatal("expected valid refs")
	}
	if ValidImageRef("nginx;rm") || ValidImageRef("") || ValidImageRef("a b") {
		t.Fatal("expected invalid refs")
	}
}

func TestComposeImages(t *testing.T) {
	got := ComposeImages("services:\n  web:\n    image: nginx:1.27\n  db:\n    image: \"postgres:16\"\n")
	if len(got) != 2 || got[0] != "nginx:1.27" || got[1] != "postgres:16" {
		t.Fatalf("%v", got)
	}
}
