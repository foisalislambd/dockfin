package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr, liveCall))
}

func liveCall(base, token, team, method, path string, body any) (int, string, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, "", err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, strings.TrimRight(base, "/")+path, rdr)
	if err != nil {
		return 0, "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if team != "" {
		req.Header.Set("X-Team-ID", team)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}

type caller func(base, token, team, method, path string, body any) (int, string, error)

func run(args []string, getenv func(string) string, stdout, stderr io.Writer, call caller) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usageText)
		if len(args) == 0 {
			return 1
		}
		return 0
	}
	base := envOr(getenv, "DOCKFIN_URL", "http://localhost:8000")
	token := envOr(getenv, "DOCKFIN_TOKEN", "")
	team := envOr(getenv, "DOCKFIN_TEAM", "")
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "version":
		fmt.Fprintln(stdout, "dfin", version.Version)
		return 0
	case "health":
		url := base + "/health"
		if len(rest) > 0 {
			url = rest[0]
		}
		code, out, err := call(url, "", "", http.MethodGet, "", nil)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, code, out)
		return exitFor(code)
	case "openapi":
		fmt.Fprintln(stdout, strings.TrimRight(base, "/")+"/openapi.json")
		return 0
	case "overview":
		return printCall(stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/overview", nil)
	case "servers":
		return printCall(stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/servers", nil)
	case "apps":
		return printCall(stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/applications", nil)
	case "services":
		return printCall(stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/services", nil)
	case "databases":
		return printCall(stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/databases", nil)
	case "deploy":
		id, flags := splitFlags(rest)
		if id == "" {
			fmt.Fprintln(stderr, "usage: dfin deploy <application-uuid> [--force] [--override-freeze]")
			return 1
		}
		body := map[string]any{
			"force_rebuild":   flags["force"] == "true",
			"override_freeze": flags["override-freeze"] == "true",
		}
		return printCall(stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/applications/"+id+"/deploy", body)
	case "rollback":
		id, flags := splitFlags(rest)
		if id == "" {
			fmt.Fprintln(stderr, "usage: dfin rollback <application-uuid> [--commit sha] [--override-freeze]")
			return 1
		}
		body := map[string]any{
			"force_rebuild":   true,
			"override_freeze": flags["override-freeze"] == "true",
		}
		if sha := flags["commit"]; sha != "" {
			body["commit_sha"] = sha
		}
		return printCall(stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/applications/"+id+"/rollback", body)
	case "stop":
		if len(rest) < 1 || strings.HasPrefix(rest[0], "-") {
			fmt.Fprintln(stderr, "usage: dfin stop <application-uuid>")
			return 1
		}
		return printCall(stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/applications/"+rest[0]+"/stop", nil)
	case "service-deploy":
		id, flags := splitFlags(rest)
		if id == "" {
			fmt.Fprintln(stderr, "usage: dfin service-deploy <service-uuid> [--override-freeze]")
			return 1
		}
		path := "/api/v1/services/" + id + "/deploy"
		if flags["override-freeze"] == "true" {
			path += "?override_freeze=1"
		}
		return printCall(stdout, stderr, call, base, token, team, http.MethodPost, path, nil)
	case "freeze":
		return freezeCmd(stdout, stderr, call, base, token, team, rest, true)
	case "unfreeze":
		return freezeCmd(stdout, stderr, call, base, token, team, rest, false)
	case "logs":
		if len(rest) < 1 || strings.HasPrefix(rest[0], "-") {
			fmt.Fprintln(stderr, "usage: dfin logs <deployment-uuid>")
			return 1
		}
		code, out, err := call(base, token, team, http.MethodGet, "/api/v1/deployments/"+rest[0], nil)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprint(stdout, formatDeployment(out))
		if !strings.HasSuffix(out, "\n") {
			fmt.Fprintln(stdout)
		}
		return exitFor(code)
	default:
		fmt.Fprint(stdout, usageText)
		return 1
	}
}

func freezeCmd(stdout, stderr io.Writer, call caller, base, token, team string, rest []string, frozen bool) int {
	positionals, flagMap := splitFlags(rest)
	ids := strings.Fields(positionals)
	if len(ids) < 2 {
		if frozen {
			fmt.Fprintln(stderr, "usage: dfin freeze <project-uuid> <environment-uuid> [--reason text]")
		} else {
			fmt.Fprintln(stderr, "usage: dfin unfreeze <project-uuid> <environment-uuid>")
		}
		return 1
	}
	body := map[string]any{"frozen": frozen, "reason": flagMap["reason"]}
	path := "/api/v1/projects/" + ids[0] + "/environments/" + ids[1] + "/freeze"
	return printCall(stdout, stderr, call, base, token, team, http.MethodPut, path, body)
}

func printCall(stdout, stderr io.Writer, call caller, base, token, team, method, path string, body any) int {
	code, out, err := call(base, token, team, method, path, body)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, code, out)
	return exitFor(code)
}

func exitFor(code int) int {
	if code >= 400 || code == 0 {
		return 1
	}
	return 0
}

// splitFlags separates a leading positional (possibly several words before a flag)
// from --name and --name value flags. Boolean flags are "true".
func splitFlags(args []string) (positional string, flags map[string]string) {
	flags = map[string]string{}
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "--") {
			pos = append(pos, a)
			continue
		}
		name := strings.TrimPrefix(a, "--")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			flags[name[:eq]] = name[eq+1:]
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			flags[name] = args[i+1]
			i++
			continue
		}
		flags[name] = "true"
	}
	return strings.Join(pos, " "), flags
}

func formatDeployment(raw string) string {
	var doc struct {
		Status       string          `json:"status"`
		CurrentStage string          `json:"current_stage"`
		ErrorMessage string          `json:"error_message"`
		Logs         json.RawMessage `json:"logs"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil || doc.Status == "" && len(doc.Logs) == 0 {
		return raw
	}
	var b strings.Builder
	fmt.Fprintf(&b, "status: %s\n", doc.Status)
	if doc.CurrentStage != "" {
		fmt.Fprintf(&b, "stage: %s\n", doc.CurrentStage)
	}
	if doc.ErrorMessage != "" {
		fmt.Fprintf(&b, "error: %s\n", doc.ErrorMessage)
	}
	var lines []map[string]any
	if len(doc.Logs) > 0 && string(doc.Logs) != "null" {
		_ = json.Unmarshal(doc.Logs, &lines)
	}
	for _, line := range lines {
		stage, _ := line["stage"].(string)
		text, _ := line["line"].(string)
		if text == "" {
			if s, ok := line["message"].(string); ok {
				text = s
			}
		}
		if stage != "" {
			fmt.Fprintf(&b, "[%s] %s\n", stage, text)
		} else if text != "" {
			fmt.Fprintf(&b, "%s\n", text)
		}
	}
	return b.String()
}

func envOr(getenv func(string) string, k, def string) string {
	if v := strings.TrimSpace(getenv(k)); v != "" {
		return v
	}
	return def
}

const usageText = `dfin — Dockfin CLI

Usage:
  dfin version
  dfin health [url]
  dfin openapi
  dfin overview
  dfin servers
  dfin apps
  dfin services
  dfin databases
  dfin deploy <application-uuid> [--force] [--override-freeze]
  dfin rollback <application-uuid> [--commit sha] [--override-freeze]
  dfin stop <application-uuid>
  dfin service-deploy <service-uuid> [--override-freeze]
  dfin freeze <project-uuid> <environment-uuid> [--reason text]
  dfin unfreeze <project-uuid> <environment-uuid>
  dfin logs <deployment-uuid>

Commands exit 0 on HTTP 2xx/3xx and 1 otherwise.

Env:
  DOCKFIN_URL    default http://localhost:8000
  DOCKFIN_TOKEN  session or API bearer token
  DOCKFIN_TEAM   optional team UUID (X-Team-ID)
`
