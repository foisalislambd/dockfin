package proxy

import (
	"fmt"
	"strings"

	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// SyncMaintenance writes the Traefik maintenance routers and the nginx page.
// An empty config removes them. Traefik watches the dynamic directory, so a
// running proxy picks the change up without a restart.
func SyncMaintenance(client *ssh.Client, network string, cfg MaintenanceConfig) error {
	if client == nil {
		return fmt.Errorf("ssh client required")
	}
	if _, _, err := sshx.RunArgs(client, "mkdir", "-p", "/data/dockfin/proxy/traefik/dynamic", MaintenanceHTMLDir); err != nil {
		return fmt.Errorf("mkdir proxy dir: %w", err)
	}
	if strings.TrimSpace(cfg.TraefikYAML) == "" {
		_, _, _ = sshx.RunArgs(client, "rm", "-f", MaintenanceFile)
		_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", MaintenanceContainer)
		return nil
	}
	if err := sshx.WriteFile(client, MaintenanceFile, []byte(cfg.TraefikYAML)); err != nil {
		return err
	}
	conf := MaintenanceHTMLDir + "/default.conf"
	if err := sshx.WriteFile(client, conf, []byte(cfg.NginxConf)); err != nil {
		return err
	}
	if network == "" {
		network = "dockfin"
	}
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", MaintenanceContainer)
	_, errOut, err := sshx.RunArgs(client,
		"docker", "run", "-d",
		"--name", MaintenanceContainer,
		"--restart", "unless-stopped",
		"--network", network,
		"-v", conf+":/etc/nginx/conf.d/default.conf:ro",
		"nginx:1.27-alpine",
	)
	if err != nil {
		return fmt.Errorf("start maintenance page: %v %s", err, strings.TrimSpace(errOut))
	}
	return nil
}

// SyncEdgeWithHosts writes maintenance and catch-all for Traefik or Caddy.
// Caddy reads /data/dockfin/proxy/caddy/dynamic/Caddyfile. A proxy started
// before that file was mounted needs a restart before Caddy loads it.
func SyncEdgeWithHosts(client *ssh.Client, proxyType, network string, hosts []MaintenanceHost, catchall CatchallConfig) error {
	cfg, err := BuildMaintenanceConfig(hosts)
	if err != nil {
		return err
	}
	if strings.EqualFold(proxyType, "caddy") {
		file, err := BuildCaddyfile(hosts, catchall)
		if err != nil {
			return err
		}
		if err := syncMaintenanceContainer(client, network, cfg); err != nil {
			return err
		}
		if _, _, err := sshx.RunArgs(client, "mkdir", "-p", CaddyDynamicDir); err != nil {
			return fmt.Errorf("mkdir caddy dir: %w", err)
		}
		if err := sshx.WriteFile(client, CaddyfilePath, []byte(file)); err != nil {
			return err
		}
		_, _, _ = sshx.RunArgs(client, "docker", "exec", ProxyContainer, "caddy", "reload", "--config", "/etc/caddy/Caddyfile")
		return nil
	}
	if err := SyncMaintenance(client, network, cfg); err != nil {
		return err
	}
	raw, err := BuildCatchallYAML(catchall)
	if err != nil {
		return err
	}
	return SyncCatchall(client, raw)
}

func syncMaintenanceContainer(client *ssh.Client, network string, cfg MaintenanceConfig) error {
	if _, _, err := sshx.RunArgs(client, "mkdir", "-p", MaintenanceHTMLDir); err != nil {
		return fmt.Errorf("mkdir proxy dir: %w", err)
	}
	if strings.TrimSpace(cfg.NginxConf) == "" {
		_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", MaintenanceContainer)
		return nil
	}
	conf := MaintenanceHTMLDir + "/default.conf"
	if err := sshx.WriteFile(client, conf, []byte(cfg.NginxConf)); err != nil {
		return err
	}
	if network == "" {
		network = "dockfin"
	}
	_, _, _ = sshx.RunArgs(client, "docker", "rm", "-f", MaintenanceContainer)
	_, errOut, err := sshx.RunArgs(client,
		"docker", "run", "-d",
		"--name", MaintenanceContainer,
		"--restart", "unless-stopped",
		"--network", network,
		"-v", conf+":/etc/nginx/conf.d/default.conf:ro",
		"nginx:1.27-alpine",
	)
	if err != nil {
		return fmt.Errorf("start maintenance page: %v %s", err, strings.TrimSpace(errOut))
	}
	return nil
}

// SyncCatchall writes or removes the unknown-host Traefik file.
func SyncCatchall(client *ssh.Client, yaml string) error {
	if client == nil {
		return fmt.Errorf("ssh client required")
	}
	if _, _, err := sshx.RunArgs(client, "mkdir", "-p", "/data/dockfin/proxy/traefik/dynamic"); err != nil {
		return fmt.Errorf("mkdir proxy dir: %w", err)
	}
	if strings.TrimSpace(yaml) == "" {
		_, _, _ = sshx.RunArgs(client, "rm", "-f", CatchallFile)
		return nil
	}
	return sshx.WriteFile(client, CatchallFile, []byte(yaml))
}
