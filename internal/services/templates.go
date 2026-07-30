package services

import (
	"bufio"
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
	Category    string `json:"category,omitempty"`
	Compose     string `json:"-"`
}

var (
	mu      sync.RWMutex
	catalog map[string]Template
	loaded  bool
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
	// Prefer Goolify's shipped catalog; Coolify checkout is optional fallback.
	dirs := []string{
		os.Getenv("GOOLIFY_TEMPLATES_DIR"),
		"templates/compose",
		filepath.Join("..", "templates", "compose"),
		"coolify/templates/compose",
		filepath.Join("..", "coolify", "templates", "compose"),
	}
	seenDirs := map[string]bool{}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		abs, err := filepath.Abs(dir)
		if err == nil {
			if seenDirs[abs] {
				continue
			}
			seenDirs[abs] = true
		}
		loadDir(dir)
	}
	loaded = true
}

func loadDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
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
		meta := parseCoolifyMeta(string(raw))
		title := meta.name
		if title == "" {
			title = humanizeType(typ)
		}
		desc := meta.slogan
		if desc == "" {
			desc = "One-click service from catalog"
		}
		// File catalog overrides builtins of the same type (richer compose).
		catalog[typ] = Template{
			Type:        typ,
			Name:        title,
			Description: desc,
			Category:    meta.category,
			Compose:     string(raw),
		}
	}
}

type coolifyMeta struct {
	name     string
	slogan   string
	category string
}

func parseCoolifyMeta(raw string) coolifyMeta {
	var m coolifyMeta
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			break
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		switch key {
		case "slogan":
			m.slogan = val
		case "category":
			m.category = val
		case "name", "title":
			m.name = val
		}
	}
	return m
}

func humanizeType(typ string) string {
	title := strings.ReplaceAll(typ, "-", " ")
	if title == "" {
		return typ
	}
	parts := strings.Fields(title)
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func ListTemplates() []Template {
	ensureLoaded()
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Template, 0, len(catalog))
	for _, t := range catalog {
		out = append(out, Template{
			Type: t.Type, Name: t.Name, Description: t.Description, Category: t.Category,
		})
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

// Builtin fallbacks used when templates/compose is missing (dev / minimal installs).
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
