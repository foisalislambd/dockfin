package backup

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// AppVolumeArchivePath returns the remote tar.gz path for an application volume backup.
func AppVolumeArchivePath(appID, filename string) string {
	return "/data/dockfin/backups/applications/" + appID + "/" + filename
}

// DefaultAppBackupFilename builds a timestamped archive name.
func DefaultAppBackupFilename(appID string) string {
	return fmt.Sprintf("app-%s-%s.tar.gz", appID[:8], time.Now().UTC().Format("20060102-150405"))
}

// TarHostPaths creates a gzipped tar of one or more host directories/files on the remote server.
func TarHostPaths(client *ssh.Client, outPath string, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths to archive")
	}
	dir := filepath.ToSlash(filepath.Dir(outPath))
	var rel []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = strings.TrimPrefix(p, "/")
		if p == "" {
			continue
		}
		rel = append(rel, shellQuote(p))
	}
	if len(rel) == 0 {
		return fmt.Errorf("no paths to archive")
	}
	cmd := fmt.Sprintf(
		`mkdir -p %s && tar czf %s -C / %s`,
		shellQuote(dir),
		shellQuote(outPath),
		strings.Join(rel, " "),
	)
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("tar: %v %s", err, errOut)
	}
	return nil
}

// TarNamedDockerVolume archives a named Docker volume via a temporary alpine container.
func TarNamedDockerVolume(client *ssh.Client, volumeName, outPath string) error {
	dir := filepath.ToSlash(filepath.Dir(outPath))
	base := filepath.Base(outPath)
	cmd := fmt.Sprintf(
		`mkdir -p %s && docker run --rm -v %s:/data:ro -v %s:/backup alpine:3.21 tar czf /backup/%s -C /data .`,
		shellQuote(dir),
		shellQuote(volumeName),
		shellQuote(dir),
		shellQuote(base),
	)
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("volume tar: %v %s", err, errOut)
	}
	return nil
}

// UntarHostPaths extracts a gzipped tar created by TarHostPaths onto the remote host (/).
func UntarHostPaths(client *ssh.Client, archivePath string) error {
	archivePath = strings.TrimSpace(archivePath)
	if archivePath == "" {
		return fmt.Errorf("archive path required")
	}
	cmd := fmt.Sprintf(`test -f %s && tar xzf %s -C /`, shellQuote(archivePath), shellQuote(archivePath))
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("untar: %v %s", err, errOut)
	}
	return nil
}

// ListDockerImages returns image refs matching prefix (e.g. dockfin/{uuid}).
func ListDockerImages(client *ssh.Client, prefix string) ([]string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, fmt.Errorf("prefix required")
	}
	cmd := fmt.Sprintf(
		`docker images --format '{{.Repository}}:{{.Tag}}' %s 2>/dev/null | head -n 50`,
		shellQuote(prefix),
	)
	out, _, err := sshx.Run(client, cmd)
	if err != nil {
		return nil, err
	}
	var images []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, ":<none>") {
			continue
		}
		images = append(images, line)
	}
	return images, nil
}

// EnforceAppBackupRetention keeps the newest keepCount archives under
// /data/dockfin/backups/applications/{appID}/.
func EnforceAppBackupRetention(client *ssh.Client, appID string, keepCount int) error {
	if keepCount <= 0 || client == nil || appID == "" {
		return nil
	}
	dir := "/data/dockfin/backups/applications/" + appID
	cmd := fmt.Sprintf(
		`cd %s 2>/dev/null && ls -1t app-*.tar.gz 2>/dev/null | tail -n +%d | xargs -r rm -f`,
		shellQuote(dir), keepCount+1,
	)
	_, _, err := sshx.Run(client, cmd)
	return err
}

// FilterExistingPaths returns only remote paths that exist (file or directory).
func FilterExistingPaths(client *ssh.Client, paths []string) []string {
	var out []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, _, err := sshx.RunArgs(client, "test", "-e", p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// ServiceVolumeArchivePath returns the remote tar.gz path for a one-click service backup.
func ServiceVolumeArchivePath(serviceID, filename string) string {
	return "/data/dockfin/backups/services/" + serviceID + "/" + filename
}

// DefaultServiceBackupFilename builds a timestamped archive name.
func DefaultServiceBackupFilename(serviceID string) string {
	short := serviceID
	if len(short) > 8 {
		short = short[:8]
	}
	return fmt.Sprintf("svc-%s-%s.tar.gz", short, time.Now().UTC().Format("20060102-150405"))
}

// EnforceServiceBackupRetention keeps the newest keepCount archives under
// /data/dockfin/backups/services/{serviceID}/.
func EnforceServiceBackupRetention(client *ssh.Client, serviceID string, keepCount int) error {
	if keepCount <= 0 || client == nil || serviceID == "" {
		return nil
	}
	dir := "/data/dockfin/backups/services/" + serviceID
	cmd := fmt.Sprintf(
		`cd %s 2>/dev/null && ls -1t svc-*.tar.gz 2>/dev/null | tail -n +%d | xargs -r rm -f`,
		shellQuote(dir), keepCount+1,
	)
	_, _, err := sshx.Run(client, cmd)
	return err
}
