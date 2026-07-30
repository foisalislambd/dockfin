package proxy

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/goolify/goolify/internal/sshx"
	"golang.org/x/crypto/ssh"
)

const ProxyContainer = TraefikContainer // shared name: goolify-proxy

// StartCaddy runs lucaslorentz/caddy-docker-proxy as the shared goolify-proxy container.
func StartCaddy(client *ssh.Client, image, network string) error {
	if network == "" {
		network = "goolify"
	}
	if err := sshx.EnsureNetwork(client, network); err != nil {
		return err
	}
	_, _, _ = sshx.Run(client, "mkdir -p /data/goolify/proxy/caddy/data /data/goolify/proxy/caddy/config")
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", ProxyContainer)

	if image == "" {
		image = "lucaslorentz/caddy-docker-proxy:2.9-alpine"
	}
	args := []string{
		"docker", "run", "-d",
		"--name", ProxyContainer,
		"--restart", "unless-stopped",
		"--network", network,
		"-p", "80:80",
		"-p", "443:443",
		"-v", "/var/run/docker.sock:/var/run/docker.sock:ro",
		"-v", "/data/goolify/proxy/caddy/data:/data",
		"-v", "/data/goolify/proxy/caddy/config:/config",
		"-e", "CADDY_INGRESS_NETWORKS=" + network,
		image,
	}
	_, errOut, err := sshx.RunArgs(client, args...)
	if err != nil {
		return fmt.Errorf("start caddy: %v %s", err, errOut)
	}
	return nil
}

// CaddyLabels builds docker labels for caddy-docker-proxy.
// When forceHTTPS is true, the site address is the bare hostname (Caddy auto-HTTPS).
// Otherwise HTTP-only routing is used.
func CaddyLabels(appName, fqdn, port string, forceHTTPS bool) []string {
	_ = appName
	host := strings.TrimSpace(fqdn)
	port = strings.TrimSpace(port)
	if host == "" || port == "" || !safeCaddyHost(host) || !safeCaddyPort(port) {
		return nil
	}
	site := host
	if !forceHTTPS {
		if strings.HasPrefix(host, "https://") {
			site = "http://" + strings.TrimPrefix(host, "https://")
		} else if !strings.HasPrefix(host, "http://") {
			site = "http://" + host
		}
	} else {
		site = strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://")
		if site == "" || !safeCaddyHost(site) {
			return nil
		}
	}
	return []string{
		"caddy=" + site,
		fmt.Sprintf("caddy.reverse_proxy={{upstreams %s}}", port),
	}
}

func safeCaddyHost(host string) bool {
	// Allow hostname or http(s)://hostname — reject shell/label metacharacters.
	h := strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://")
	if h == "" || len(h) > 253 {
		return false
	}
	for _, r := range h {
		ok := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '*'
		if !ok {
			return false
		}
	}
	return true
}

func safeCaddyPort(port string) bool {
	if port == "" || len(port) > 5 {
		return false
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// StartProxy starts Traefik or Caddy based on proxyType.
func StartProxy(client *ssh.Client, proxyType, traefikImage, caddyImage, network, acmeEmail string) error {
	if network == "" {
		network = "goolify"
	}
	switch strings.ToLower(proxyType) {
	case "", "traefik":
		return StartTraefik(client, traefikImage, network, acmeEmail)
	case "caddy":
		return StartCaddy(client, caddyImage, network)
	case "none":
		return fmt.Errorf("proxy is disabled for this server")
	default:
		return fmt.Errorf("unsupported proxy_type %q", proxyType)
	}
}
