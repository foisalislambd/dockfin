package deploy

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
)

func injectBuildArgsEnabled(app *store.Application) bool {
	if app == nil {
		return true
	}
	return app.InjectBuildArgsToDockerfile
}

func deployCommit(req Request) string {
	if c := strings.TrimSpace(req.CommitSHA); c != "" && c != "HEAD" {
		return c
	}
	if req.App != nil {
		if c := strings.TrimSpace(req.App.GitCommitSHA); c != "" && c != "HEAD" {
			return c
		}
	}
	return ""
}

func (p *Pipeline) resolveGitHEAD(client *ssh.Client, srcDir string) string {
	out, _, err := sshx.RunArgs(client, "git", "-C", srcDir, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(out)
	if sha == "" || strings.EqualFold(sha, "HEAD") {
		return ""
	}
	return sha
}

func (p *Pipeline) persistDeployedCommit(client *ssh.Client, req Request, srcDir string) {
	if p.Store == nil || req.App == nil || req.PullRequestID > 0 {
		return
	}
	sha := deployCommit(req)
	if sha == "" && srcDir != "" {
		sha = p.resolveGitHEAD(client, srcDir)
	}
	if sha != "" {
		_ = p.Store.UpdateApplicationGitCommitSHA(context.Background(), req.App.ID, sha)
		req.App.GitCommitSHA = sha
	}
}

func appendTraefikMiddleware(labels []string, router, mw string) []string {
	key := fmt.Sprintf("traefik.http.routers.%s.middlewares=", router)
	for i, l := range labels {
		if strings.HasPrefix(l, key) {
			cur := strings.TrimPrefix(l, key)
			if cur == "" {
				labels[i] = key + mw
			} else if !strings.Contains(","+cur+",", ","+mw+",") {
				labels[i] = key + cur + "," + mw
			}
			return labels
		}
	}
	return append(labels, key+mw)
}

func pathPrefixFromFQDN(fqdn string) string {
	raw := strings.TrimSpace(strings.Split(fqdn, ",")[0])
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(u.Path)
	if p == "" || p == "/" {
		return ""
	}
	return p
}

func portsMappingArgs(app *store.Application) []string {
	if app == nil {
		return nil
	}
	raw := strings.TrimSpace(app.PortsMappings)
	if raw == "" {
		return nil
	}
	var args []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	}) {
		part = strings.TrimSpace(part)
		if part == "" || !strings.Contains(part, ":") {
			continue
		}
		args = append(args, "-p", part)
	}
	return args
}

func networkAliasArgs(app *store.Application, network string) []string {
	if app == nil || network == "" {
		return nil
	}
	raw := strings.TrimSpace(app.CustomNetworkAliases)
	if raw == "" {
		return nil
	}
	var args []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	}) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		args = append(args, "--network-alias", part)
	}
	return args
}

func (p *Pipeline) dockerRunBaseArgs(ctx context.Context, client *ssh.Client, req Request) []string {
	args := []string{
		"--restart", restartPolicy(req.App),
		"--network", req.Destination.Network,
	}
	args = append(args, networkAliasArgs(req.App, req.Destination.Network)...)
	args = append(args, portsMappingArgs(req.App)...)
	args = append(args, p.limitArgs(req.App)...)
	args = append(args, p.gpuArgs(req.App)...)
	args = append(args, p.stopTimeoutArgs(req.App)...)
	args = append(args, p.volumeArgs(ctx, client, req)...)
	args = append(args, p.runtimeEnvArgs(ctx, req)...)
	args = append(args, p.customDockerRunArgs(req.App)...)
	return args
}

func (p *Pipeline) shouldSkipRebuild(client *ssh.Client, req Request, imageTag string) bool {
	return p.shouldSkipBuild(client, req, imageTag)
}

func (p *Pipeline) shouldSkipBuild(client *ssh.Client, req Request, imageTag string) bool {
	if req.ForceRebuild || req.App == nil || !req.App.SkipRebuildIfUnchanged {
		return false
	}
	if _, _, err := sshx.RunArgs(client, "docker", "image", "inspect", imageTag); err != nil {
		return false
	}
	commit := strings.TrimSpace(req.CommitSHA)
	if commit == "" || strings.EqualFold(commit, "HEAD") {
		return false
	}
	stored := strings.TrimSpace(req.App.GitCommitSHA)
	if stored == "" || stored == "HEAD" {
		return false
	}
	if commit == stored {
		p.log("build", "Skipping rebuild — image and commit unchanged")
		return true
	}
	return false
}

func (p *Pipeline) pruneAppImages(client *ssh.Client, appID string, keep int) {
	p.pruneOldImages(client, appID, keep)
}

func (p *Pipeline) pruneOldImages(client *ssh.Client, appID string, keep int) {
	if keep <= 0 || client == nil || appID == "" {
		return
	}
	prefix := "dockfin/" + appID
	cmd := fmt.Sprintf(
		`docker images --format '{{.ID}} {{.CreatedAt}}' %s 2>/dev/null | awk 'NR>%d {print $1}' | xargs -r docker rmi -f 2>/dev/null || true`,
		shellSingleQuote(prefix), keep,
	)
	_, _, _ = sshx.Run(client, cmd)
}

func buildStaticDockerfile(app *store.Application) (dockerfile, nginxConf string) {
	pub := strings.TrimSpace(app.PublishDirectory)
	if pub == "" || pub == "/" || pub == "." {
		pub = "."
	}
	pub = strings.TrimPrefix(pub, "/")

	nginx := strings.TrimSpace(app.CustomNginxConfiguration)
	if nginx == "" {
		tryFiles := "try_files $uri $uri/ =404;"
		if app.IsSPA {
			tryFiles = "try_files $uri $uri/ /index.html;"
		}
		nginx = fmt.Sprintf(`server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;
    location / {
        %s
    }
}
`, tryFiles)
	}
	dockerfile = fmt.Sprintf(`FROM nginx:alpine
COPY %s /usr/share/nginx/html
COPY nginx.dockfin.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
`, pub)
	return dockerfile, nginx
}

func railpackCommandEnv(app *store.Application) string {
	if app == nil {
		return ""
	}
	var b strings.Builder
	add := func(k, v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		b.WriteString(" --env ")
		b.WriteString(shellSingleQuote(k + "=" + v))
	}
	add("RAILPACK_INSTALL_CMD", app.InstallCommand)
	add("RAILPACK_BUILD_CMD", app.BuildCommand)
	add("RAILPACK_START_CMD", app.StartCommand)
	add("RAILPACK_STATIC_DIR", app.PublishDirectory)
	return b.String()
}

func sanitizeContainerName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if ok {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return ""
	}
	if len(out) > 63 {
		out = out[:63]
	}
	return out
}

func (p *Pipeline) dockerBuildSecretArgs(ctx context.Context, req Request) (flags []string, exports []string) {
	if req.App == nil || !req.App.UseBuildSecrets || p.Store == nil {
		return nil, nil
	}
	vars, err := p.Store.ListEnvVars(ctx, req.TeamID, "application", req.App.ID, true)
	if err != nil {
		return nil, nil
	}
	for _, v := range vars {
		if !v.IsBuildSecret || strings.TrimSpace(v.Key) == "" {
			continue
		}
		envName := "DOCKFIN_SECRET_" + v.Key
		exports = append(exports, fmt.Sprintf("export %s=%s", envName, shellSingleQuote(v.Value)))
		flags = append(flags, "--secret", fmt.Sprintf("id=%s,env=%s", v.Key, envName))
	}
	return flags, exports
}

func (p *Pipeline) isSwarmDeploy(req Request) bool {
	if req.Destination == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(req.Destination.Kind), "swarm")
}

func (p *Pipeline) runSwarmService(ctx context.Context, client *ssh.Client, req Request, name, image string) error {
	replicas := 1
	if req.App != nil && req.App.SwarmReplicas > 0 {
		replicas = req.App.SwarmReplicas
	}
	p.log("run", fmt.Sprintf("Deploying swarm service %s (replicas=%d)", name, replicas))
	_, _, _ = sshx.RunArgs(client, "docker", "service", "rm", name)

	args := []string{
		"docker", "service", "create",
		"--name", name,
		"--replicas", strconv.Itoa(replicas),
		"--network", req.Destination.Network,
		"--with-registry-auth",
	}
	if req.App != nil {
		if c := strings.TrimSpace(req.App.SwarmPlacementConstraints); c != "" {
			args = append(args, "--constraint", c)
		}
		if req.App.IsSwarmOnlyWorkerNodes {
			args = append(args, "--constraint", "node.role==worker")
		}
		for _, part := range strings.FieldsFunc(strings.TrimSpace(req.App.PortsMappings), func(r rune) bool {
			return r == ',' || r == ' ' || r == ';'
		}) {
			part = strings.TrimSpace(part)
			if part == "" || !strings.Contains(part, ":") {
				continue
			}
			args = append(args, "--publish", part)
		}
	}
	volArgs := p.volumeArgs(ctx, client, req)
	for i := 0; i < len(volArgs); i++ {
		if volArgs[i] != "-v" || i+1 >= len(volArgs) {
			continue
		}
		spec := volArgs[i+1]
		i++
		host, cont, ok := strings.Cut(spec, ":")
		if !ok {
			continue
		}
		contPath := cont
		if j := strings.IndexByte(cont, ':'); j >= 0 {
			contPath = cont[:j]
		}
		args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,destination=%s", host, contPath))
	}
	args = append(args, p.limitArgs(req.App)...)
	args = append(args, p.runtimeEnvArgs(ctx, req)...)
	labelArgs := p.proxyLabelArgsReq(req)
	for i := 0; i < len(labelArgs); i++ {
		if labelArgs[i] == "-l" && i+1 < len(labelArgs) {
			args = append(args, "--label", labelArgs[i+1])
			i++
		}
	}
	args = append(args, image)
	_, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		return fmt.Errorf("docker service create: %v %s", err, errOut)
	}
	if err := p.runPostDeployCommand(client, req, ""); err != nil {
		return err
	}
	p.log("finalize", "Swarm service deployment finished")
	if p.Store != nil && req.PullRequestID == 0 {
		_ = p.Store.UpdateApplicationStatus(context.Background(), req.App.ID, "running")
	}
	if p.Store != nil && req.PullRequestID > 0 {
		_ = p.Store.UpdatePreviewStatus(context.Background(), req.TeamID, req.App.ID, req.PullRequestID, "running")
	}
	if req.App != nil && req.App.DockerImagesToKeep > 0 {
		p.pruneOldImages(client, req.App.ID.String(), req.App.DockerImagesToKeep)
	}
	return p.fanOutAdditionalDestinations(ctx, client, req, image)
}

// waitStartPeriod kept for callers; pipeline waitHealthy already sleeps start period.
func waitStartPeriod(app *store.Application, logfn func(string)) {
	if app == nil || app.HealthCheckStartPeriod <= 0 {
		return
	}
	if logfn != nil {
		logfn(fmt.Sprintf("Waiting health start period %ds", app.HealthCheckStartPeriod))
	}
	time.Sleep(time.Duration(app.HealthCheckStartPeriod) * time.Second)
}

func swarmStackOverrides(app *store.Application) string {
	if app == nil {
		return ""
	}
	var parts []string
	if app.SwarmReplicas > 1 {
		parts = append(parts, fmt.Sprintf("replicas: %d", app.SwarmReplicas))
	}
	if c := strings.TrimSpace(app.SwarmPlacementConstraints); c != "" {
		parts = append(parts, "placement:\n        constraints: ["+strconv.Quote(c)+"]")
	}
	if app.IsSwarmOnlyWorkerNodes {
		parts = append(parts, `placement:
        constraints: ["node.role == worker"]`)
	}
	return strings.Join(parts, "\n      ")
}
