package database

import (
	"fmt"
	"path"
	"strings"

	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/storagemount"
	"golang.org/x/crypto/ssh"
)

const (
	initContainerDir = "/docker-entrypoint-initdb.d"
	maxInitScripts   = 20
	maxInitBytes     = 1 << 20
)

// InitScript is one file the database image runs the first time its data directory is empty.
type InitScript struct {
	Filename string
	Content  string
}

// InitDir is the host directory mounted into the image init directory.
// It stays outside the engine data directory.
func InitDir(dbID string) string {
	return "/data/dockfin/database-init/" + dbID
}

// InitSupported reports whether the official image runs /docker-entrypoint-initdb.d.
func InitSupported(engine string) bool {
	switch engine {
	case "postgresql", "mysql", "mariadb", "mongodb":
		return true
	default:
		return false
	}
}

// ReservesInitDir reports whether a container path would hide the image init directory.
func ReservesInitDir(mount string) bool {
	mount = path.Clean(mount)
	return mount == initContainerDir || strings.HasPrefix(mount, initContainerDir+"/")
}

// NormalizeInitScripts checks filenames and bodies for one engine.
func NormalizeInitScripts(engine string, in []InitScript) ([]InitScript, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if !InitSupported(engine) {
		return nil, fmt.Errorf("init scripts are not supported for %s", engine)
	}
	if len(in) > maxInitScripts {
		return nil, fmt.Errorf("a database can have at most %d init scripts", maxInitScripts)
	}
	seen := map[string]bool{}
	out := make([]InitScript, 0, len(in))
	for _, sc := range in {
		name, err := cleanInitFilename(engine, sc.Filename)
		if err != nil {
			return nil, err
		}
		if seen[name] {
			return nil, fmt.Errorf("init script %s is listed twice", name)
		}
		seen[name] = true
		if len(sc.Content) > maxInitBytes {
			return nil, fmt.Errorf("%s is larger than %d bytes", name, maxInitBytes)
		}
		if strings.ContainsRune(sc.Content, 0) {
			return nil, fmt.Errorf("%s must be text", name)
		}
		out = append(out, InitScript{Filename: name, Content: sc.Content})
	}
	return out, nil
}

func cleanInitFilename(engine, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.Contains(name, "..") {
		return "", fmt.Errorf("init script filename is not allowed")
	}
	if strings.ContainsAny(name, "/\\:\n\r\x00") || path.Base(name) != name {
		return "", fmt.Errorf("init script filename must be a single file name")
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("init script filename must use letters, numbers, dots, dashes, or underscores")
	}
	if len(name) > 80 {
		return "", fmt.Errorf("init script filename is too long")
	}
	if strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("init script filename must not start with a dot")
	}
	ext := ""
	if i := strings.LastIndex(name, "."); i > 0 {
		ext = name[i:]
	}
	switch engine {
	case "mongodb":
		if ext == ".js" || ext == ".sh" {
			return name, nil
		}
		return "", fmt.Errorf("mongodb init scripts must end in .js or .sh")
	default:
		if ext == ".sql" || ext == ".sh" {
			return name, nil
		}
		return "", fmt.Errorf("%s init scripts must end in .sql or .sh", engine)
	}
}

// InitVolumeArgs mounts the host init directory read-only when scripts exist.
func InitVolumeArgs(dbID string, scripts []InitScript) []string {
	if len(scripts) == 0 {
		return nil
	}
	return []string{"-v", InitDir(dbID) + ":" + initContainerDir + ":ro"}
}

// InstallInitScripts writes the current scripts and removes files that are no longer saved.
func InstallInitScripts(client *ssh.Client, dbID, engine string, scripts []InitScript) error {
	scripts, err := NormalizeInitScripts(engine, scripts)
	if err != nil {
		return err
	}
	if !InitSupported(engine) {
		return nil
	}
	if client == nil {
		return fmt.Errorf("server connection is required to write init scripts")
	}
	dir := InitDir(dbID)
	if err := prepareInitDir(client, dir); err != nil {
		return err
	}
	if _, errOut, err := sshx.RunArgs(client, "chmod", "755", dir); err != nil {
		return fmt.Errorf("chmod %s: %v %s", dir, err, errOut)
	}
	if err := clearInitFiles(client, dir); err != nil {
		return err
	}
	for _, sc := range scripts {
		target := dir + "/" + sc.Filename
		if err := sshx.WriteFile(client, target, []byte(sc.Content)); err != nil {
			return err
		}
		// 644 keeps .sh non-executable so PostgreSQL, MySQL, and MariaDB source it.
		// An executable script without a shebang makes the first start fail.
		if _, errOut, err := sshx.RunArgs(client, "chmod", "644", target); err != nil {
			return fmt.Errorf("chmod %s: %v %s", target, err, errOut)
		}
	}
	return nil
}

// RemoveInit deletes the host init directory when it is a real directory at the expected path.
func RemoveInit(client *ssh.Client, dbID string) error {
	if client == nil {
		return fmt.Errorf("server connection is required")
	}
	dir := InitDir(dbID)
	canonical, kind, err := storagemount.Inspect(client, dir)
	if err != nil {
		return err
	}
	if kind == "missing" {
		return nil
	}
	if err := storagemount.AcceptCanonical(dir, canonical); err != nil {
		return err
	}
	if kind != "dir" {
		return fmt.Errorf("%s is not a directory", dir)
	}
	_, errOut, err := sshx.RunArgs(client, "rm", "-rf", dir)
	if err != nil {
		return fmt.Errorf("remove init scripts: %v %s", err, errOut)
	}
	return nil
}

func prepareInitDir(client *ssh.Client, dir string) error {
	canonical, kind, err := storagemount.Inspect(client, dir)
	if err != nil {
		return err
	}
	if err := storagemount.AcceptCanonical(dir, canonical); err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	switch kind {
	case "dir":
		return nil
	case "missing":
		if _, errOut, err := sshx.RunArgs(client, "mkdir", "-p", dir); err != nil {
			return fmt.Errorf("mkdir %s: %v %s", dir, err, errOut)
		}
		canonical, kind, err = storagemount.Inspect(client, dir)
		if err != nil {
			return err
		}
		if err := storagemount.AcceptCanonical(dir, canonical); err != nil {
			return fmt.Errorf("%s: %w", dir, err)
		}
		if kind != "dir" {
			return fmt.Errorf("%s is not a directory", dir)
		}
		return nil
	default:
		return fmt.Errorf("%s is not a directory", dir)
	}
}

func clearInitFiles(client *ssh.Client, dir string) error {
	stdout, errOut, err := sshx.RunArgs(client, "find", dir, "-mindepth", "1", "-maxdepth", "1", "-print0")
	if err != nil {
		return fmt.Errorf("list init scripts: %v %s", err, errOut)
	}
	for _, entry := range strings.Split(stdout, "\x00") {
		if entry == "" {
			continue
		}
		base := path.Base(entry)
		if base == "." || base == ".." || base != path.Clean(base) || strings.ContainsAny(base, "/\\") {
			return fmt.Errorf("refusing to clean %s", entry)
		}
		target := dir + "/" + base
		if path.Dir(target) != dir {
			return fmt.Errorf("refusing to clean %s", target)
		}
		canonical, kind, err := storagemount.Inspect(client, target)
		if err != nil {
			if strings.Contains(err.Error(), "symbolic link") {
				if _, errOut, rmErr := sshx.RunArgs(client, "rm", "-f", target); rmErr != nil {
					return fmt.Errorf("remove %s: %v %s", target, rmErr, errOut)
				}
				continue
			}
			return err
		}
		if err := storagemount.AcceptCanonical(target, canonical); err != nil {
			return fmt.Errorf("%s: %w", target, err)
		}
		if kind == "dir" {
			return fmt.Errorf("%s contains a directory", dir)
		}
		if kind == "missing" {
			continue
		}
		if _, errOut, err := sshx.RunArgs(client, "rm", "-f", target); err != nil {
			return fmt.Errorf("remove %s: %v %s", target, err, errOut)
		}
	}
	return nil
}
