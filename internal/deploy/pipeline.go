package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/proxy"
	"github.com/goolify/goolify/internal/sshx"
	"github.com/goolify/goolify/internal/store"
	"golang.org/x/crypto/ssh"
)

type LogFn func(stage, line string)

type Pipeline struct {
	Store *store.Store
	SSH   *sshx.Pool
	Log   LogFn
}

type Request struct {
	DeploymentID    uuid.UUID
	TeamID          uuid.UUID
	App             *store.Application
	Server          *store.Server
	Destination     *store.Destination
	PrivateKey      []byte
	BuildServer     *store.Server
	BuildPrivateKey []byte
	ForceRebuild    bool
}

func (p *Pipeline) log(stage, line string) {
	if p.Log != nil {
		p.Log(stage, line)
	}
}

// repairAppFQDN assigns or replaces free domains that embed loopback magic IPs
// (same rules as one-click service deploy).
func (p *Pipeline) repairAppFQDN(ctx context.Context, req *Request) {
	if req == nil || req.App == nil || req.Server == nil {
		return
	}
	needs := req.App.FQDN == "" || proxy.FQDNUsesUnusableMagicIP(req.App.FQDN)
	if !needs {
		return
	}
	magicIP := proxy.PreferMagicIP(req.Server.IP, req.Server.PublicIP)
	fqdn := proxy.GenerateFQDN(req.App.Name, req.App.ID, magicIP, req.Server.WildcardDomain, req.Server.MagicDomain)
	if fqdn == "" || fqdn == req.App.FQDN {
		if proxy.FQDNUsesUnusableMagicIP(req.App.FQDN) {
			p.log("prepare", "Warning: server has no public IP — app domain still points at localhost")
		}
		return
	}
	if req.App.FQDN == "" {
		p.log("prepare", "Assigned free domain "+fqdn)
	} else {
		p.log("prepare", fmt.Sprintf("Updating domain %s → %s", req.App.FQDN, fqdn))
	}
	req.App.FQDN = fqdn
	if p.Store != nil {
		_ = p.Store.UpdateApplication(ctx, req.App)
	}
}

func (p *Pipeline) Run(ctx context.Context, req Request) error {
	p.repairAppFQDN(ctx, &req)

	p.log("prepare", "Connecting to deploy server via SSH…")
	deployClient, err := p.dialServer(ctx, req.Server, req.PrivateKey)
	if err != nil {
		return err
	}

	buildClient := deployClient
	if req.BuildServer != nil && req.BuildServer.ID != req.Server.ID {
		p.log("prepare", "Connecting to build server via SSH…")
		key := req.BuildPrivateKey
		if len(key) == 0 {
			key = req.PrivateKey
		}
		buildClient, err = p.dialServer(ctx, req.BuildServer, key)
		if err != nil {
			return fmt.Errorf("build server ssh: %w", err)
		}
		p.log("prepare", "Using dedicated build server "+req.BuildServer.Name)
	}

	p.log("prepare", "Ensuring data directories")
	_ = sshx.EnsureDataDirs(deployClient)
	if buildClient != deployClient {
		_ = sshx.EnsureDataDirs(buildClient)
	}

	p.log("prepare", fmt.Sprintf("Ensuring Docker network %q", req.Destination.Network))
	if err := p.ensureDestinationNetwork(deployClient, req.Destination); err != nil {
		return err
	}

	switch req.App.BuildPack {
	case "dockerimage":
		return p.deployImage(ctx, deployClient, req)
	case "dockerfile":
		return p.deployDockerfile(ctx, buildClient, deployClient, req)
	case "dockercompose":
		return p.deployCompose(ctx, deployClient, req)
	case "static":
		return p.deployStatic(ctx, buildClient, deployClient, req)
	case "nixpacks", "railpack":
		return p.deployNixpacks(ctx, buildClient, deployClient, req)
	default:
		return fmt.Errorf("unsupported build pack: %s", req.App.BuildPack)
	}
}

func (p *Pipeline) dialServer(ctx context.Context, srv *store.Server, privKey []byte) (*ssh.Client, error) {
	res, err := p.SSH.Dial(sshx.Target{
		Host:                srv.IP,
		Port:                srv.Port,
		User:                srv.UserName,
		PrivateKey:          privKey,
		ExpectedFingerprint: srv.HostKeyFingerprint,
		ExpectedKeyType:     srv.HostKeyType,
	})
	if err != nil {
		return nil, fmt.Errorf("ssh: %w", err)
	}
	if res.IsNewHost && p.Store != nil {
		_ = p.Store.UpdateServerHostKey(ctx, srv.ID, res.Fingerprint, res.KeyType)
		p.log("prepare", "Trusted new host key "+res.Fingerprint)
	}
	return res.Client, nil
}

func (p *Pipeline) ensureDestinationNetwork(client *ssh.Client, dest *store.Destination) error {
	if dest.Kind == "swarm" {
		_, _, err := sshx.RunArgs(client, "docker", "network", "inspect", dest.Network)
		if err == nil {
			return nil
		}
		_, errOut, err := sshx.RunArgs(client, "docker", "network", "create", "-d", "overlay", "--attachable", dest.Network)
		if err != nil {
			return fmt.Errorf("create overlay network %s: %v %s", dest.Network, err, errOut)
		}
		return nil
	}
	return sshx.EnsureNetwork(client, dest.Network)
}

func (p *Pipeline) transferIfNeeded(buildClient, deployClient *ssh.Client, req Request, imageTag string) error {
	if req.BuildServer == nil || req.BuildServer.ID == req.Server.ID {
		return nil
	}
	p.log("transfer", "Transferring image to deploy server…")
	if err := TransferImage(buildClient, deployClient, imageTag); err != nil {
		return fmt.Errorf("image transfer: %w", err)
	}
	p.log("transfer", "Image transfer complete")
	return nil
}

func (p *Pipeline) runtimeEnvArgs(ctx context.Context, req Request) []string {
	if p.Store == nil {
		return nil
	}
	var projectID, envID, serverID *uuid.UUID
	envID = &req.App.EnvironmentID
	serverID = &req.Destination.ServerID
	if env, err := p.Store.GetEnvironment(ctx, req.TeamID, req.App.EnvironmentID); err == nil {
		projectID = &env.ProjectID
	}
	m, err := p.Store.ResolvedEnvMap(ctx, req.TeamID, "application", req.App.ID, projectID, envID, serverID)
	if err != nil || len(m) == 0 {
		return nil
	}
	args := make([]string, 0, len(m)*2)
	for k, v := range m {
		args = append(args, "-e", k+"="+v)
	}
	return args
}

func (p *Pipeline) limitArgs(app *store.Application) []string {
	var args []string
	if mem := strings.TrimSpace(app.LimitsMemory); mem != "" && mem != "0" {
		args = append(args, "--memory", mem)
	}
	if cpus := strings.TrimSpace(app.LimitsCpus); cpus != "" && cpus != "0" {
		args = append(args, "--cpus", cpus)
	}
	return args
}

func (p *Pipeline) proxyLabelArgs(app *store.Application, proxyType string) []string {
	if app.FQDN == "" {
		return nil
	}
	host := firstHost(app.FQDN)
	port := firstPort(app.PortsExposes)
	var labels []string
	switch strings.ToLower(proxyType) {
	case "caddy":
		labels = proxy.CaddyLabels(app.Name, host, port, app.IsForceHTTPS)
	case "none":
		return nil
	default:
		// Magic/custom domains terminate TLS at Traefik (Let's Encrypt).
		labels = proxy.TraefikLabelsHTTPS(app.Name, host, port, true)
	}
	var args []string
	for _, l := range labels {
		args = append(args, "-l", l)
	}
	return args
}

func (p *Pipeline) waitHealthy(client *ssh.Client, name string, app *store.Application) error {
	if !app.HealthCheckEnabled {
		return nil
	}
	retries := app.HealthCheckRetries
	if retries <= 0 {
		retries = 10
	}
	interval := app.HealthCheckInterval
	if interval <= 0 {
		interval = 5
	}
	p.log("healthcheck", fmt.Sprintf("Waiting for container %s to be running (%d retries)", name, retries))
	for i := 0; i < retries; i++ {
		out, _, err := sshx.RunArgs(client, "docker", "inspect", "-f", "{{.State.Running}}", name)
		if err == nil && strings.TrimSpace(out) == "true" {
			// Optional: check health status if HEALTHCHECK is defined
			health, _, hErr := sshx.RunArgs(client, "docker", "inspect", "-f", "{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}", name)
			status := strings.TrimSpace(health)
			if hErr == nil && (status == "none" || status == "healthy" || status == "") {
				p.log("healthcheck", "Container is running")
				return nil
			}
			if status == "healthy" {
				p.log("healthcheck", "Container is healthy")
				return nil
			}
			if status == "none" || status == "" {
				return nil
			}
			p.log("healthcheck", "health="+status)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	return fmt.Errorf("health check failed: container %s not healthy after %d attempts", name, retries)
}

func (p *Pipeline) deployImage(ctx context.Context, client *ssh.Client, req Request) error {
	image := req.App.DockerRegistryImageName
	if image == "" {
		return fmt.Errorf("docker registry image name is required")
	}
	tag := req.App.DockerRegistryImageTag
	if tag == "" {
		tag = "latest"
	}
	full := image + ":" + tag
	name := containerName(req.App)

	p.log("fetch", "Pulling image "+full)
	_, errOut, err := sshx.RunArgs(client, "docker", "pull", full)
	if err != nil {
		return fmt.Errorf("docker pull: %v %s", err, errOut)
	}

	p.log("run", "Stopping previous container if any")
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", name)

	args := []string{
		"docker", "run", "-d",
		"--name", name,
		"--restart", "unless-stopped",
		"--network", req.Destination.Network,
	}
	args = append(args, p.limitArgs(req.App)...)
	args = append(args, p.runtimeEnvArgs(ctx, req)...)
	args = append(args, p.proxyLabelArgs(req.App, req.Server.ProxyType)...)
	args = append(args, full)

	p.log("run", "Starting container "+name)
	_, errOut, err = sshx.RunArgs(client, args...)
	if err != nil {
		return fmt.Errorf("docker run: %v %s", err, errOut)
	}
	if err := p.waitHealthy(client, name, req.App); err != nil {
		return err
	}
	p.log("finalize", "Deployment finished")
	if p.Store != nil {
		_ = p.Store.UpdateApplicationStatus(context.Background(), req.App.ID, "running")
	}
	return nil
}

func (p *Pipeline) deployDockerfile(ctx context.Context, buildClient, deployClient *ssh.Client, req Request) error {
	if req.App.GitRepository == "" {
		return fmt.Errorf("git repository is required for %s builds", req.App.BuildPack)
	}
	name := containerName(req.App)
	workdir := "/data/goolify/applications/" + req.App.ID.String()
	imageTag := "goolify/" + req.App.ID.String() + ":latest"

	p.log("prepare", "Preparing remote workdir "+workdir)
	_, errOut, err := sshx.RunArgs(buildClient, "mkdir", "-p", workdir)
	if err != nil {
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}

	if err := p.gitClone(ctx, buildClient, req, workdir+"/src"); err != nil {
		return err
	}

	dockerfile := req.App.DockerfileLocation
	if dockerfile == "" {
		dockerfile = "/Dockerfile"
	}
	dockerfile = strings.TrimPrefix(dockerfile, "/")

	p.log("build", "Building image "+imageTag)
	buildArgs := []string{"docker", "build", "-t", imageTag, "-f", workdir + "/src/" + dockerfile}
	if req.ForceRebuild {
		buildArgs = append(buildArgs, "--no-cache")
	}
	buildArgs = append(buildArgs, workdir+"/src")
	_, errOut, err = sshx.RunArgs(buildClient, buildArgs...)
	if err != nil {
		return fmt.Errorf("docker build: %v %s", err, errOut)
	}

	if err := p.transferIfNeeded(buildClient, deployClient, req, imageTag); err != nil {
		return err
	}
	return p.runBuiltImage(ctx, deployClient, req, name, imageTag)
}

func (p *Pipeline) runBuiltImage(ctx context.Context, client *ssh.Client, req Request, name, imageTag string) error {
	p.log("run", "Replacing container")
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", name)
	args := []string{
		"docker", "run", "-d",
		"--name", name,
		"--restart", "unless-stopped",
		"--network", req.Destination.Network,
	}
	args = append(args, p.limitArgs(req.App)...)
	args = append(args, p.runtimeEnvArgs(ctx, req)...)
	args = append(args, p.proxyLabelArgs(req.App, req.Server.ProxyType)...)
	args = append(args, imageTag)
	_, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		return fmt.Errorf("docker run: %v %s", err, errOut)
	}
	if err := p.waitHealthy(client, name, req.App); err != nil {
		return err
	}
	p.log("finalize", "Deployment finished")
	if p.Store != nil {
		_ = p.Store.UpdateApplicationStatus(context.Background(), req.App.ID, "running")
	}
	return nil
}

func (p *Pipeline) deployStatic(ctx context.Context, buildClient, deployClient *ssh.Client, req Request) error {
	if req.App.GitRepository == "" {
		return fmt.Errorf("git repository is required for static builds")
	}
	name := containerName(req.App)
	workdir := "/data/goolify/applications/" + req.App.ID.String()
	imageTag := "goolify/" + req.App.ID.String() + ":latest"

	p.log("prepare", "Preparing static site workdir "+workdir)
	_, errOut, err := sshx.RunArgs(buildClient, "mkdir", "-p", workdir)
	if err != nil {
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}

	if err := p.gitClone(ctx, buildClient, req, workdir+"/src"); err != nil {
		return err
	}

	dockerfile := `FROM nginx:alpine
COPY . /usr/share/nginx/html
EXPOSE 80
`
	p.log("build", "Writing nginx Dockerfile for static site")
	writeCmd := fmt.Sprintf("cat > %s/src/Dockerfile.goolify-static <<'GOOLIFY_EOF'\n%sGOOLIFY_EOF", workdir, dockerfile)
	_, errOut, err = sshx.Run(buildClient, writeCmd)
	if err != nil {
		return fmt.Errorf("write dockerfile: %v %s", err, errOut)
	}

	p.log("build", "Building static image "+imageTag)
	buildArgs := []string{"docker", "build", "-t", imageTag, "-f", workdir + "/src/Dockerfile.goolify-static"}
	if req.ForceRebuild {
		buildArgs = append(buildArgs, "--no-cache")
	}
	buildArgs = append(buildArgs, workdir+"/src")
	_, errOut, err = sshx.RunArgs(buildClient, buildArgs...)
	if err != nil {
		return fmt.Errorf("docker build: %v %s", err, errOut)
	}

	appCopy := *req.App
	if appCopy.PortsExposes == "" || appCopy.PortsExposes == "3000" {
		appCopy.PortsExposes = "80"
	}
	req.App = &appCopy
	if err := p.transferIfNeeded(buildClient, deployClient, req, imageTag); err != nil {
		return err
	}
	return p.runBuiltImage(ctx, deployClient, req, name, imageTag)
}

func (p *Pipeline) deployNixpacks(ctx context.Context, buildClient, deployClient *ssh.Client, req Request) error {
	if req.App.GitRepository == "" {
		return fmt.Errorf("git repository is required for nixpacks builds")
	}
	name := containerName(req.App)
	workdir := "/data/goolify/applications/" + req.App.ID.String()
	imageTag := "goolify/" + req.App.ID.String() + ":latest"

	p.log("prepare", "Preparing nixpacks workdir "+workdir)
	_, errOut, err := sshx.RunArgs(buildClient, "mkdir", "-p", workdir)
	if err != nil {
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}

	if err := p.gitClone(ctx, buildClient, req, workdir+"/src"); err != nil {
		return err
	}

	p.log("build", "Building with nixpacks (best-effort via railwayapp/nixpacks image)")
	nixArgs := []string{
		"docker", "run", "--rm",
		"-v", workdir + "/src:/app",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-w", "/app",
		"ghcr.io/railwayapp/nixpacks:latest",
		"build", ".", "--name", imageTag,
	}
	if req.ForceRebuild {
		nixArgs = append(nixArgs, "--no-cache")
	}
	_, errOut, err = sshx.RunArgs(buildClient, nixArgs...)
	if err != nil {
		return fmt.Errorf("nixpacks build failed (ensure Docker can pull ghcr.io/railwayapp/nixpacks:latest): %v %s", err, errOut)
	}

	if err := p.transferIfNeeded(buildClient, deployClient, req, imageTag); err != nil {
		return err
	}
	return p.runBuiltImage(ctx, deployClient, req, name, imageTag)
}

func (p *Pipeline) deployCompose(ctx context.Context, client *ssh.Client, req Request) error {
	_ = ctx
	if req.App.GitRepository == "" {
		return fmt.Errorf("git repository is required for dockercompose")
	}
	workdir := "/data/goolify/applications/" + req.App.ID.String()
	p.log("prepare", "Preparing compose workdir")
	_, errOut, err := sshx.RunArgs(client, "mkdir", "-p", workdir)
	if err != nil {
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}
	if err := p.gitClone(ctx, client, req, workdir+"/src"); err != nil {
		return err
	}
	composeFile := strings.TrimPrefix(req.App.DockerComposeLocation, "/")
	if composeFile == "" {
		composeFile = "docker-compose.yaml"
	}
	project := "goolify-" + req.App.ID.String()[:8]
	composePath := workdir + "/src/" + composeFile
	if req.Destination.Kind == "swarm" {
		p.log("run", "docker stack deploy")
		_, errOut, err = sshx.RunArgs(client, "docker", "stack", "deploy", "-c", composePath, "--with-registry-auth", project)
	} else {
		p.log("run", "docker compose up -d")
		_, errOut, err = sshx.RunArgs(client, "docker", "compose", "-p", project, "-f", composePath, "up", "-d", "--build", "--remove-orphans")
	}
	if err != nil {
		return fmt.Errorf("compose deploy: %v %s", err, errOut)
	}
	p.log("finalize", "Compose stack is up")
	if p.Store != nil {
		_ = p.Store.UpdateApplicationStatus(context.Background(), req.App.ID, "running")
	}
	return nil
}

func containerName(app *store.Application) string {
	return "goolify-" + app.ID.String()
}

func firstHost(fqdn string) string {
	parts := strings.Split(fqdn, ",")
	return strings.TrimSpace(parts[0])
}

func firstPort(ports string) string {
	parts := strings.FieldsFunc(ports, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	if len(parts) == 0 {
		return "80"
	}
	return strings.TrimSpace(parts[0])
}
