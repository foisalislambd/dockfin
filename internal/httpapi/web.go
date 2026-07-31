package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/goolify/goolify/internal/services"
)

func resolveWebDir(configured string) string {
	candidates := []string{configured, "apps/web/dist", "web", "/app/web", "/opt/goolify/web"}
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		index := filepath.Join(dir, "index.html")
		if st, err := os.Stat(index); err == nil && !st.IsDir() {
			return dir
		}
	}
	return ""
}

func (a *API) mountWebOrRoot(r chi.Router) {
	if dir := services.ResolveLogosDir(); dir != "" {
		fileServer := http.StripPrefix("/svgs/", http.FileServer(http.Dir(dir)))
		r.Get("/svgs/*", func(w http.ResponseWriter, req *http.Request) {
			fileServer.ServeHTTP(w, req)
		})
	}

	webDir := resolveWebDir(a.Cfg.WebDir)
	if webDir != "" {
		fileServer := http.FileServer(http.Dir(webDir))
		r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
			path := req.URL.Path
			if path == "/" {
				http.ServeFile(w, req, filepath.Join(webDir, "index.html"))
				return
			}
			full := filepath.Join(webDir, filepath.Clean("/"+path))
			if !strings.HasPrefix(full, filepath.Clean(webDir)+string(os.PathSeparator)) && filepath.Clean(full) != filepath.Clean(webDir) {
				http.NotFound(w, req)
				return
			}
			if st, err := os.Stat(full); err == nil && !st.IsDir() {
				fileServer.ServeHTTP(w, req)
				return
			}
			http.ServeFile(w, req, filepath.Join(webDir, "index.html"))
		})
		return
	}

	r.Get("/", a.handleRoot)
}

func (a *API) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Goolify API</title>
  <style>
    :root { color-scheme: dark; }
    body { margin: 0; font-family: ui-sans-serif, system-ui, sans-serif; background: #0b1220; color: #e5eefc; }
    main { max-width: 40rem; margin: 4rem auto; padding: 0 1.25rem; }
    h1 { font-size: 1.75rem; margin: 0 0 .5rem; letter-spacing: -.02em; }
    p { color: #9fb3d1; line-height: 1.55; }
    code, a { color: #5eead4; }
    ul { padding-left: 1.1rem; color: #c7d7ee; }
    li { margin: .35rem 0; }
    .box { margin-top: 1.5rem; padding: 1rem 1.1rem; border: 1px solid #1e2a44; border-radius: 12px; background: #111a2e; }
  </style>
</head>
<body>
  <main>
    <h1>Goolify API is running</h1>
    <p>This port serves the control-plane API. Opening <code>/</code> in a browser is not the dashboard UI.</p>
    <div class="box">
      <ul>
        <li>Health: <a href="/health">/health</a></li>
        <li>Version: <a href="/api/v1/version">/api/v1/version</a></li>
        <li>API base: <code>/api/v1</code></li>
      </ul>
    </div>
    <p>Build the UI with <code>cd apps/web &amp;&amp; npm i &amp;&amp; npm run build</code>, set <code>GOOLIFY_WEB_DIR=apps/web/dist</code>, restart <code>goolify serve</code>, then reload this page.</p>
  </main>
</body>
</html>`))
}
