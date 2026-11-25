package worker

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"io"
	"net"
	"testing"

	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func TestHandleDeploysThroughStoredKey(t *testing.T) {
	addr, pemKey := startFakeSSH(t)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}
	st := testkit.OpenStore(t)
	ctx := t.Context()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "worker@example.com", "Worker", "hash")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := st.Box.EncryptString(string(pemKey))
	if err != nil {
		t.Fatal(err)
	}
	key, err := st.CreatePrivateKey(ctx, team.ID, "edge", "", "ssh-ed25519 AAAA", enc, "fp")
	if err != nil {
		t.Fatal(err)
	}
	server, err := st.CreateServer(ctx, team.ID, &key.ID, "edge", "", host, "root", port, "traefik")
	if err != nil {
		t.Fatal(err)
	}
	var destID uuid.UUID
	if err := st.Pool.QueryRow(ctx, `SELECT id FROM destinations WHERE server_id=$1 LIMIT 1`, server.ID).Scan(&destID); err != nil {
		t.Fatal(err)
	}
	_, env, err := st.CreateProject(ctx, team.ID, "work", "")
	if err != nil {
		t.Fatal(err)
	}
	app, err := st.CreateApplication(ctx, &store.Application{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "web", BuildPack: "dockerfile",
		Dockerfile: "FROM nginx:alpine\n", PortsExposes: "80",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE applications SET destination_id=$2 WHERE id=$1`, app.ID, destID); err != nil {
		t.Fatal(err)
	}
	dep, err := st.CreateDeployment(ctx, team.ID, app.ID, &server.ID, "", "", false, false, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	q := NewQueue(st, sshx.NewPool(), nil, 1)
	q.handle(ctx, DeployJob{DeploymentID: dep.ID, TeamID: team.ID})
	got, err := st.GetDeployment(ctx, team.ID, dep.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == "queued" || got.Status == "in_progress" {
		t.Fatalf("deployment stuck at %s %s", got.Status, got.ErrorMessage)
	}
	if got.Status != "finished" && got.Status != "failed" {
		t.Fatal(got.Status)
	}
}

func startFakeSSH(t *testing.T) (addr string, clientKey []byte) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	clientKey = pem.EncodeToMemory(block)
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if ssh.FingerprintSHA256(key) != ssh.FingerprintSHA256(signer.PublicKey()) {
			return nil, errRejected
		}
		return nil, nil
	}}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveFake(conn, cfg)
		}
	}()
	return ln.Addr().String(), clientKey
}

type rejectErr string

func (e rejectErr) Error() string { return string(e) }

const errRejected = rejectErr("rejected")

func serveFake(conn net.Conn, cfg *ssh.ServerConfig) {
	sc, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	defer sc.Close()
	go ssh.DiscardRequests(reqs)
	for ch := range chans {
		if ch.ChannelType() != "session" {
			_ = ch.Reject(ssh.UnknownChannelType, "no")
			continue
		}
		channel, requests, err := ch.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer channel.Close()
			for req := range requests {
				ok := req.Type == "exec" || req.Type == "shell" || req.Type == "pty-req"
				_ = req.Reply(ok, nil)
				if req.Type == "exec" || req.Type == "shell" {
					_, _ = io.Copy(io.Discard, channel)
					_, _ = channel.Write([]byte("ok\n"))
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
					return
				}
			}
		}()
	}
}
