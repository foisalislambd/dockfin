package terminal

import (
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/ssh"
)

// Session wraps a remote interactive shell/exec for browser terminal bridging.
type Session struct {
	SSH     *ssh.Session
	Stdin   io.WriteCloser
	Stdout  io.Reader
	Stderr  io.Reader
	Created time.Time
}

func Start(client *ssh.Client, command string) (*Session, error) {
	sess, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm-256color", 40, 120, modes); err != nil {
		_ = sess.Close()
		return nil, fmt.Errorf("pty: %w", err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		_ = sess.Close()
		return nil, err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		_ = sess.Close()
		return nil, err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		_ = sess.Close()
		return nil, err
	}
	if command == "" {
		err = sess.Shell()
	} else {
		err = sess.Start(command)
	}
	if err != nil {
		_ = sess.Close()
		return nil, err
	}
	return &Session{SSH: sess, Stdin: stdin, Stdout: stdout, Stderr: stderr, Created: time.Now()}, nil
}

func (s *Session) Close() error {
	return s.SSH.Close()
}

// DockerExec builds a docker exec argv for container shells.
func DockerExec(container string) string {
	return fmt.Sprintf("docker exec -it %s sh", container)
}
