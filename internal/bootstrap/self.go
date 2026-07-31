// Package bootstrap registers the install VPS as a deploy server (Coolify-style).
package bootstrap

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	DefaultSSHUser    = "root"
	DefaultSSHPort    = 22
	DefaultServerName = "localhost"
	keyFileName       = "id_ed25519"
)

// DetectPublicIP discovers the VPS public IPv4 via external services, then local interfaces.
func DetectPublicIP() string {
	client := &http.Client{Timeout: 5 * time.Second}
	for _, url := range []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	} {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "goolify-bootstrap/1.0")
		res, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(res.Body, 128))
		_ = res.Body.Close()
		if res.StatusCode != http.StatusOK || len(body) == 0 {
			continue
		}
		ip := strings.TrimSpace(string(body))
		if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() != nil && !parsed.IsLoopback() {
			return parsed.String()
		}
	}
	// Last resort: first non-loopback IPv4 from host interfaces
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip4 := ip.To4(); ip4 != nil && !ip4.IsLoopback() && !ip4.IsLinkLocalUnicast() {
					return ip4.String()
				}
			}
		}
	}
	return ""
}

// EnsureKeyPair loads or creates an ed25519 key under dataDir/ssh/.
func EnsureKeyPair(dataDir string) (privatePEM, publicAuthorized string, err error) {
	dir := filepath.Join(dataDir, "ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	privPath := filepath.Join(dir, keyFileName)
	pubPath := privPath + ".pub"

	if b, err := os.ReadFile(privPath); err == nil && len(b) > 0 {
		signer, err := ssh.ParsePrivateKey(b)
		if err != nil {
			return "", "", fmt.Errorf("parse existing bootstrap key: %w", err)
		}
		return string(b), string(ssh.MarshalAuthorizedKey(signer.PublicKey())), nil
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	sshPriv, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return "", "", err
	}
	// OpenSSH PEM private key
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return "", "", err
	}
	pemBytes := pem.EncodeToMemory(block)
	pub := string(ssh.MarshalAuthorizedKey(sshPriv.PublicKey()))

	if err := os.WriteFile(privPath, pemBytes, 0o600); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(pubPath, []byte(pub), 0o644); err != nil {
		return "", "", err
	}
	return string(pemBytes), pub, nil
}

// AuthorizePublicKey appends the key to the SSH user's authorized_keys if missing.
func AuthorizePublicKey(sshUser, publicAuthorized string) error {
	publicAuthorized = strings.TrimSpace(publicAuthorized)
	if publicAuthorized == "" {
		return fmt.Errorf("empty public key")
	}
	home, err := homeDirFor(sshUser)
	if err != nil {
		return err
	}
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	authPath := filepath.Join(sshDir, "authorized_keys")
	existing, _ := os.ReadFile(authPath)
	line := strings.TrimSpace(publicAuthorized)
	for _, l := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(l) == line {
			return nil
		}
	}
	f, err := os.OpenFile(authPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(line + "\n")
	return err
}

func homeDirFor(sshUser string) (string, error) {
	if sshUser == "" || sshUser == "root" {
		if u, err := user.Lookup("root"); err == nil && u.HomeDir != "" {
			return u.HomeDir, nil
		}
		return "/root", nil
	}
	u, err := user.Lookup(sshUser)
	if err != nil {
		return "", fmt.Errorf("lookup user %q: %w", sshUser, err)
	}
	return u.HomeDir, nil
}

// ResolvePublicIP returns override if valid, otherwise DetectPublicIP.
func ResolvePublicIP(override string) string {
	override = strings.TrimSpace(override)
	if override != "" {
		if parsed := net.ParseIP(override); parsed != nil && !parsed.IsLoopback() && !parsed.IsUnspecified() {
			return parsed.String()
		}
	}
	return DetectPublicIP()
}
