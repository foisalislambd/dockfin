package deploy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
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
	// CommitSHA / GitBranch override app defaults (rollback, webhook, PR preview).
	CommitSHA string
	GitBranch string
	// PullRequestID > 0 means a preview deploy — separate container + FQDN from production.
	PullRequestID int
	PreviewFQDN   string
}

func (p *Pipeline) log(stage, line string) {
	if p.Log != nil {
		p.Log(stage, line)
	}
}

func (p *Pipeline) checkCancelled(ctx context.Context, deploymentID uuid.UUID) error {
	if p.Store == nil || deploymentID == uuid.Nil {
		return nil
	}
	cancelled, err := p.Store.IsDeploymentCancelled(ctx, deploymentID)
	if err != nil {
		return nil
	}
	if cancelled {
		return fmt.Errorf("deployment cancelled")
	}
	return nil
}

// repairAppFQDN assigns or replaces free domains that embed loopback magic IPs
// (same rules as one-click service deploy). Skip for PR previews — they use PreviewFQDN.
func (p *Pipeline) repairAppFQDN(ctx context.Context, req *Request) {
	if req == nil || req.App == nil || req.Server == nil {
		return
	}
	if req.PullRequestID > 0 {
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
	if err := p.checkCancelled(ctx, req.DeploymentID); err != nil {
		return err
	}
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

	if err := p.checkCancelled(ctx, req.DeploymentID); err != nil {
		return err
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

	if err := p.checkCancelled(ctx, req.DeploymentID); err != nil {
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
		// Coolify: TLS only when URL scheme is https (magic sslip stays http).
		forceHTTPS := app.IsForceHTTPS || strings.HasPrefix(proxy.PrimaryPublicURL(app.FQDN), "https://")
		labels = proxy.TraefikLabelsHTTPS(app.Name, app.FQDN, port, forceHTTPS)
	}
	var args []string
	for _, l := range labels {
		args = append(args, "-l", l)
	}
	return args
}

// proxyLabelArgsReq uses preview FQDN / unique router name when PullRequestID > 0
// so preview deploys never overwrite production Traefik routes or containers.
func (p *Pipeline) proxyLabelArgsReq(req Request) []string {
	app := *req.App
	if req.PullRequestID > 0 {
		// Never reuse production FQDN on a preview container (would steal Traefik Host rules).
		app.FQDN = strings.TrimSpace(req.PreviewFQDN)
		app.Name = fmt.Sprintf("%s-pr-%d", req.App.Name, req.PullRequestID)
	}
	return p.proxyLabelArgs(&app, req.Server.ProxyType)
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
	timeout := app.HealthCheckTimeout
	if timeout <= 0 {
		timeout = 5
	}
	path := strings.TrimSpace(app.HealthCheckPath)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	port := firstPort(app.PortsExposes)
	if app.HealthCheckPort != nil && *app.HealthCheckPort > 0 {
		port = fmt.Sprintf("%d", *app.HealthCheckPort)
	}
	method := strings.ToUpper(strings.TrimSpace(app.HealthCheckMethod))
	switch method {
	case "GET", "HEAD", "POST":
	default:
		method = "GET"
	}
	wantCode := app.HealthCheckReturnCode
	if wantCode <= 0 {
		wantCode = 200
	}

	p.log("healthcheck", fmt.Sprintf(
		"Waiting for HTTP %s http://container:%s%s → %d (%d retries, %ds interval)",
		method, port, path, wantCode, retries, interval,
	))

	for i := 0; i < retries; i++ {
		out, _, err := sshx.RunArgs(client, "docker", "inspect", "-f", "{{.State.Running}}", name)
		if err != nil || strings.TrimSpace(out) != "true" {
			p.log("healthcheck", "container not running yet")
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		code, checkErr := httpHealthStatus(client, name, method, port, path, timeout)
		if checkErr != nil {
			// Fall back to Docker HEALTHCHECK / running when curl/wget unavailable.
			if strings.Contains(checkErr.Error(), "no http client") {
				health, _, hErr := sshx.RunArgs(client, "docker", "inspect", "-f", "{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}", name)
				status := strings.TrimSpace(health)
				if hErr != nil || status == "none" || status == "" {
					p.log("healthcheck", "Container is running (no in-container HTTP client)")
					return nil
				}
				if status == "healthy" {
					p.log("healthcheck", "Container is healthy")
					return nil
				}
				p.log("healthcheck", "health="+status)
			} else {
				p.log("healthcheck", checkErr.Error())
			}
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}
		if code == wantCode {
			p.log("healthcheck", fmt.Sprintf("HTTP %d OK", code))
			return nil
		}
		p.log("healthcheck", fmt.Sprintf("HTTP %d (want %d)", code, wantCode))
		time.Sleep(time.Duration(interval) * time.Second)
	}
	return fmt.Errorf("health check failed: container %s not healthy after %d attempts", name, retries)
}

// httpHealthStatus probes the app from inside the container (127.0.0.1) so it works
// without published ports. Prefers curl, then wget.
func httpHealthStatus(client *ssh.Client, container, method, port, path string, timeoutSec int) (int, error) {
	url := fmt.Sprintf("http://127.0.0.1:%s%s", port, path)
	timeout := fmt.Sprintf("%d", timeoutSec)
	// curl path
	out, _, err := sshx.RunArgs(client, "docker", "exec", container, "sh", "-lc",
		fmt.Sprintf(`if command -v curl >/dev/null 2>&1; then curl -s -o /dev/null -w '%%{http_code}' -X %s --max-time %s %q; elif command -v wget >/dev/null 2>&1; then wget -q -S -O /dev/null --timeout=%s %q 2>&1 | awk '/^  HTTP\//{c=$2} END{print c+0}'; else echo NOCLIENT; fi`,
			method, timeout, url, timeout, url))
	codeStr := strings.TrimSpace(out)
	if codeStr == "NOCLIENT" || (err != nil && codeStr == "") {
		return 0, fmt.Errorf("no http client in container")
	}
	var code int
	if _, scanErr := fmt.Sscanf(codeStr, "%d", &code); scanErr != nil || code <= 0 {
		if err != nil {
			return 0, fmt.Errorf("health probe failed: %v (%s)", err, codeStr)
		}
		return 0, fmt.Errorf("health probe returned %q", codeStr)
	}
	return code, nil
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
	name := containerNameFor(req)

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
	args = append(args, p.proxyLabelArgsReq(req)...)
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
	if p.Store != nil && req.PullRequestID == 0 {
		_ = p.Store.UpdateApplicationStatus(context.Background(), req.App.ID, "running")
	}
	if p.Store != nil && req.PullRequestID > 0 {
		_ = p.Store.UpdatePreviewStatus(context.Background(), req.TeamID, req.App.ID, req.PullRequestID, "running")
	}
	return nil
}

func (p *Pipeline) deployDockerfile(ctx context.Context, buildClient, deployClient *ssh.Client, req Request) error {
	if req.App.GitRepository == "" {
		return fmt.Errorf("git repository is required for %s builds", req.App.BuildPack)
	}
	name := containerNameFor(req)
	workdir := "/data/dockfin/applications/" + req.App.ID.String()
	imageTag := "dockfin/" + req.App.ID.String() + ":latest"
	if req.PullRequestID > 0 {
		workdir = fmt.Sprintf("/data/dockfin/applications/%s-pr-%d", req.App.ID.String(), req.PullRequestID)
		imageTag = fmt.Sprintf("dockfin/%s-pr-%d:latest", req.App.ID.String(), req.PullRequestID)
	}

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
	args = append(args, p.proxyLabelArgsReq(req)...)
	args = append(args, imageTag)
	_, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		return fmt.Errorf("docker run: %v %s", err, errOut)
	}
	if err := p.waitHealthy(client, name, req.App); err != nil {
		return err
	}
	p.log("finalize", "Deployment finished")
	if p.Store != nil && req.PullRequestID == 0 {
		_ = p.Store.UpdateApplicationStatus(context.Background(), req.App.ID, "running")
	}
	if p.Store != nil && req.PullRequestID > 0 {
		_ = p.Store.UpdatePreviewStatus(context.Background(), req.TeamID, req.App.ID, req.PullRequestID, "running")
	}
	return nil
}

func (p *Pipeline) deployStatic(ctx context.Context, buildClient, deployClient *ssh.Client, req Request) error {
	if req.App.GitRepository == "" {
		return fmt.Errorf("git repository is required for static builds")
	}
	name := containerNameFor(req)
	workdir := "/data/dockfin/applications/" + req.App.ID.String()
	imageTag := "dockfin/" + req.App.ID.String() + ":latest"
	if req.PullRequestID > 0 {
		workdir = fmt.Sprintf("/data/dockfin/applications/%s-pr-%d", req.App.ID.String(), req.PullRequestID)
		imageTag = fmt.Sprintf("dockfin/%s-pr-%d:latest", req.App.ID.String(), req.PullRequestID)
	}

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
	writeCmd := fmt.Sprintf("cat > %s/src/Dockerfile.dockfin-static <<'DOCKFIN_EOF'\n%sDOCKFIN_EOF", workdir, dockerfile)
	_, errOut, err = sshx.Run(buildClient, writeCmd)
	if err != nil {
		return fmt.Errorf("write dockerfile: %v %s", err, errOut)
	}

	p.log("build", "Building static image "+imageTag)
	buildArgs := []string{"docker", "build", "-t", imageTag, "-f", workdir + "/src/Dockerfile.dockfin-static"}
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
	name := containerNameFor(req)
	workdir := "/data/dockfin/applications/" + req.App.ID.String()
	imageTag := "dockfin/" + req.App.ID.String() + ":latest"
	if req.PullRequestID > 0 {
		workdir = fmt.Sprintf("/data/dockfin/applications/%s-pr-%d", req.App.ID.String(), req.PullRequestID)
		imageTag = fmt.Sprintf("dockfin/%s-pr-%d:latest", req.App.ID.String(), req.PullRequestID)
	}

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
	workdir := "/data/dockfin/applications/" + req.App.ID.String()
	project := "dockfin-" + req.App.ID.String()[:8]
	if req.PullRequestID > 0 {
		workdir = fmt.Sprintf("/data/dockfin/applications/%s-pr-%d", req.App.ID.String(), req.PullRequestID)
		project = fmt.Sprintf("dockfin-%s-pr-%d", req.App.ID.String()[:8], req.PullRequestID)
	}
	p.log("prepare", "Preparing compose workdir "+workdir)
	_, errOut, err := sshx.RunArgs(client, "mkdir", "-p", workdir)
	if err != nil {
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}
	if err := p.gitClone(ctx, client, req, workdir+"/src"); err != nil {
		return err
	}
	composePath, loc, err := p.resolveComposePath(client, req, workdir+"/src")
	if err != nil {
		return err
	}
	if loc != "" && req.PullRequestID == 0 && p.Store != nil && loc != req.App.DockerComposeLocation {
		req.App.DockerComposeLocation = loc
		_ = p.Store.UpdateApplication(context.Background(), req.App)
		p.log("prepare", "Using compose file "+loc)
	}
	deployPath := composePath

	if req.App.ComposePrepare {
		adapted, err := p.adaptComposeForDockfin(ctx, client, req, composePath)
		if err != nil {
			return err
		}
		deployPath = adapted
	} else {
		p.log("prepare", "Compose prepare disabled — deploying repository file as-is")
	}

	// Write Dockfin env as a sidecar file (never overwrite the repo's .env).
	envFile, envErr := p.writeComposeEnvFile(ctx, client, req, deployPath)
	if envErr != nil {
		p.log("prepare", "Warning: could not write env file: "+envErr.Error())
	}

	if req.Destination.Kind == "swarm" {
		p.log("run", "docker stack deploy")
		// swarm stack deploy has no --env-file; vars should already be baked via prepare when enabled.
		_, errOut, err = sshx.RunArgs(client, "docker", "stack", "deploy", "-c", deployPath, "--with-registry-auth", project)
	} else {
		p.log("run", "docker compose up -d")
		args := []string{"docker", "compose", "-p", project}
		if envFile != "" {
			args = append(args, "--env-file", envFile)
		}
		args = append(args, "-f", deployPath, "up", "-d", "--build", "--remove-orphans")
		_, errOut, err = sshx.RunArgs(client, args...)
	}
	if err != nil {
		return fmt.Errorf("compose deploy: %v %s", err, errOut)
	}
	p.log("finalize", "Compose stack is up")
	if p.Store != nil && req.PullRequestID == 0 {
		_ = p.Store.UpdateApplicationStatus(context.Background(), req.App.ID, "running")
	}
	if p.Store != nil && req.PullRequestID > 0 {
		_ = p.Store.UpdatePreviewStatus(context.Background(), req.TeamID, req.App.ID, req.PullRequestID, "running")
	}
	return nil
}

// resolveComposePath picks the compose file after clone.
// Tries the configured location first; if missing (or empty/auto), scans common names.
// Returns absolute remote path and repo-relative location ("/docker-compose.yml").
func (p *Pipeline) resolveComposePath(client *ssh.Client, req Request, srcDir string) (absPath, location string, err error) {
	preferred := services.NormalizeComposeLocation(req.App.DockerComposeLocation)
	if preferred != "" {
		rel := strings.TrimPrefix(preferred, "/")
		candidate := srcDir + "/" + rel
		if remoteFileExists(client, candidate) {
			return candidate, preferred, nil
		}
		p.log("prepare", fmt.Sprintf("Compose path %s not found — auto-detecting…", preferred))
	} else {
		p.log("prepare", "Compose path empty — auto-detecting…")
	}

	found, findErr := p.findComposeFilesRemote(client, srcDir)
	if findErr != nil {
		return "", "", findErr
	}
	best := services.PreferComposeFile(found)
	if best == "" {
		hint := "docker-compose.yaml / docker-compose.yml / compose.yaml"
		if preferred != "" {
			return "", "", fmt.Errorf("compose file %s not found (also searched for %s)", preferred, hint)
		}
		return "", "", fmt.Errorf("no compose file found in repository (looked for %s under the repo, max depth 3)", hint)
	}
	if len(found) > 1 {
		p.log("prepare", fmt.Sprintf("Found %d compose files; using %s", len(found), best))
	} else {
		p.log("prepare", "Auto-detected compose file "+best)
	}
	return srcDir + "/" + strings.TrimPrefix(best, "/"), best, nil
}

func remoteFileExists(client *ssh.Client, path string) bool {
	_, _, err := sshx.RunArgs(client, "test", "-f", path)
	return err == nil
}

func (p *Pipeline) findComposeFilesRemote(client *ssh.Client, srcDir string) ([]string, error) {
	// -maxdepth 3 ≡ local WalkDir (dirs up to depth 2): ./a/b/compose.yml is included.
	// -iname for case-insensitive names on Linux remotes.
	script := fmt.Sprintf(`cd %s && find . -maxdepth 3 \( -iname 'docker-compose.yaml' -o -iname 'docker-compose.yml' -o -iname 'compose.yaml' -o -iname 'compose.yml' \) ! -path './.git/*' ! -path '*/node_modules/*' ! -path '*/vendor/*' ! -path '*/.venv/*' 2>/dev/null | sed 's|^\./||' || true`, shellSingleQuote(srcDir))
	out, errOut, err := sshx.Run(client, script)
	if err != nil {
		// Fall back to root-only probes when find is unavailable.
		p.log("prepare", fmt.Sprintf("find compose warning: %v %s — trying root filenames", err, errOut))
		var found []string
		for _, name := range services.CommonComposeFilenames {
			candidate := srcDir + "/" + name
			if remoteFileExists(client, candidate) {
				found = append(found, "/"+name)
			}
		}
		return services.SortComposeFiles(found), nil
	}
	var found []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		found = append(found, "/"+strings.TrimPrefix(filepath.ToSlash(line), "/"))
	}
	return services.SortComposeFiles(found), nil
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// adaptComposeForDockfin reads the cloned compose file, runs PrepareCompose (Traefik
// labels, shared network, strip host ports, magic env), and writes docker-compose.dockfin.yml
// beside the original so relative build contexts stay valid.
func (p *Pipeline) adaptComposeForDockfin(ctx context.Context, client *ssh.Client, req Request, composePath string) (string, error) {
	p.log("prepare", "Adapting compose for Dockfin (Traefik, network, ports)…")
	raw, errOut, err := sshx.RunArgs(client, "cat", composePath)
	if err != nil {
		return "", fmt.Errorf("read compose %s: %v %s", composePath, err, errOut)
	}
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("compose file is empty: %s", composePath)
	}

	domainList := strings.TrimSpace(req.App.FQDN)
	if req.PullRequestID > 0 {
		domainList = strings.TrimSpace(req.PreviewFQDN)
	}
	fqdn := firstHost(domainList)
	baseURL := proxy.PrimaryPublicURL(domainList)
	if req.App.IsForceHTTPS && fqdn != "" && !proxy.IsMagicDomainHost(fqdn) {
		baseURL = "https://" + fqdn
	}
	routerName := req.App.Name + "-" + req.App.ID.String()[:8]
	if req.PullRequestID > 0 {
		routerName = fmt.Sprintf("%s-pr-%d", req.App.Name, req.PullRequestID)
	}
	// Non-empty PortsExposes overrides; empty → auto-detect from compose (# port:, ports:).
	port := services.DetectProxyPortForGitCompose(raw, req.App.PortsExposes)
	opts := services.PrepareOpts{
		ServiceID:  req.App.ID.String(),
		Network:    "",
		BaseURL:    baseURL,
		FQDN:       domainList,
		RouterName: routerName,
		Port:       port,
	}
	if req.Destination != nil {
		opts.Network = req.Destination.Network
	}
	if p.Store != nil {
		opts.ExistingEnv = p.composeExistingEnv(ctx, req)
	}

	prepared, magicEnv, err := services.PrepareCompose(raw, opts)
	if err != nil {
		return "", fmt.Errorf("prepare compose: %w", err)
	}

	// Keep prepared file next to the original so build: . / relative volumes resolve.
	dir := composePath
	if i := strings.LastIndex(composePath, "/"); i >= 0 {
		dir = composePath[:i]
	}
	preparedPath := dir + "/docker-compose.dockfin.yml"
	writeCmd := fmt.Sprintf("cat > %s <<'DOCKFIN_COMPOSE_EOF'\n%s\nDOCKFIN_COMPOSE_EOF", preparedPath, prepared)
	_, errOut, err = sshx.Run(client, writeCmd)
	if err != nil {
		return "", fmt.Errorf("write prepared compose: %v %s", err, errOut)
	}
	p.log("prepare", fmt.Sprintf("Wrote adapted compose %s (proxy port %s)", preparedPath, port))

	// Persist magic secrets/URLs into application env so redeploys reuse passwords.
	if req.PullRequestID == 0 {
		p.syncApplicationComposeEnv(ctx, req, raw, prepared, magicEnv)
	}
	return preparedPath, nil
}

func (p *Pipeline) composeExistingEnv(ctx context.Context, req Request) map[string]string {
	if p.Store == nil || req.App == nil {
		return nil
	}
	var projectID, envID, serverID *uuid.UUID
	envID = &req.App.EnvironmentID
	if req.Destination != nil {
		serverID = &req.Destination.ServerID
	}
	if env, err := p.Store.GetEnvironment(ctx, req.TeamID, req.App.EnvironmentID); err == nil {
		projectID = &env.ProjectID
	}
	m, err := p.Store.ResolvedEnvMap(ctx, req.TeamID, "application", req.App.ID, projectID, envID, serverID)
	if err != nil {
		return nil
	}
	return m
}

// writeComposeEnvFile writes Dockfin-managed vars to .env.dockfin (does not clobber repo .env).
// Returns the remote path when a file was written, or "" when there was nothing to write.
func (p *Pipeline) writeComposeEnvFile(ctx context.Context, client *ssh.Client, req Request, composePath string) (string, error) {
	envMap := p.composeExistingEnv(ctx, req)
	if len(envMap) == 0 {
		return "", nil
	}
	dir := composePath
	if i := strings.LastIndex(composePath, "/"); i >= 0 {
		dir = composePath[:i]
	}
	envPath := dir + "/.env.dockfin"
	body := services.FormatEnvFile(envMap)
	writeCmd := fmt.Sprintf("cat > %s <<'DOCKFIN_ENV_EOF'\n%sDOCKFIN_ENV_EOF", envPath, body)
	_, errOut, err := sshx.Run(client, writeCmd)
	if err != nil {
		return "", fmt.Errorf("%v %s", err, errOut)
	}
	p.log("prepare", "Wrote compose env file "+envPath)
	return envPath, nil
}

// syncApplicationComposeEnv stores Coolify-style SERVICE_* values on the application
// so redeploy reuses passwords and the Environment Variables UI shows URLs.
func (p *Pipeline) syncApplicationComposeEnv(ctx context.Context, req Request, raw, prepared string, fullEnv map[string]string) {
	if p.Store == nil || req.App == nil {
		return
	}
	ui := services.CoolifyEnvForUI(raw, fullEnv)
	if len(ui) == 0 {
		ui = services.CoolifyEnvForUI(raw, services.ExtractMagicEnv(prepared))
	}
	if len(ui) == 0 {
		return
	}
	for key, val := range ui {
		preserve := strings.HasPrefix(key, "SERVICE_PASSWORD_") ||
			strings.HasPrefix(key, "SERVICE_BASE64_") ||
			strings.HasPrefix(key, "SERVICE_HEX_") ||
			strings.HasPrefix(key, "SERVICE_USER_")
		bypassLock := strings.HasPrefix(key, "SERVICE_URL_") || strings.HasPrefix(key, "SERVICE_FQDN_")
		_, err := p.Store.UpsertEnvVar(ctx, req.TeamID, "application", req.App.ID, store.UpsertEnvVarInput{
			Key:        key,
			Value:      val,
			Runtime:    true,
			Buildtime:  true,
			Literal:    true,
			KeepValue:  preserve,
			BypassLock: bypassLock,
		})
		if err != nil {
			p.log("prepare", "Warning: could not sync env "+key+": "+err.Error())
		}
	}
}

func containerName(app *store.Application) string {
	return "dockfin-" + app.ID.String()
}

func containerNameFor(req Request) string {
	if req.PullRequestID > 0 && req.App != nil {
		return fmt.Sprintf("dockfin-%s-pr-%d", req.App.ID.String(), req.PullRequestID)
	}
	if req.App == nil {
		return ""
	}
	return containerName(req.App)
}

func firstHost(fqdn string) string {
	return proxy.PrimaryHost(fqdn)
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
