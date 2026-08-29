package backup

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ParseDatabaseURL extracts user, password, and db name from a postgres URL.
func ParseDatabaseURL(raw string) (user, password, dbName string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", err
	}
	user = "dockfin"
	if u.User != nil {
		user = u.User.Username()
		password, _ = u.User.Password()
	}
	dbName = strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		dbName = "dockfin"
	}
	return user, password, dbName, nil
}

// DetectPostgresContainer finds a running postgres container for the instance DB.
func DetectPostgresContainer(preferred string) (string, error) {
	if preferred = strings.TrimSpace(preferred); preferred != "" {
		out, err := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", preferred).Output()
		if err == nil && strings.TrimSpace(string(out)) == "true" {
			return preferred, nil
		}
		// Preferred exists but is not running — fall through to auto-detect.
	}
	candidates := []string{"compose-postgres-1", "dockfin-postgres-1", "postgres", "coolify-db"}
	for _, name := range candidates {
		out, err := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", name).Output()
		if err == nil && strings.TrimSpace(string(out)) == "true" {
			return name, nil
		}
	}
	// Fallback: first running container with postgres in the image name.
	out, err := exec.Command("docker", "ps", "--format", "{{.Names}}\t{{.Image}}").Output()
	if err != nil {
		return "", fmt.Errorf("detect postgres container: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		if strings.Contains(strings.ToLower(parts[1]), "postgres") {
			return strings.TrimSpace(parts[0]), nil
		}
	}
	return "", fmt.Errorf("no running postgres container found")
}

// InstanceDumpDir returns the local directory for instance DB dumps.
func InstanceDumpDir(dataDir string) string {
	return filepath.Join(dataDir, "backups", "dockfin")
}

// InstanceDumpFilename builds a Coolify-like dump filename (unique per call).
func InstanceDumpFilename() string {
	return fmt.Sprintf("pg-dump-dockfin-%s-%d.sql", time.Now().UTC().Format("20060102-150405"), time.Now().UTC().UnixNano()%1_000_000)
}

// DumpInstanceLocal runs pg_dump inside the instance postgres container onto the host filesystem.
func DumpInstanceLocal(dataDir, container, user, password, dbName, filename string) (absPath string, size int64, err error) {
	if container == "" {
		return "", 0, fmt.Errorf("container required")
	}
	if user == "" {
		user = "dockfin"
	}
	if dbName == "" {
		dbName = "dockfin"
	}
	if filename == "" {
		filename = InstanceDumpFilename()
	}
	dir := InstanceDumpDir(dataDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", 0, err
	}
	absPath = filepath.Join(dir, filename)

	cmd := exec.Command("docker", "exec", "-e", "PGPASSWORD="+password, container,
		"pg_dump", "-U", user, "--no-owner", "--no-acl", dbName)
	f, err := os.Create(absPath)
	if err != nil {
		return "", 0, err
	}
	cmd.Stdout = f
	var stderr strings.Builder
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	_ = f.Close()
	if runErr != nil {
		_ = os.Remove(absPath)
		return "", 0, fmt.Errorf("pg_dump: %v %s", runErr, strings.TrimSpace(stderr.String()))
	}
	fi, err := os.Stat(absPath)
	if err != nil {
		return absPath, 0, nil
	}
	return absPath, fi.Size(), nil
}

// RestoreInstanceLocal loads a SQL dump produced by DumpInstanceLocal into the instance Postgres.
func RestoreInstanceLocal(dataDir, container, user, password, dbName, filename string) error {
	if container == "" {
		return fmt.Errorf("container required")
	}
	if user == "" {
		user = "dockfin"
	}
	if dbName == "" {
		dbName = "dockfin"
	}
	if filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, "..") {
		return fmt.Errorf("invalid filename")
	}
	if !strings.HasPrefix(filename, "pg-dump-dockfin-") || !strings.HasSuffix(filename, ".sql") {
		return fmt.Errorf("invalid dump filename")
	}
	absPath := filepath.Join(InstanceDumpDir(dataDir), filename)
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("dump not found: %w", err)
	}
	term := exec.Command("docker", "exec", "-e", "PGPASSWORD="+password, container,
		"psql", "-U", user, "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c",
		fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s AND pid <> pg_backend_pid();`, pqLiteral(dbName)))
	var termErr strings.Builder
	term.Stderr = &termErr
	_ = term.Run()

	// Dumps from DumpInstanceLocal are plain SQL without DROP/CREATE, so a live
	// Dockfin database already has colliding tables. Recreate public first.
	reset := exec.Command("docker", "exec", "-e", "PGPASSWORD="+password, container,
		"psql", "-U", user, "-d", dbName, "-v", "ON_ERROR_STOP=1", "-c",
		`DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public; GRANT ALL ON SCHEMA public TO PUBLIC; GRANT ALL ON SCHEMA public TO `+pgIdent(user))
	var resetErr strings.Builder
	reset.Stderr = &resetErr
	if err := reset.Run(); err != nil {
		return fmt.Errorf("reset schema: %v %s", err, strings.TrimSpace(resetErr.String()))
	}

	cmd := exec.Command("docker", "exec", "-i", "-e", "PGPASSWORD="+password, container,
		"psql", "-U", user, "-d", dbName, "-v", "ON_ERROR_STOP=1")
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd.Stdin = f
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("psql restore: %v %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func pqLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func pgIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// EnforceLocalRetention keeps the newest keepCount dump files; 0 means unlimited.
func EnforceLocalRetention(dataDir string, keepCount int) error {
	if keepCount <= 0 {
		return nil
	}
	dir := InstanceDumpDir(dataDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	type fileInfo struct {
		name string
		mod  time.Time
	}
	var files []fileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "pg-dump-dockfin-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{name: e.Name(), mod: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	for i := keepCount; i < len(files); i++ {
		_ = os.Remove(filepath.Join(dir, files[i].name))
	}
	return nil
}
