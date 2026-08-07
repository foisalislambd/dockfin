package proxy

import (
	"fmt"
	"strings"

	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

const TraefikContainer = "dockfin-proxy"

func StartTraefik(client *ssh.Client, image, network, acmeEmail string) error {
	if network == "" {
		network = "dockfin"
	}
	if image == "" {
		// Docker Engine 29+ requires Traefik ≥ v3.6 (API negotiation).
		image = "traefik:v3.6"
	}
	if acmeEmail == "" {
		acmeEmail = "admin@sslip.io"
	}
	if err := sshx.EnsureNetwork(client, network); err != nil {
		return err
	}
	_, _, _ = sshx.Run(client, "mkdir -p /data/dockfin/proxy/traefik/letsencrypt && touch /data/dockfin/proxy/traefik/letsencrypt/acme.json && chmod 600 /data/dockfin/proxy/traefik/letsencrypt/acme.json")
	// Remove existing if any
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", TraefikContainer)

	args := []string{
		"docker", "run", "-d",
		"--name", TraefikContainer,
		"--restart", "unless-stopped",
		"--network", network,
		"-p", "80:80",
		"-p", "443:443",
		"-v", "/var/run/docker.sock:/var/run/docker.sock:ro",
		"-v", "/data/dockfin/proxy/traefik/letsencrypt:/letsencrypt",
		image,
		"--providers.docker=true",
		"--providers.docker.exposedbydefault=false",
		"--providers.docker.network=" + network,
		"--entrypoints.http.address=:80",
		"--entrypoints.https.address=:443",
		"--certificatesresolvers.letsencrypt.acme.httpchallenge=true",
		"--certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=http",
		"--certificatesresolvers.letsencrypt.acme.email=" + acmeEmail,
		"--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json",
		"--api.dashboard=false",
	}
	_, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		return fmt.Errorf("start traefik: %v %s", err, errOut)
	}
	return nil
}

func StopProxy(client *ssh.Client) error {
	_, errOut, err := sshx.RunArgs(client, "docker", "rm", "-f", TraefikContainer)
	if err != nil && !strings.Contains(errOut, "No such container") {
		return fmt.Errorf("stop proxy: %v %s", err, errOut)
	}
	return nil
}

func ProxyStatus(client *ssh.Client) string {
	out, _, err := sshx.RunArgs(client, "docker", "inspect", "-f", "{{.State.Status}}", TraefikContainer)
	if err != nil {
		return "exited"
	}
	s := strings.TrimSpace(out)
	if s == "" {
		return "unknown"
	}
	return s
}

// TraefikLabels builds docker labels for an HTTP service.
func TraefikLabels(appName, fqdn, port string) []string {
	return TraefikLabelsHTTPS(appName, fqdn, port, false)
}

// TraefikLabelsHTTPS builds Traefik labels; when forceHTTPS is true, routes HTTPS
// with TLS and redirects HTTP to HTTPS. fqdn may be a Coolify multi-domain list.
func TraefikLabelsHTTPS(appName, fqdn, port string, forceHTTPS bool) []string {
	router := sanitize(appName)
	rule := TraefikHostRule(HostsFromDomainList(fqdn))
	if rule == "" {
		if h := HostFromDomainEntry(fqdn); h != "" {
			rule = fmt.Sprintf("Host(`%s`)", h)
		} else {
			return nil
		}
	}
	labels := []string{
		"traefik.enable=true",
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%s", router, port),
	}
	if forceHTTPS {
		labels = append(labels,
			fmt.Sprintf("traefik.http.routers.%s.rule=%s", router, rule),
			fmt.Sprintf("traefik.http.routers.%s.entrypoints=https", router),
			fmt.Sprintf("traefik.http.routers.%s.tls=true", router),
			fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=letsencrypt", router),
			fmt.Sprintf("traefik.http.routers.%s-http.rule=%s", router, rule),
			fmt.Sprintf("traefik.http.routers.%s-http.entrypoints=http", router),
			fmt.Sprintf("traefik.http.routers.%s-http.middlewares=%s-redirect", router, router),
			fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.scheme=https", router),
			fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.permanent=true", router),
		)
	} else {
		labels = append(labels,
			fmt.Sprintf("traefik.http.routers.%s.rule=%s", router, rule),
			fmt.Sprintf("traefik.http.routers.%s.entrypoints=http", router),
		)
	}
	return labels
}

func sanitize(s string) string {
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
