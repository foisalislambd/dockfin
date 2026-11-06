package proxy

import (
	"fmt"
	"strings"

	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

const swarmStackFile = "/data/dockfin/proxy/docker-stack.yml"

// SwarmManager reports whether this Docker daemon is a Swarm manager.
func SwarmManager(client *ssh.Client) bool {
	if client == nil {
		return false
	}
	out, _, err := sshx.RunArgs(client, "docker", "info", "--format", "{{.Swarm.ControlAvailable}}")
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(out), "true")
}

func safeStackToken(s string) bool {
	if s == "" || len(s) > 200 {
		return false
	}
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' || r == '/' || r == ':'
		if !ok {
			return false
		}
	}
	return true
}

// CaddyStackCompose is a Swarm stack for caddy-docker-proxy.
// The dynamic Caddyfile is bind-mounted so maintenance and catch-all survive a service update.
func CaddyStackCompose(image, network string) (string, error) {
	if image == "" {
		image = "lucaslorentz/caddy-docker-proxy:2.9-alpine"
	}
	if network == "" {
		network = "dockfin"
	}
	if !safeStackToken(image) || !safeStackToken(network) {
		return "", fmt.Errorf("invalid swarm caddy image or network")
	}
	return fmt.Sprintf(`services:
  caddy:
    image: %s
    ports:
      - target: 80
        published: 80
        protocol: tcp
        mode: host
      - target: 443
        published: 443
        protocol: tcp
        mode: host
    environment:
      CADDY_DOCKER_CADDYFILE_PATH: /etc/caddy/Caddyfile
      CADDY_INGRESS_NETWORKS: %s
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /data/dockfin/proxy/caddy/data:/data
      - /data/dockfin/proxy/caddy/config:/config
      - %s:/dynamic
      - %s:/etc/caddy/Caddyfile
    networks:
      - %s
    deploy:
      replicas: 1
      placement:
        constraints:
          - node.role == manager
networks:
  %s:
    external: true
`, image, network, CaddyDynamicDir, CaddyfilePath, network, network), nil
}

// TraefikStackCompose is a Swarm stack that watches the same dynamic directory as standalone Traefik.
func TraefikStackCompose(image, network, acmeEmail string) (string, error) {
	if image == "" {
		image = "traefik:v3.6"
	}
	if network == "" {
		network = "dockfin"
	}
	if acmeEmail == "" {
		acmeEmail = "admin@sslip.io"
	}
	if !safeStackToken(image) || !safeStackToken(network) || strings.ContainsAny(acmeEmail, "\n\r'\"") {
		return "", fmt.Errorf("invalid swarm traefik settings")
	}
	return fmt.Sprintf(`services:
  traefik:
    image: %s
    command:
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --providers.docker.network=%s
      - --providers.docker.swarmMode=true
      - --providers.file.directory=/dynamic
      - --providers.file.watch=true
      - --entrypoints.http.address=:80
      - --entrypoints.https.address=:443
      - --certificatesresolvers.letsencrypt.acme.httpchallenge=true
      - --certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=http
      - --certificatesresolvers.letsencrypt.acme.email=%s
      - --certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json
      - --api.dashboard=false
    ports:
      - target: 80
        published: 80
        protocol: tcp
        mode: host
      - target: 443
        published: 443
        protocol: tcp
        mode: host
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /data/dockfin/proxy/traefik/letsencrypt:/letsencrypt
      - /data/dockfin/proxy/traefik/dynamic:/dynamic
    networks:
      - %s
    deploy:
      replicas: 1
      placement:
        constraints:
          - node.role == manager
networks:
  %s:
    external: true
`, image, network, acmeEmail, network, network), nil
}

func deploySwarmStack(client *ssh.Client, compose string) error {
	if _, _, err := sshx.RunArgs(client, "mkdir", "-p", "/data/dockfin/proxy"); err != nil {
		return err
	}
	if err := sshx.WriteFile(client, swarmStackFile, []byte(compose)); err != nil {
		return err
	}
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", ProxyContainer)
	_, errOut, err := sshx.RunArgs(client, "docker", "stack", "deploy", "--detach=true", "-c", swarmStackFile, "dockfin-proxy")
	if err != nil {
		return fmt.Errorf("swarm proxy: %v %s", err, strings.TrimSpace(errOut))
	}
	return nil
}

func syncSwarmMaintenance(client *ssh.Client, network string, cfg MaintenanceConfig) error {
	if _, _, err := sshx.RunArgs(client, "mkdir", "-p", MaintenanceHTMLDir); err != nil {
		return err
	}
	_, _, _ = sshx.RunArgs(client, "docker", "service", "rm", MaintenanceContainer)
	if strings.TrimSpace(cfg.NginxConf) == "" {
		return nil
	}
	conf := MaintenanceHTMLDir + "/default.conf"
	if err := sshx.WriteFile(client, conf, []byte(cfg.NginxConf)); err != nil {
		return err
	}
	if network == "" {
		network = "dockfin"
	}
	_, errOut, err := sshx.RunArgs(client,
		"docker", "service", "create",
		"--name", MaintenanceContainer,
		"--network", network,
		"--constraint", "node.role==manager",
		"--mount", "type=bind,source="+conf+",target=/etc/nginx/conf.d/default.conf,readonly",
		"nginx:1.27-alpine",
	)
	if err != nil {
		return fmt.Errorf("swarm maintenance: %v %s", err, strings.TrimSpace(errOut))
	}
	return nil
}

func reloadSwarmCaddy(client *ssh.Client) {
	out, _, err := sshx.RunArgs(client, "docker", "ps", "-q", "--filter", "name=dockfin-proxy")
	if err != nil {
		return
	}
	id := strings.TrimSpace(strings.Split(out, "\n")[0])
	if id == "" {
		return
	}
	_, _, _ = sshx.RunArgs(client, "docker", "exec", id, "caddy", "reload", "--config", "/etc/caddy/Caddyfile")
}
