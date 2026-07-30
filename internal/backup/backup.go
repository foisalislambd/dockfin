package backup

import (
	"fmt"
	"strings"
	"time"

	"github.com/goolify/goolify/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// DumpPostgres runs pg_dump inside a database container and writes to a remote path.
func DumpPostgres(client *ssh.Client, container, password, outPath string) error {
	cmd := fmt.Sprintf(
		`mkdir -p /data/goolify/backups && docker exec -e PGPASSWORD=%s %s pg_dump -U goolify goolify > %s`,
		shellQuote(password), shellQuote(container), shellQuote(outPath),
	)
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("pg_dump: %v %s", err, errOut)
	}
	return nil
}

// RestorePostgres pipes a SQL dump into the database container.
func RestorePostgres(client *ssh.Client, container, password, dumpPath string) error {
	cmd := fmt.Sprintf(
		`docker exec -i -e PGPASSWORD=%s %s psql -U goolify goolify < %s`,
		shellQuote(password), shellQuote(container), shellQuote(dumpPath),
	)
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("psql restore: %v %s", err, errOut)
	}
	return nil
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
	return fmt.Sprintf("%s-%s-%s.sql", engine, id, time.Now().UTC().Format("20060102-150405"))
}

func DumpPath(filename string) string {
	return "/data/goolify/backups/" + filename
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
