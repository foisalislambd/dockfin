package services

import "testing"

func TestPortFromDockerfile(t *testing.T) {
	if got := PortFromDockerfile("FROM nginx\nEXPOSE 8080\n"); got != 8080 {
		t.Fatalf("got %d", got)
	}
	if got := PortFromDockerfile("FROM nginx\n"); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestEnvFromDockerfile(t *testing.T) {
	df := `FROM alpine
ENV FOO=bar
ENV BAZ qux
ENV A=1 B=2
# ENV SKIP=me
`
	got := EnvFromDockerfile(df)
	if got["FOO"] != "bar" || got["BAZ"] != "qux" || got["A"] != "1" || got["B"] != "2" {
		t.Fatalf("unexpected: %#v", got)
	}
	if _, ok := got["SKIP"]; ok {
		t.Fatal("commented ENV should be skipped")
	}
}
