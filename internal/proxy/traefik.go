package proxy

import (
	"fmt"
	"strings"

	"github.com/goolify/goolify/internal/sshx"
	"golang.org/x/crypto/ssh"
)

const TraefikContainer = "goolify-proxy"

func StartTraefik(client *ssh.Client, image, network string) error {
	if err := sshx.EnsureNetwork(client, network); err != nil {
		return err
	}
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
		image,
		"--providers.docker=true",
		"--providers.docker.exposedbydefault=false",
		"--entrypoints.http.address=:80",
		"--entrypoints.https.address=:443",
		"--certificatesresolvers.letsencrypt.acme.httpchallenge=true",
		"--certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=http",
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
// with TLS and redirects HTTP to HTTPS.
func TraefikLabelsHTTPS(appName, fqdn, port string, forceHTTPS bool) []string {
	router := sanitize(appName)
	labels := []string{
		"traefik.enable=true",
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%s", router, port),
	}
	if forceHTTPS {
		labels = append(labels,
			fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", router, fqdn),
			fmt.Sprintf("traefik.http.routers.%s.entrypoints=https", router),
			fmt.Sprintf("traefik.http.routers.%s.tls=true", router),
			fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=letsencrypt", router),
			fmt.Sprintf("traefik.http.routers.%s-http.rule=Host(`%s`)", router, fqdn),
			fmt.Sprintf("traefik.http.routers.%s-http.entrypoints=http", router),
			fmt.Sprintf("traefik.http.routers.%s-http.middlewares=%s-redirect", router, router),
			fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.scheme=https", router),
			fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.permanent=true", router),
		)
	} else {
		labels = append(labels,
			fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", router, fqdn),
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
