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
	DeploymentID uuid.UUID
	TeamID       uuid.UUID
	App          *store.Application
	Server       *store.Server
	Destination  *store.Destination
	PrivateKey   []byte
	ForceRebuild bool
}

func (p *Pipeline) log(stage, line string) {
	if p.Log != nil {
		p.Log(stage, line)
	}
}

func (p *Pipeline) Run(ctx context.Context, req Request) error {
	p.log("prepare", "Connecting to server via SSH…")
	res, err := p.SSH.Dial(sshx.Target{
		Host:                req.Server.IP,
		Port:                req.Server.Port,
		User:                req.Server.UserName,
		PrivateKey:          req.PrivateKey,
		ExpectedFingerprint: req.Server.HostKeyFingerprint,
		ExpectedKeyType:     req.Server.HostKeyType,
	})
	if err != nil {
		return fmt.Errorf("ssh: %w", err)
	}
	client := res.Client
	if res.IsNewHost && p.Store != nil {
		_ = p.Store.UpdateServerHostKey(ctx, req.Server.ID, res.Fingerprint, res.KeyType)
		p.log("prepare", "Trusted new host key "+res.Fingerprint)
	}

	p.log("prepare", "Ensuring data directories")
	_ = sshx.EnsureDataDirs(client)

	p.log("prepare", fmt.Sprintf("Ensuring Docker network %q", req.Destination.Network))
	if err := sshx.EnsureNetwork(client, req.Destination.Network); err != nil {
		return err
	}

	switch req.App.BuildPack {
	case "dockerimage":
		return p.deployImage(ctx, client, req)
	case "dockerfile":
		return p.deployDockerfile(ctx, client, req)
	case "dockercompose":
		return p.deployCompose(ctx, client, req)
	case "static":
		return p.deployStatic(ctx, client, req)
	case "nixpacks", "railpack":
		return p.deployNixpacks(ctx, client, req)
	default:
		return fmt.Errorf("unsupported build pack: %s", req.App.BuildPack)
	}
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

func (p *Pipeline) traefikLabelArgs(app *store.Application) []string {
	if app.FQDN == "" {
		return nil
	}
	var args []string
	for _, l := range proxy.TraefikLabelsHTTPS(app.Name, firstHost(app.FQDN), firstPort(app.PortsExposes), app.IsForceHTTPS) {
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
	args = append(args, p.traefikLabelArgs(req.App)...)
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

func (p *Pipeline) deployDockerfile(ctx context.Context, client *ssh.Client, req Request) error {
	if req.App.GitRepository == "" {
		return fmt.Errorf("git repository is required for %s builds", req.App.BuildPack)
	}
	name := containerName(req.App)
	workdir := "/data/goolify/applications/" + req.App.ID.String()
	imageTag := "goolify/" + req.App.ID.String() + ":latest"

	p.log("prepare", "Preparing remote workdir "+workdir)
	_, errOut, err := sshx.RunArgs(client, "mkdir", "-p", workdir)
	if err != nil {
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}

	p.log("fetch", "Cloning "+req.App.GitRepository)
	_, _, _ = sshx.RunArgs(client, "rm", "-rf", workdir+"/src")
	_, errOut, err = sshx.RunArgs(client, "git", "clone", "--branch", req.App.GitBranch, "--depth", "1", req.App.GitRepository, workdir+"/src")
	if err != nil {
		return fmt.Errorf("git clone: %v %s", err, errOut)
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
	_, errOut, err = sshx.RunArgs(client, buildArgs...)
	if err != nil {
		return fmt.Errorf("docker build: %v %s", err, errOut)
	}

	return p.runBuiltImage(ctx, client, req, name, imageTag)
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
	args = append(args, p.traefikLabelArgs(req.App)...)
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

func (p *Pipeline) deployStatic(ctx context.Context, client *ssh.Client, req Request) error {
	if req.App.GitRepository == "" {
		return fmt.Errorf("git repository is required for static builds")
	}
	name := containerName(req.App)
	workdir := "/data/goolify/applications/" + req.App.ID.String()
	imageTag := "goolify/" + req.App.ID.String() + ":latest"

	p.log("prepare", "Preparing static site workdir "+workdir)
	_, errOut, err := sshx.RunArgs(client, "mkdir", "-p", workdir)
	if err != nil {
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}

	p.log("fetch", "Cloning "+req.App.GitRepository)
	_, _, _ = sshx.RunArgs(client, "rm", "-rf", workdir+"/src")
	_, errOut, err = sshx.RunArgs(client, "git", "clone", "--branch", req.App.GitBranch, "--depth", "1", req.App.GitRepository, workdir+"/src")
	if err != nil {
		return fmt.Errorf("git clone: %v %s", err, errOut)
	}

	dockerfile := `FROM nginx:alpine
COPY . /usr/share/nginx/html
EXPOSE 80
`
	p.log("build", "Writing nginx Dockerfile for static site")
	writeCmd := fmt.Sprintf("cat > %s/src/Dockerfile.goolify-static <<'GOOLIFY_EOF'\n%sGOOLIFY_EOF", workdir, dockerfile)
	_, errOut, err = sshx.Run(client, writeCmd)
	if err != nil {
		return fmt.Errorf("write dockerfile: %v %s", err, errOut)
	}

	p.log("build", "Building static image "+imageTag)
	buildArgs := []string{"docker", "build", "-t", imageTag, "-f", workdir + "/src/Dockerfile.goolify-static"}
	if req.ForceRebuild {
		buildArgs = append(buildArgs, "--no-cache")
	}
	buildArgs = append(buildArgs, workdir+"/src")
	_, errOut, err = sshx.RunArgs(client, buildArgs...)
	if err != nil {
		return fmt.Errorf("docker build: %v %s", err, errOut)
	}

	// Static sites expose port 80 by default for Traefik
	appCopy := *req.App
	if appCopy.PortsExposes == "" || appCopy.PortsExposes == "3000" {
		appCopy.PortsExposes = "80"
	}
	req.App = &appCopy
	return p.runBuiltImage(ctx, client, req, name, imageTag)
}

func (p *Pipeline) deployNixpacks(ctx context.Context, client *ssh.Client, req Request) error {
	if req.App.GitRepository == "" {
		return fmt.Errorf("git repository is required for nixpacks builds")
	}
	name := containerName(req.App)
	workdir := "/data/goolify/applications/" + req.App.ID.String()
	imageTag := "goolify/" + req.App.ID.String() + ":latest"

	p.log("prepare", "Preparing nixpacks workdir "+workdir)
	_, errOut, err := sshx.RunArgs(client, "mkdir", "-p", workdir)
	if err != nil {
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}

	p.log("fetch", "Cloning "+req.App.GitRepository)
	_, _, _ = sshx.RunArgs(client, "rm", "-rf", workdir+"/src")
	_, errOut, err = sshx.RunArgs(client, "git", "clone", "--branch", req.App.GitBranch, "--depth", "1", req.App.GitRepository, workdir+"/src")
	if err != nil {
		return fmt.Errorf("git clone: %v %s", err, errOut)
	}

	p.log("build", "Building with nixpacks (best-effort via railwayapp/nixpacks image)")
	// Run nixpacks builder with docker.sock so it can build the final image on the host.
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
	_, errOut, err = sshx.RunArgs(client, nixArgs...)
	if err != nil {
		return fmt.Errorf("nixpacks build failed (ensure Docker can pull ghcr.io/railwayapp/nixpacks:latest): %v %s", err, errOut)
	}

	return p.runBuiltImage(ctx, client, req, name, imageTag)
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
	_, _, _ = sshx.RunArgs(client, "rm", "-rf", workdir+"/src")
	p.log("fetch", "Cloning repository")
	_, errOut, err = sshx.RunArgs(client, "git", "clone", "--branch", req.App.GitBranch, "--depth", "1", req.App.GitRepository, workdir+"/src")
	if err != nil {
		return fmt.Errorf("git clone: %v %s", err, errOut)
	}
	composeFile := strings.TrimPrefix(req.App.DockerComposeLocation, "/")
	if composeFile == "" {
		composeFile = "docker-compose.yaml"
	}
	project := "goolify-" + req.App.ID.String()[:8]
	p.log("run", "docker compose up -d")
	_, errOut, err = sshx.RunArgs(client, "docker", "compose", "-p", project, "-f", workdir+"/src/"+composeFile, "up", "-d", "--build", "--remove-orphans")
	if err != nil {
		return fmt.Errorf("compose up: %v %s", err, errOut)
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
