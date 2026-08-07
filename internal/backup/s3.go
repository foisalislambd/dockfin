package backup

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// S3Creds holds decrypted S3 connection details.
type S3Creds struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	PathStyle bool
}

// UploadRemoteToS3 uploads a dump already on the remote host to S3 via minio/mc.
func UploadRemoteToS3(client *ssh.Client, remotePath, objectKey string, c S3Creds) error {
	if c.Endpoint == "" || c.Bucket == "" || c.AccessKey == "" || c.SecretKey == "" {
		return fmt.Errorf("incomplete s3 credentials")
	}
	endpoint := strings.TrimRight(c.Endpoint, "/")
	scheme := "https"
	rest := endpoint
	if strings.HasPrefix(endpoint, "http://") {
		scheme = "http"
		rest = strings.TrimPrefix(endpoint, "http://")
	} else if strings.HasPrefix(endpoint, "https://") {
		rest = strings.TrimPrefix(endpoint, "https://")
	}
	mcHost := fmt.Sprintf("%s://%s:%s@%s", scheme, url.PathEscape(c.AccessKey), url.PathEscape(c.SecretKey), rest)
	// remotePath is under /data/dockfin/backups/…; the container mounts that dir at /backups.
	rel := strings.TrimPrefix(remotePath, "/data/dockfin/backups/")
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || rel == remotePath {
		rel = path.Base(remotePath)
	}
	cmd := fmt.Sprintf(
		`docker run --rm -v /data/dockfin/backups:/backups:ro -e MC_HOST_dockfin=%s minio/mc:RELEASE.2024-11-17T19-35-25Z cp /backups/%s dockfin/%s/%s`,
		shellQuote(mcHost), shellQuote(rel), shellQuote(c.Bucket), shellQuote(objectKey),
	)
	_, errOut, err := sshx.Run(client, cmd)
	if err != nil {
		return fmt.Errorf("s3 upload: %v %s", err, errOut)
	}
	return nil
}
