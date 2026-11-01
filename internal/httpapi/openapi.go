package httpapi

import (
	"encoding/json"
	"html"
	"net/http"
	"sort"
	"strings"

	"github.com/dockfin/dockfin/internal/version"
	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

// openAPIRoute is one method+path the router actually serves.
type openAPIRoute struct {
	Method string
	Path   string
}

func (a *API) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(a.openAPIJSON)
}

func (a *API) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"/>`)
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1"/>`)
	b.WriteString(`<title>Dockfin API</title><style>`)
	b.WriteString(`:root{color-scheme:dark}body{margin:0;font-family:ui-sans-serif,system-ui,sans-serif;background:#0b1220;color:#e5eefc}`)
	b.WriteString(`main{max-width:64rem;margin:2rem auto;padding:0 1.25rem}h1{font-size:1.6rem;margin:0 0 .4rem}`)
	b.WriteString(`p,td{color:#c7d7ee}a,code{color:#5eead4}table{width:100%;border-collapse:collapse;margin-top:1rem}`)
	b.WriteString(`th,td{text-align:left;padding:.45rem .5rem;border-bottom:1px solid #1e2a44;vertical-align:top}`)
	b.WriteString(`th{color:#9fb3d1;font-size:.75rem;letter-spacing:.04em;text-transform:uppercase}.m{font-weight:600;color:#e5eefc}`)
	b.WriteString(`</style></head><body><main><h1>Dockfin API</h1>`)
	b.WriteString(`<p>This index is generated from the running router. Machine-readable spec: <a href="/openapi.json">/openapi.json</a>`)
	b.WriteString(` · <a href="/metrics">/metrics</a> · <a href="/health">/health</a></p><table><thead><tr><th>Method</th><th>Path</th><th>Summary</th><th>Auth</th></tr></thead><tbody>`)
	for _, op := range a.openAPIOps {
		auth := "session or token"
		if op.Public {
			auth = "public"
		}
		b.WriteString("<tr><td class=\"m\">")
		b.WriteString(html.EscapeString(op.Method))
		b.WriteString("</td><td><code>")
		b.WriteString(html.EscapeString(op.Path))
		b.WriteString("</code></td><td>")
		b.WriteString(html.EscapeString(op.Summary))
		b.WriteString("</td><td>")
		b.WriteString(html.EscapeString(auth))
		b.WriteString("</td></tr>")
	}
	b.WriteString(`</tbody></table></main></body></html>`)
	_, _ = w.Write([]byte(b.String()))
}

type documentedOp struct {
	Method  string
	Path    string
	Summary string
	Public  bool
}

func (a *API) buildOpenAPI(mux chi.Routes) {
	ops := collectOperations(mux)
	a.openAPIOps = ops
	doc := buildOpenAPIDocument(ops, a.ratePolicy())
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		raw = []byte(`{"openapi":"3.1.0","info":{"title":"Dockfin API","version":"dev"},"paths":{}}`)
	}
	raw = append(raw, '\n')
	a.openAPIJSON = raw
	// The committed YAML is the default contract. Release builds inject
	// version.Version, which must not churn the checked-in file.
	if info, ok := doc["info"].(map[string]any); ok {
		info["version"] = "dev"
	}
	y, err := yaml.Marshal(doc)
	if err != nil {
		y = []byte("openapi: 3.1.0\n")
	}
	a.openAPIYAML = y
}

func collectOperations(mux chi.Routes) []documentedOp {
	var ops []documentedOp
	_ = chi.Walk(mux, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		method = strings.ToUpper(method)
		if method == "" || method == http.MethodHead || method == http.MethodOptions {
			return nil
		}
		if strings.Contains(route, "*") {
			return nil
		}
		route = cleanRoute(route)
		if route == "" {
			return nil
		}
		ops = append(ops, documentedOp{
			Method:  method,
			Path:    route,
			Summary: summarizeRoute(method, route),
			Public:  isPublicRoute(method, route),
		})
		return nil
	})
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path == ops[j].Path {
			return ops[i].Method < ops[j].Method
		}
		return ops[i].Path < ops[j].Path
	})
	deduped := ops[:0]
	var prev string
	for _, op := range ops {
		key := op.Method + " " + op.Path
		if key == prev {
			continue
		}
		prev = key
		deduped = append(deduped, op)
	}
	return deduped
}

func cleanRoute(route string) string {
	route = strings.TrimSpace(route)
	for strings.Contains(route, "//") {
		route = strings.ReplaceAll(route, "//", "/")
	}
	if route == "" {
		return "/"
	}
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	return route
}

func isPublicRoute(method, path string) bool {
	switch path {
	case "/health", "/metrics", "/openapi.json", "/api/docs", "/api/v1/openapi.json", "/api/v1/version":
		return method == http.MethodGet
	case "/api/v1/auth/register",
		"/api/v1/auth/login",
		"/api/v1/auth/login/2fa",
		"/api/v1/auth/forgot-password",
		"/api/v1/auth/reset-password",
		"/api/v1/webhooks/github/app/events",
		"/api/v1/sentinel/metrics",
		"/api/v1/sentinel/traffic",
		"/api/v1/deploy":
		return method == http.MethodPost || method == http.MethodGet && (path == "/api/v1/deploy")
	case "/api/v1/auth/registration",
		"/api/v1/auth/oauth/providers",
		"/api/v1/webhooks/github/app/callback",
		"/api/v1/webhooks/github/app/manifest",
		"/api/v1/invitations/preview":
		return method == http.MethodGet
	}
	if method == http.MethodGet && strings.HasPrefix(path, "/api/v1/auth/oauth/") {
		return true
	}
	if strings.HasPrefix(path, "/api/v1/webhooks/") {
		return true
	}
	return false
}

func summarizeRoute(method, path string) string {
	if s, ok := routeSummaries[method+" "+path]; ok {
		return s
	}
	trimmed := strings.TrimPrefix(path, "/api/v1")
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return method + " " + path
	}
	parts := strings.Split(trimmed, "/")
	last := parts[len(parts)-1]
	resource := last
	if strings.HasPrefix(last, "{") && len(parts) > 1 {
		resource = singularWord(parts[len(parts)-2])
	}
	resource = strings.ReplaceAll(resource, "-", " ")
	resource = strings.ReplaceAll(resource, "_", " ")
	switch method {
	case http.MethodGet:
		if strings.HasPrefix(last, "{") {
			return "Get " + resource
		}
		return "List " + resource
	case http.MethodPost:
		if !strings.HasPrefix(last, "{") && last != resource {
			return titleWord(strings.ReplaceAll(last, "-", " ")) + " " + parentResource(parts)
		}
		if isActionWord(last) {
			return titleWord(strings.ReplaceAll(last, "-", " ")) + " " + parentResource(parts)
		}
		return "Create " + singularWord(resource)
	case http.MethodPatch, http.MethodPut:
		if isActionWord(last) {
			return titleWord(strings.ReplaceAll(last, "-", " ")) + " " + parentResource(parts)
		}
		return "Update " + resource
	case http.MethodDelete:
		return "Delete " + resource
	default:
		return method + " " + resource
	}
}

func parentResource(parts []string) string {
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasPrefix(parts[i], "{") {
			continue
		}
		if isActionWord(parts[i]) {
			continue
		}
		return singularWord(strings.ReplaceAll(parts[i], "-", " "))
	}
	return "resource"
}

func isActionWord(s string) bool {
	switch s {
	case "deploy", "rollback", "start", "stop", "restart", "cancel", "clone", "validate",
		"freeze", "restore", "import", "regenerate", "rotate-token", "bootstrap-self",
		"generate", "cleanup-unused", "sync", "test", "run", "check", "attach", "enable",
		"disable", "setup", "accept", "switch-team", "logout", "register", "login":
		return true
	default:
		return false
	}
}

func singularWord(s string) string {
	switch s {
	case "environments":
		return "environment"
	case "applications":
		return "application"
	case "databases":
		return "database"
	case "services":
		return "service"
	case "deployments":
		return "deployment"
	case "servers":
		return "server"
	case "projects":
		return "project"
	case "notifications":
		return "notification"
	case "destinations":
		return "destination"
	case "volumes":
		return "volume"
	case "backups":
		return "backup"
	case "previews":
		return "preview"
	case "domains":
		return "domain"
	case "images":
		return "image"
	case "keys":
		return "key"
	case "tags":
		return "tag"
	}
	if strings.HasSuffix(s, "s") && len(s) > 3 {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

func titleWord(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func tagFor(path string) string {
	switch path {
	case "/health", "/metrics", "/openapi.json", "/api/docs":
		return "system"
	}
	rest := strings.TrimPrefix(path, "/api/v1/")
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	if rest == "" || strings.HasPrefix(rest, "{") {
		return "api"
	}
	return rest
}

var routeSummaries = map[string]string{
	"GET /health":                                "Health check",
	"GET /metrics":                               "Prometheus metrics",
	"GET /openapi.json":                          "OpenAPI document",
	"GET /api/v1/openapi.json":                   "OpenAPI document",
	"GET /api/docs":                              "API route index",
	"GET /api/v1/version":                        "Control plane version",
	"GET /api/v1/overview":                       "Team pulse",
	"POST /api/v1/auth/register":                 "Register the first user or a new account",
	"POST /api/v1/auth/login":                    "Log in",
	"POST /api/v1/auth/login/2fa":                "Complete a two-factor login",
	"POST /api/v1/auth/forgot-password":          "Request a password reset",
	"POST /api/v1/auth/reset-password":           "Reset a password",
	"GET /api/v1/auth/me":                        "Current user and team",
	"POST /api/v1/applications/{appID}/deploy":   "Queue an application deployment",
	"POST /api/v1/applications/{appID}/rollback": "Roll back to a finished deployment",
	"PUT /api/v1/projects/{projectID}/environments/{envID}/freeze": "Pause or resume deployments for an environment",
	"POST /api/v1/webhooks/git/{appID}":                            "Git webhook for an application",
	"GET /api/v1/deploy":                                           "Deploy by resource UUID",
	"POST /api/v1/deploy":                                          "Deploy by resource UUID",
}

func buildOpenAPIDocument(ops []documentedOp, policy RatePolicy) map[string]any {
	paths := map[string]any{}
	tagSet := map[string]struct{}{}
	for _, op := range ops {
		tag := tagFor(op.Path)
		tagSet[tag] = struct{}{}
		item, _ := paths[op.Path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[op.Path] = item
		}
		item[strings.ToLower(op.Method)] = operationObject(op, tag)
	}
	tags := make([]any, 0, len(tagSet))
	names := make([]string, 0, len(tagSet))
	for name := range tagSet {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		tags = append(tags, map[string]any{"name": name})
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "Dockfin API",
			"version": version.Version,
			"description": "Self-hosted PaaS control plane. This document is generated from the " +
				"live router, so every path here is a route the server registers. " +
				"Authenticated routes accept a session cookie or a bearer API token. " +
				defaultLimitText(policy),
		},
		"servers": []any{map[string]any{"url": "/"}},
		"tags":    tags,
		"paths":   paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"cookieAuth": map[string]any{
					"type": "apiKey", "in": "cookie", "name": "dockfin_session",
				},
				"bearerAuth": map[string]any{
					"type": "http", "scheme": "bearer",
				},
			},
			"schemas": map[string]any{
				"Error": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"error": map[string]any{"type": "string"},
					},
					"required": []string{"error"},
				},
			},
		},
	}
}

func defaultLimitText(p RatePolicy) string {
	return "Configured rate limits per client IP: " +
		itoa(p.GlobalPerMinute) + " requests/minute globally, " +
		itoa(p.AuthPer15Min) + " auth attempts per 15 minutes, " +
		itoa(p.WebhookPerMinute) + " webhook calls/minute, and " +
		itoa(p.SentinelPerMinute) + " sentinel ingests/minute. " +
		"Set DOCKFIN_RATE_LIMIT_PER_MINUTE, DOCKFIN_AUTH_RATE_LIMIT_PER_15M, or DOCKFIN_WEBHOOK_RATE_LIMIT_PER_MINUTE to override. Zero disables a bucket. /health and /metrics are exempt."
}

func itoa(n int) string {
	return strconvItoa(n)
}

func strconvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func operationObject(op documentedOp, tag string) map[string]any {
	obj := map[string]any{
		"tags":        []string{tag},
		"summary":     op.Summary,
		"operationId": operationID(op.Method, op.Path),
		"responses":   responseObject(op),
	}
	if params := pathParams(op.Path); len(params) > 0 {
		obj["parameters"] = params
	}
	if body := requestBody(op.Method, op.Path); body != nil {
		obj["requestBody"] = body
	}
	if !op.Public {
		obj["security"] = []any{
			map[string]any{"cookieAuth": []any{}},
			map[string]any{"bearerAuth": []any{}},
		}
	}
	return obj
}

func operationID(method, path string) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(method))
	for _, part := range strings.Split(path, "/") {
		if part == "" {
			continue
		}
		part = strings.Trim(part, "{}")
		part = strings.ReplaceAll(part, "-", " ")
		part = strings.ReplaceAll(part, "_", " ")
		for _, word := range strings.Fields(part) {
			if word == "" {
				continue
			}
			b.WriteString(strings.ToUpper(word[:1]))
			b.WriteString(word[1:])
		}
	}
	return b.String()
}

func pathParams(path string) []any {
	var out []any
	for _, part := range strings.Split(path, "/") {
		if !strings.HasPrefix(part, "{") || !strings.HasSuffix(part, "}") {
			continue
		}
		name := strings.Trim(part, "{}")
		out = append(out, map[string]any{
			"name":     name,
			"in":       "path",
			"required": true,
			"schema":   map[string]any{"type": "string"},
		})
	}
	return out
}

func responseObject(op documentedOp) map[string]any {
	res := map[string]any{
		"200": map[string]any{"description": "OK"},
		"429": errorResponse("Rate limit exceeded"),
		"500": errorResponse("Internal error"),
	}
	if !op.Public {
		res["401"] = errorResponse("Unauthorized")
		res["403"] = errorResponse("Forbidden")
	}
	if op.Method != http.MethodGet && op.Method != http.MethodHead {
		res["400"] = errorResponse("Invalid request")
	}
	if strings.HasSuffix(op.Path, "/deploy") || strings.HasSuffix(op.Path, "/rollback") || strings.HasSuffix(op.Path, "/freeze") {
		res["423"] = errorResponse("Deployments are frozen for this environment")
	}
	return res
}

func errorResponse(desc string) map[string]any {
	return map[string]any{
		"description": desc,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": "#/components/schemas/Error"},
			},
		},
	}
}

func requestBody(method, path string) map[string]any {
	schema, ok := requestSchemas[method+" "+path]
	if !ok {
		if method == http.MethodGet || method == http.MethodDelete {
			return nil
		}
		return map[string]any{
			"required": false,
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{"type": "object", "additionalProperties": true},
				},
			},
		}
	}
	return map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{"schema": schema},
		},
	}
}

func objSchema(required []string, props map[string]any) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

var requestSchemas = map[string]any{
	"POST /api/v1/auth/register": objSchema([]string{"email", "name", "password"}, map[string]any{
		"email":    map[string]any{"type": "string"},
		"name":     map[string]any{"type": "string"},
		"password": map[string]any{"type": "string", "minLength": 8},
	}),
	"POST /api/v1/auth/login": objSchema([]string{"email", "password"}, map[string]any{
		"email":    map[string]any{"type": "string"},
		"password": map[string]any{"type": "string"},
	}),
	"POST /api/v1/auth/login/2fa": objSchema([]string{"challenge", "code"}, map[string]any{
		"challenge": map[string]any{"type": "string"},
		"code":      map[string]any{"type": "string"},
	}),
	"POST /api/v1/auth/forgot-password": objSchema([]string{"email"}, map[string]any{
		"email": map[string]any{"type": "string"},
	}),
	"POST /api/v1/auth/reset-password": objSchema([]string{"token", "password"}, map[string]any{
		"token":    map[string]any{"type": "string"},
		"password": map[string]any{"type": "string", "minLength": 8},
	}),
	"POST /api/v1/applications/{appID}/deploy": objSchema(nil, map[string]any{
		"force_rebuild":   map[string]any{"type": "boolean"},
		"commit_sha":      map[string]any{"type": "string"},
		"override_freeze": map[string]any{"type": "boolean", "description": "Owner or admin only. Deploy once while the environment is frozen."},
	}),
	"POST /api/v1/applications/{appID}/rollback": objSchema(nil, map[string]any{
		"force_rebuild":   map[string]any{"type": "boolean"},
		"commit_sha":      map[string]any{"type": "string", "description": "Finished deployment commit. Blank uses the previous finished deploy."},
		"override_freeze": map[string]any{"type": "boolean"},
	}),
	"PUT /api/v1/projects/{projectID}/environments/{envID}/freeze": objSchema([]string{"frozen"}, map[string]any{
		"frozen": map[string]any{"type": "boolean"},
		"reason": map[string]any{"type": "string", "maxLength": 240},
	}),
	"POST /api/v1/services/{serviceID}/deploy": objSchema(nil, map[string]any{
		"override_freeze": map[string]any{"type": "boolean"},
	}),
}
