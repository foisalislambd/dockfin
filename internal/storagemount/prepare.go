// Package storagemount writes file and directory mounts on a server before Docker
// bind-mounts them. A missing file source becomes a directory, so files are created first.
package storagemount

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

const (
	// MaxContentBytes is the largest file body Dockfin stores and writes.
	MaxContentBytes = 1 << 20
)

const inspectScript = `
path=$1
if [ -L "$path" ]; then echo LINK; exit 0; fi
cur=$path
rest=
while [ ! -e "$cur" ]; do
  if [ -L "$cur" ]; then echo LINK; exit 0; fi
  rest="/${cur##*/}$rest"
  next=${cur%/*}
  [ "$next" = "$cur" ] && echo ROOT && exit 0
  cur=$next
done
if [ -L "$cur" ]; then echo LINK; exit 0; fi
real=$(readlink -f "$cur") || { echo UNRESOLVED; exit 0; }
case "$rest/" in */./*|*/../*) echo ESCAPE; exit 0 ;; esac
printf 'RESOLVED %s\n' "${real%/}$rest"
if [ -d "$path" ]; then echo KIND dir
elif [ -f "$path" ]; then echo KIND file
elif [ -e "$path" ]; then echo KIND other
else echo KIND missing
fi
`

// DefaultHost is the host path used when a mount does not set one.
// Database mounts stay outside the engine data directory.
func DefaultHost(resourceType, resourceID, name string, isFile bool) string {
	kind := "volumes"
	if isFile {
		kind = "files"
	}
	switch resourceType {
	case "database":
		return path.Join("/data/dockfin/database-files", resourceID, kind, pathSegment(name))
	default:
		return path.Join("/data/dockfin/applications", resourceID, kind, pathSegment(name))
	}
}

// ResolveHost returns the cleaned host path that will be created and mounted.
func ResolveHost(resourceType, resourceID, name, stored string, isFile bool) (string, error) {
	host := strings.TrimSpace(stored)
	if host == "" {
		host = DefaultHost(resourceType, resourceID, name, isFile)
	}
	host = path.Clean(host)
	if err := ValidateResourceHost(resourceType, resourceID, host); err != nil {
		return "", err
	}
	return host, nil
}

// ValidateResourceHost keeps a mount out of another resource and out of control-plane directories.
func ValidateResourceHost(resourceType, resourceID, host string) error {
	host = path.Clean(strings.TrimSpace(host))
	if host == "/" || !strings.HasPrefix(host, "/") || strings.Contains(host, "..") {
		return fmt.Errorf("invalid mount host path")
	}
	if host == "/data/dockfin/log-drain.env" {
		return fmt.Errorf("host path %s is reserved", host)
	}
	for _, root := range []string{
		"/data/dockfin/databases",
		"/data/dockfin/database-ssl",
		"/data/dockfin/ca",
		"/data/dockfin/proxy",
		"/data/dockfin/backups",
		"/data/dockfin/services",
	} {
		if within(host, root) {
			return fmt.Errorf("host path %s is reserved", host)
		}
	}
	if within(host, "/data/dockfin/applications") {
		own := "/data/dockfin/applications/" + resourceID
		if resourceType != "application" || !within(host, own) {
			return fmt.Errorf("host path belongs to another application")
		}
	}
	if within(host, "/data/dockfin/database-files") {
		own := "/data/dockfin/database-files/" + resourceID
		if resourceType != "database" || !within(host, own) {
			return fmt.Errorf("host path belongs to another database")
		}
	}
	return nil
}

func within(host, root string) bool {
	host = path.Clean(host)
	root = path.Clean(root)
	return host == root || strings.HasPrefix(host, root+"/")
}

func pathSegment(name string) string {
	name = strings.TrimSpace(name)
	if name != "" && name != "." && name != ".." && len(name) <= 80 && safeSegment(name) {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	return sanitizeName(name) + "-" + hex.EncodeToString(sum[:4])
}

func safeSegment(name string) bool {
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func sanitizeName(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "mount"
	}
	return out
}

// ValidateMountPath checks a container path used in a bind mount.
func ValidateMountPath(mountPath string) error {
	p := strings.TrimSpace(mountPath)
	if p == "" || !strings.HasPrefix(p, "/") || p == "/" {
		return fmt.Errorf("mount path must be an absolute path other than /")
	}
	if strings.Contains(p, "..") || strings.ContainsAny(p, ":\n\r\x00") {
		return fmt.Errorf("mount path contains characters that cannot be mounted")
	}
	if path.Clean(p) != p {
		return fmt.Errorf("mount path must be a clean absolute path")
	}
	return nil
}

// ValidateContent rejects binary and oversized file bodies.
func ValidateContent(content string) error {
	if len(content) > MaxContentBytes {
		return fmt.Errorf("file content is larger than %d bytes", MaxContentBytes)
	}
	if strings.ContainsRune(content, 0) {
		return fmt.Errorf("file content must be text")
	}
	return nil
}

// AcceptCanonical refuses a path whose real location differs from the path Dockfin was given.
// A symlink, including one in a parent directory, makes the two differ.
func AcceptCanonical(requested, canonical string) error {
	requested = path.Clean(requested)
	canonical = path.Clean(canonical)
	if requested == "/" || canonical == "/" {
		return fmt.Errorf("refusing the root directory")
	}
	if requested != canonical {
		return fmt.Errorf("path resolves through a symbolic link")
	}
	return nil
}

// Prepare creates a directory mount or writes a file mount on the server.
// Empty file content leaves an existing file unchanged and creates an empty file when it is missing.
func Prepare(client *ssh.Client, host string, isFile bool, content string) error {
	if client == nil {
		return fmt.Errorf("server connection is required to prepare a mount")
	}
	host = path.Clean(strings.TrimSpace(host))
	if !strings.HasPrefix(host, "/") || strings.Contains(host, "..") || host == "/" {
		return fmt.Errorf("invalid mount host path")
	}
	if isFile {
		if err := ValidateContent(content); err != nil {
			return err
		}
	}
	canonical, kind, err := Inspect(client, host)
	if err != nil {
		return err
	}
	if err := AcceptCanonical(host, canonical); err != nil {
		return fmt.Errorf("%s: %w", host, err)
	}
	switch kind {
	case "other":
		return fmt.Errorf("%s is not a regular file or directory", host)
	case "dir":
		if isFile {
			return fmt.Errorf("%s is a directory on the server", host)
		}
		return nil
	case "file":
		if !isFile {
			return fmt.Errorf("%s is a file on the server", host)
		}
		if content == "" {
			return nil
		}
		if err := sshx.WriteFile(client, host, []byte(content)); err != nil {
			return err
		}
		return chmodFile(client, host)
	case "missing":
		if !isFile {
			_, errOut, err := sshx.RunArgs(client, "mkdir", "-p", host)
			if err != nil {
				return fmt.Errorf("mkdir %s: %v %s", host, err, errOut)
			}
			return nil
		}
		parent := path.Dir(host)
		if parent != "" && parent != "/" {
			if _, errOut, err := sshx.RunArgs(client, "mkdir", "-p", parent); err != nil {
				return fmt.Errorf("mkdir %s: %v %s", parent, err, errOut)
			}
		}
		if content == "" {
			if _, errOut, err := sshx.RunArgs(client, "touch", host); err != nil {
				return fmt.Errorf("create %s: %v %s", host, err, errOut)
			}
		} else if err := sshx.WriteFile(client, host, []byte(content)); err != nil {
			return err
		}
		return chmodFile(client, host)
	default:
		return fmt.Errorf("could not inspect %s", host)
	}
}

func chmodFile(client *ssh.Client, host string) error {
	_, errOut, err := sshx.RunArgs(client, "chmod", "644", host)
	if err != nil {
		return fmt.Errorf("chmod %s: %v %s", host, err, errOut)
	}
	return nil
}

// Inspect reports the canonical path and whether it is missing, a file, a directory, or something else.
func Inspect(client *ssh.Client, host string) (canonical, kind string, err error) {
	stdout, errOut, err := sshx.RunArgs(client, "sh", "-c", inspectScript, "sh", host)
	if err != nil {
		return "", "", fmt.Errorf("inspect %s: %v %s", host, err, errOut)
	}
	var resolved, state string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "LINK":
			return "", "", fmt.Errorf("%s is a symbolic link", host)
		case line == "ESCAPE" || line == "ROOT" || line == "UNRESOLVED":
			return "", "", fmt.Errorf("could not resolve %s", host)
		case strings.HasPrefix(line, "RESOLVED "):
			resolved = strings.TrimPrefix(line, "RESOLVED ")
		case strings.HasPrefix(line, "KIND "):
			state = strings.TrimPrefix(line, "KIND ")
		}
	}
	if resolved == "" || state == "" {
		return "", "", fmt.Errorf("could not inspect %s", host)
	}
	return resolved, state, nil
}

// ManagedFile reports whether host is a file Dockfin created for this resource.
func ManagedFile(resourceType, resourceID, host string) bool {
	prefix := path.Join(path.Dir(path.Dir(DefaultHost(resourceType, resourceID, "x", true))), "files") + "/"
	return strings.HasPrefix(path.Clean(host), prefix)
}
