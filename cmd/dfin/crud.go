package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func resourceCmd(asJSON bool, stdout, stderr io.Writer, call caller, base, token, team, kind string, rest []string) int {
	if kind == "applications" {
		kind = "apps"
	}
	sub := "list"
	args := rest
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		switch rest[0] {
		case "list", "get", "create", "update", "delete", "set", "unset":
			sub = rest[0]
			args = rest[1:]
		}
	}
	pos, flags := splitFlags(args)
	ids := strings.Fields(pos)
	need := func(n int, usage string) bool {
		if len(ids) < n {
			fmt.Fprintln(stderr, "usage:", usage)
			return false
		}
		return true
	}
	bodyFrom := func(keys ...string) map[string]any {
		body := map[string]any{}
		for _, k := range keys {
			if v, ok := flags[k]; ok && v != "" && v != "true" {
				body[strings.ReplaceAll(k, "-", "_")] = v
			}
		}
		return body
	}
	switch kind {
	case "servers":
		switch sub {
		case "list":
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/servers", nil)
		case "get":
			if !need(1, "dfin servers get <server-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/servers/"+ids[0], nil)
		case "create":
			body := bodyFrom("name", "description", "ip", "user", "proxy", "private-key-id")
			if flags["user"] != "" {
				body["user_name"] = flags["user"]
				delete(body, "user")
			}
			if flags["proxy"] != "" {
				body["proxy_type"] = flags["proxy"]
				delete(body, "proxy")
			}
			if flags["port"] != "" {
				body["port"] = atoiFlag(flags["port"])
			}
			if body["name"] == nil || body["ip"] == nil {
				fmt.Fprintln(stderr, "usage: dfin servers create --name NAME --ip HOST [--port 22] [--user root] [--proxy traefik]")
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/servers", body)
		case "delete":
			if !need(1, "dfin servers delete <server-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodDelete, "/api/v1/servers/"+ids[0], nil)
		default:
			fmt.Fprintln(stderr, "usage: dfin servers list|get|create|delete")
			return 1
		}
	case "projects":
		switch sub {
		case "list":
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/projects", nil)
		case "get":
			if !need(1, "dfin projects get <project-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/projects/"+ids[0], nil)
		case "create":
			if flags["name"] == "" {
				fmt.Fprintln(stderr, "usage: dfin projects create --name NAME [--description text]")
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/projects", bodyFrom("name", "description"))
		case "update":
			if !need(1, "dfin projects update <project-uuid> --name NAME") || flags["name"] == "" {
				fmt.Fprintln(stderr, "usage: dfin projects update <project-uuid> --name NAME [--description text]")
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPatch, "/api/v1/projects/"+ids[0], bodyFrom("name", "description"))
		case "delete":
			if !need(1, "dfin projects delete <project-uuid> --confirm NAME") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodDelete, "/api/v1/projects/"+ids[0], confirmBody(flags))
		default:
			fmt.Fprintln(stderr, "usage: dfin projects list|get|create|update|delete")
			return 1
		}
	case "environments":
		switch sub {
		case "list":
			if !need(1, "dfin environments list <project-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/projects/"+ids[0]+"/environments", nil)
		case "get":
			if !need(2, "dfin environments get <project-uuid> <environment-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/projects/"+ids[0]+"/environments/"+ids[1], nil)
		case "create":
			if !need(1, "dfin environments create <project-uuid> --name NAME") || flags["name"] == "" {
				fmt.Fprintln(stderr, "usage: dfin environments create <project-uuid> --name NAME")
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/projects/"+ids[0]+"/environments", bodyFrom("name", "description"))
		case "update":
			if !need(2, "dfin environments update <project-uuid> <environment-uuid> --name NAME") || flags["name"] == "" {
				fmt.Fprintln(stderr, "usage: dfin environments update <project-uuid> <environment-uuid> --name NAME")
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPatch, "/api/v1/projects/"+ids[0]+"/environments/"+ids[1], bodyFrom("name", "description"))
		case "delete":
			if !need(2, "dfin environments delete <project-uuid> <environment-uuid> --confirm NAME") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodDelete, "/api/v1/projects/"+ids[0]+"/environments/"+ids[1], confirmBody(flags))
		default:
			fmt.Fprintln(stderr, "usage: dfin environments list|get|create|update|delete")
			return 1
		}
	case "apps":
		switch sub {
		case "list":
			path := "/api/v1/applications"
			if flags["environment"] != "" {
				path += "?environment_id=" + flags["environment"]
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, path, nil)
		case "get":
			if !need(1, "dfin apps get <application-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/applications/"+ids[0], nil)
		case "create":
			body := bodyFrom("name", "description", "environment", "git", "branch", "build-pack", "fqdn")
			if flags["environment"] != "" {
				body["environment_id"] = flags["environment"]
				delete(body, "environment")
			}
			if flags["git"] != "" {
				body["git_repository"] = flags["git"]
				delete(body, "git")
			}
			if flags["branch"] != "" {
				body["git_branch"] = flags["branch"]
				delete(body, "branch")
			}
			if flags["build-pack"] != "" {
				body["build_pack"] = flags["build-pack"]
				delete(body, "build_pack")
			}
			if body["name"] == nil || body["environment_id"] == nil {
				fmt.Fprintln(stderr, "usage: dfin apps create --environment ENV --name NAME [--git URL] [--branch main] [--build-pack dockerfile]")
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/applications", body)
		case "delete":
			if !need(1, "dfin apps delete <application-uuid> --confirm NAME --password PASS") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodDelete, "/api/v1/applications/"+ids[0], confirmBody(flags))
		default:
			fmt.Fprintln(stderr, "usage: dfin apps list|get|create|delete")
			return 1
		}
	case "services":
		switch sub {
		case "list":
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/services", nil)
		case "get":
			if !need(1, "dfin services get <service-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/services/"+ids[0], nil)
		case "create":
			body := map[string]any{}
			if flags["environment"] != "" {
				body["environment_id"] = flags["environment"]
			}
			if flags["name"] != "" {
				body["name"] = flags["name"]
			}
			if flags["template"] != "" {
				body["template"] = flags["template"]
			}
			if flags["fqdn"] != "" {
				body["fqdn"] = flags["fqdn"]
			}
			if flags["server"] != "" {
				body["server_id"] = flags["server"]
			}
			if body["name"] == nil || body["environment_id"] == nil {
				fmt.Fprintln(stderr, "usage: dfin services create --environment ENV --name NAME [--template slug] [--server SERVER]")
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/services", body)
		case "delete":
			if !need(1, "dfin services delete <service-uuid> --confirm NAME --password PASS") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodDelete, "/api/v1/services/"+ids[0], confirmBody(flags))
		default:
			fmt.Fprintln(stderr, "usage: dfin services list|get|create|delete")
			return 1
		}
	case "databases":
		switch sub {
		case "list":
			path := "/api/v1/databases"
			if flags["environment"] != "" {
				path += "?environment_id=" + flags["environment"]
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, path, nil)
		case "get":
			if !need(1, "dfin databases get <database-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/databases/"+ids[0], nil)
		case "create":
			body := map[string]any{}
			if flags["environment"] != "" {
				body["environment_id"] = flags["environment"]
			}
			if flags["name"] != "" {
				body["name"] = flags["name"]
			}
			if flags["engine"] != "" {
				body["engine"] = flags["engine"]
			}
			if body["name"] == nil || body["environment_id"] == nil || body["engine"] == nil {
				fmt.Fprintln(stderr, "usage: dfin databases create --environment ENV --name NAME --engine postgres")
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/databases", body)
		case "delete":
			if !need(1, "dfin databases delete <database-uuid> --confirm NAME --password PASS") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodDelete, "/api/v1/databases/"+ids[0], confirmBody(flags))
		default:
			fmt.Fprintln(stderr, "usage: dfin databases list|get|create|delete")
			return 1
		}
	case "env":
		switch sub {
		case "list":
			if flags["resource-type"] == "" || flags["resource-id"] == "" {
				fmt.Fprintln(stderr, "usage: dfin env list --resource-type application --resource-id UUID")
				return 1
			}
			path := "/api/v1/env-vars?resource_type=" + flags["resource-type"] + "&resource_id=" + flags["resource-id"]
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, path, nil)
		case "set":
			if flags["resource-type"] == "" || flags["resource-id"] == "" || flags["key"] == "" {
				fmt.Fprintln(stderr, "usage: dfin env set --resource-type application --resource-id UUID --key KEY --value VALUE")
				return 1
			}
			body := map[string]any{
				"resource_type": flags["resource-type"],
				"resource_id":   flags["resource-id"],
				"key":           flags["key"],
				"value":         flags["value"],
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/env-vars", body)
		case "unset":
			if !need(1, "dfin env unset <var-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodDelete, "/api/v1/env-vars/"+ids[0], nil)
		default:
			fmt.Fprintln(stderr, "usage: dfin env list|set|unset")
			return 1
		}
	case "domains":
		switch sub {
		case "list":
			if !need(1, "dfin domains list <service-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/services/"+ids[0]+"/domains", nil)
		case "create":
			if !need(1, "dfin domains create <service-uuid> --host example.com") || flags["host"] == "" {
				fmt.Fprintln(stderr, "usage: dfin domains create <service-uuid> --host example.com")
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/services/"+ids[0]+"/domains", map[string]any{"host": flags["host"]})
		case "delete":
			if !need(2, "dfin domains delete <service-uuid> <domain-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodDelete, "/api/v1/services/"+ids[0]+"/domains/"+ids[1], nil)
		default:
			fmt.Fprintln(stderr, "usage: dfin domains list|create|delete")
			return 1
		}
	default:
		fmt.Fprintln(stderr, "unknown resource")
		return 1
	}
}

func parityCmd(asJSON bool, stdout, stderr io.Writer, call caller, base, token, team, cmd string, rest []string) int {
	sub := "list"
	args := rest
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		sub = rest[0]
		args = rest[1:]
	}
	pos, flags := splitFlags(args)
	ids := strings.Fields(pos)
	need := func(n int, usage string) bool {
		if len(ids) < n {
			fmt.Fprintln(stderr, "usage:", usage)
			return false
		}
		return true
	}
	switch cmd {
	case "tokens":
		switch sub {
		case "list":
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/api-tokens", nil)
		case "pause", "resume":
			if !need(1, "dfin tokens pause|resume <token-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPatch, "/api/v1/api-tokens/"+ids[0], map[string]any{"paused": sub == "pause"})
		case "roll":
			if !need(1, "dfin tokens roll <token-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/api-tokens/"+ids[0]+"/roll", map[string]any{})
		default:
			fmt.Fprintln(stderr, "usage: dfin tokens list|pause|resume|roll")
			return 1
		}
	case "maintenance":
		if sub != "get" && sub != "set" {
			fmt.Fprintln(stderr, "usage: dfin maintenance get|set <applications|services> <uuid> [--enabled true] [--message text]")
			return 1
		}
		if !need(2, "dfin maintenance "+sub+" <applications|services> <uuid>") {
			return 1
		}
		if ids[0] != "applications" && ids[0] != "services" {
			fmt.Fprintln(stderr, "resource must be applications or services")
			return 1
		}
		path := "/api/v1/" + ids[0] + "/" + ids[1] + "/maintenance"
		if sub == "get" {
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, path, nil)
		}
		enabled := flags["enabled"] == "true" || flags["enabled"] == "1"
		return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPut, path, map[string]any{
			"enabled": enabled,
			"message": flags["message"],
		})
	case "files":
		if !need(2, "dfin files list|read|write|mkdir|rm <applications|services|databases> <uuid> --path /") {
			return 1
		}
		kind := ids[0]
		if kind != "applications" && kind != "services" && kind != "databases" {
			fmt.Fprintln(stderr, "resource must be applications, services, or databases")
			return 1
		}
		if flags["path"] == "" {
			flags["path"] = "/"
		}
		q := url.Values{}
		q.Set("path", flags["path"])
		if flags["container"] != "" {
			q.Set("container", flags["container"])
		}
		basePath := "/api/v1/" + kind + "/" + ids[1] + "/files"
		switch sub {
		case "list":
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, basePath+"?"+q.Encode(), nil)
		case "read":
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, basePath+"/content?"+q.Encode(), nil)
		case "write":
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPut, basePath+"/content", map[string]any{
				"path": flags["path"], "container": flags["container"], "content": flags["content"],
			})
		case "mkdir":
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, basePath+"/mkdir", map[string]any{
				"path": flags["path"], "container": flags["container"],
			})
		case "rm":
			if flags["confirm"] == "" {
				fmt.Fprintln(stderr, "usage: dfin files rm <kind> <uuid> --path /file --confirm filename")
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodDelete, basePath+"/content", map[string]any{
				"path": flags["path"], "container": flags["container"], "confirm": flags["confirm"],
			})
		default:
			fmt.Fprintln(stderr, "usage: dfin files list|read|write|mkdir|rm")
			return 1
		}
	case "flush-cache":
		id := sub
		if id == "list" || strings.HasPrefix(id, "-") {
			fmt.Fprintln(stderr, "usage: dfin flush-cache <database-uuid>")
			return 1
		}
		return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/databases/"+id+"/flush-cache", map[string]any{"confirm": "FLUSH"})
	case "server-health":
		id := sub
		if id == "list" || strings.HasPrefix(id, "-") {
			fmt.Fprintln(stderr, "usage: dfin server-health <server-uuid> [--check]")
			return 1
		}
		path := "/api/v1/servers/" + id + "/health"
		if flags["check"] == "true" {
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, path+"/check", map[string]any{})
		}
		return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, path, nil)
	case "fleet":
		return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/fleet/metrics", nil)
	case "scan":
		repo := sub
		if repo == "list" || strings.HasPrefix(repo, "-") {
			fmt.Fprintln(stderr, "usage: dfin scan <owner/repo> [--branch name]")
			return 1
		}
		return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/git/scan", map[string]any{"repository": repo, "branch": flags["branch"]})
	case "transfer":
		if sub == "list" || len(ids) < 1 {
			fmt.Fprintln(stderr, "usage: dfin transfer <server-uuid> <team-uuid>")
			return 1
		}
		return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/servers/"+sub+"/transfer", map[string]any{"team_id": ids[0]})
	case "vulns":
		switch sub {
		case "list":
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/image-scans", nil)
		case "run":
			if !need(1, "dfin vulns run <application-uuid>") {
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/applications/"+ids[0]+"/image-scans", map[string]any{})
		case "policy":
			if flags["enabled"] != "" || flags["gate"] != "" {
				enabled := flags["enabled"] != "false"
				gate := flags["gate"]
				if gate == "" {
					gate = "off"
				}
				return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPut, "/api/v1/settings/image-scanning", map[string]any{"enabled": enabled, "gate": gate})
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodGet, "/api/v1/settings/image-scanning", nil)
		default:
			fmt.Fprintln(stderr, "usage: dfin vulns | dfin vulns run <application-uuid> | dfin vulns policy")
			return 1
		}
	case "bundle":
		pass := flags["passphrase"]
		if pass == "" {
			pass = os.Getenv("DOCKFIN_BUNDLE_PASSPHRASE")
		}
		switch sub {
		case "export":
			if pass == "" || len(ids) < 1 {
				fmt.Fprintln(stderr, "usage: dfin bundle export <server-uuid> --passphrase TEXT")
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/servers/"+ids[0]+"/bundle", map[string]any{"passphrase": pass})
		case "import":
			path := flags["file"]
			if pass == "" || path == "" {
				fmt.Fprintln(stderr, "usage: dfin bundle import --passphrase TEXT --file path")
				return 1
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			return printCall(asJSON, stdout, stderr, call, base, token, team, http.MethodPost, "/api/v1/server-bundles", map[string]any{"passphrase": pass, "bundle": string(raw)})
		default:
			fmt.Fprintln(stderr, "usage: dfin bundle export|import")
			return 1
		}
	default:
		fmt.Fprintln(stderr, "unknown command")
		return 1
	}
}

func confirmBody(flags map[string]string) map[string]any {
	body := map[string]any{}
	if flags["confirm"] != "" {
		body["confirmation_name"] = flags["confirm"]
	}
	if flags["password"] != "" {
		body["password"] = flags["password"]
	}
	return body
}

func atoiFlag(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
