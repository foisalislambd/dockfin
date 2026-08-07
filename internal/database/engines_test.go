package database

import (
	"strings"
	"testing"
)

func TestHostDataDir(t *testing.T) {
	got := HostDataDir("abc-123")
	want := "/data/dockfin/databases/abc-123"
	if got != want {
		t.Fatalf("HostDataDir = %q, want %q", got, want)
	}
}

func TestVolumeArgs(t *testing.T) {
	cases := []struct {
		engine string
		path   string
	}{
		{"postgresql", "/var/lib/postgresql/data"},
		{"mysql", "/var/lib/mysql"},
		{"mariadb", "/var/lib/mysql"},
		{"mongodb", "/data/db"},
		{"redis", "/data"},
		{"keydb", "/data"},
		{"dragonfly", "/data"},
		{"clickhouse", "/var/lib/clickhouse"},
	}
	id := "db-uuid"
	for _, tc := range cases {
		args := VolumeArgs(id, tc.engine)
		if len(args) != 2 || args[0] != "-v" {
			t.Fatalf("%s: unexpected args %v", tc.engine, args)
		}
		want := HostDataDir(id) + ":" + tc.path
		if args[1] != want {
			t.Fatalf("%s: mount = %q, want %q", tc.engine, args[1], want)
		}
	}
}

func TestEngineCmdRedisPersists(t *testing.T) {
	cmd := engineCmd("redis", "secret")
	joined := strings.Join(cmd, " ")
	if !strings.Contains(joined, "--appendonly yes") {
		t.Fatalf("redis cmd missing appendonly: %v", cmd)
	}
	if !strings.Contains(joined, "--dir /data") {
		t.Fatalf("redis cmd missing dir: %v", cmd)
	}
	if !strings.Contains(joined, "--requirepass secret") {
		t.Fatalf("redis cmd missing password: %v", cmd)
	}
}

func TestEngineCmdNoPasswordNonKV(t *testing.T) {
	if got := engineCmd("postgresql", ""); got != nil {
		t.Fatalf("postgresql cmd = %v, want nil", got)
	}
}
