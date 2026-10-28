package database

import (
	"strings"
	"testing"
)

func TestSSLCommandLeavesPlainEnginesUntouched(t *testing.T) {
	got := appendSSLCommand("postgresql", nil)
	if len(got) == 0 || got[0] != "postgres" || !strings.Contains(strings.Join(got, " "), "ssl=on") {
		t.Fatalf("%#v", got)
	}
	redis := appendSSLCommand("redis", engineCmd("redis", "secret"))
	joined := strings.Join(redis, " ")
	if !strings.Contains(joined, "--requirepass") || !strings.Contains(joined, "--tls-port") || !strings.Contains(joined, "6380") {
		t.Fatalf("%s", joined)
	}
	mysql := strings.Join(sslCommand("mysql"), " ")
	if !strings.Contains(mysql, "--require-secure-transport=ON") || !strings.Contains(mysql, "dockfin-ca.crt") {
		t.Fatalf("%s", mysql)
	}
	if SSLSupported("clickhouse") {
		t.Fatal("clickhouse should not claim ssl support")
	}
	vols := strings.Join(sslVolumeArgs("abc", "mongodb"), " ")
	if !strings.Contains(vols, "server.pem") || !strings.Contains(vols, "ca.pem") {
		t.Fatalf("%s", vols)
	}
	data := HostDataDir("abc")
	ssl := sslDir("abc")
	if ssl == data || strings.HasPrefix(ssl, data+"/") {
		t.Fatalf("ssl dir %s is inside the data volume %s", ssl, data)
	}
	if tlsFileOwner("postgresql") != "postgres" || tlsFileOwner("mysql") != "mysql" {
		t.Fatal("ssl files must be owned by the image user, not a fixed uid")
	}
	if !CoversDataDir("postgresql", "/var/lib/postgresql") || !CoversDataDir("postgresql", "/var/lib/postgresql/data/base") {
		t.Fatal("a parent or child of the data directory should be rejected")
	}
	if CoversDataDir("postgresql", "/etc/postgresql") {
		t.Fatal("a path outside the data directory was rejected")
	}
	if !CoversDataDir("redis", "/data/db") {
		t.Fatal("redis data subdirectory should be rejected")
	}
}
