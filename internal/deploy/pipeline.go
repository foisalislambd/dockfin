package deploy

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
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
	// skipFanOut prevents recursive additional-destination deploys.
	skipFanOut bool
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
		// Bare domain.com → https://domain.com (or http:// for magic).
		if n := proxy.NormalizeDomains(req.App.FQDN); n != "" && n != req.App.FQDN {
			req.App.FQDN = n
			if p.Store != nil {
				_ = p.Store.UpdateApplication(ctx, req.App)
			}
		}
		return
	}
	magicIP := proxy.PreferMagicIP(req.Server.IP, req.Server.PublicIP)
	fqdn := proxy.GenerateFQDN(req.App.Name, req.App.ID, magicIP, req.Server.WildcardDomain, req.Server.MagicDomain)
	fqdn = proxy.NormalizeDomains(fqdn)
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
	case "railpack":
		return p.deployRailpack(ctx, buildClient, deployClient, req)
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

// fanOutAdditionalDestinations deploys the same image (or compose stack) to extra destinations.
// Failures after primary success mark the deployment failed.
func (p *Pipeline) fanOutAdditionalDestinations(ctx context.Context, primaryClient *ssh.Client, req Request, image string) error {
	if req.skipFanOut || req.PullRequestID != 0 || p.Store == nil || req.App == nil {
		return nil
	}
	ids, err := p.Store.ListAdditionalDestinations(ctx, req.TeamID, req.App.ID)
	if err != nil {
		return fmt.Errorf("list additional destinations: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	primaryServerID := uuid.Nil
	if req.Destination != nil {
		primaryServerID = req.Destination.ServerID
	}
	for _, destID := range ids {
		p.log("transfer", fmt.Sprintf("Fan-out to additional destination %s…", destID))
		dest, err := p.Store.GetDestination(ctx, req.TeamID, destID)
		if err != nil {
			return fmt.Errorf("additional destination %s: %w", destID, err)
		}
		if primaryServerID != uuid.Nil && dest.ServerID == primaryServerID {
			p.log("transfer", fmt.Sprintf("Skipping additional destination %s — same server as primary (would clobber)", destID))
			continue
		}
		srv, err := p.Store.GetServer(ctx, req.TeamID, dest.ServerID)
		if err != nil {
			return fmt.Errorf("additional destination server: %w", err)
		}
		if srv.PrivateKeyID == nil {
			return fmt.Errorf("additional destination server %s has no private key", srv.Name)
		}
		enc, err := p.Store.GetPrivateKeyMaterial(ctx, req.TeamID, *srv.PrivateKeyID)
		if err != nil {
			return fmt.Errorf("additional destination key: %w", err)
		}
		plain, err := p.Store.Box.DecryptString(enc)
		if err != nil {
			return fmt.Errorf("decrypt additional destination key: %w", err)
		}
		client, err := p.dialServer(ctx, srv, []byte(plain))
		if err != nil {
			return fmt.Errorf("additional destination ssh: %w", err)
		}
		_ = sshx.EnsureDataDirs(client)
		if err := p.ensureDestinationNetwork(client, dest); err != nil {
			return fmt.Errorf("additional destination network: %w", err)
		}
		alt := req
		alt.Destination = dest
		alt.Server = srv
		alt.PrivateKey = []byte(plain)
		alt.skipFanOut = true
		alt.BuildServer = nil

		if req.App.BuildPack == "dockercompose" {
			if err := p.deployCompose(ctx, client, alt); err != nil {
				return fmt.Errorf("additional destination %s: %w", destID, err)
			}
			continue
		}
		if strings.TrimSpace(image) == "" {
			return fmt.Errorf("additional destination %s: no image to transfer", destID)
		}
		if err := TransferImage(primaryClient, client, image); err != nil {
			return fmt.Errorf("additional destination %s transfer: %w", destID, err)
		}
		name := containerNameFor(alt)
		if err := p.runWithHealthCutover(ctx, client, alt, name, image); err != nil {
			return fmt.Errorf("additional destination %s: %w", destID, err)
		}
	}
	p.log("transfer", "Additional destinations updated")
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
	var m map[string]string
	var err error
	if req.PullRequestID > 0 {
		m, err = p.Store.ResolvedEnvMapPreview(ctx, req.TeamID, "application", req.App.ID, projectID, envID, serverID)
	} else {
		m, err = p.Store.ResolvedEnvMap(ctx, req.TeamID, "application", req.App.ID, projectID, envID, serverID)
	}
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
		// Auto SSL for custom domains; never for magic free domains.
		labels = proxy.CaddyLabels(app.Name, host, port, proxy.WantAutoHTTPS(app.FQDN) || (app.IsForceHTTPS && !proxy.IsMagicDomainHost(host)))
		if user := strings.TrimSpace(app.HTTPBasicAuthUsername); user != "" && strings.TrimSpace(app.HTTPBasicAuthPasswordEnc) != "" {
			if p.Store != nil {
				if plain, err := p.Store.Box.DecryptString(app.HTTPBasicAuthPasswordEnc); err == nil && plain != "" {
					labels = append(labels, fmt.Sprintf("caddy.basicauth=%s %s", user, plain))
				}
			}
		}
		if redir := strings.ToLower(strings.TrimSpace(app.Redirect)); (redir == "www" || redir == "non-www") && !proxy.IsMagicDomainHost(host) {
			labels = append(labels, proxy.CaddyWWWRedirectLabels(app.Name, host, redir)...)
		}
	case "none":
		return nil
	default:
		// Auto Let's Encrypt for custom domains; magic sslip/nip stay HTTP-only.
		forceHTTPS := app.IsForceHTTPS
		labels = proxy.TraefikLabelsHTTPS(app.Name, app.FQDN, port, forceHTTPS)
		var middlewares []string
		if user := strings.TrimSpace(app.HTTPBasicAuthUsername); user != "" && strings.TrimSpace(app.HTTPBasicAuthPasswordEnc) != "" {
			if p.Store != nil {
				if plain, err := p.Store.Box.DecryptString(app.HTTPBasicAuthPasswordEnc); err == nil && plain != "" {
					hash := htpasswdBcrypt(user, plain)
					authLabels := proxy.TraefikBasicAuthLabels(app.Name, hash)
					for _, l := range authLabels {
						if strings.Contains(l, ".middlewares=") {
							if i := strings.IndexByte(l, '='); i > 0 {
								middlewares = append(middlewares, l[i+1:])
							}
							continue
						}
						labels = append(labels, l)
					}
				}
			}
		}
		if redir := strings.ToLower(strings.TrimSpace(app.Redirect)); (redir == "www" || redir == "non-www") && !proxy.IsMagicDomainHost(host) {
			wwwLabels := proxy.TraefikWWWRedirectLabels(app.Name, redir)
			for _, l := range wwwLabels {
				if strings.Contains(l, ".middlewares=") {
					if i := strings.IndexByte(l, '='); i > 0 {
						middlewares = append(middlewares, l[i+1:])
					}
					continue
				}
				labels = append(labels, l)
			}
		}
		if len(middlewares) > 0 {
			router := sanitizeProxyRouter(app.Name)
			labels = append(labels, fmt.Sprintf("traefik.http.routers.%s.middlewares=%s", router, strings.Join(middlewares, ",")))
		}
		router := sanitizeProxyRouter(app.Name)
		if app.IsGzipEnabled {
			gzipMW := router + "-gzip"
			labels = append(labels, fmt.Sprintf("traefik.http.middlewares.%s.compress=true", gzipMW))
			labels = appendTraefikMiddleware(labels, router, gzipMW)
		}
		if app.IsStripPrefixEnabled {
			if path := pathPrefixFromFQDN(app.FQDN); path != "" && path != "/" {
				stripMW := router + "-stripprefix"
				labels = append(labels, fmt.Sprintf("traefik.http.middlewares.%s.stripprefix.prefixes=%s", stripMW, path))
				labels = appendTraefikMiddleware(labels, router, stripMW)
			}
		}
	}
	for _, l := range proxy.ParseCustomLabels(app.CustomLabels) {
		labels = append(labels, l)
	}
	var args []string
	for _, l := range labels {
		args = append(args, "-l", l)
	}
	return args
}

func sanitizeProxyRouter(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
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
	startPeriod := app.HealthCheckStartPeriod
	if startPeriod > 0 {
		p.log("healthcheck", fmt.Sprintf("Waiting %ds before health checks (start period)", startPeriod))
		time.Sleep(time.Duration(startPeriod) * time.Second)
	}

	checkType := strings.ToLower(strings.TrimSpace(app.HealthCheckType))
	if checkType == "" {
		checkType = "http"
	}

	if checkType == "cmd" {
		cmd := strings.TrimSpace(app.HealthCheckCommand)
		if cmd == "" {
			return fmt.Errorf("health check type cmd requires health_check_command")
		}
		wantText := strings.TrimSpace(app.HealthCheckResponseText)
		p.log("healthcheck", fmt.Sprintf("Waiting for command health check (%d retries, %ds interval)", retries, interval))
		for i := 0; i < retries; i++ {
			out, _, err := sshx.RunArgs(client, "docker", "exec", name, "sh", "-lc", cmd)
			if err == nil {
				if wantText == "" || strings.Contains(out, wantText) {
					p.log("healthcheck", "Command health check OK")
					return nil
				}
				p.log("healthcheck", "command output missing expected text")
			} else {
				p.log("healthcheck", "command health check failed")
			}
			time.Sleep(time.Duration(interval) * time.Second)
		}
		return fmt.Errorf("health check failed: command not healthy after %d attempts", retries)
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
	scheme := strings.ToLower(strings.TrimSpace(app.HealthCheckScheme))
	if scheme == "" {
		scheme = "http"
	}
	host := strings.TrimSpace(app.HealthCheckHost)
	if host == "" {
		host = "localhost"
	}
	wantText := strings.TrimSpace(app.HealthCheckResponseText)

	p.log("healthcheck", fmt.Sprintf(
		"Waiting for HTTP %s %s://%s:%s%s → %d (%d retries, %ds interval)",
		method, scheme, host, port, path, wantCode, retries, interval,
	))

	for i := 0; i < retries; i++ {
		out, _, err := sshx.RunArgs(client, "docker", "inspect", "-f", "{{.State.Running}}", name)
		if err != nil || strings.TrimSpace(out) != "true" {
			p.log("healthcheck", "container not running yet")
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		code, body, checkErr := httpHealthProbe(client, name, method, scheme, host, port, path, timeout)
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
		if code == wantCode && (wantText == "" || strings.Contains(body, wantText)) {
			p.log("healthcheck", fmt.Sprintf("HTTP %d OK", code))
			return nil
		}
		if code == wantCode && wantText != "" {
			p.log("healthcheck", "HTTP status OK but response text mismatch")
		} else {
			p.log("healthcheck", fmt.Sprintf("HTTP %d (want %d)", code, wantCode))
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	return fmt.Errorf("health check failed: container %s not healthy after %d attempts", name, retries)
}

// httpHealthProbe probes the app from inside the container so it works without
// published ports. Prefers curl, then wget. Returns status code and response body.
func httpHealthProbe(client *ssh.Client, container, method, scheme, host, port, path string, timeoutSec int) (int, string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	switch method {
	case "GET", "HEAD", "POST":
	default:
		method = "GET"
	}
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme != "http" && scheme != "https" {
		scheme = "http"
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "localhost"
	}
	if strings.ContainsAny(host, " \t\n\r\"'`$;&|<>(){}") {
		return 0, "", fmt.Errorf("invalid health check host")
	}
	if !regexp.MustCompile(`^[0-9]{1,5}$`).MatchString(port) {
		return 0, "", fmt.Errorf("invalid health check port")
	}
	var portNum int
	if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil || portNum < 1 || portNum > 65535 {
		return 0, "", fmt.Errorf("invalid health check port")
	}
	if path == "" || path[0] != '/' || strings.ContainsAny(path, " \t\n\r\"'`$;&|<>(){}") {
		return 0, "", fmt.Errorf("invalid health check path")
	}
	if !ValidContainerNameForHealth(container) {
		return 0, "", fmt.Errorf("invalid container name")
	}
	url := fmt.Sprintf("%s://%s:%s%s", scheme, host, port, path)
	timeout := fmt.Sprintf("%d", timeoutSec)
	out, _, err := sshx.RunArgs(client, "docker", "exec", container, "sh", "-lc",
		fmt.Sprintf(`if command -v curl >/dev/null 2>&1; then body=$(mktemp); code=$(curl -s -o "$body" -w '%%{http_code}' -X %s --max-time %s %q); cat "$body"; echo "___DOCKFIN_HC___$code"; rm -f "$body"; elif command -v wget >/dev/null 2>&1; then wget -q -O - --timeout=%s %q 2>/dev/null; echo "___DOCKFIN_HC___200"; else echo NOCLIENT; fi`,
			method, timeout, url, timeout, url))
	raw := strings.TrimSpace(out)
	if raw == "NOCLIENT" || (err != nil && raw == "") {
		return 0, "", fmt.Errorf("no http client in container")
	}
	const marker = "___DOCKFIN_HC___"
	idx := strings.LastIndex(raw, marker)
	if idx < 0 {
		if err != nil {
			return 0, "", fmt.Errorf("health probe failed: %v (%s)", err, raw)
		}
		return 0, "", fmt.Errorf("health probe returned %q", raw)
	}
	body := raw[:idx]
	codeStr := strings.TrimSpace(raw[idx+len(marker):])
	var code int
	if _, scanErr := fmt.Sscanf(codeStr, "%d", &code); scanErr != nil || code <= 0 {
		if err != nil {
			return 0, body, fmt.Errorf("health probe failed: %v (%s)", err, codeStr)
		}
		return 0, body, fmt.Errorf("health probe returned %q", codeStr)
	}
	return code, body, nil
}

func ValidContainerNameForHealth(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-'
		if !ok {
			return false
		}
	}
	return true
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
	workdir := "/data/dockfin/applications/" + req.App.ID.String()
	if req.PullRequestID > 0 {
		workdir = fmt.Sprintf("/data/dockfin/applications/%s-pr-%d", req.App.ID.String(), req.PullRequestID)
	}

	if err := p.dockerLoginIfNeeded(ctx, client, req); err != nil {
		return err
	}

	p.log("fetch", "Pulling image "+full)
	_, errOut, err := sshx.RunArgs(client, "docker", "pull", full)
	if err != nil {
		return fmt.Errorf("docker pull: %v %s", err, errOut)
	}
	if err := p.runPreDeployCommand(client, req, workdir); err != nil {
		return err
	}

	return p.runWithHealthCutover(ctx, client, req, name, full)
}

func (p *Pipeline) dockerLoginIfNeeded(ctx context.Context, client *ssh.Client, req Request) error {
	if req.App == nil || req.App.DockerRegistryID == nil || p.Store == nil {
		return nil
	}
	reg, err := p.Store.GetDockerRegistry(ctx, req.TeamID, *req.App.DockerRegistryID)
	if err != nil {
		return fmt.Errorf("docker registry: %w", err)
	}
	if strings.TrimSpace(reg.Username) == "" || strings.TrimSpace(reg.PasswordEnc) == "" {
		p.log("fetch", "Registry credentials incomplete — skipping docker login")
		return nil
	}
	plain, err := p.Store.Box.DecryptString(reg.PasswordEnc)
	if err != nil {
		return fmt.Errorf("decrypt registry password: %w", err)
	}
	server := strings.TrimSpace(reg.URL)
	if server == "" || server == "docker.io" || server == "https://index.docker.io/v1/" {
		server = ""
	}
	p.log("fetch", "docker login "+reg.Name)
	loginCmd := fmt.Sprintf("printf '%%s' %s | docker login -u %s --password-stdin",
		shellSingleQuote(plain), shellSingleQuote(reg.Username))
	if server != "" {
		loginCmd += " " + shellSingleQuote(server)
	}
	_, errOut, err := sshx.Run(client, loginCmd)
	if err != nil {
		return fmt.Errorf("docker login: %v %s", err, errOut)
	}
	return nil
}

func (p *Pipeline) deployDockerfile(ctx context.Context, buildClient, deployClient *ssh.Client, req Request) error {
	name := containerNameFor(req)
	workdir := "/data/dockfin/applications/" + req.App.ID.String()
	imageTag := "dockfin/" + req.App.ID.String() + ":latest"
	if req.PullRequestID > 0 {
		workdir = fmt.Sprintf("/data/dockfin/applications/%s-pr-%d", req.App.ID.String(), req.PullRequestID)
		imageTag = fmt.Sprintf("dockfin/%s-pr-%d:latest", req.App.ID.String(), req.PullRequestID)
	}

	p.log("prepare", "Preparing remote workdir "+workdir)
	_, errOut, err := sshx.RunArgs(buildClient, "mkdir", "-p", workdir+"/src")
	if err != nil {
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}

	if err := p.dockerLoginIfNeeded(ctx, buildClient, req); err != nil {
		return err
	}

	buildKeys, buildVals := p.dockerBuildtimeEnv(ctx, req)
	dockerfilePath := ""
	inline := strings.TrimSpace(req.App.Dockerfile)
	if inline != "" {
		// Coolify SimpleDockerfile: write pasted content (no Git).
		content := inline
		if injectBuildArgsEnabled(req.App) && len(buildKeys) > 0 {
			content = services.InjectDockerfileBuildArgs(inline, buildKeys)
		}
		dockerfilePath = workdir + "/src/Dockerfile"
		p.log("prepare", "Writing inline Dockerfile")
		if err := sshx.WriteFile(buildClient, dockerfilePath, []byte(content+"\n")); err != nil {
			return fmt.Errorf("write dockerfile: %w", err)
		}
		if err := p.runPreDeployCommand(buildClient, req, workdir); err != nil {
			return err
		}
	} else {
		if req.App.GitRepository == "" {
			return fmt.Errorf("git repository or dockerfile content is required for dockerfile builds")
		}
		if err := p.gitClone(ctx, buildClient, req, workdir+"/src"); err != nil {
			return err
		}
		if err := p.runPreDeployCommand(buildClient, req, workdir); err != nil {
			return err
		}
		if sha := p.resolveGitHEAD(buildClient, workdir+"/src"); sha != "" {
			req.CommitSHA = sha
		}
		dockerfile := req.App.DockerfileLocation
		if dockerfile == "" {
			dockerfile = "/Dockerfile"
		}
		dockerfile = filepath.ToSlash(strings.TrimSpace(dockerfile))
		dockerfile = strings.TrimPrefix(dockerfile, "/")
		if dockerfile == "" || strings.Contains(dockerfile, "..") {
			return fmt.Errorf("invalid dockerfile path")
		}
		cleaned := filepath.ToSlash(filepath.Clean("/" + dockerfile))
		if cleaned == "/" || strings.Contains(cleaned, "..") {
			return fmt.Errorf("invalid dockerfile path")
		}
		dockerfile = strings.TrimPrefix(cleaned, "/")
		srcPath := workdir + "/src/" + dockerfile
		dockerfilePath = srcPath
		if injectBuildArgsEnabled(req.App) && len(buildKeys) > 0 {
			raw, errOut, err := sshx.RunArgs(buildClient, "cat", srcPath)
			if err != nil {
				return fmt.Errorf("read dockerfile: %v %s", err, errOut)
			}
			injected := services.InjectDockerfileBuildArgs(raw, buildKeys)
			dockerfilePath = workdir + "/src/Dockerfile.dockfin-args"
			if err := sshx.WriteFile(buildClient, dockerfilePath, []byte(injected)); err != nil {
				return fmt.Errorf("write injected dockerfile: %w", err)
			}
			p.log("build", "Injected ARG declarations for build-time env vars")
		}
	}

	if p.shouldSkipBuild(buildClient, req, imageTag) {
		if err := p.transferIfNeeded(buildClient, deployClient, req, imageTag); err != nil {
			return err
		}
		if err := p.runBuiltImage(ctx, deployClient, req, name, imageTag); err != nil {
			return err
		}
		p.persistDeployedCommit(buildClient, req, workdir+"/src")
		return nil
	}

	p.log("build", "Building image "+imageTag)
	buildArgs := []string{"docker", "build", "-t", imageTag, "-f", dockerfilePath}
	if target := strings.TrimSpace(req.App.DockerfileTargetBuild); target != "" {
		buildArgs = append(buildArgs, "--target", target)
	}
	if req.ForceRebuild || (req.App != nil && req.App.IsDisableBuildCache) {
		buildArgs = append(buildArgs, "--no-cache")
	}
	for _, k := range buildKeys {
		buildArgs = append(buildArgs, "--build-arg", k+"="+buildVals[k])
	}
	if req.App.IncludeSourceCommitInBuild {
		if commit := deployCommit(req); commit != "" {
			buildArgs = append(buildArgs, "--build-arg", "SOURCE_COMMIT="+commit)
		}
	}
	secretFlags, secretExports := p.dockerBuildSecretArgs(ctx, req)
	if len(secretFlags) > 0 {
		buildArgs = append(buildArgs, secretFlags...)
	}
	buildArgs = append(buildArgs, workdir+"/src")
	if len(secretExports) > 0 {
		var quoted []string
		for _, a := range buildArgs[1:] {
			quoted = append(quoted, shellSingleQuote(a))
		}
		buildCmd := strings.Join(secretExports, "\n") + "\nexport DOCKER_BUILDKIT=1\ndocker " + strings.Join(quoted, " ")
		_, errOut, err = sshx.Run(buildClient, buildCmd)
	} else {
		_, errOut, err = sshx.RunArgs(buildClient, buildArgs...)
	}
	if err != nil {
		return fmt.Errorf("docker build: %v %s", err, errOut)
	}

	if err := p.transferIfNeeded(buildClient, deployClient, req, imageTag); err != nil {
		return err
	}
	if err := p.runBuiltImage(ctx, deployClient, req, name, imageTag); err != nil {
		return err
	}
	p.persistDeployedCommit(buildClient, req, workdir+"/src")
	return nil
}

// dockerBuildtimeEnv returns sorted build-time env keys and their values.
// Production builds exclude is_preview vars; PR preview builds merge production
// then overlay preview overrides (preview wins).
func (p *Pipeline) dockerBuildtimeEnv(ctx context.Context, req Request) (keys []string, vals map[string]string) {
	vals = map[string]string{}
	if p.Store == nil || req.App == nil {
		return nil, vals
	}
	vars, err := p.Store.ListEnvVars(ctx, req.TeamID, "application", req.App.ID, true)
	if err != nil || len(vars) == 0 {
		return nil, vals
	}
	apply := func(previewOnly bool) {
		for _, v := range vars {
			if !v.IsBuildtime || v.IsPreview != previewOnly {
				continue
			}
			if v.Value == "" || strings.ContainsAny(v.Value, "\n\r") {
				continue
			}
			vals[v.Key] = v.Value
		}
	}
	apply(false)
	if req.PullRequestID > 0 {
		apply(true)
	}
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, vals
}

func (p *Pipeline) runBuiltImage(ctx context.Context, client *ssh.Client, req Request, name, imageTag string) error {
	if p.isSwarmDeploy(req) {
		return p.runSwarmService(ctx, client, req, name, imageTag)
	}
	return p.runWithHealthCutover(ctx, client, req, name, imageTag)
}

// runWithHealthCutover starts a candidate container without Traefik labels, waits
// for health, then swaps traffic by replacing the production container. If health
// fails, the previous container is left running.
func (p *Pipeline) runWithHealthCutover(ctx context.Context, client *ssh.Client, req Request, name, image string) error {
	candidate := name + "-new"
	p.log("run", "Starting candidate container "+candidate)

	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", candidate)

	oldRunning := false
	if out, _, err := sshx.RunArgs(client, "docker", "inspect", "-f", "{{.State.Running}}", name); err == nil && strings.TrimSpace(out) == "true" {
		oldRunning = true
	}

	// Omit Traefik labels on the candidate so production traffic stays on the old
	// container until health passes (avoids duplicate Host routers).
	args := []string{
		"docker", "run", "-d",
		"--name", candidate,
	}
	args = append(args, p.dockerRunBaseArgs(ctx, client, req)...)
	args = append(args, image)

	_, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		return fmt.Errorf("docker run: %v %s", err, errOut)
	}

	if err := p.waitHealthy(client, candidate, req.App); err != nil {
		p.log("run", "Candidate unhealthy — removing it and keeping previous container")
		_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", candidate)
		if !oldRunning {
			return err
		}
		return fmt.Errorf("%w (previous container left running)", err)
	}

	p.log("run", "Candidate healthy — cutting over with proxy labels")
	// Production needs Traefik labels; rename cannot add them, so recreate under
	// the final name then drop the unlabeled candidate.
	finalArgs := []string{
		"docker", "run", "-d",
		"--name", name + "-cutover",
	}
	finalArgs = append(finalArgs, p.dockerRunBaseArgs(ctx, client, req)...)
	finalArgs = append(finalArgs, p.proxyLabelArgsReq(req)...)
	finalArgs = append(finalArgs, image)

	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", name+"-cutover")
	_, errOut, err = sshx.RunArgs(client, finalArgs...)
	if err != nil {
		_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", candidate)
		return fmt.Errorf("docker run (cutover): %v %s", err, errOut)
	}

	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", name)
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", candidate)
	_, errOut, err = sshx.RunArgs(client, "docker", "rename", name+"-cutover", name)
	if err != nil {
		return fmt.Errorf("docker rename: %v %s", err, errOut)
	}

	if err := p.runPostDeployCommand(client, req, name); err != nil {
		return err
	}

	p.log("finalize", "Deployment finished")
	if p.Store != nil && req.PullRequestID == 0 {
		_ = p.Store.UpdateApplicationStatus(context.Background(), req.App.ID, "running")
	}
	if p.Store != nil && req.PullRequestID > 0 {
		_ = p.Store.UpdatePreviewStatus(context.Background(), req.TeamID, req.App.ID, req.PullRequestID, "running")
	}
	if err := p.fanOutAdditionalDestinations(ctx, client, req, image); err != nil {
		return err
	}
	if req.App != nil && req.App.DockerImagesToKeep > 0 {
		p.pruneOldImages(client, req.App.ID.String(), req.App.DockerImagesToKeep)
	}
	return nil
}

func restartPolicy(app *store.Application) string {
	if app == nil {
		return "unless-stopped"
	}
	p := strings.TrimSpace(app.CustomDockerRestartPolicy)
	if p == "" {
		p = "unless-stopped"
	}
	if app.CustomDockerMaxRestartCount > 0 && strings.HasPrefix(p, "on-failure") {
		return fmt.Sprintf("on-failure:%d", app.CustomDockerMaxRestartCount)
	}
	return p
}

func (p *Pipeline) gpuArgs(app *store.Application) []string {
	if app == nil || !app.IsGPUEnabled {
		return nil
	}
	if ids := strings.TrimSpace(app.GPUDeviceIDs); ids != "" {
		return []string{"--gpus", fmt.Sprintf("device=%s", ids)}
	}
	if app.GPUCount > 0 {
		return []string{"--gpus", strconv.Itoa(app.GPUCount)}
	}
	return []string{"--gpus", "all"}
}

func (p *Pipeline) stopTimeoutArgs(app *store.Application) []string {
	if app == nil || app.CustomDockerStopTimeout <= 0 {
		return nil
	}
	return []string{"--stop-timeout", strconv.Itoa(app.CustomDockerStopTimeout)}
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
	if err := p.runPreDeployCommand(buildClient, req, workdir); err != nil {
		return err
	}

	srcDir := workdir + "/src"
	if sha := p.resolveGitHEAD(buildClient, srcDir); sha != "" {
		req.CommitSHA = sha
	}

	if p.shouldSkipRebuild(buildClient, req, imageTag) {
		p.log("build", "Skipping rebuild — image unchanged")
		if err := p.transferIfNeeded(buildClient, deployClient, req, imageTag); err != nil {
			return err
		}
		if err := p.runBuiltImage(ctx, deployClient, req, name, imageTag); err != nil {
			return err
		}
		p.persistDeployedCommit(buildClient, req, srcDir)
		return nil
	}

	if install := strings.TrimSpace(req.App.InstallCommand); install != "" {
		p.log("build", "Running install command")
		if _, errOut, err := sshx.Run(buildClient, fmt.Sprintf("cd %s && %s", shellQuotePath(srcDir), install)); err != nil {
			return fmt.Errorf("install command: %v %s", err, errOut)
		}
	}
	if build := strings.TrimSpace(req.App.BuildCommand); build != "" {
		p.log("build", "Running build command")
		if _, errOut, err := sshx.Run(buildClient, fmt.Sprintf("cd %s && %s", shellQuotePath(srcDir), build)); err != nil {
			return fmt.Errorf("build command: %v %s", err, errOut)
		}
	}

	dockerfile, nginxConf := buildStaticDockerfile(req.App)
	p.log("build", "Writing nginx Dockerfile for static site")
	dfPath := workdir + "/src/Dockerfile.dockfin-static"
	if err := sshx.WriteFile(buildClient, dfPath, []byte(dockerfile)); err != nil {
		return fmt.Errorf("write dockerfile: %w", err)
	}
	if err := sshx.WriteFile(buildClient, workdir+"/src/nginx.dockfin.conf", []byte(nginxConf)); err != nil {
		return fmt.Errorf("write nginx.conf: %w", err)
	}

	p.log("build", "Building static image "+imageTag)
	buildArgs := []string{"docker", "build", "-t", imageTag, "-f", dfPath}
	if req.ForceRebuild || (req.App != nil && req.App.IsDisableBuildCache) {
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
	if err := p.runBuiltImage(ctx, deployClient, req, name, imageTag); err != nil {
		return err
	}
	p.persistDeployedCommit(buildClient, req, workdir+"/src")
	return nil
}

func (p *Pipeline) deployRailpack(ctx context.Context, buildClient, deployClient *ssh.Client, req Request) error {
	if req.App.GitRepository == "" {
		return fmt.Errorf("git repository is required for railpack builds")
	}
	name := containerNameFor(req)
	workdir := "/data/dockfin/applications/" + req.App.ID.String()
	imageTag := "dockfin/" + req.App.ID.String() + ":latest"
	if req.PullRequestID > 0 {
		workdir = fmt.Sprintf("/data/dockfin/applications/%s-pr-%d", req.App.ID.String(), req.PullRequestID)
		imageTag = fmt.Sprintf("dockfin/%s-pr-%d:latest", req.App.ID.String(), req.PullRequestID)
	}

	p.log("prepare", "Preparing railpack workdir "+workdir)
	_, errOut, err := sshx.RunArgs(buildClient, "mkdir", "-p", workdir)
	if err != nil {
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}

	if err := p.gitClone(ctx, buildClient, req, workdir+"/src"); err != nil {
		return err
	}
	if err := p.runPreDeployCommand(buildClient, req, workdir); err != nil {
		return err
	}

	if sha := p.resolveGitHEAD(buildClient, workdir+"/src"); sha != "" {
		req.CommitSHA = sha
	}

	if p.shouldSkipRebuild(buildClient, req, imageTag) {
		p.log("build", "Skipping rebuild — image unchanged")
		if err := p.transferIfNeeded(buildClient, deployClient, req, imageTag); err != nil {
			return err
		}
		if err := p.runBuiltImage(ctx, deployClient, req, name, imageTag); err != nil {
			return err
		}
		p.persistDeployedCommit(buildClient, req, workdir+"/src")
		return nil
	}

	srcDir := workdir + "/src"
	base := strings.TrimSpace(req.App.BaseDirectory)
	if base != "" && base != "/" && base != "." {
		joined := services.JoinBaseAndComposePath(base, "/")
		rel := strings.Trim(strings.TrimPrefix(joined, "/"), "/")
		if rel != "" {
			srcDir = srcDir + "/" + rel
		}
	}

	if err := p.ensureRemoteCLI(buildClient, "railpack", "https://railpack.com/install.sh"); err != nil {
		return err
	}
	p.log("build", "Building with railpack CLI")
	envFlags := p.railpackEnvFlags(ctx, req) + railpackCommandEnv(req.App)
	noCache := ""
	if req.ForceRebuild || (req.App != nil && req.App.IsDisableBuildCache) {
		noCache = " --no-cache"
	}
	buildCmd := fmt.Sprintf(
		`set -euo pipefail
export PATH="/usr/local/bin:/usr/bin:$HOME/.local/bin:$PATH"
# Railpack needs a BuildKit backend (same approach as local railpack docs).
if ! docker ps --format '{{.Names}}' 2>/dev/null | grep -qx dockfin-buildkit; then
  docker rm -f dockfin-buildkit >/dev/null 2>&1 || true
  docker run -d --privileged --name dockfin-buildkit moby/buildkit:latest >/dev/null
fi
export BUILDKIT_HOST='docker-container://dockfin-buildkit'
cd %s
railpack build . --name %s%s%s
`,
		shellSingleQuote(srcDir), shellSingleQuote(imageTag), noCache, envFlags,
	)
	_, errOut, err = sshx.Run(buildClient, buildCmd)
	if err != nil {
		return fmt.Errorf("railpack build failed: %v %s", err, errOut)
	}

	if err := p.transferIfNeeded(buildClient, deployClient, req, imageTag); err != nil {
		return err
	}
	if err := p.runBuiltImage(ctx, deployClient, req, name, imageTag); err != nil {
		return err
	}
	p.persistDeployedCommit(buildClient, req, workdir+"/src")
	return nil
}
func (p *Pipeline) ensureRemoteCLI(client *ssh.Client, bin, installURL string) error {
	check := fmt.Sprintf(
		`export PATH="/usr/local/bin:/usr/bin:$HOME/.local/bin:$PATH"; command -v %s`,
		shellSingleQuote(bin),
	)
	if _, _, err := sshx.Run(client, check); err == nil {
		return nil
	}
	p.log("prepare", fmt.Sprintf("Installing %s CLI on build server…", bin))
	install := fmt.Sprintf(
		`set -euo pipefail
export PATH="/usr/local/bin:/usr/bin:$HOME/.local/bin:$PATH"
curl -fsSL %s | bash -s -- -y
export PATH="/usr/local/bin:/usr/bin:$HOME/.local/bin:$PATH"
command -v %s >/dev/null
`,
		shellSingleQuote(installURL), shellSingleQuote(bin),
	)
	_, errOut, err := sshx.Run(client, install)
	if err != nil {
		return fmt.Errorf("install %s: %v %s", bin, err, errOut)
	}
	return nil
}

func (p *Pipeline) railpackEnvFlags(ctx context.Context, req Request) string {
	envMap := p.composeExistingEnv(ctx, req)
	if len(envMap) == 0 {
		return ""
	}
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := envMap[k]
		// Skip empty / multiline secrets that would break the remote shell.
		if v == "" || strings.ContainsAny(v, "\n\r") {
			continue
		}
		b.WriteString(" --env ")
		b.WriteString(shellSingleQuote(k + "=" + v))
	}
	return b.String()
}

func (p *Pipeline) deployCompose(ctx context.Context, client *ssh.Client, req Request) error {
	workdir := "/data/dockfin/applications/" + req.App.ID.String()
	project := "dockfin-" + req.App.ID.String()[:8]
	if req.PullRequestID > 0 {
		workdir = fmt.Sprintf("/data/dockfin/applications/%s-pr-%d", req.App.ID.String(), req.PullRequestID)
		project = fmt.Sprintf("dockfin-%s-pr-%d", req.App.ID.String()[:8], req.PullRequestID)
	}
	p.log("prepare", "Preparing compose workdir "+workdir)
	_, errOut, err := sshx.RunArgs(client, "mkdir", "-p", workdir+"/src")
	if err != nil {
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}

	rawCompose := strings.TrimSpace(req.App.DockerComposeRaw)
	if req.App.GitRepository != "" {
		if err := p.gitClone(ctx, client, req, workdir+"/src"); err != nil {
			return err
		}
	} else if rawCompose != "" {
		p.log("prepare", "Using pasted docker_compose_raw (no Git)")
		composeFile := workdir + "/src/docker-compose.yaml"
		if err := sshx.WriteFile(client, composeFile, []byte(rawCompose+"\n")); err != nil {
			return fmt.Errorf("write compose: %w", err)
		}
		req.App.DockerComposeLocation = "/docker-compose.yaml"
	} else {
		return fmt.Errorf("git repository or docker_compose_raw is required for dockercompose")
	}
	if err := p.runPreDeployCommand(client, req, workdir); err != nil {
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

	if err := p.dockerLoginIfNeeded(ctx, client, req); err != nil {
		return err
	}

	if req.Destination.Kind == "swarm" {
		p.log("run", "docker stack deploy")
		// swarm stack deploy has no --env-file; vars should already be baked via prepare when enabled.
		_, errOut, err = sshx.RunArgs(client, "docker", "stack", "deploy", "-c", deployPath, "--with-registry-auth", project)
	} else {
		p.log("run", "docker compose up -d")
		buildCmd := strings.TrimSpace(req.App.DockerComposeCustomBuildCommand)
		startCmd := strings.TrimSpace(req.App.DockerComposeCustomStartCommand)
		noCache := req.ForceRebuild || req.App.IsDisableBuildCache
		if buildCmd != "" || startCmd != "" {
			if buildCmd != "" {
				if noCache && !strings.Contains(buildCmd, "--no-cache") {
					p.log("run", "warning: disable build cache is on but custom build command may still use cache — add --no-cache to the command")
				}
				p.log("run", "custom compose build")
				_, errOut, err = sshx.Run(client, fmt.Sprintf("cd %s && %s", shellQuotePath(workdir), buildCmd))
				if err != nil {
					return fmt.Errorf("custom build: %v %s", err, errOut)
				}
			} else if noCache {
				buildArgs := []string{"docker", "compose", "-p", project}
				if envFile != "" {
					buildArgs = append(buildArgs, "--env-file", envFile)
				}
				buildArgs = append(buildArgs, "-f", deployPath, "build", "--no-cache")
				p.log("run", "docker compose build --no-cache (before custom start)")
				_, errOut, err = sshx.RunArgs(client, buildArgs...)
				if err != nil {
					return fmt.Errorf("compose build: %v %s", err, errOut)
				}
			}
			if startCmd != "" {
				p.log("run", "custom compose start")
				_, errOut, err = sshx.Run(client, fmt.Sprintf("cd %s && %s", shellQuotePath(workdir), startCmd))
			} else {
				args := []string{"docker", "compose", "-p", project}
				if envFile != "" {
					args = append(args, "--env-file", envFile)
				}
				args = append(args, "-f", deployPath, "up", "-d", "--remove-orphans")
				_, errOut, err = sshx.RunArgs(client, args...)
			}
		} else if noCache {
			buildArgs := []string{"docker", "compose", "-p", project}
			if envFile != "" {
				buildArgs = append(buildArgs, "--env-file", envFile)
			}
			buildArgs = append(buildArgs, "-f", deployPath, "build", "--no-cache")
			p.log("run", "docker compose build --no-cache")
			_, errOut, err = sshx.RunArgs(client, buildArgs...)
			if err != nil {
				return fmt.Errorf("compose build: %v %s", err, errOut)
			}
			args := []string{"docker", "compose", "-p", project}
			if envFile != "" {
				args = append(args, "--env-file", envFile)
			}
			args = append(args, "-f", deployPath, "up", "-d", "--remove-orphans")
			_, errOut, err = sshx.RunArgs(client, args...)
		} else {
			args := []string{"docker", "compose", "-p", project}
			if envFile != "" {
				args = append(args, "--env-file", envFile)
			}
			args = append(args, "-f", deployPath, "up", "-d", "--build", "--remove-orphans")
			_, errOut, err = sshx.RunArgs(client, args...)
		}
	}
	if err != nil {
		return fmt.Errorf("compose deploy: %v %s", err, errOut)
	}
	if err := p.runPostDeployCommand(client, req, ""); err != nil {
		return err
	}
	p.log("finalize", "Compose stack is up")
	if p.Store != nil && req.PullRequestID == 0 {
		_ = p.Store.UpdateApplicationStatus(context.Background(), req.App.ID, "running")
	}
	if p.Store != nil && req.PullRequestID > 0 {
		_ = p.Store.UpdatePreviewStatus(context.Background(), req.TeamID, req.App.ID, req.PullRequestID, "running")
	}
	if err := p.fanOutAdditionalDestinations(ctx, client, req, ""); err != nil {
		return err
	}
	return nil
}

// resolveComposePath picks the compose file after clone.
// Tries the configured location first; if missing (or empty/auto), scans common names.
// Returns absolute remote path and repo-relative location ("/docker-compose.yml").
func (p *Pipeline) resolveComposePath(client *ssh.Client, req Request, srcDir string) (absPath, location string, err error) {
	// Coolify: base_directory + docker_compose_location → effective path.
	joined := services.JoinBaseAndComposePath(req.App.BaseDirectory, req.App.DockerComposeLocation)
	preferred := services.NormalizeComposeLocation(joined)
	if preferred == "" && strings.TrimSpace(req.App.DockerComposeLocation) != "" &&
		strings.TrimSpace(req.App.DockerComposeLocation) != "auto" &&
		strings.TrimSpace(req.App.DockerComposeLocation) != "auto-detect" {
		// Fall back to location alone when join produced nothing useful.
		preferred = services.NormalizeComposeLocation(req.App.DockerComposeLocation)
	}
	if req.App.DockerComposeLocation != "" && preferred == "" &&
		strings.TrimSpace(req.App.DockerComposeLocation) != "auto" &&
		strings.TrimSpace(req.App.DockerComposeLocation) != "auto-detect" {
		return "", "", fmt.Errorf("invalid compose path")
	}
	if preferred != "" {
		rel := strings.TrimPrefix(preferred, "/")
		candidate := srcDir + "/" + rel
		// Ensure candidate stays under srcDir.
		if !strings.HasPrefix(candidate, srcDir+"/") && candidate != srcDir {
			return "", "", fmt.Errorf("compose path escapes repository")
		}
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
	// Auto HTTPS + Let's Encrypt for custom domains; magic sslip/nip stay http://.
	baseURL := proxy.AutoPublicURL(domainList)
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
		Redirect:   req.App.Redirect,
	}
	if req.Destination != nil {
		opts.Network = req.Destination.Network
	}
	if p.Store != nil {
		opts.ExistingEnv = p.composeExistingEnv(ctx, req)
	}
	if user := strings.TrimSpace(req.App.HTTPBasicAuthUsername); user != "" && strings.TrimSpace(req.App.HTTPBasicAuthPasswordEnc) != "" && p.Store != nil {
		if plain, err := p.Store.Box.DecryptString(req.App.HTTPBasicAuthPasswordEnc); err == nil && plain != "" {
			opts.BasicAuthUsers = htpasswdBcryptCompose(user, plain)
		}
	}
	opts.ExtraLabels = proxy.ParseCustomLabels(req.App.CustomLabels)
	if req.App.IsGPUEnabled {
		opts.GPUEnabled = true
		opts.GPUCount = req.App.GPUCount
		p.log("prepare", "GPU enabled — injecting gpus + deploy.resources device reservation (compose)")
	}
	if rp := strings.TrimSpace(req.App.CustomDockerRestartPolicy); rp != "" {
		opts.RestartPolicy = rp
	}
	if req.App.CustomDockerStopTimeout > 0 {
		opts.StopGracePeriodSec = req.App.CustomDockerStopTimeout
	}
	if req.Destination != nil && req.Destination.Kind == "swarm" {
		opts.SwarmReplicas = req.App.SwarmReplicas
		opts.SwarmWorkersOnly = req.App.IsSwarmOnlyWorkerNodes
		for _, c := range strings.FieldsFunc(req.App.SwarmPlacementConstraints, func(r rune) bool {
			return r == ',' || r == '\n' || r == ';'
		}) {
			c = strings.TrimSpace(c)
			if c != "" {
				opts.SwarmPlacementConstraints = append(opts.SwarmPlacementConstraints, c)
			}
		}
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
	if err := sshx.WriteFile(client, preparedPath, []byte(prepared)); err != nil {
		return "", fmt.Errorf("write prepared compose: %w", err)
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
	var (
		m   map[string]string
		err error
	)
	if req.PullRequestID > 0 {
		m, err = p.Store.ResolvedEnvMapPreview(ctx, req.TeamID, "application", req.App.ID, projectID, envID, serverID)
	} else {
		m, err = p.Store.ResolvedEnvMap(ctx, req.TeamID, "application", req.App.ID, projectID, envID, serverID)
	}
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
	if err := sshx.WriteFile(client, envPath, []byte(body)); err != nil {
		return "", err
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
	// Persist compose preview for the General page (raw + deployable).
	preparedStore := prepared
	if !req.App.ComposePrepare {
		preparedStore = raw
	}
	if err := p.Store.UpdateApplicationComposePreview(ctx, req.TeamID, req.App.ID, raw, preparedStore); err != nil {
		p.log("prepare", "Warning: could not persist compose preview: "+err.Error())
	}

	ui := services.CoolifyEnvForUI(raw, fullEnv)
	if len(ui) == 0 {
		ui = services.CoolifyEnvForUI(raw, services.ExtractMagicEnv(prepared))
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
	// Always sync ${VAR}/:- /:? refs even when there are no SERVICE_* magic keys.
	for _, ref := range services.ComposeEnvForUI(raw) {
		_, err := p.Store.UpsertEnvVar(ctx, req.TeamID, "application", req.App.ID, store.UpsertEnvVarInput{
			Key:       ref.Key,
			Value:     ref.Value,
			Runtime:   true,
			Buildtime: true,
			Literal:   true,
			Comment:   ref.Comment,
			KeepValue: true,
		})
		if err != nil {
			p.log("prepare", "Warning: could not sync compose env "+ref.Key+": "+err.Error())
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
	if req.App.IsConsistentContainerNameEnabled {
		if custom := sanitizeContainerName(req.App.CustomInternalName); custom != "" {
			if strings.HasPrefix(custom, "dockfin-") {
				return custom
			}
			return "dockfin-" + custom
		}
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

func htpasswdBcrypt(user, pass string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return user + ":" + string(h)
}

// htpasswdBcryptCompose doubles `$` so Docker Compose variable substitution
// leaves a single `$` for Traefik. Docker run -l (shell-quoted) must use htpasswdBcrypt.
func htpasswdBcryptCompose(user, pass string) string {
	return strings.ReplaceAll(htpasswdBcrypt(user, pass), "$", "$$")
}

func (p *Pipeline) volumeArgs(ctx context.Context, client *ssh.Client, req Request) []string {
	if p.Store == nil || req.App == nil || req.App.BuildPack == "dockercompose" {
		return nil
	}
	vols, err := p.Store.ListVolumes(ctx, req.TeamID, "application", req.App.ID)
	if err != nil || len(vols) == 0 {
		return nil
	}
	var args []string
	for _, v := range vols {
		host := strings.TrimSpace(v.HostPath)
		if host == "" {
			host = "/data/dockfin/applications/" + req.App.ID.String() + "/volumes/" + v.Name
		}
		if client != nil && !v.IsFile {
			_, _, _ = sshx.RunArgs(client, "mkdir", "-p", host)
		} else if client != nil && v.IsFile {
			parent := filepath.Dir(host)
			if parent != "" && parent != "." {
				_, _, _ = sshx.RunArgs(client, "mkdir", "-p", parent)
			}
		}
		mount := v.MountPath
		if !strings.HasPrefix(mount, "/") {
			mount = "/" + mount
		}
		args = append(args, "-v", host+":"+mount)
	}
	return args
}

func (p *Pipeline) customDockerRunArgs(app *store.Application) []string {
	if app == nil {
		return nil
	}
	return splitDockerRunOptions(app.CustomDockerRunOptions)
}

func splitDockerRunOptions(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	for _, r := range raw {
		switch {
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			inQuote = r
		case unicode.IsSpace(r):
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func shellQuotePath(p string) string {
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}

func (p *Pipeline) runPreDeployCommand(client *ssh.Client, req Request, workdir string) error {
	cmd := strings.TrimSpace(req.App.PreDeploymentCommand)
	if cmd == "" {
		return nil
	}
	p.log("prepare", "Running pre-deployment command")
	if cname := strings.TrimSpace(req.App.PreDeploymentCommandContainer); cname != "" {
		_, errOut, err := sshx.RunArgs(client, "docker", "exec", cname, "sh", "-lc", cmd)
		if err == nil {
			return nil
		}
		p.log("prepare", "pre-deploy docker exec failed, trying host: "+errOut)
	}
	if workdir == "" {
		workdir = "/data/dockfin/applications/" + req.App.ID.String()
	}
	_, errOut, err := sshx.Run(client, fmt.Sprintf("mkdir -p %s && cd %s && %s", shellQuotePath(workdir), shellQuotePath(workdir), cmd))
	if err != nil {
		p.log("prepare", "pre-deployment command failed: "+errOut)
		return fmt.Errorf("pre-deployment command: %v %s", err, errOut)
	}
	return nil
}

func (p *Pipeline) runPostDeployCommand(client *ssh.Client, req Request, container string) error {
	cmd := strings.TrimSpace(req.App.PostDeploymentCommand)
	if cmd == "" {
		return nil
	}
	p.log("finalize", "Running post-deployment command")
	target := strings.TrimSpace(req.App.PostDeploymentCommandContainer)
	if target == "" {
		target = container
	}
	if target != "" {
		_, errOut, err := sshx.RunArgs(client, "docker", "exec", target, "sh", "-lc", cmd)
		if err == nil {
			return nil
		}
		p.log("finalize", "post-deploy docker exec failed, trying host: "+errOut)
	}
	workdir := "/data/dockfin/applications/" + req.App.ID.String()
	_, errOut, err := sshx.Run(client, fmt.Sprintf("cd %s && %s", shellQuotePath(workdir), cmd))
	if err != nil {
		p.log("finalize", "post-deployment command failed: "+errOut)
		return fmt.Errorf("post-deployment command: %v %s", err, errOut)
	}
	return nil
}
