package services

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
)

// DeployParams drives a one-click / custom compose service deploy on a remote host.
type DeployParams struct {
	Store    *store.Store
	Client   *ssh.Client
	TeamID   uuid.UUID
	Service  *store.Service
	Server   *store.Server
	ServerID uuid.UUID
	Dest     *store.Destination
	Force    bool
	Emit     func(stage, line string)
}

// RunDeploy prepares compose (if needed) and runs docker compose up on the remote host.
func RunDeploy(ctx context.Context, p DeployParams) error {
	emit := p.Emit
	if emit == nil {
		emit = func(string, string) {}
	}
	svc := p.Service
	id := svc.ID
	teamID := p.TeamID

	emit("prepare", "Preparing docker compose…")
	composeYAML := svc.DockerCompose
	rawCompose := svc.DockerComposeRaw
	if rawCompose == "" {
		rawCompose = composeYAML
	}

	srv := p.Server
	if srv != nil {
		if publicIP := strings.TrimSpace(srv.PublicIP); publicIP == "" || proxy.IsUnusableMagicIP(publicIP) {
			if detected := detectPublicIP(p.Client); detected != "" {
				_ = p.Store.SetServerPublicIP(ctx, teamID, p.ServerID, detected)
				srv.PublicIP = detected
			}
		}
		needsFQDN := svc.FQDN == "" || proxy.FQDNUsesUnusableMagicIP(svc.FQDN)
		if needsFQDN {
			magicIP := proxy.PreferMagicIP(srv.IP, srv.PublicIP)
			if fqdn := proxy.GenerateFQDN(svc.Name, svc.ID, magicIP, srv.WildcardDomain, srv.MagicDomain); fqdn != "" {
				fqdn = proxy.NormalizeDomains(fqdn)
				if fqdn != svc.FQDN {
					emit("prepare", fmt.Sprintf("Updating domain %s → %s", svc.FQDN, fqdn))
				} else {
					emit("prepare", fmt.Sprintf("Assigned free domain %s", fqdn))
				}
				svc.FQDN = fqdn
				_ = p.Store.UpdateServiceFQDN(ctx, id, fqdn)
			} else if proxy.FQDNUsesUnusableMagicIP(svc.FQDN) {
				emit("prepare", "Warning: server has no public IP — domain still points at localhost. Set Public IP on the server or run Validate.")
			}
		} else if n := proxy.NormalizeDomains(svc.FQDN); n != "" && n != svc.FQDN {
			svc.FQDN = n
			_ = p.Store.UpdateServiceFQDN(ctx, id, n)
			emit("prepare", fmt.Sprintf("Normalized domain → %s", n))
		}
	}

	fqdnHost := proxy.PrimaryHost(svc.FQDN)
	needPrepare := composeYAML == "" || composeYAML == svc.DockerComposeRaw || looksLikeUnpreparedCompose(svc.DockerComposeRaw, composeYAML)
	if fqdnHost != "" && composeYAML != "" {
		wantURL := proxy.AutoPublicURL(svc.FQDN)
		if !strings.Contains(composeYAML, fqdnHost) || !strings.Contains(composeYAML, wantURL) {
			needPrepare = true
		}
		if proxy.WantAutoHTTPS(svc.FQDN) && !strings.Contains(composeYAML, "certresolver") {
			needPrepare = true
		}
	}
	if proxy.FQDNUsesUnusableMagicIP(composeYAML) {
		needPrepare = true
	}

	if needPrepare {
		existing := ExtractMagicEnv(composeYAML)
		for k, v := range loadServiceMagicEnv(ctx, p.Store, teamID, id) {
			existing[k] = v
		}
		opts := PrepareOpts{
			ServiceID:   id.String(),
			BaseURL:     "http://127.0.0.1",
			RouterName:  svc.Name + "-" + id.String()[:8],
			ExistingEnv: existing,
		}
		if p.Dest != nil {
			opts.Network = p.Dest.Network
		}
		if u, host := PreferURLFromMagicEnv(existing); u != "" {
			opts.BaseURL = u
			if host != "" && host == fqdnHost && strings.TrimSpace(svc.FQDN) != "" {
				opts.FQDN = svc.FQDN
			} else {
				opts.FQDN = host
			}
			if host != "" && host != fqdnHost {
				svc.FQDN = proxy.NormalizeDomains(host)
				_ = p.Store.UpdateServiceFQDN(ctx, id, svc.FQDN)
				fqdnHost = proxy.PrimaryHost(svc.FQDN)
				emit("prepare", fmt.Sprintf("Using domain from Environment Variables: %s", svc.FQDN))
			}
			domainForSSL := opts.FQDN
			if strings.TrimSpace(domainForSSL) == "" {
				domainForSSL = host
			}
			if strings.TrimSpace(svc.FQDN) != "" {
				domainForSSL = svc.FQDN
			}
			if proxy.WantAutoHTTPS(domainForSSL) {
				opts.BaseURL = proxy.AutoPublicURL(domainForSSL)
				opts.FQDN = proxy.NormalizeDomains(domainForSSL)
				if proxy.WantAutoHTTPS(svc.FQDN) && strings.Contains(svc.FQDN, ",") {
					opts.FQDN = svc.FQDN
				}
			}
		} else if svc.FQDN != "" {
			opts.BaseURL = proxy.AutoPublicURL(svc.FQDN)
			opts.FQDN = svc.FQDN
		}
		prepared, fullEnv, err := PrepareCompose(rawCompose, opts)
		if err != nil {
			return fmt.Errorf("prepare compose: %w", err)
		}
		composeYAML = prepared
		_ = p.Store.UpdateServiceCompose(ctx, id, prepared)
		syncServiceCoolifyEnv(ctx, p.Store, teamID, id, rawCompose, prepared, fullEnv)
		if svc.FQDN != "" {
			emit("prepare", fmt.Sprintf("Compose prepared · domain %s", svc.FQDN))
			if proxy.WantAutoHTTPS(svc.FQDN) {
				emit("prepare", "Auto SSL enabled (Let's Encrypt) for custom domain")
			}
		} else {
			emit("prepare", "Compose prepared (volumes + magic env)")
		}
	} else {
		emit("prepare", "Using stored compose")
		if svc.FQDN != "" {
			emit("prepare", fmt.Sprintf("Public URL: %s", proxy.PublicURL(svc.FQDN)))
		}
		syncServiceCoolifyEnv(ctx, p.Store, teamID, id, rawCompose, composeYAML, ExtractMagicEnv(composeYAML))
	}

	_ = p.Store.UpdateServiceStatus(ctx, id, "deploying")
	emit("connect", "Connecting to server over SSH…")

	if p.Dest != nil && p.Dest.Network != "" {
		emit("network", fmt.Sprintf("Ensuring Docker network %q…", p.Dest.Network))
		if _, _, err := sshx.RunArgs(p.Client, "docker", "network", "inspect", p.Dest.Network); err != nil {
			err = sshx.RunArgsStreaming(p.Client, func(line string) { emit("network", line) }, "docker", "network", "create", p.Dest.Network)
			if err != nil {
				_ = p.Store.UpdateServiceStatus(ctx, id, "exited")
				return fmt.Errorf("create network: %w", err)
			}
		} else {
			emit("network", "Network already exists")
		}
	}

	remoteDir := "/data/dockfin/services/" + id.String()
	emit("setup", fmt.Sprintf("Creating remote dir %s", remoteDir))
	_, errOut, err := sshx.RunArgs(p.Client, "mkdir", "-p", remoteDir)
	if err != nil {
		_ = p.Store.UpdateServiceStatus(ctx, id, "exited")
		return fmt.Errorf("mkdir: %v %s", err, errOut)
	}

	composePath := remoteDir + "/docker-compose.yml"
	emit("setup", "Writing docker-compose.yml…")
	if err := sshx.WriteFile(p.Client, composePath, []byte(composeYAML)); err != nil {
		_ = p.Store.UpdateServiceStatus(ctx, id, "exited")
		return fmt.Errorf("write compose: %w", err)
	}
	emit("setup", "Compose file written")

	project := "dockfin-svc-" + id.String()[:8]
	upArgs := []string{"docker", "compose", "-p", project, "-f", composePath, "up", "-d", "--remove-orphans"}
	if p.Force {
		upArgs = append(upArgs, "--force-recreate")
		emit("compose", fmt.Sprintf("docker compose -p %s up -d --remove-orphans --force-recreate", project))
	} else {
		emit("compose", fmt.Sprintf("docker compose -p %s up -d --remove-orphans", project))
	}
	err = sshx.RunArgsStreaming(p.Client, func(line string) {
		emit("compose", line)
	}, upArgs...)
	if err != nil {
		_ = p.Store.UpdateServiceStatus(ctx, id, "exited")
		return fmt.Errorf("compose up: %w", err)
	}

	if fqdnHost != "" {
		waitServiceHTTPReady(p.Client, fqdnHost, emit)
	}

	_ = p.Store.UpdateServiceStatus(ctx, id, "running")
	return nil
}

func detectPublicIP(client *ssh.Client) string {
	if client == nil {
		return ""
	}
	out, _, err := sshx.RunArgs(client, "sh", "-c",
		`curl -4 -fsS --max-time 5 https://api.ipify.org 2>/dev/null || curl -4 -fsS --max-time 5 https://ifconfig.me/ip 2>/dev/null || curl -4 -fsS --max-time 5 https://icanhazip.com 2>/dev/null`)
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(out)
	if proxy.IsUnusableMagicIP(ip) {
		return ""
	}
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

func waitServiceHTTPReady(client *ssh.Client, fqdn string, emit func(stage, line string)) {
	host := proxy.PrimaryHost(fqdn)
	if host == "" {
		host = strings.TrimSpace(strings.Split(fqdn, ",")[0])
	}
	if host == "" || client == nil {
		return
	}
	emit("ready", fmt.Sprintf("Waiting for http://%s to become ready…", host))
	deadline := time.Now().Add(90 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		safeHost := strings.ReplaceAll(host, "'", `'\''`)
		cmd := fmt.Sprintf(
			`curl -sS -o /dev/null -w '%%{http_code}' --connect-timeout 2 --max-time 5 -H 'Host: %s' http://127.0.0.1/ 2>/dev/null || echo 000`,
			safeHost,
		)
		out, _, _ := sshx.Run(client, cmd)
		code := strings.TrimSpace(out)
		if i := strings.LastIndexAny(code, "\n\r"); i >= 0 {
			code = strings.TrimSpace(code[i+1:])
		}
		last = code
		n, _ := strconv.Atoi(code)
		if n >= 200 && n < 500 && n != 404 {
			emit("ready", fmt.Sprintf("Service reachable (HTTP %s)", code))
			return
		}
		time.Sleep(1500 * time.Millisecond)
	}
	emit("ready", fmt.Sprintf("Timed out waiting for ready (last HTTP %s) — open the URL in a few seconds", last))
}

func looksLikeUnpreparedCompose(raw, prepared string) bool {
	if prepared == "" || prepared == raw {
		return true
	}
	if strings.Contains(prepared, "$SERVICE_") {
		return true
	}
	if regexp.MustCompile(`(?m)^\s*-\s+SERVICE_(URL|FQDN|PASSWORD|USER)_[A-Z0-9_]+\s*$`).MatchString(prepared) {
		return true
	}
	return hasNamedVolumeMounts(raw) && !hasTopLevelVolumes(prepared)
}

func hasTopLevelVolumes(compose string) bool {
	return regexp.MustCompile(`(?m)^volumes:\s*$`).MatchString(compose) ||
		strings.Contains(compose, "\nvolumes:\n") ||
		strings.HasPrefix(compose, "volumes:\n")
}

func hasNamedVolumeMounts(raw string) bool {
	return regexp.MustCompile(`(?m)^\s*-\s+([a-zA-Z][a-zA-Z0-9_.-]*):/`).MatchString(raw)
}

func loadServiceMagicEnv(ctx context.Context, st *store.Store, teamID, serviceID uuid.UUID) map[string]string {
	out := map[string]string{}
	vars, err := st.ListEnvVars(ctx, teamID, "service", serviceID, true)
	if err != nil {
		return out
	}
	for _, v := range vars {
		if strings.HasPrefix(v.Key, "SERVICE_") && strings.TrimSpace(v.Value) != "" {
			out[v.Key] = strings.TrimSpace(v.Value)
		}
	}
	return out
}

func syncServiceCoolifyEnv(ctx context.Context, st *store.Store, teamID, serviceID uuid.UUID, raw, prepared string, fullEnv map[string]string) {
	ui := CoolifyEnvForUI(raw, fullEnv)
	if len(ui) == 0 {
		ui = CoolifyEnvForUI(raw, ExtractMagicEnv(prepared))
	}
	if len(ui) == 0 {
		return
	}
	keep := map[string]bool{}
	for key, val := range ui {
		keep[key] = true
		preserve := strings.HasPrefix(key, "SERVICE_PASSWORD_") ||
			strings.HasPrefix(key, "SERVICE_BASE64_") ||
			strings.HasPrefix(key, "SERVICE_HEX_") ||
			strings.HasPrefix(key, "SERVICE_USER_")
		bypassLock := strings.HasPrefix(key, "SERVICE_URL_") || strings.HasPrefix(key, "SERVICE_FQDN_")
		_, _ = st.UpsertEnvVar(ctx, teamID, "service", serviceID, store.UpsertEnvVarInput{
			Key:        key,
			Value:      val,
			Runtime:    true,
			Buildtime:  true,
			Literal:    true,
			Comment:    "",
			KeepValue:  preserve,
			BypassLock: bypassLock,
		})
	}
	vars, err := st.ListEnvVars(ctx, teamID, "service", serviceID, false)
	if err != nil {
		return
	}
	for _, v := range vars {
		if !strings.HasPrefix(v.Key, "SERVICE_URL_") && !strings.HasPrefix(v.Key, "SERVICE_FQDN_") {
			continue
		}
		if keep[v.Key] {
			continue
		}
		_ = st.DeleteEnvVar(ctx, teamID, v.ID)
	}
}

// PreferURLFromMagicEnv picks SERVICE_URL_* as public base URL.
func PreferURLFromMagicEnv(env map[string]string) (baseURL, fqdn string) {
	keys := make([]string, 0)
	for k := range env {
		if strings.HasPrefix(k, "SERVICE_URL_") && env[k] != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		raw := strings.TrimSpace(env[k])
		host := proxy.HostFromDomainEntry(raw)
		if host == "" {
			u, err := url.Parse(raw)
			if err != nil || u.Host == "" {
				continue
			}
			host = u.Hostname()
		}
		if proxy.FQDNUsesUnusableMagicIP(host) {
			continue
		}
		return proxy.PublicURL(raw), host
	}
	fqKeys := make([]string, 0)
	for k, v := range env {
		if strings.HasPrefix(k, "SERVICE_FQDN_") && v != "" && !strings.Contains(v, "://") {
			fqKeys = append(fqKeys, k)
		}
	}
	sort.Strings(fqKeys)
	for _, k := range fqKeys {
		host := proxy.HostFromDomainEntry(env[k])
		if host == "" || proxy.FQDNUsesUnusableMagicIP(host) {
			continue
		}
		return proxy.PublicURL(host), host
	}
	return "", ""
}
