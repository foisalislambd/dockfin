package database

import (
	"strings"
	"testing"
)

func TestCacheFlushArgs(t *testing.T) {
	args, err := CacheFlushArgs("redis", "dockfin-db-1", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "REDISCLI_AUTH=s3cret") || !strings.HasSuffix(joined, "redis-cli FLUSHALL") {
		t.Fatal(joined)
	}
	args, err = CacheFlushArgs("keydb", "db", "")
	if err != nil || strings.Contains(strings.Join(args, " "), "REDISCLI_AUTH") || args[len(args)-2] != "keydb-cli" {
		t.Fatal(args, err)
	}
	if _, err := CacheFlushArgs("postgresql", "db", ""); err == nil {
		t.Fatal("postgres")
	}
}
