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
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
)

func (a *API) handleStartApplication(w http.ResponseWriter, r *http.Request) {
	a.runApplicationAction(w, r, "start", "running")
}

func (a *API) handleStopApplication(w http.ResponseWriter, r *http.Request) {
	a.runApplicationAction(w, r, "stop", "exited")
}

func (a *API) handleRestartApplication(w http.ResponseWriter, r *http.Request) {
	a.runApplicationAction(w, r, "restart", "running")
}

func (a *API) runApplicationAction(w http.ResponseWriter, r *http.Request, action, statusOnOK string) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if app.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "application has no destination")
		return
	}
	dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	client, err := a.dialServer(r, dest.ServerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var errOut string
	if app.BuildPack == "dockercompose" {
		errOut, err = a.runAppComposeAction(client, app, action)
	} else {
		cname := "dockfin-" + app.ID.String()
		if action == "stop" && app.CustomDockerStopTimeout > 0 {
			_, errOut, err = sshx.RunArgs(client, "docker", "stop", "-t", strconv.Itoa(app.CustomDockerStopTimeout), cname)
		} else {
			_, errOut, err = sshx.RunArgs(client, "docker", action, cname)
		}
		if err != nil && action == "start" {
			// Container may not exist yet.
			writeError(w, http.StatusBadRequest, "container not found — deploy first")
			return
		}
	}
	if err != nil {
		if action == "stop" {
			// Idempotent stop when nothing is running.
			_ = a.Store.UpdateApplicationStatus(r.Context(), app.ID, "exited")
			app.Status = "exited"
			writeJSON(w, http.StatusOK, appWithLinks(app))
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%v %s", err, strings.TrimSpace(errOut)))
		return
	}

	// Mirror lifecycle to additional destinations (best-effort; primary already succeeded).
	if extras, e := a.Store.ListAdditionalDestinations(r.Context(), teamID, app.ID); e == nil {
		for _, destID := range extras {
			extraDest, e := a.Store.GetDestination(r.Context(), teamID, destID)
			if e != nil {
				continue
			}
			if extraDest.ServerID == dest.ServerID {
				continue
			}
			extraClient, e := a.dialServer(r, extraDest.ServerID)
			if e != nil {
				continue
			}
			if app.BuildPack == "dockercompose" {
				_, _ = a.runAppComposeAction(extraClient, app, action)
			} else {
				cname := "dockfin-" + app.ID.String()
				if action == "stop" && app.CustomDockerStopTimeout > 0 {
					_, _, _ = sshx.RunArgs(extraClient, "docker", "stop", "-t", strconv.Itoa(app.CustomDockerStopTimeout), cname)
				} else {
					_, _, _ = sshx.RunArgs(extraClient, "docker", action, cname)
				}
			}
		}
	}

	_ = a.Store.UpdateApplicationStatus(r.Context(), app.ID, statusOnOK)
	app.Status = statusOnOK
	writeJSON(w, http.StatusOK, appWithLinks(app))
}

func (a *API) runAppComposeAction(client *ssh.Client, app *store.Application, action string) (string, error) {
	workdir := "/data/dockfin/applications/" + app.ID.String()
	project := "dockfin-" + app.ID.String()[:8]
	composePath := resolveAppComposeFile(client, app, workdir)

	if composePath == "" {
		if action == "stop" {
			return a.runAppComposeContainers(client, project, "stop", app.CustomDockerStopTimeout)
		}
		if action == "restart" {
			return a.runAppComposeContainers(client, project, "restart", 0)
		}
		if action == "start" {
			// Prefer starting existing containers; otherwise require a prior deploy.
			if errOut, err := a.runAppComposeContainers(client, project, "start", 0); err == nil {
				return errOut, nil
			}
			return "", fmt.Errorf("compose file missing — deploy first")
		}
		return "", fmt.Errorf("compose file missing — deploy first")
	}

	args := []string{"docker", "compose", "-p", project, "-f", composePath, action}
	if action == "stop" && app.CustomDockerStopTimeout > 0 {
		args = []string{"docker", "compose", "-p", project, "-f", composePath, "stop", "-t", strconv.Itoa(app.CustomDockerStopTimeout)}
	}
	_, errOut, err := sshx.RunArgs(client, args...)
	if err != nil && action == "start" {
		// compose start fails when no containers exist yet — fall back to up -d.
		_, errOut2, err2 := sshx.RunArgs(client, "docker", "compose", "-p", project, "-f", composePath, "up", "-d", "--remove-orphans")
		return errOut2, err2
	}
	return errOut, err
}

// resolveAppComposeFile finds the compose file used by the last successful deploy.
// Prefer docker-compose.dockfin.yml (PrepareCompose output) beside the repo compose path.
func resolveAppComposeFile(client *ssh.Client, app *store.Application, workdir string) string {
	composeRel := services.NormalizeComposeLocation(services.JoinBaseAndComposePath(app.BaseDirectory, app.DockerComposeLocation))
	var candidates []string
	if composeRel != "" {
		srcPath := workdir + "/src" + composeRel
		dir := srcPath
		if i := strings.LastIndex(srcPath, "/"); i >= 0 {
			dir = srcPath[:i]
		}
		if app.ComposePrepare {
			candidates = append(candidates, dir+"/docker-compose.dockfin.yml")
		}
		candidates = append(candidates, srcPath)
	} else if app.ComposePrepare {
		candidates = append(candidates, workdir+"/src/docker-compose.dockfin.yml")
	}
	candidates = append(candidates,
		workdir+"/docker-compose.dockfin.yml",
		workdir+"/docker-compose.yml",
		workdir+"/src/docker-compose.yaml",
		workdir+"/src/docker-compose.yml",
		workdir+"/src/compose.yaml",
		workdir+"/src/compose.yml",
	)
	seen := map[string]bool{}
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		if _, _, e := sshx.RunArgs(client, "test", "-f", c); e == nil {
			return c
		}
	}
	return ""
}

func (a *API) runAppComposeContainers(client *ssh.Client, project, action string, stopTimeout int) (string, error) {
	out, errOut, err := sshx.RunArgs(client, "docker", "ps", "-a", "--filter", "label=com.docker.compose.project="+project, "--format", "{{.Names}}")
	if err != nil {
		return errOut, err
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no containers for project %s", project)
	}
	args := []string{"docker", action}
	if action == "stop" && stopTimeout > 0 {
		args = append(args, "-t", strconv.Itoa(stopTimeout))
	}
	args = append(args, names...)
	_, errOut, err = sshx.RunArgs(client, args...)
	return errOut, err
}

func (a *API) handleListApplicationContainers(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	names := []string{}
	if app.DestinationID == nil {
		writeJSON(w, http.StatusOK, map[string]any{"containers": names})
		return
	}
	dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	client, err := a.dialServer(r, dest.ServerID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"containers": names})
		return
	}
	if app.BuildPack == "dockercompose" {
		project := "dockfin-" + app.ID.String()[:8]
		out, _, err := sshx.RunArgs(client, "docker", "ps", "-a", "--filter", "label=com.docker.compose.project="+project, "--format", "{{.Names}}")
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					names = append(names, line)
				}
			}
		}
	} else {
		cname := "dockfin-" + app.ID.String()
		if out, _, err := sshx.RunArgs(client, "docker", "inspect", "-f", "{{.Name}}", cname); err == nil {
			names = append(names, strings.TrimPrefix(strings.TrimSpace(out), "/"))
		} else {
			names = append(names, cname)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": names})
}

func (a *API) handleApplicationLogsStream(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	app, err := a.Store.GetApplication(r.Context(), teamID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if app.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "application has no destination")
		return
	}
	dest, err := a.Store.GetDestination(r.Context(), teamID, *app.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	client, err := a.dialServer(r, dest.ServerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	container := strings.TrimSpace(r.URL.Query().Get("container"))
	if container == "" {
		if app.BuildPack == "dockercompose" {
			project := "dockfin-" + app.ID.String()[:8]
			out, _, _ := sshx.RunArgs(client, "docker", "ps", "-a", "--filter", "label=com.docker.compose.project="+project, "--format", "{{.Names}}")
			for _, line := range strings.Split(out, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					container = line
					break
				}
			}
			if container == "" {
				writeError(w, http.StatusBadRequest, "no containers — deploy first")
				return
			}
		} else {
			container = "dockfin-" + app.ID.String()
		}
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
		_ = sshx.RunArgsStreaming(client, func(line string) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			b, _ := json.Marshal(map[string]string{"line": line})
			send("log", string(b))
		}, "docker", "logs", "-f", "--tail", strconv.Itoa(tail), container)
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