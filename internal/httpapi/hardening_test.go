package httpapi

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDecodeJSONOptionalEmptyAndUnknown(t *testing.T) {
	var body struct {
		ForceRebuild bool `json:"force_rebuild"`
	}
	req, _ := http.NewRequest(http.MethodPost, "/", http.NoBody)
	if err := decodeJSONOptional(req, &body); err != nil {
		t.Fatalf("empty: %v", err)
	}

	req, _ = http.NewRequest(http.MethodPost, "/", strings.NewReader(`{"force_rebuild":true}`))
	if err := decodeJSONOptional(req, &body); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if !body.ForceRebuild {
		t.Fatal("expected force_rebuild")
	}

	req, _ = http.NewRequest(http.MethodPost, "/", strings.NewReader(`{"nope":1}`))
	if err := decodeJSONOptional(req, &body); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestDecodeJSONMaxBytes(t *testing.T) {
	big := bytes.Repeat([]byte("a"), maxJSONBody+10)
	payload := `{"x":"` + string(big) + `"}`
	var dst map[string]string
	req, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	err := decodeJSON(req, &dst)
	if err == nil {
		t.Fatal("expected max bytes error")
	}
	var maxErr *http.MaxBytesError
	if !errors.As(err, &maxErr) {
		t.Fatalf("got %T %v", err, err)
	}
}

func TestValidateVolumeHostPath(t *testing.T) {
	if err := validateVolumeHostPath("/data/dockfin/applications/x/volumes/a", false); err != nil {
		t.Fatal(err)
	}
	if err := validateVolumeHostPath("/var/run/docker.sock", true); err == nil {
		t.Fatal("docker.sock should be blocked")
	}
	if err := validateVolumeHostPath("/etc/passwd", false); err == nil {
		t.Fatal("etc should be blocked for members")
	}
	if err := validateVolumeHostPath("/opt/data", false); err == nil {
		t.Fatal("paths outside /data/dockfin should require admin")
	}
	if err := validateVolumeHostPath("/opt/data", true); err != nil {
		t.Fatal(err)
	}
	if err := validateVolumeHostPath("../etc", false); err == nil {
		t.Fatal("relative path")
	}
}

func TestStringPtrChanged(t *testing.T) {
	same := "echo hi"
	if stringPtrChanged(nil, same) {
		t.Fatal("omitted field is not a change")
	}
	if stringPtrChanged(&same, "echo hi") {
		t.Fatal("echoed value is not a change")
	}
	next := "rm -rf /"
	if !stringPtrChanged(&next, same) {
		t.Fatal("changed value must be detected")
	}
}

func TestHealthWithoutStore(t *testing.T) {
	code, body := handleHealthStatus(&API{}, nil)
	if code != http.StatusOK {
		t.Fatalf("code %d", code)
	}
	if body["status"] != "ok" {
		t.Fatalf("%v", body)
	}
}

func TestRawOauthBool(t *testing.T) {
	if !rawOauthBool(map[string]any{"email_verified": true}, "email_verified") {
		t.Fatal("true")
	}
	if rawOauthBool(map[string]any{"email_verified": false}, "email_verified") {
		t.Fatal("false")
	}
}

func TestLoginLimiterDoesNotLeakEmptyIPs(t *testing.T) {
	l := &loginLimiter{attempts: map[string][]time.Time{}}
	if !l.allow("1.2.3.4") {
		t.Fatal("should allow")
	}
	if len(l.attempts) != 0 {
		t.Fatalf("leaked keys: %v", l.attempts)
	}
}
