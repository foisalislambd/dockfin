package services

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Template struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Compose     string `json:"-"`
}

var (
	mu       sync.RWMutex
	catalog  map[string]Template
	loaded   bool
)

func ensureLoaded() {
	mu.Lock()
	defer mu.Unlock()
	if loaded {
		return
	}
	catalog = map[string]Template{}
	for k, v := range builtin {
		catalog[k] = v
	}
	dirs := []string{
		os.Getenv("GOOLIFY_TEMPLATES_DIR"),
		"templates/compose",
		"coolify/templates/compose",
		filepath.Join("..", "coolify", "templates", "compose"),
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			typ := strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
			title := strings.ReplaceAll(typ, "-", " ")
			if len(title) > 0 {
				title = strings.ToUpper(title[:1]) + title[1:]
			}
			catalog[typ] = Template{
				Type:        typ,
				Name:        title,
				Description: "One-click service from catalog",
				Compose:     string(raw),
			}
		}
		break // first successful dir wins extras
	}
	loaded = true
}

func ListTemplates() []Template {
	ensureLoaded()
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Template, 0, len(catalog))
	for _, t := range catalog {
		out = append(out, Template{Type: t.Type, Name: t.Name, Description: t.Description})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func GetTemplate(typ string) (Template, error) {
	ensureLoaded()
	mu.RLock()
	defer mu.RUnlock()
	t, ok := catalog[typ]
	if !ok {
		return Template{}, fmt.Errorf("unknown template %q", typ)
	}
	return t, nil
}

var builtin = map[string]Template{
	"uptime-kuma": {
		Type: "uptime-kuma", Name: "Uptime Kuma", Description: "Self-hosted monitoring tool",
		Compose: `services:
  uptime-kuma:
    image: louislam/uptime-kuma:1
    volumes:
      - uptime-kuma-data:/app/data
    restart: unless-stopped
volumes:
  uptime-kuma-data:
`,
	},
	"n8n": {
		Type: "n8n", Name: "n8n", Description: "Workflow automation",
		Compose: `services:
  n8n:
    image: n8nio/n8n:latest
    volumes:
      - n8n-data:/home/node/.n8n
    restart: unless-stopped
volumes:
  n8n-data:
`,
	},
	"vaultwarden": {
		Type: "vaultwarden", Name: "Vaultwarden", Description: "Bitwarden-compatible password manager",
		Compose: `services:
  vaultwarden:
    image: vaultwarden/server:latest
    volumes:
      - vw-data:/data
    restart: unless-stopped
volumes:
  vw-data:
`,
	},
}
