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
	Logo        string `json:"logo,omitempty"`
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
	// Prefer Dockfin's shipped catalog (optional DOCKFIN_TEMPLATES_DIR override).
	dirs := []string{
		os.Getenv("DOCKFIN_TEMPLATES_DIR"),
		"templates/compose",
		filepath.Join("..", "templates", "compose"),
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
		logo := normalizeLogoPath(meta.logo)
		if logo == "" {
			logo = guessLogo(typ)
		}
		// File catalog overrides builtins of the same type (richer compose).
		catalog[typ] = Template{
			Type:        typ,
			Name:        title,
			Description: desc,
			Category:    meta.category,
			Logo:        logo,
			Compose:     string(raw),
		}
	}
}

type coolifyMeta struct {
	name     string
	slogan   string
	category string
	logo     string
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
		case "logo":
			m.logo = val
		}
	}
	return m
}

// normalizeLogoPath turns Coolify "svgs/n8n.png" into a public "/svgs/n8n.png" URL.
func normalizeLogoPath(logo string) string {
	logo = strings.TrimSpace(logo)
	if logo == "" {
		return ""
	}
	logo = strings.TrimPrefix(logo, "./")
	if strings.HasPrefix(logo, "http://") || strings.HasPrefix(logo, "https://") {
		return logo
	}
	if strings.HasPrefix(logo, "/svgs/") {
		return logo
	}
	if strings.HasPrefix(logo, "svgs/") {
		return "/" + logo
	}
	if strings.Contains(logo, "/") {
		base := filepath.Base(logo)
		return "/svgs/" + base
	}
	return "/svgs/" + logo
}

func LogosDirs() []string {
	dirs := []string{
		os.Getenv("DOCKFIN_LOGOS_DIR"),
		"templates/svgs",
		filepath.Join("..", "templates", "svgs"),
		"apps/web/public/svgs",
		"apps/web/dist/svgs",
		"/opt/dockfin/svgs",
	}
	var out []string
	seen := map[string]bool{}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if seen[abs] {
			continue
		}
		st, err := os.Stat(abs)
		if err != nil || !st.IsDir() {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
	return out
}

func ResolveLogosDir() string {
	dirs := LogosDirs()
	if len(dirs) == 0 {
		return ""
	}
	return dirs[0]
}

func logoFileExists(filename string) bool {
	for _, dir := range LogosDirs() {
		if st, err := os.Stat(filepath.Join(dir, filename)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func guessLogo(typ string) string {
	base := typ
	// strip common suffixes for variants (e.g. n8n-with-postgres → n8n)
	for _, sep := range []string{"-with-", "-and-"} {
		if i := strings.Index(base, sep); i > 0 {
			base = base[:i]
			break
		}
	}
	candidates := []string{
		typ + ".svg", typ + ".png", typ + ".webp",
		base + ".svg", base + ".png", base + ".webp",
		strings.ReplaceAll(typ, "-", "") + ".svg",
		strings.ReplaceAll(base, "-", "") + ".svg",
	}
	// known Coolify filename quirks
		aliases := map[string][]string{
		"pocket-id":   {"pocketid-logo.png", "pocket-id.svg"},
		"denoKV":      {"denokv.svg", "deno.svg"},
		"pydio-cells": {"cells.svg", "pydio.svg"},
		"goatcounter": {"goatcounter.svg", "goatcounter.png"},
		"vaultwarden": {"bitwarden.svg", "vaultwarden.svg"},
	}
	if extra, ok := aliases[typ]; ok {
		candidates = append(extra, candidates...)
	}
	if extra, ok := aliases[base]; ok {
		candidates = append(extra, candidates...)
	}
	for _, name := range candidates {
		if logoFileExists(name) {
			return "/svgs/" + name
		}
	}
	return ""
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
		logo := t.Logo
		if logo == "" {
			logo = guessLogo(t.Type)
		}
		out = append(out, Template{
			Type: t.Type, Name: t.Name, Description: t.Description, Category: t.Category, Logo: logo,
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
	if t.Logo == "" {
		t.Logo = guessLogo(t.Type)
	}
	return t, nil
}

func LogoForType(typ string) string {
	if t, err := GetTemplate(typ); err == nil && t.Logo != "" {
		return t.Logo
	}
	return guessLogo(typ)
}

// Builtin fallbacks used when templates/compose is missing (dev / minimal installs).
var builtin = map[string]Template{
	"uptime-kuma": {
		Type: "uptime-kuma", Name: "Uptime Kuma", Description: "Self-hosted monitoring tool",
		Logo: "/svgs/uptime-kuma.svg",
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
		Logo: "/svgs/n8n.png",
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
		Logo: "/svgs/bitwarden.svg",
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
