package sshx

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type Target struct {
	Host                string
	Port                int
	User                string
	PrivateKey          []byte
	ExpectedFingerprint string // empty = TOFU (trust on first use)
	ExpectedKeyType     string
}

type DialResult struct {
	Client      *ssh.Client
	Fingerprint string
	KeyType     string
	IsNewHost   bool
}

type Pool struct {
	mu    sync.Mutex
	conns map[string]*ssh.Client
}

func NewPool() *Pool {
	return &Pool{conns: make(map[string]*ssh.Client)}
}

func (p *Pool) key(t Target) string {
	return fmt.Sprintf("%s@%s:%d", t.User, t.Host, t.Port)
}

func FingerprintSHA256(key ssh.PublicKey) string {
	sum := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

func (p *Pool) Dial(t Target) (*DialResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	k := p.key(t)
	if c, ok := p.conns[k]; ok {
		_, _, err := c.SendRequest("goolify-keepalive", true, nil)
		if err == nil {
			return &DialResult{Client: c, Fingerprint: t.ExpectedFingerprint, KeyType: t.ExpectedKeyType}, nil
		}
		_ = c.Close()
		delete(p.conns, k)
	}
	signer, err := ssh.ParsePrivateKey(t.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	var gotKey ssh.PublicKey
	var isNew bool
	callback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		gotKey = key
		fp := FingerprintSHA256(key)
		if t.ExpectedFingerprint == "" {
			isNew = true
			return nil // TOFU
		}
		if fp != t.ExpectedFingerprint {
			return fmt.Errorf("host key mismatch for %s: got %s want %s (possible MITM)", hostname, fp, t.ExpectedFingerprint)
		}
		return nil
	}

	cfg := &ssh.ClientConfig{
		User:            t.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: callback,
		Timeout:         15 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", t.Host, t.Port)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	p.conns[k] = client
	fp := ""
	kt := ""
	if gotKey != nil {
		fp = FingerprintSHA256(gotKey)
		kt = gotKey.Type()
	}
	return &DialResult{Client: client, Fingerprint: fp, KeyType: kt, IsNewHost: isNew}, nil
}

// DialClient is a convenience wrapper returning only the client.
func (p *Pool) DialClient(t Target) (*ssh.Client, error) {
	res, err := p.Dial(t)
	if err != nil {
		return nil, err
	}
	return res.Client, nil
}

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, c := range p.conns {
		_ = c.Close()
		delete(p.conns, k)
	}
}

func Run(client *ssh.Client, command string) (stdout, stderr string, err error) {
	session, err := client.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()
	var outBuf, errBuf bytes.Buffer
	session.Stdout = &outBuf
	session.Stderr = &errBuf
	err = session.Run(command)
	return outBuf.String(), errBuf.String(), err
}

// LineFn receives each stdout/stderr line as it arrives.
type LineFn func(line string)

// RunStreaming runs a remote command and streams combined stdout/stderr lines to onLine.
func RunStreaming(client *ssh.Client, command string, onLine LineFn) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return err
	}

	if err := session.Start(command); err != nil {
		return err
	}

	var wg sync.WaitGroup
	scan := func(r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			if onLine != nil {
				onLine(sc.Text())
			}
		}
	}
	wg.Add(2)
	go scan(stdout)
	go scan(stderr)
	wg.Wait()
	return session.Wait()
}

func RunArgs(client *ssh.Client, argv ...string) (stdout, stderr string, err error) {
	if len(argv) == 0 {
		return "", "", fmt.Errorf("empty argv")
	}
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellQuote(a)
	}
	return Run(client, strings.Join(parts, " "))
}

// RunArgsStreaming is RunArgs with live line streaming.
func RunArgsStreaming(client *ssh.Client, onLine LineFn, argv ...string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty argv")
	}
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellQuote(a)
	}
	return RunStreaming(client, strings.Join(parts, " "), onLine)
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return !(r == '-' || r == '_' || r == '.' || r == '/' || r == ':' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func ValidateDocker(client *ssh.Client) (version string, err error) {
	out, errOut, err := RunArgs(client, "docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return "", fmt.Errorf("docker not available: %v %s", err, strings.TrimSpace(errOut))
	}
	return strings.TrimSpace(out), nil
}

func EnsureNetwork(client *ssh.Client, name string) error {
	_, _, err := RunArgs(client, "docker", "network", "inspect", name)
	if err == nil {
		return nil
	}
	_, errOut, err := RunArgs(client, "docker", "network", "create", name)
	if err != nil {
		return fmt.Errorf("create network %s: %v %s", name, err, errOut)
	}
	return nil
}

func EnsureDataDirs(client *ssh.Client) error {
	_, errOut, err := RunArgs(client, "mkdir", "-p",
		"/data/goolify/applications",
		"/data/goolify/databases",
		"/data/goolify/services",
		"/data/goolify/backups",
		"/data/goolify/proxy",
	)
	if err != nil {
		return fmt.Errorf("mkdir data dirs: %v %s", err, errOut)
	}
	return nil
}

func TCPReachable(host string, port int, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return err
	}
	return conn.Close()
}
