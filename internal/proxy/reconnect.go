package proxy

import (
	"fmt"
	"strings"

	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// SkipProxyNetworkConnect reports networks that must never be attached to dockfin-proxy.
func SkipProxyNetworkConnect(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "bridge", "host", "none", "ingress", "docker_gwbridge":
		return true
	default:
		return false
	}
}

// EnsureProxyJoinedResourceNetworks attaches dockfin-proxy to compose/project
// networks used by Traefik-enabled containers. Docker drops extra attachments
// after a host reboot even when the proxy container itself comes back up.
func EnsureProxyJoinedResourceNetworks(client *ssh.Client) error {
	if client == nil {
		return fmt.Errorf("ssh client is nil")
	}
	script := `
set +e
proxy=dockfin-proxy
docker inspect "$proxy" >/dev/null 2>&1 || exit 0
running=$(docker inspect -f '{{.State.Running}}' "$proxy" 2>/dev/null || echo false)
[ "$running" = "true" ] || exit 0
docker ps -q | while read -r id; do
  [ -n "$id" ] || continue
  name=$(docker inspect -f '{{.Name}}' "$id" 2>/dev/null || true)
  case "$name" in
    /dockfin-proxy) continue ;;
  esac
  en=$(docker inspect -f '{{index .Config.Labels "traefik.enable"}}' "$id" 2>/dev/null || true)
  keep=0
  if [ "$en" = "true" ] || [ "$en" = "True" ] || [ "$en" = "TRUE" ]; then keep=1; fi
  case "$name" in
    /dockfin-*|/dockfin-svc-*) keep=1 ;;
  esac
  [ "$keep" = 1 ] || continue
  docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{println $k}}{{end}}' "$id" 2>/dev/null
done | sort -u | while read -r net; do
  [ -n "$net" ] || continue
  case "$net" in
    bridge|host|none|ingress|docker_gwbridge) continue ;;
  esac
  docker network connect "$net" "$proxy" >/dev/null 2>&1 || true
done
exit 0
`
	_, errOut, err := sshx.RunArgs(client, "sh", "-c", script)
	if err != nil {
		return fmt.Errorf("proxy network reconnect: %v %s", err, strings.TrimSpace(errOut))
	}
	return nil
}
