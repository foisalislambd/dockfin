package database

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const sqliteDataDir = "/var/lib/sqlite"

// sqliteFile matches one database file name. Paths and shell characters are rejected.
var sqliteFile = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// NormalizeSqliteDatabases turns a comma-separated list into the value of SQLITE_DATABASES.
// An empty value becomes database.sqlite.
func NormalizeSqliteDatabases(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "database.sqlite"
	}
	parts := strings.Split(raw, ",")
	seen := map[string]struct{}{}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || !sqliteFile.MatchString(part) || strings.HasPrefix(part, ".") || strings.Contains(part, "..") {
			return "", fmt.Errorf("sqlite file name %q is not allowed", part)
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	if len(out) > 8 {
		return "", fmt.Errorf("at most 8 sqlite files")
	}
	if len(out) == 0 {
		return "", fmt.Errorf("sqlite file name is required")
	}
	return strings.Join(out, ","), nil
}

// SqliteDatabases reads sqlite_databases from engine_config.
func SqliteDatabases(config json.RawMessage) (string, error) {
	raw := ""
	if len(config) > 0 && string(config) != "null" {
		var body struct {
			Databases string `json:"sqlite_databases"`
		}
		if err := json.Unmarshal(config, &body); err != nil {
			return "", fmt.Errorf("invalid engine_config")
		}
		raw = body.Databases
	}
	return NormalizeSqliteDatabases(raw)
}

// SqliteDataDirID reports whether host is exactly one database data directory.
// A path inside that directory is not accepted.
func SqliteDataDirID(host string) (string, bool) {
	host = path.Clean(strings.TrimSpace(host))
	const prefix = "/data/dockfin/databases/"
	if !strings.HasPrefix(host, prefix) {
		return "", false
	}
	id := strings.TrimPrefix(host, prefix)
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}
