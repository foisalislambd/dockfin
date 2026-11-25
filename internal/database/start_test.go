package database

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func TestStartStopAndRemoveUseTheRemote(t *testing.T) {
	client := dialFake(t)
	port := 5432
	db := &store.Database{
		ID: uuid.New(), Engine: "postgresql", Image: "postgres:16-alpine",
		IsPublic: true, PublicPort: &port,
	}
	if err := Start(client, db, "dockfin", "s3cret", []string{"-e", "EXTRA=1"}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	env := engineEnv("postgresql", "s3cret")
	if len(env) < 4 || env[1] != "POSTGRES_PASSWORD=s3cret" {
		t.Fatal(env)
	}
	if engineEnv("redis", "s3cret") != nil {
		t.Fatal("redis env")
	}
	db.EnableSSL = true
	if err := Start(client, db, "dockfin", "s3cret", nil, nil, nil, nil); err == nil {
		t.Fatal("ssl without material")
	}
	db.Engine = "sqlite"
	db.IsPublic = true
	if err := Start(client, db, "dockfin", "s3cret", nil, nil, nil, nil); err == nil {
		t.Fatal("public sqlite")
	}
	for _, engine := range []string{"mysql", "mongodb", "redis", "clickhouse"} {
		other := *db
		other.Engine = engine
		other.EnableSSL = false
		other.IsPublic = engine != "sqlite"
		if err := Start(client, &other, "dockfin", "s3cret", nil, nil, nil, nil); err != nil {
			t.Fatal(engine, err)
		}
	}
	if err := Stop(client, db.ID.String()); err != nil {
		t.Fatal(err)
	}
	if err := RemoveData(client, db.ID.String()); err != nil {
		t.Fatal(err)
	}
}

func dialFake(t *testing.T) *ssh.Client {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
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
			go func() {
				sc, chans, reqs, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				defer sc.Close()
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					channel, requests, err := ch.Accept()
					if err != nil {
						continue
					}
					go func() {
						defer channel.Close()
						for req := range requests {
							_ = req.Reply(req.Type == "exec" || req.Type == "shell", nil)
							if req.Type == "exec" || req.Type == "shell" {
								var msg struct{ Command string }
								_ = ssh.Unmarshal(req.Payload, &msg)
								target := "/data/dockfin"
								for _, field := range strings.Fields(msg.Command) {
									if strings.HasPrefix(field, "/data/") {
										target = field
									}
								}
								_, _ = io.Copy(io.Discard, channel)
								if strings.Contains(msg.Command, "readlink -f") {
									_, _ = channel.Write([]byte("RESOLVED " + target + "\nKIND dir\n"))
								}
								_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
								return
							}
						}
					}()
				}
			}()
		}
	}()
	client, err := ssh.Dial("tcp", ln.Addr().String(), &ssh.ClientConfig{
		User: "root", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
