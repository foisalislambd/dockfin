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
// Content is base64-encoded on stdin so large payloads do not hit ARG_MAX
// (the previous printf-in-command form broke multi‑MB backup imports).
func WriteFile(client *ssh.Client, remotePath string, data []byte) error {
	if client == nil {
		return fmt.Errorf("nil ssh client")
	}
	if !ValidRemotePath(remotePath) {
		return fmt.Errorf("invalid remote path")
	}
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}
	var errBuf strings.Builder
	session.Stderr = &errBuf

	cmd := fmt.Sprintf("base64 -d > %s", shellQuote(remotePath))
	if err := session.Start(cmd); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("write remote file: %v", err)
	}
	enc := base64.NewEncoder(base64.StdEncoding, stdin)
	if _, err := enc.Write(data); err != nil {
		_ = stdin.Close()
		_ = session.Wait()
		return fmt.Errorf("write remote file: %v %s", err, errBuf.String())
	}
	if err := enc.Close(); err != nil {
		_ = stdin.Close()
		_ = session.Wait()
		return fmt.Errorf("write remote file: %v %s", err, errBuf.String())
	}
	if err := stdin.Close(); err != nil {
		_ = session.Wait()
		return fmt.Errorf("write remote file: %v %s", err, errBuf.String())
	}
	if err := session.Wait(); err != nil {
		return fmt.Errorf("write remote file: %v %s", err, errBuf.String())
	}
	return nil
}
