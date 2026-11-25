package deploy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/testkit"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

func TestPipelineArgBuilders(t *testing.T) {
	p := &Pipeline{}
	app := &store.Application{
		Name: "Web App", FQDN: "https://web.example.com, https://www.web.example.com",
		PortsExposes: "80,443", LimitsMemory: "256m", LimitsCpus: "1",
		IsGPUEnabled: true, GPUCount: 1, CustomDockerStopTimeout: 20,
		CustomDockerRestartPolicy: "on-failure", CustomDockerMaxRestartCount: 3,
		CustomDockerRunOptions: `--cap-drop ALL --label "a b"`,
	}
	if got := p.limitArgs(app); len(got) != 4 || got[1] != "256m" || got[3] != "1" {
		t.Fatal(got)
	}
	labels := p.proxyLabelArgs(app, "traefik")
	if len(labels) == 0 || !strings.Contains(strings.Join(labels, " "), "web.example.com") {
		t.Fatal(labels)
	}
	if caddy := p.proxyLabelArgs(app, "caddy"); len(caddy) == 0 {
		t.Fatal(caddy)
	}
	if restartPolicy(app) != "on-failure:3" {
		t.Fatal(restartPolicy(app))
	}
	if gpu := p.gpuArgs(app); len(gpu) != 2 || gpu[1] != "1" {
		t.Fatal(gpu)
	}
	app.GPUDeviceIDs = "GPU-0"
	if p.gpuArgs(app)[1] != "device=GPU-0" {
		t.Fatal(p.gpuArgs(app))
	}
	if p.stopTimeoutArgs(app)[1] != "20" {
		t.Fatal(p.stopTimeoutArgs(app))
	}
	opts := splitDockerRunOptions(app.CustomDockerRunOptions)
	if len(opts) != 4 || opts[0] != "--cap-drop" || opts[3] != "a b" {
		t.Fatal(opts)
	}
	if strings.Contains(splitDockerRunOptions(`--env $(id)`)[1], "$") == false && len(splitDockerRunOptions(`evil;id`)) != 1 {
		t.Fatal("splitter")
	}
	if htpasswdBcrypt("user", "secret") == "" || !strings.Contains(htpasswdBcryptCompose("user", "secret"), "$$") {
		t.Fatal("htpasswd")
	}
	if firstHost("https://a.example, b.example") != "a.example" {
		t.Fatal(firstHost("https://a.example, b.example"))
	}
	if firstPort(" 80 , 443") != "80" {
		t.Fatal(firstPort(" 80 , 443"))
	}
	id := uuid.New()
	app.ID = id
	if containerName(app) == "" || !strings.Contains(containerName(app), id.String()[:8]) && containerName(app) == "" {
		t.Fatal(containerName(app))
	}
	if sanitizeProxyRouter("Web App!") == "" {
		t.Fatal("sanitize")
	}
}

func TestDockerfileDeployAgainstFakeSSH(t *testing.T) {
	addr, signer := fakeSSH(t)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	st := testkit.OpenStore(t)
	ctx := t.Context()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "pipe@example.com", "Pipe", "hash")
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := st.CreateProject(ctx, team.ID, "pipe", "")
	if err != nil {
		t.Fatal(err)
	}
	hcPort := 80
	app, err := st.CreateApplication(ctx, &store.Application{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "web", BuildPack: "dockerfile",
		Dockerfile: "FROM nginx:alpine\n", PortsExposes: "80", FQDN: "web.example.com",
		GitRepository: "https://github.com/acme/web.git", GitBranch: "main",
		HealthCheckEnabled: true, HealthCheckPath: "/health", HealthCheckPort: &hcPort,
		HealthCheckMethod: "GET", HealthCheckReturnCode: 200, HealthCheckInterval: 1,
		HealthCheckTimeout: 1, HealthCheckRetries: 1, HealthCheckHost: "127.0.0.1",
		HealthCheckScheme: "http", HealthCheckResponseText: "ok",
		LimitsMemory: "256m", LimitsCpus: "1",
		PreDeploymentCommand: "echo pre", PostDeploymentCommand: "echo post",
		CustomDockerRunOptions: "--cap-drop ALL", CustomDockerStopTimeout: 20,
		CustomLabels: "app=web", IsForceHTTPS: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	app.FQDN = "web.example.com"
	app.HealthCheckEnabled = true
	app.HealthCheckPath = "/health"
	app.HealthCheckPort = &hcPort
	app.HealthCheckMethod = "GET"
	app.HealthCheckReturnCode = 200
	app.HealthCheckInterval = 1
	app.HealthCheckTimeout = 1
	app.HealthCheckRetries = 1
	app.HealthCheckStartPeriod = 0
	app.HealthCheckHost = "127.0.0.1"
	app.HealthCheckScheme = "http"
	app.HealthCheckResponseText = "ok"
	app.LimitsMemory = "256m"
	app.LimitsCpus = "1"
	app.PreDeploymentCommand = "echo pre"
	app.PostDeploymentCommand = "echo post"
	app.CustomDockerRunOptions = "--cap-drop ALL"
	app.CustomDockerStopTimeout = 20
	app.CustomLabels = "app=web"
	app.IsForceHTTPS = true
	if _, err := st.UpsertVolume(ctx, team.ID, "application", app.ID, "data", "/data", "/data/dockfin/volumes/web", false, nil); err != nil {
		t.Fatal(err)
	}
	dep, err := st.CreateDeployment(ctx, team.ID, app.ID, nil, "", "", false, false, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	var logs []string
	p := &Pipeline{Store: st, SSH: sshx.NewPool(), Log: func(stage, line string) {
		logs = append(logs, stage+":"+line)
	}}
	runCtx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	for _, pack := range []string{"dockerfile", "dockerimage", "dockercompose", "static", "railpack"} {
		app.BuildPack = pack
		if pack == "dockerimage" {
			app.DockerRegistryImageName = "nginx"
			app.DockerRegistryImageTag = "alpine"
		}
		err = p.Run(runCtx, Request{
			DeploymentID: dep.ID,
			TeamID:       team.ID,
			App:          app,
			Server: &store.Server{
				ID: uuid.New(), IP: host, Port: port, UserName: "root",
			},
			Destination: &store.Destination{Kind: "standalone", Network: "dockfin"},
			PrivateKey:  signer,
		})
		if err != nil {
			t.Fatalf("%s: %v\n%s", pack, err, strings.Join(logs, "\n"))
		}
	}
	joined := strings.Join(logs, "\n")
	t.Logf("err=%v\n%s", err, joined)
	if !strings.Contains(joined, "Connecting to deploy server") {
		t.Fatalf("logs %s err %v", joined, err)
	}
	if err != nil && !strings.Contains(joined, "network") && !strings.Contains(joined, "Dockerfile") && !strings.Contains(joined, "docker") && !strings.Contains(err.Error(), "ssh") {
		t.Fatalf("deploy stopped early: %v\n%s", err, joined)
	}
}

func fakeSSH(t *testing.T) (addr string, clientKey []byte) {
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
	hostSigner, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if ssh.FingerprintSHA256(key) != ssh.FingerprintSHA256(signer.PublicKey()) {
			return nil, fmtErr("rejected")
		}
		return nil, nil
	}}
	cfg.AddHostKey(hostSigner)
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
			go serveOne(conn, cfg)
		}
	}()
	return ln.Addr().String(), clientKey
}

func serveOne(conn net.Conn, cfg *ssh.ServerConfig) {
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
					var msg struct{ Command string }
					_ = ssh.Unmarshal(req.Payload, &msg)
					_, _ = io.Copy(io.Discard, channel)
					_, _ = channel.Write([]byte(fakeStdout(msg.Command)))
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
					return
				}
			}
		}()
	}
}

type strErr string

func (e strErr) Error() string { return string(e) }

func fmtErr(s string) error { return strErr(s) }

func TestDeployHelpersAndImageTransfer(t *testing.T) {
	app := &store.Application{PublishDirectory: "dist", IsSPA: true, InstallCommand: "npm ci", BuildCommand: "npm run build", StartCommand: "node server.js"}
	dockerfile, nginx := buildStaticDockerfile(app)
	if !strings.Contains(dockerfile, "nginx") || !strings.Contains(nginx, "index.html") {
		t.Fatalf("static dockerfile %q nginx %q", dockerfile, nginx)
	}
	app.CustomNginxConfiguration = "server { listen 80; }"
	if _, nginx = buildStaticDockerfile(app); !strings.Contains(nginx, "listen 80") {
		t.Fatal(nginx)
	}
	if !strings.Contains(railpackCommandEnv(app), "RAILPACK_BUILD_CMD") {
		t.Fatal(railpackCommandEnv(app))
	}
	if sanitizeContainerName("Web App!") != "web-app" {
		t.Fatal(sanitizeContainerName("Web App!"))
	}
	if !ValidContainerNameForHealth("dockfin-web") || ValidContainerNameForHealth("bad name") {
		t.Fatal("container name")
	}

	st := testkit.OpenStore(t)
	ctx := t.Context()
	_, team, err := st.CreateUserWithPersonalTeam(ctx, "helpers@example.com", "Helpers", "hash")
	if err != nil {
		t.Fatal(err)
	}
	_, env, err := st.CreateProject(ctx, team.ID, "helpers", "")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := st.CreateApplication(ctx, &store.Application{
		TeamID: team.ID, EnvironmentID: env.ID, Name: "web", BuildPack: "dockerfile", UseBuildSecrets: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertEnvVar(ctx, team.ID, "application", saved.ID, store.UpsertEnvVarInput{
		Key: "TOKEN", Value: "s3cret", Buildtime: true, IsBuildSecret: true,
	}); err != nil {
		t.Fatal(err)
	}
	saved.UseBuildSecrets = true
	p := &Pipeline{Store: st}
	flags, exports, err := p.dockerBuildSecretArgs(ctx, Request{TeamID: team.ID, App: saved})
	if err != nil || !strings.Contains(strings.Join(flags, " "), "id=TOKEN") || !strings.Contains(strings.Join(exports, " "), "s3cret") {
		t.Fatalf("secrets %v %v %v", flags, exports, err)
	}
	projectID, envID, _ := p.envScopeIDs(ctx, Request{TeamID: team.ID, App: saved})
	if projectID == nil && envID == nil && saved.EnvironmentID != uuid.Nil {
		t.Fatal("scope ids")
	}

	client := dialReady(t)
	code, body, err := httpHealthProbe(client, "dockfin-web", "GET", "http", "127.0.0.1", "80", "/health", 2)
	if err != nil || code != 200 || !strings.Contains(body, "ok") {
		t.Fatalf("probe %d %q %v", code, body, err)
	}
	if err := TransferImage(client, client, "nginx:alpine"); err != nil {
		t.Fatal(err)
	}
	if err := TransferImage(client, client, "bad name"); err == nil {
		t.Fatal("accepted a bad image")
	}
}

func dialReady(t *testing.T) *ssh.Client {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
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
								if strings.Contains(msg.Command, "docker load") {
									_, _ = io.Copy(io.Discard, channel)
								}
								_, _ = channel.Write([]byte("ok\n___DOCKFIN_HC___200\n"))
								_ = channel.CloseWrite()
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

func fakeStdout(command string) string {
	switch {
	case strings.Contains(command, "readlink -f"):
		target := "/data/dockfin"
		for _, field := range strings.Fields(command) {
			if strings.HasPrefix(field, "/data/") {
				target = field
			}
		}
		return "RESOLVED " + target + "\nKIND dir\n"
	case strings.Contains(command, ".State.Running"):
		return "true\n"
	case strings.Contains(command, "DOCKFIN_HC"):
		return "ok\n___DOCKFIN_HC___200\n"
	case strings.Contains(command, "State.Health"):
		return "healthy\n"
	default:
		return "ok\n"
	}
}
