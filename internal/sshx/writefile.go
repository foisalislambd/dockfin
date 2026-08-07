package sshx

import (
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	"golang.org/x/crypto/ssh"
)

// ValidRemotePath reports whether p is a safe absolute path for remote file writes.
func ValidRemotePath(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" || !strings.HasPrefix(p, "/") {
		return false
	}
	if strings.Contains(p, "\x00") || strings.ContainsAny(p, "\n\r") {
		return false
	}
	if strings.Contains(p, "..") {
		return false
	}
	cleaned := path.Clean(p)
	if cleaned != p {
		return false
	}
	for _, r := range p {
		if r > 127 {
			return false
		}
	}
	return true
}

// WriteFile writes data to an absolute remote path without shell heredocs.
// Content is base64-encoded so user-controlled payloads cannot break out of delimiters.
func WriteFile(client *ssh.Client, remotePath string, data []byte) error {
	if client == nil {
		return fmt.Errorf("nil ssh client")
	}
	if !ValidRemotePath(remotePath) {
		return fmt.Errorf("invalid remote path")
	}
	enc := base64.StdEncoding.EncodeToString(data)
	cmd := fmt.Sprintf("printf '%%s' %s | base64 -d > %s", shellQuote(enc), shellQuote(remotePath))
	_, errOut, err := Run(client, cmd)
	if err != nil {
		return fmt.Errorf("write remote file: %v %s", err, errOut)
	}
	return nil
}
