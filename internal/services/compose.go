package services

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/dockfin/dockfin/internal/proxy"
	"gopkg.in/yaml.v3"
)

type PrepareOpts struct {
	ServiceID   string
	Network     string            // optional Docker network to attach (external)
	BaseURL     string            // e.g. http://app.example.com — used for SERVICE_URL_*
	FQDN        string            // hostname for Traefik labels (derived from BaseURL if empty)
	RouterName  string            // Traefik router name (defaults to ServiceID or "svc")
	Port        string            // container port for Traefik (default: from SERVICE_URL_*_PORT or "80")
	ExistingEnv map[string]string // reuse passwords/users when re-preparing
	// KeepPublishedPorts leaves host port mappings intact. Default false: Dockfin
	// strips published ports so Traefik owns 80/443 and stacks avoid port conflicts.
	KeepPublishedPorts bool
	// ExtraLabels are key=value Traefik/custom labels merged onto the primary web service.
	ExtraLabels []string
	// BasicAuthUsers is Traefik basicauth users= value (user:hash); empty disables.
	BasicAuthUsers string
	// Redirect is both|www|non-www (Coolify Direction). Magic domains should stay "both".
	Redirect string
	// GPUEnabled injects NVIDIA device reservations on the primary service (compose).
	GPUEnabled bool
	GPUCount   int
	// RestartPolicy sets compose `restart:` on services (unless-stopped, always, on-failure, no).
	RestartPolicy string
	// StopGracePeriodSec sets compose `stop_grace_period` (seconds); 0 leaves default.
	StopGracePeriodSec int
	// Swarm deploy block on the primary service (destination kind swarm).
	SwarmReplicas             int
	SwarmPlacementConstraints []string
	SwarmWorkersOnly          bool
	// SkipHTTPSRedirect keeps Let's Encrypt TLS but does not bounce HTTP→HTTPS.
	SkipHTTPSRedirect bool
}

var reMagicKey = regexp.MustCompile(`SERVICE_(?:PASSWORD|USER|FQDN|URL|BASE64|HEX)_[A-Z0-9_]+`)

// PrepareCompose normalizes Coolify-style compose templates so Docker Compose accepts them:
// - declares missing named volumes
// - expands SERVICE_* magic variables into concrete values
// - optionally attaches services to an external network
// - strips published host ports (Traefik owns 80/443; container ports stay in expose)
// - injects Traefik labels on the primary web service
func PrepareCompose(raw string, opts PrepareOpts) (string, map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, fmt.Errorf("empty compose")
	}

	// Auto SSL for custom domains: force https BaseURL so SERVICE_URL_* and Traefik
	// Let's Encrypt labels stay aligned (magic domains stay http).
	domainHint := strings.TrimSpace(opts.FQDN)
	if domainHint == "" {
		domainHint = strings.TrimSpace(opts.BaseURL)
	}
	if proxy.WantAutoHTTPS(domainHint) {
		opts.BaseURL = proxy.AutoPublicURL(domainHint)
		if strings.TrimSpace(opts.FQDN) == "" {
			opts.FQDN = proxy.PrimaryHost(domainHint)
		}
	}

	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return "", nil, fmt.Errorf("parse compose: %w", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}

	env := collectAndGenerateMagic(raw, opts)
	ensureNamedVolumes(doc)
	if opts.Network != "" {
		ensureExternalNetwork(doc, opts.Network)
	}
	expandMagicInDoc(doc, env)
	persistMagicSecrets(doc, env, CoolifyEnvForUI(raw, env))
	injectCompatEnv(doc, env)
	opts.Port = DetectProxyPort(raw, opts.Port)
	injectProxyLabels(doc, opts)
	injectGPU(doc, opts)
	injectRestartAndStopGrace(doc, opts)
	injectSwarmDeploy(doc, opts)
	if !opts.KeepPublishedPorts {
		stripPublishedPorts(doc)
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", nil, err
	}
	return string(out), env, nil
}

func collectAndGenerateMagic(raw string, opts PrepareOpts) map[string]string {
	keys := map[string]struct{}{}
	for _, m := range reMagicKey.FindAllString(raw, -1) {
		keys[m] = struct{}{}
	}

	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1"
	}
	fqdn := proxy.HostFromDomainEntry(baseURL)
	if fqdn == "" {
		fqdn = "127.0.0.1"
	}

	env := map[string]string{}
	reuse := opts.ExistingEnv
	if reuse == nil {
		reuse = map[string]string{}
	}

	for key := range keys {
		// Domain keys always follow this prepare's BaseURL — never sticky-reuse
		// stale SERVICE_URL_*/FQDN_* from ExistingEnv (Domains / preferURL set BaseURL).
		isDomainKey := strings.HasPrefix(key, "SERVICE_URL_") || strings.HasPrefix(key, "SERVICE_FQDN_")
		if !isDomainKey {
			if v := strings.TrimSpace(reuse[key]); v != "" {
				env[key] = v
				continue
			}
		}
		switch {
		case strings.HasPrefix(key, "SERVICE_PASSWORD_"):
			env[key] = randomAlnum(passwordMagicLength(key))
		case strings.HasPrefix(key, "SERVICE_USER_"):
			suffix := strings.ToLower(strings.TrimPrefix(key, "SERVICE_USER_"))
			suffix = strings.ReplaceAll(suffix, "_", "")
			if len(suffix) > 12 {
				suffix = suffix[:12]
			}
			if suffix == "" {
				suffix = "user"
			}
			env[key] = suffix
		case strings.HasPrefix(key, "SERVICE_URL_"):
			env[key] = baseURL
		case strings.HasPrefix(key, "SERVICE_FQDN_"):
			env[key] = fqdn
		case strings.HasPrefix(key, "SERVICE_BASE64_"):
			if v := strings.TrimSpace(reuse[key]); v != "" {
				env[key] = v
			} else {
				env[key] = base64.RawStdEncoding.EncodeToString([]byte(randomAlnum(18)))
			}
		case strings.HasPrefix(key, "SERVICE_HEX_"):
			if v := strings.TrimSpace(reuse[key]); v != "" {
				env[key] = v
			} else {
				b := make([]byte, 16)
				_, _ = rand.Read(b)
				env[key] = hex.EncodeToString(b)
			}
		}
	}

	// Coolify: SERVICE_URL_APP_3000 also implies SERVICE_URL_APP / SERVICE_FQDN_APP(_3000).
	for key := range keys {
		if !strings.HasPrefix(key, "SERVICE_URL_") && !strings.HasPrefix(key, "SERVICE_FQDN_") {
			continue
		}
		name, port, hasPort := parseServiceURLNamePort(key)
		if name == "" {
			continue
		}
		ensureMagicPair(env, name, "", baseURL, fqdn)
		if hasPort {
			ensureMagicPair(env, name, port, baseURL, fqdn)
		}
	}

	// Coolify uses http:// for sslip magic domains. n8n's template defaults
	// N8N_PROTOCOL to https which breaks secure cookies on HTTP — align protocol.
	if strings.HasPrefix(strings.ToLower(baseURL), "http://") {
		env["N8N_PROTOCOL"] = "http"
		env["N8N_SECURE_COOKIE"] = "false"
	}
	return env
}

func ensureMagicPair(env map[string]string, name, port, baseURL, fqdn string) {
	suffix := name
	if port != "" {
		suffix = name + "_" + port
	}
	urlKey := "SERVICE_URL_" + suffix
	fqKey := "SERVICE_FQDN_" + suffix
	if _, ok := env[urlKey]; !ok {
		env[urlKey] = baseURL
	}
	if _, ok := env[fqKey]; !ok {
		env[fqKey] = fqdn
	}
}

// parseServiceURLNamePort mirrors Coolify parseServiceEnvironmentVariable.
// SERVICE_URL_APP_3000 → name=APP, port=3000; SERVICE_URL_MY_APP → name=MY_APP.
func parseServiceURLNamePort(key string) (name, port string, hasPort bool) {
	var rest string
	switch {
	case strings.HasPrefix(key, "SERVICE_URL_"):
		rest = strings.TrimPrefix(key, "SERVICE_URL_")
	case strings.HasPrefix(key, "SERVICE_FQDN_"):
		rest = strings.TrimPrefix(key, "SERVICE_FQDN_")
	default:
		return "", "", false
	}
	if rest == "" {
		return "", "", false
	}
	i := strings.LastIndexByte(rest, '_')
	if i > 0 {
		last := rest[i+1:]
		if isAllDigits(last) {
			return rest[:i], last, true
		}
	}
	return rest, "", false
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// DetectProxyPort returns the container port for Traefik.
// Order: explicit opts → Coolify `# port:` metadata → SERVICE_URL_*_PORT suffix →
// published/expose targets in compose → "80".
func DetectProxyPort(raw, explicit string) string {
	if p := strings.TrimSpace(explicit); p != "" {
		return p
	}
	if m := regexp.MustCompile(`(?m)^#\s*port:\s*(\d+)\s*$`).FindStringSubmatch(raw); len(m) == 2 {
		return m[1]
	}
	// Legacy Coolify keys with port suffix (SERVICE_URL_N8N_5678).
	re := regexp.MustCompile(`SERVICE_(?:URL|FQDN)_[A-Z0-9_]+`)
	for _, key := range re.FindAllString(raw, -1) {
		if _, port, hasPort := parseServiceURLNamePort(key); hasPort {
			return port
		}
	}
	if p := InferContainerPortFromCompose(raw); p != "" {
		return p
	}
	return "80"
}

// InferContainerPortFromCompose finds a likely HTTP container port from ports:/expose
// on the Traefik-facing service (or the first service). Used so GitHub compose apps
// work without the user setting Ports Exposes by hand.
func InferContainerPortFromCompose(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return ""
	}
	services, _ := doc["services"].(map[string]any)
	if len(services) == 0 {
		return ""
	}
	name := pickProxyService(services)
	svc, _ := services[name].(map[string]any)
	if svc == nil {
		return ""
	}
	// Prefer explicit expose, then ports: container targets.
	for _, t := range stringListFromAny(svc["expose"]) {
		if isAllDigits(t) && t != "0" {
			return t
		}
	}
	targets := portMappingTargets(svc["ports"])
	for _, t := range targets {
		if isAllDigits(t) && t != "0" {
			return t
		}
	}
	return ""
}

// DetectProxyPortForGitCompose resolves the Traefik container port for Git compose apps.
// Non-empty portsExposes is an explicit override; otherwise compose metadata / ports: win.
func DetectProxyPortForGitCompose(raw, portsExposes string) string {
	if p := strings.TrimSpace(portsExposes); p != "" {
		parts := strings.FieldsFunc(p, func(r rune) bool {
			return r == ',' || r == ' ' || r == ';'
		})
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	return DetectProxyPort(raw, "")
}

// CoolifyEnvForUI returns the Coolify-style env set shown in Environment Variables:
// passwords/users/hex/base64 referenced in the template, plus SERVICE_URL/FQDN pairs
// for each domain key declared in compose environment (WordPress → SERVICE_URL_WORDPRESS
// + SERVICE_FQDN_WORDPRESS). Keys that only appear inside ${...} substitutions are
// companions for baking and are not listed in the UI.
func CoolifyEnvForUI(raw string, env map[string]string) map[string]string {
	declared := map[string]struct{}{}
	for _, m := range reMagicKey.FindAllString(raw, -1) {
		declared[m] = struct{}{}
	}
	envDeclared := domainKeysDeclaredInEnvironment(raw)
	out := map[string]string{}
	for key := range declared {
		switch {
		case strings.HasPrefix(key, "SERVICE_PASSWORD_"),
			strings.HasPrefix(key, "SERVICE_USER_"),
			strings.HasPrefix(key, "SERVICE_BASE64_"),
			strings.HasPrefix(key, "SERVICE_HEX_"):
			if v := strings.TrimSpace(env[key]); v != "" {
				out[key] = v
			}
		case strings.HasPrefix(key, "SERVICE_URL_"), strings.HasPrefix(key, "SERVICE_FQDN_"):
			if len(envDeclared) > 0 {
				if _, ok := envDeclared[key]; !ok {
					continue
				}
			}
			name, port, hasPort := parseServiceURLNamePort(key)
			if name == "" {
				continue
			}
			suffix := name
			if hasPort {
				suffix = name + "_" + port
			}
			uk := "SERVICE_URL_" + suffix
			fk := "SERVICE_FQDN_" + suffix
			if v := strings.TrimSpace(env[uk]); v != "" {
				out[uk] = v
			}
			if v := strings.TrimSpace(env[fk]); v != "" {
				out[fk] = v
			}
		}
	}
	return out
}

// domainKeysDeclaredInEnvironment finds SERVICE_URL_/FQDN_ keys listed under
// environment: (bare or KEY=...), not merely referenced via ${SERVICE_URL_*}.
func domainKeysDeclaredInEnvironment(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	// List style: - SERVICE_URL_APP / - SERVICE_URL_APP=...
	reList := regexp.MustCompile(`(?m)^\s*-\s*(SERVICE_(?:URL|FQDN)_[A-Z0-9_]+)\b`)
	for _, m := range reList.FindAllStringSubmatch(raw, -1) {
		out[m[1]] = struct{}{}
	}
	// Map style: SERVICE_URL_APP: ...
	reMap := regexp.MustCompile(`(?m)^\s*(SERVICE_(?:URL|FQDN)_[A-Z0-9_]+)\s*:`)
	for _, m := range reMap.FindAllStringSubmatch(raw, -1) {
		out[m[1]] = struct{}{}
	}
	return out
}

// ExtractMagicEnv pulls already-expanded SERVICE_* values from prepared compose YAML text.
func ExtractMagicEnv(compose string) map[string]string {
	out := map[string]string{}
	re := regexp.MustCompile(`(?m)(SERVICE_(?:PASSWORD|USER|BASE64|HEX|URL|FQDN)_[A-Z0-9_]+)=([^\s"']+)`)
	for _, m := range re.FindAllStringSubmatch(compose, -1) {
		if len(m) == 3 {
			out[m[1]] = m[2]
		}
	}
	reQ := regexp.MustCompile(`(?m)(SERVICE_(?:PASSWORD|USER|BASE64|HEX|URL|FQDN)_[A-Z0-9_]+)="([^"]*)"`)
	for _, m := range reQ.FindAllStringSubmatch(compose, -1) {
		if len(m) == 3 {
			out[m[1]] = m[2]
		}
	}
	return out
}

func expandMagicInDoc(doc map[string]any, env map[string]string) {
	services, _ := doc["services"].(map[string]any)
	if services == nil {
		return
	}
	for name, svcAny := range services {
		svc, ok := svcAny.(map[string]any)
		if !ok {
			continue
		}
		if envAny, ok := svc["environment"]; ok {
			svc["environment"] = expandEnvironment(envAny, env)
		}
		walkSubstitute(svc, env)
		services[name] = svc
	}
	doc["services"] = services

	// Coolify also substitutes magic vars inside top-level compose configs.
	if configs, ok := doc["configs"].(map[string]any); ok {
		for cname, cfgAny := range configs {
			cfg, ok := cfgAny.(map[string]any)
			if !ok {
				continue
			}
			walkSubstitute(cfg, env)
			configs[cname] = cfg
		}
		doc["configs"] = configs
	}
}

// persistMagicSecrets keeps Coolify SERVICE_* values (passwords + URL/FQDN pairs)
// in compose env so redeploys can reuse them via ExtractMagicEnv.
// secrets should be CoolifyEnvForUI(...) — not every in-memory companion key.
func persistMagicSecrets(doc map[string]any, env map[string]string, secrets map[string]string) {
	if len(secrets) == 0 {
		// Fallback: passwords only (legacy callers / empty UI set).
		secrets = map[string]string{}
		for k, v := range env {
			if strings.HasPrefix(k, "SERVICE_PASSWORD_") ||
				strings.HasPrefix(k, "SERVICE_USER_") ||
				strings.HasPrefix(k, "SERVICE_BASE64_") ||
				strings.HasPrefix(k, "SERVICE_HEX_") {
				secrets[k] = v
			}
		}
	}
	if len(secrets) == 0 {
		return
	}
	services, _ := doc["services"].(map[string]any)
	if services == nil {
		return
	}
	for name, svcAny := range services {
		svc, ok := svcAny.(map[string]any)
		if !ok {
			continue
		}
		svc["environment"] = mergeSecretEnv(svc["environment"], secrets)
		services[name] = svc
	}
	doc["services"] = services
}

func mergeSecretEnv(envAny any, secrets map[string]string) any {
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	switch list := envAny.(type) {
	case []any:
		have := map[string]bool{}
		out := make([]any, 0, len(list)+len(keys))
		for _, item := range list {
			s, ok := item.(string)
			if ok {
				name := s
				if i := strings.IndexByte(s, '='); i > 0 {
					name = s[:i]
				}
				if _, isSecret := secrets[name]; isSecret {
					have[name] = true
					out = append(out, name+"="+secrets[name])
					continue
				}
			}
			out = append(out, item)
		}
		for _, k := range keys {
			if !have[k] {
				out = append(out, k+"="+secrets[k])
			}
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, v := range list {
			out[k] = v
		}
		for _, k := range keys {
			out[k] = secrets[k]
		}
		return out
	case nil:
		out := make([]any, 0, len(keys))
		for _, k := range keys {
			out = append(out, k+"="+secrets[k])
		}
		return out
	default:
		return envAny
	}
}

func expandEnvironment(envAny any, magic map[string]string) any {
	switch list := envAny.(type) {
	case []any:
		out := make([]any, 0, len(list))
		for _, item := range list {
			s, ok := item.(string)
			if !ok {
				out = append(out, item)
				continue
			}
			s = strings.TrimSpace(s)
			// Bare Coolify magic: "- SERVICE_URL_WORDPRESS"
			if reMagicKey.MatchString(s) && !strings.Contains(s, "=") && !strings.Contains(s, "$") {
				if v, ok := magic[s]; ok {
					out = append(out, s+"="+v)
					continue
				}
			}
			out = append(out, substituteMagic(s, magic))
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, v := range list {
			if vs, ok := v.(string); ok {
				if reMagicKey.MatchString(vs) && !strings.Contains(vs, "$") && !strings.Contains(vs, "=") {
					if mv, ok := magic[vs]; ok {
						out[k] = mv
						continue
					}
				}
				out[k] = substituteMagic(vs, magic)
			} else {
				out[k] = v
			}
		}
		return out
	default:
		return envAny
	}
}

func walkSubstitute(m map[string]any, magic map[string]string) {
	for k, v := range m {
		if k == "environment" {
			continue
		}
		switch t := v.(type) {
		case string:
			m[k] = substituteMagic(t, magic)
		case []any:
			for i, item := range t {
				if s, ok := item.(string); ok {
					t[i] = substituteMagic(s, magic)
				} else if nested, ok := item.(map[string]any); ok {
					walkSubstitute(nested, magic)
				}
			}
		case map[string]any:
			walkSubstitute(t, magic)
		}
	}
}

func substituteMagic(s string, magic map[string]string) string {
	re := regexp.MustCompile(`\$\{?(SERVICE_(?:PASSWORD|USER|FQDN|URL|BASE64|HEX)_[A-Z0-9_]+)\}?`)
	return re.ReplaceAllStringFunc(s, func(m string) string {
		key := strings.TrimPrefix(m, "$")
		key = strings.TrimPrefix(key, "{")
		key = strings.TrimSuffix(key, "}")
		if v, ok := magic[key]; ok {
			return v
		}
		return m
	})
}

// injectCompatEnv writes HTTP-compat keys into services that already reference them
// (e.g. n8n), and resolves ${N8N_PROTOCOL:-https}-style Coolify template defaults.
func injectCompatEnv(doc map[string]any, env map[string]string) {
	compat := map[string]string{}
	for _, k := range []string{"N8N_PROTOCOL", "N8N_SECURE_COOKIE"} {
		if v := strings.TrimSpace(env[k]); v != "" {
			compat[k] = v
		}
	}
	if len(compat) == 0 {
		return
	}
	services, _ := doc["services"].(map[string]any)
	if services == nil {
		return
	}
	for name, svcAny := range services {
		svc, ok := svcAny.(map[string]any)
		if !ok {
			continue
		}
		if !envMentionsKeys(svc["environment"], []string{"N8N_PROTOCOL", "N8N_EDITOR_BASE_URL", "N8N_HOST", "WEBHOOK_URL"}) {
			continue
		}
		svc["environment"] = mergeCompatEnv(svc["environment"], compat)
		services[name] = svc
	}
	doc["services"] = services
}

func envMentionsKeys(envAny any, keys []string) bool {
	has := map[string]bool{}
	switch list := envAny.(type) {
	case []any:
		for _, item := range list {
			s, ok := item.(string)
			if !ok {
				continue
			}
			name := s
			if i := strings.IndexByte(s, '='); i > 0 {
				name = s[:i]
			}
			has[name] = true
			for _, k := range keys {
				if strings.Contains(s, k) {
					has[k] = true
				}
			}
		}
	case map[string]any:
		for k := range list {
			has[k] = true
		}
	}
	for _, k := range keys {
		if has[k] {
			return true
		}
	}
	return false
}

func mergeCompatEnv(envAny any, compat map[string]string) any {
	reDefault := regexp.MustCompile(`\$\{([A-Z0-9_]+):-[^}]*\}`)
	applyLine := func(s string) string {
		s = strings.TrimSpace(s)
		if i := strings.IndexByte(s, '='); i > 0 {
			key := s[:i]
			if v, ok := compat[key]; ok {
				return key + "=" + v
			}
		}
		// N8N_PROTOCOL=${N8N_PROTOCOL:-https}
		if i := strings.IndexByte(s, '='); i > 0 {
			key, val := s[:i], s[i+1:]
			if _, ok := compat[key]; ok {
				if m := reDefault.FindStringSubmatch(val); len(m) == 2 {
					if v, ok := compat[m[1]]; ok {
						return key + "=" + v
					}
				}
			}
		}
		return reDefault.ReplaceAllStringFunc(s, func(m string) string {
			sub := reDefault.FindStringSubmatch(m)
			if len(sub) == 2 {
				if v, ok := compat[sub[1]]; ok {
					return v
				}
			}
			return m
		})
	}

	keys := make([]string, 0, len(compat))
	for k := range compat {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	switch list := envAny.(type) {
	case []any:
		have := map[string]bool{}
		out := make([]any, 0, len(list)+len(keys))
		for _, item := range list {
			s, ok := item.(string)
			if !ok {
				out = append(out, item)
				continue
			}
			s = applyLine(s)
			out = append(out, s)
			if i := strings.IndexByte(s, '='); i > 0 {
				have[s[:i]] = true
			}
		}
		for _, k := range keys {
			if !have[k] {
				out = append(out, k+"="+compat[k])
			}
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, v := range list {
			if vs, ok := v.(string); ok {
				out[k] = applyLine(k + "=" + vs)[len(k)+1:]
			} else {
				out[k] = v
			}
		}
		for _, k := range keys {
			out[k] = compat[k]
		}
		return out
	case nil:
		out := make([]any, 0, len(keys))
		for _, k := range keys {
			out = append(out, k+"="+compat[k])
		}
		return out
	default:
		return envAny
	}
}

func ensureNamedVolumes(doc map[string]any) {
	services, _ := doc["services"].(map[string]any)
	if services == nil {
		return
	}
	needed := map[string]struct{}{}
	for _, svcAny := range services {
		svc, ok := svcAny.(map[string]any)
		if !ok {
			continue
		}
		vols, ok := svc["volumes"]
		if !ok {
			continue
		}
		list, ok := vols.([]any)
		if !ok {
			continue
		}
		for _, item := range list {
			name := namedVolumeFromMount(item)
			if name != "" {
				needed[name] = struct{}{}
			}
		}
	}
	if len(needed) == 0 {
		return
	}

	volSection, _ := doc["volumes"].(map[string]any)
	if volSection == nil {
		volSection = map[string]any{}
	}
	for name := range needed {
		if _, exists := volSection[name]; !exists {
			volSection[name] = nil
		}
	}
	doc["volumes"] = volSection
}

func namedVolumeFromMount(item any) string {
	switch v := item.(type) {
	case string:
		parts := strings.SplitN(v, ":", 3)
		if len(parts) < 2 {
			return ""
		}
		src := parts[0]
		if src == "" || strings.HasPrefix(src, ".") || strings.HasPrefix(src, "/") || strings.HasPrefix(src, "~") {
			return ""
		}
		if strings.Contains(src, "/") {
			return ""
		}
		return src
	case map[string]any:
		typ, _ := v["type"].(string)
		if typ != "" && typ != "volume" {
			return ""
		}
		src, _ := v["source"].(string)
		if src == "" || strings.HasPrefix(src, ".") || strings.HasPrefix(src, "/") {
			return ""
		}
		if strings.Contains(src, "/") {
			return ""
		}
		return src
	}
	return ""
}

// ensureExternalNetwork attaches only the Traefik-facing service to the shared
// proxy network. All services stay on the Compose project default network so
// short names like "postgresql" resolve inside the stack only — not to another
// project's DB on the shared dockfin network (which caused Planka 404s).
func ensureExternalNetwork(doc map[string]any, network string) {
	if network == "" {
		return
	}
	services, _ := doc["services"].(map[string]any)
	if services == nil {
		return
	}

	proxySvc := pickProxyService(services)

	for name, svcAny := range services {
		svc, ok := svcAny.(map[string]any)
		if !ok {
			continue
		}
		if _, has := svc["network_mode"]; has {
			continue
		}
		nets := normalizeNetworkList(svc["networks"])
		// Always keep the project-local default network for inter-service DNS.
		nets = ensureNetworkName(nets, "default")
		if name == proxySvc {
			nets = ensureNetworkName(nets, network)
		} else {
			nets = removeNetworkName(nets, network)
		}
		svc["networks"] = nets
		services[name] = svc
	}
	doc["services"] = services

	netSection, _ := doc["networks"].(map[string]any)
	if netSection == nil {
		netSection = map[string]any{}
	}
	if _, ok := netSection["default"]; !ok {
		netSection["default"] = map[string]any{}
	}
	netSection[network] = map[string]any{"external": true}
	doc["networks"] = netSection
}

func normalizeNetworkList(v any) []any {
	switch t := v.(type) {
	case []any:
		return append([]any{}, t...)
	case map[string]any:
		out := make([]any, 0, len(t))
		for name := range t {
			out = append(out, name)
		}
		return out
	default:
		return nil
	}
}

func ensureNetworkName(nets []any, name string) []any {
	for _, n := range nets {
		if ns, ok := n.(string); ok && ns == name {
			return nets
		}
	}
	return append(nets, name)
}

func removeNetworkName(nets []any, name string) []any {
	out := make([]any, 0, len(nets))
	for _, n := range nets {
		if ns, ok := n.(string); ok && ns == name {
			continue
		}
		out = append(out, n)
	}
	return out
}

// stripPublishedPorts removes host port publications (HOST:CONTAINER) so Traefik
// can bind 80/443 without conflicts. Container ports are preserved under expose.
func stripPublishedPorts(doc map[string]any) {
	services, _ := doc["services"].(map[string]any)
	if services == nil {
		return
	}
	for name, svcAny := range services {
		svc, ok := svcAny.(map[string]any)
		if !ok {
			continue
		}
		ports := svc["ports"]
		if ports == nil {
			continue
		}
		targets := portMappingTargets(ports)
		if len(targets) == 0 {
			delete(svc, "ports")
			services[name] = svc
			continue
		}
		expose := stringListFromAny(svc["expose"])
		for _, t := range targets {
			expose = appendUniqueString(expose, t)
		}
		if len(expose) > 0 {
			out := make([]any, len(expose))
			for i, s := range expose {
				out[i] = s
			}
			svc["expose"] = out
		}
		delete(svc, "ports")
		services[name] = svc
	}
	doc["services"] = services
}

func portMappingTargets(ports any) []string {
	var out []string
	switch p := ports.(type) {
	case []any:
		for _, item := range p {
			if t := portMappingTarget(item); t != "" {
				out = appendUniqueString(out, t)
			}
		}
	case map[string]any:
		if t := portMappingTarget(p); t != "" {
			out = append(out, t)
		}
	case string:
		if t := portMappingTarget(p); t != "" {
			out = append(out, t)
		}
	case int:
		if t := portMappingTarget(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func portMappingTarget(item any) string {
	switch v := item.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return ""
		}
		// Drop protocol suffix: 80/tcp, 8080:80/udp
		if i := strings.IndexByte(s, '/'); i >= 0 {
			s = s[:i]
		}
		parts := strings.Split(s, ":")
		switch len(parts) {
		case 1:
			return strings.TrimSpace(parts[0])
		case 2, 3:
			return strings.TrimSpace(parts[len(parts)-1])
		default:
			return strings.TrimSpace(parts[len(parts)-1])
		}
	case int:
		if v > 0 {
			return fmt.Sprintf("%d", v)
		}
	case int64:
		if v > 0 {
			return fmt.Sprintf("%d", v)
		}
	case float64:
		if v > 0 {
			return fmt.Sprintf("%d", int(v))
		}
	case map[string]any:
		if t, ok := v["target"]; ok {
			return portMappingTarget(t)
		}
	}
	return ""
}

func stringListFromAny(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s := strings.TrimSpace(fmt.Sprint(item))
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return append([]string{}, t...)
	default:
		return nil
	}
}

func appendUniqueString(list []string, s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return list
	}
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// injectProxyLabels adds Traefik labels to the primary web service when FQDN is set.
func injectProxyLabels(doc map[string]any, opts PrepareOpts) {
	fqdn := strings.TrimSpace(opts.FQDN)
	if fqdn == "" {
		base := strings.TrimRight(opts.BaseURL, "/")
		fqdn = strings.TrimPrefix(strings.TrimPrefix(base, "https://"), "http://")
		fqdn = strings.Split(fqdn, "/")[0]
	}
	primary := proxy.PrimaryHost(fqdn)
	if primary == "" || primary == "127.0.0.1" || primary == "localhost" {
		return
	}
	services, _ := doc["services"].(map[string]any)
	if services == nil || len(services) == 0 {
		return
	}
	port := DetectProxyPort("", opts.Port)
	router := strings.TrimSpace(opts.RouterName)
	if router == "" {
		router = strings.TrimSpace(opts.ServiceID)
	}
	if router == "" {
		router = "svc"
	}
	router = sanitizeRouter(router)

	target := pickProxyService(services)
	if target == "" {
		return
	}
	svc, ok := services[target].(map[string]any)
	if !ok {
		return
	}
	labels := map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", router): port,
	}
	hosts := proxy.HostsFromDomainList(fqdn)
	if len(hosts) == 0 {
		hosts = []string{primary}
	}
	rule := proxy.TraefikHostRule(hosts)
	if rule == "" {
		return
	}
	useHTTPS := proxy.WantAutoHTTPS(fqdn) || proxy.WantAutoHTTPS(opts.BaseURL)
	if useHTTPS {
		labels[fmt.Sprintf("traefik.http.routers.%s.rule", router)] = rule
		labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", router)] = "https"
		labels[fmt.Sprintf("traefik.http.routers.%s.tls", router)] = "true"
		labels[fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", router)] = "letsencrypt"
		labels[fmt.Sprintf("traefik.http.routers.%s-http.rule", router)] = rule
		labels[fmt.Sprintf("traefik.http.routers.%s-http.entrypoints", router)] = "http"
		if !opts.SkipHTTPSRedirect {
			labels[fmt.Sprintf("traefik.http.routers.%s-http.middlewares", router)] = router + "-redirect"
			labels[fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.scheme", router)] = "https"
			labels[fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.permanent", router)] = "true"
		}
	} else {
		labels[fmt.Sprintf("traefik.http.routers.%s.rule", router)] = rule
		labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", router)] = "http"
	}
	if opts.Network != "" {
		labels["traefik.docker.network"] = opts.Network
	}
	if users := strings.TrimSpace(opts.BasicAuthUsers); users != "" {
		mw := router + "-basicauth"
		labels[fmt.Sprintf("traefik.http.middlewares.%s.basicauth.users", mw)] = users
		key := fmt.Sprintf("traefik.http.routers.%s.middlewares", router)
		if existing := labels[key]; existing != "" {
			labels[key] = existing + "," + mw
		} else {
			labels[key] = mw
		}
	}
	redir := strings.ToLower(strings.TrimSpace(opts.Redirect))
	if (redir == "www" || redir == "non-www") && !proxy.IsMagicDomainHost(primary) {
		var mw, regex, replacement string
		if redir == "www" {
			mw = router + "-to-www"
			regex = `^(http|https)://(?:www\.)?(.+)`
			replacement = `${1}://www.${2}`
		} else {
			mw = router + "-to-non-www"
			regex = `^(http|https)://www\.(.+)`
			replacement = `${1}://${2}`
		}
		labels[fmt.Sprintf("traefik.http.middlewares.%s.redirectregex.regex", mw)] = regex
		labels[fmt.Sprintf("traefik.http.middlewares.%s.redirectregex.replacement", mw)] = replacement
		labels[fmt.Sprintf("traefik.http.middlewares.%s.redirectregex.permanent", mw)] = "false"
		key := fmt.Sprintf("traefik.http.routers.%s.middlewares", router)
		if existing := labels[key]; existing != "" && !strings.Contains(existing, mw) {
			labels[key] = existing + "," + mw
		} else if labels[key] == "" {
			labels[key] = mw
		}
	}
	for _, raw := range opts.ExtraLabels {
		raw = strings.TrimSpace(raw)
		if raw == "" || !strings.Contains(raw, "=") {
			continue
		}
		i := strings.IndexByte(raw, '=')
		k := strings.TrimSpace(raw[:i])
		v := strings.TrimSpace(raw[i+1:])
		if k != "" {
			labels[k] = v
		}
	}
	mergeLabels(svc, labels)
	services[target] = svc
	doc["services"] = services
}

// injectGPU adds NVIDIA GPU access on the primary web service when GPUEnabled.
// Sets both Compose Spec `gpus:` (plain `docker compose up`) and Swarm-style
// deploy.resources.reservations.devices (Coolify-compatible / swarm).
func injectGPU(doc map[string]any, opts PrepareOpts) {
	if !opts.GPUEnabled {
		return
	}
	services, _ := doc["services"].(map[string]any)
	if services == nil || len(services) == 0 {
		return
	}
	target := pickProxyService(services)
	if target == "" {
		return
	}
	svc, ok := services[target].(map[string]any)
	if !ok {
		return
	}
	count := opts.GPUCount
	if count <= 0 {
		count = 1
	}
	// Non-swarm compose shortcut.
	if opts.GPUCount <= 0 {
		svc["gpus"] = "all"
	} else {
		svc["gpus"] = count
	}
	device := map[string]any{
		"driver":       "nvidia",
		"capabilities": []any{"gpu"},
		"count":        count,
	}
	if opts.GPUCount <= 0 {
		device["count"] = "all"
	}
	deploy, _ := svc["deploy"].(map[string]any)
	if deploy == nil {
		deploy = map[string]any{}
	}
	resources, _ := deploy["resources"].(map[string]any)
	if resources == nil {
		resources = map[string]any{}
	}
	reservations, _ := resources["reservations"].(map[string]any)
	if reservations == nil {
		reservations = map[string]any{}
	}
	devices, _ := reservations["devices"].([]any)
	devices = append(devices, device)
	reservations["devices"] = devices
	resources["reservations"] = reservations
	deploy["resources"] = resources
	svc["deploy"] = deploy
	services[target] = svc
	doc["services"] = services
}

// injectRestartAndStopGrace applies restart policy and stop_grace_period to all services.
func injectRestartAndStopGrace(doc map[string]any, opts PrepareOpts) {
	policy := strings.TrimSpace(opts.RestartPolicy)
	grace := opts.StopGracePeriodSec
	if policy == "" && grace <= 0 {
		return
	}
	services, _ := doc["services"].(map[string]any)
	if services == nil {
		return
	}
	for name, raw := range services {
		svc, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if policy != "" {
			svc["restart"] = policy
		}
		if grace > 0 {
			svc["stop_grace_period"] = fmt.Sprintf("%ds", grace)
		}
		services[name] = svc
	}
	doc["services"] = services
}

// injectSwarmDeploy adds deploy.replicas and placement constraints on the primary service for swarm stacks.
func injectSwarmDeploy(doc map[string]any, opts PrepareOpts) {
	if opts.SwarmReplicas <= 0 && len(opts.SwarmPlacementConstraints) == 0 && !opts.SwarmWorkersOnly {
		return
	}
	services, _ := doc["services"].(map[string]any)
	if services == nil || len(services) == 0 {
		return
	}
	target := pickProxyService(services)
	if target == "" {
		return
	}
	svc, ok := services[target].(map[string]any)
	if !ok {
		return
	}
	deploy, _ := svc["deploy"].(map[string]any)
	if deploy == nil {
		deploy = map[string]any{}
	}
	if opts.SwarmReplicas > 0 {
		deploy["replicas"] = opts.SwarmReplicas
	}
	var constraints []any
	for _, c := range opts.SwarmPlacementConstraints {
		c = strings.TrimSpace(c)
		if c != "" {
			constraints = append(constraints, c)
		}
	}
	if opts.SwarmWorkersOnly {
		constraints = append(constraints, "node.role == worker")
	}
	if len(constraints) > 0 {
		placement, _ := deploy["placement"].(map[string]any)
		if placement == nil {
			placement = map[string]any{}
		}
		placement["constraints"] = constraints
		deploy["placement"] = placement
	}
	svc["deploy"] = deploy
	services[target] = svc
	doc["services"] = services
}

func pickProxyService(services map[string]any) string {
	// Prefer exact name matches, then prefix matches. Avoid short substrings like "web"→"webhook".
	preferred := []string{"wordpress", "ghost", "n8n", "umami", "frontend", "nginx", "caddy", "apache", "app", "web"}
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, p := range preferred {
		for _, name := range names {
			if strings.EqualFold(name, p) {
				return name
			}
		}
	}
	for _, p := range preferred {
		if len(p) < 4 {
			continue
		}
		for _, name := range names {
			lower := strings.ToLower(name)
			if strings.HasPrefix(lower, p+"-") || strings.HasSuffix(lower, "-"+p) {
				return name
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func mergeLabels(svc map[string]any, add map[string]string) {
	existing := map[string]string{}
	switch cur := svc["labels"].(type) {
	case map[string]any:
		for k, v := range cur {
			existing[k] = fmt.Sprint(v)
		}
	case []any:
		for _, item := range cur {
			s, ok := item.(string)
			if !ok {
				continue
			}
			if i := strings.IndexByte(s, '='); i > 0 {
				existing[s[:i]] = s[i+1:]
			}
		}
	}
	for k, v := range add {
		existing[k] = v
	}
	out := map[string]any{}
	for k, v := range existing {
		out[k] = v
	}
	svc["labels"] = out
}

func sanitizeRouter(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "svc"
	}
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}

func FormatEnvFile(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := env[k]
		if strings.ContainsAny(v, " \t#\"'") {
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteByte('"')
			b.WriteString(strings.ReplaceAll(v, `"`, `\"`))
			b.WriteByte('"')
			b.WriteByte('\n')
		} else {
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(v)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func passwordMagicLength(key string) int {
	rest := strings.TrimPrefix(key, "SERVICE_PASSWORD_")
	switch {
	case rest == "64" || strings.HasPrefix(rest, "64_"):
		return 64
	case rest == "32" || strings.HasPrefix(rest, "32_"):
		return 32
	default:
		return 24
	}
}

func randomAlnum(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("x", n)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}
