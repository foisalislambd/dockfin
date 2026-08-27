package backup

import (
	"fmt"
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// DumpPostgres runs pg_dump -Fc (custom format) inside a database container.
func DumpPostgres(client *ssh.Client, container, password, outPath string) error {
	cmd := fmt.Sprintf(
		`mkdir -p /data/dockfin/backups && docker exec -e PGPASSWORD=%s %s pg_dump -Fc -U dockfin dockfin > %s && test -s %s`,
		shellQuote(password), shellQuote(container), shellQuote(outPath), shellQuote(outPath),
	)
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("pg_dump: %v %s", err, errOut)
	}
	return nil
}

// DumpMySQL runs mysqldump inside a MySQL/MariaDB container.
func DumpMySQL(client *ssh.Client, container, password, outPath string) error {
	cmd := fmt.Sprintf(
		`mkdir -p /data/dockfin/backups && docker exec -e MYSQL_PWD=%s %s mysqldump -uroot --single-transaction --routines --triggers dockfin > %s`,
		shellQuote(password), shellQuote(container), shellQuote(outPath),
	)
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("mysqldump: %v %s", err, errOut)
	}
	return nil
}

// DumpRedis copies the Redis RDB (or AOF) from the container data dir via docker cp.
func DumpRedis(client *ssh.Client, container, password, outPath string) error {
	c := shellQuote(container)
	out := shellQuote(outPath)
	auth := ""
	if password != "" {
		// REDISCLI_AUTH avoids putting the password on the process argv list.
		auth = fmt.Sprintf("-e REDISCLI_AUTH=%s ", shellQuote(password))
	}
	cli := "redis-cli"
	cmd := fmt.Sprintf(
		`mkdir -p /data/dockfin/backups && docker exec %s%s %s BGSAVE >/dev/null 2>&1 || docker exec %s%s keydb-cli BGSAVE >/dev/null 2>&1 || true; sleep 1; `+
			`if docker exec %s test -f /data/dump.rdb; then docker cp %s:/data/dump.rdb %s; `+
			`elif docker exec %s test -f /data/appendonly.aof; then docker cp %s:/data/appendonly.aof %s; `+
			`else echo 'no redis dump file found' >&2; exit 1; fi`,
		auth, c, cli, auth, c, c, c, out, c, c, out,
	)
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("redis dump: %v %s", err, errOut)
	}
	return nil
}

// DumpDatabase dispatches to the engine-specific dump implementation.
func DumpDatabase(client *ssh.Client, engine, container, password, outPath string) error {
	switch engine {
	case "postgresql":
		return DumpPostgres(client, container, password, outPath)
	case "mysql", "mariadb":
		return DumpMySQL(client, container, password, outPath)
	case "redis", "keydb":
		return DumpRedis(client, container, password, outPath)
	default:
		return fmt.Errorf("backup not supported for engine %q", engine)
	}
}

// RestorePostgres restores a dump into the database container.
// Custom-format files (.dump / .backup / .pgdump) use pg_restore; everything else uses psql.
func RestorePostgres(client *ssh.Client, container, password, dumpPath string) error {
	var cmd string
	if PostgresCustomFormat(dumpPath) {
		cmd = fmt.Sprintf(
			`docker exec -i -e PGPASSWORD=%s %s pg_restore -U dockfin -d dockfin --no-owner --no-acl --clean --if-exists < %s`,
			shellQuote(password), shellQuote(container), shellQuote(dumpPath),
		)
	} else {
		cmd = fmt.Sprintf(
			`docker exec -i -e PGPASSWORD=%s %s psql -U dockfin dockfin < %s`,
			shellQuote(password), shellQuote(container), shellQuote(dumpPath),
		)
	}
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		if PostgresCustomFormat(dumpPath) {
			return fmt.Errorf("pg_restore: %v %s", err, errOut)
		}
		return fmt.Errorf("psql restore: %v %s", err, errOut)
	}
	return nil
}

// PostgresCustomFormat reports whether dumpPath is a pg_dump custom-format archive.
func PostgresCustomFormat(dumpPath string) bool {
	lower := strings.ToLower(strings.TrimSpace(dumpPath))
	return strings.HasSuffix(lower, ".dump") || strings.HasSuffix(lower, ".backup") || strings.HasSuffix(lower, ".pgdump")
}

// RestoreMySQL pipes a SQL dump into MySQL/MariaDB.
func RestoreMySQL(client *ssh.Client, container, password, dumpPath string) error {
	cmd := fmt.Sprintf(
		`docker exec -i -e MYSQL_PWD=%s %s mysql -uroot dockfin < %s`,
		shellQuote(password), shellQuote(container), shellQuote(dumpPath),
	)
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("mysql restore: %v %s", err, errOut)
	}
	return nil
}

// RestoreRedis copies an RDB/AOF dump into the container data dir and restarts Redis/KeyDB.
func RestoreRedis(client *ssh.Client, container, dumpPath string) error {
	c := shellQuote(container)
	dump := shellQuote(dumpPath)
	dest := "/data/dump.rdb"
	if strings.HasSuffix(strings.ToLower(dumpPath), ".aof") {
		dest = "/data/appendonly.aof"
	}
	cmd := fmt.Sprintf(
		`if [ ! -f %s ]; then echo 'dump not found' >&2; exit 1; fi; `+
			`docker stop %s >/dev/null; docker cp %s %s:%s; docker start %s >/dev/null`,
		dump, c, dump, c, dest, c,
	)
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("redis restore: %v %s", err, errOut)
	}
	return nil
}

// RestoreDatabase dispatches to the engine-specific restore implementation.
func RestoreDatabase(client *ssh.Client, engine, container, password, dumpPath string) error {
	switch engine {
	case "postgresql":
		return RestorePostgres(client, container, password, dumpPath)
	case "mysql", "mariadb":
		return RestoreMySQL(client, container, password, dumpPath)
	case "redis", "keydb":
		return RestoreRedis(client, container, dumpPath)
	default:
		return fmt.Errorf("restore not supported for engine %q", engine)
	}
}

// FileSize returns remote file size in bytes (best-effort).
func FileSize(client *ssh.Client, path string) int64 {
	out, _, err := sshx.Run(client, fmt.Sprintf(`stat -c %%s %s 2>/dev/null || wc -c < %s`, shellQuote(path), shellQuote(path)))
	if err != nil {
		return 0
	}
	var n int64
	_, _ = fmt.Sscanf(strings.TrimSpace(out), "%d", &n)
	return n
}

func DefaultFilename(engine, id string) string {
	ext := "sql"
	switch engine {
	case "postgresql":
		ext = "dump"
	case "redis", "keydb", "dragonfly":
		ext = "rdb"
	}
	return fmt.Sprintf("%s-%s-%s.%s", engine, id, time.Now().UTC().Format("20060102-150405"), ext)
}

func DumpPath(filename string) string {
	return "/data/dockfin/backups/" + filename
}

// EnforceRemoteRetention keeps the newest keepCount dumps matching prefix on the remote host.
func EnforceRemoteRetention(client *ssh.Client, engine, resourceID string, keepCount int) error {
	if keepCount <= 0 || client == nil {
		return nil
	}
	prefix := fmt.Sprintf("%s-%s-", engine, resourceID)
	// Keep the glob literal-safe: only allow alnum, dash, underscore, dot.
	safe := make([]rune, 0, len(prefix))
	for _, r := range prefix {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			safe = append(safe, r)
		}
	}
	if len(safe) == 0 {
		return nil
	}
	cmd := fmt.Sprintf(
		`cd /data/dockfin/backups 2>/dev/null && ls -1t %s* 2>/dev/null | tail -n +%d | xargs -r rm -f`,
		string(safe), keepCount+1,
	)
	_, _, err := sshx.Run(client, cmd)
	return err
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
