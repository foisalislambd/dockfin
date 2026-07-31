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
	user = "goolify"
	if u.User != nil {
		user = u.User.Username()
		password, _ = u.User.Password()
	}
	dbName = strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		dbName = "goolify"
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
	candidates := []string{"compose-postgres-1", "goolify-postgres-1", "postgres", "coolify-db"}
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
	return filepath.Join(dataDir, "backups", "goolify")
}

// InstanceDumpFilename builds a Coolify-like dump filename (unique per call).
func InstanceDumpFilename() string {
	return fmt.Sprintf("pg-dump-goolify-%s-%d.sql", time.Now().UTC().Format("20060102-150405"), time.Now().UTC().UnixNano()%1_000_000)
}

// DumpInstanceLocal runs pg_dump inside the instance postgres container onto the host filesystem.
func DumpInstanceLocal(dataDir, container, user, password, dbName, filename string) (absPath string, size int64, err error) {
	if container == "" {
		return "", 0, fmt.Errorf("container required")
	}
	if user == "" {
		user = "goolify"
	}
	if dbName == "" {
		dbName = "goolify"
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
		if e.IsDir() || !strings.HasPrefix(e.Name(), "pg-dump-goolify-") {
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
