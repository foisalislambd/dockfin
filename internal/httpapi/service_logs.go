package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/sshx"
	"golang.org/x/crypto/ssh"
)

// serviceProject is the compose project name used for a service stack.
func serviceProject(id uuid.UUID) string {
	return "dockfin-svc-" + id.String()[:8]
}

// serviceContainerName maps a compose unit name to its default container name.
func serviceContainerName(id uuid.UUID, unit string) string {
	return fmt.Sprintf("%s-%s-1", serviceProject(id), unit)
}

// listServiceContainerNames prefers live containers on the host and falls back to
// the names derived from the stored compose file (stack not deployed yet).
func listServiceContainerNames(client *ssh.Client, id uuid.UUID, composeYAML string) []string {
	var names []string
	if client != nil {
		out, _, err := sshx.RunArgs(client, "docker", "ps", "-a",
			"--filter", "label=com.docker.compose.project="+serviceProject(id), "--format", "{{.Names}}")
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				if line = strings.TrimSpace(line); line != "" {
					names = append(names, line)
				}
			}
		}
	}
	if len(names) > 0 {
		return names
	}
	for _, u := range services.ParseComposeUnits(composeYAML) {
		names = append(names, serviceContainerName(id, u.Name))
	}
	return names
}

func (a *API) handleListServiceContainers(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	compose := svc.DockerCompose
	if compose == "" {
		compose = svc.DockerComposeRaw
	}
	var client *ssh.Client
	if serverID, _, err := a.resolveServiceTarget(r.Context(), teamID, svc); err == nil {
		client, _ = a.dialServer(r, serverID)
	}
	names := listServiceContainerNames(client, id, compose)
	if names == nil {
		names = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": names})
}

func (a *API) handleServiceMetrics(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	compose := svc.DockerCompose
	if compose == "" {
		compose = svc.DockerComposeRaw
	}
	serverID, _, err := a.resolveServiceTarget(r.Context(), teamID, svc)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"containers": []any{}})
		return
	}
	client, err := a.dialServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	names := listServiceContainerNames(client, id, compose)
	if len(names) > 12 {
		names = names[:12]
	}
	if len(names) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"containers": []any{}})
		return
	}
	containers, err := dockerStatsCached("svc:"+id.String(), client, names)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": containers})
}

func (a *API) handleServiceLogsStream(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	serverID, _, err := a.resolveServiceTarget(r.Context(), teamID, svc)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	client, err := a.dialServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	compose := svc.DockerCompose
	if compose == "" {
		compose = svc.DockerComposeRaw
	}
	container := strings.TrimSpace(r.URL.Query().Get("container"))
	if container != "" && !strings.HasPrefix(container, "dockfin-") {
		// Accept a bare compose unit name too.
		container = serviceContainerName(id, container)
	}
	if container == "" {
		names := listServiceContainerNames(client, id, compose)
		if len(names) == 0 {
			writeError(w, http.StatusBadRequest, "no containers — deploy first")
			return
		}
		container = names[0]
	}
	tail := 200
	if t := r.URL.Query().Get("tail"); t != "" {
		if n, err := strconv.Atoi(t); err == nil && n > 0 && n <= 5000 {
			tail = n
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	var mu sync.Mutex
	send := func(event, data string) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}
	meta, _ := json.Marshal(map[string]string{"container": container})
	send("meta", string(meta))

	ctx := r.Context()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := sshx.RunArgsStreaming(client, func(line string) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			b, _ := json.Marshal(map[string]string{"line": line})
			send("log", string(b))
		}, "docker", "logs", "-f", "--tail", strconv.Itoa(tail), container); err != nil {
			select {
			case <-ctx.Done():
			default:
				b, _ := json.Marshal(map[string]string{"line": "docker logs: " + err.Error()})
				send("log", string(b))
			}
		}
	}()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			send("done", `{"status":"ended"}`)
			return
		case <-ticker.C:
			send("ping", `{}`)
		}
	}
}
