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
		`docker exec -e PGPASSWORD=%s %s pg_dump -U goolify goolify > %s`,
		shellQuote(password), shellQuote(container), shellQuote(outPath),
	)
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("pg_dump: %v %s", err, errOut)
	}
	return nil
}

func DefaultFilename(engine, id string) string {
	return fmt.Sprintf("%s-%s-%s.sql", engine, id, time.Now().UTC().Format("20060102-150405"))
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
