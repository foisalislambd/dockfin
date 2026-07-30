package services

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type PrepareOpts struct {
	ServiceID   string
	Network     string // optional Docker network to attach (external)
	BaseURL     string // e.g. http://app.example.com — used for SERVICE_URL_*
	FQDN        string // hostname for Traefik labels (derived from BaseURL if empty)
	RouterName  string // Traefik router name (defaults to ServiceID or "svc")
	Port        string // container port for Traefik (default: from SERVICE_URL_*_PORT or "80")
	ExistingEnv map[string]string // reuse passwords/users when re-preparing
}

var reMagicKey = regexp.MustCompile(`SERVICE_(?:PASSWORD|USER|FQDN|URL|BASE64|HEX)_[A-Z0-9_]+`)

// PrepareCompose normalizes Coolify-style compose templates so Docker Compose accepts them:
// - declares missing named volumes
// - expands SERVICE_* magic variables into concrete values
// - optionally attaches services to an external network
func PrepareCompose(raw string, opts PrepareOpts) (string, map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, fmt.Errorf("empty compose")
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
	persistMagicSecrets(doc, env)
	opts.Port = DetectProxyPort(raw, opts.Port)
	injectProxyLabels(doc, opts)

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
	fqdn := strings.TrimPrefix(strings.TrimPrefix(baseURL, "https://"), "http://")
	fqdn = strings.Split(fqdn, "/")[0]
	if fqdn == "" {
		fqdn = "127.0.0.1"
	}

	env := map[string]string{}
	reuse := opts.ExistingEnv
	if reuse == nil {
		reuse = map[string]string{}
	}

	for key := range keys {
		if v := strings.TrimSpace(reuse[key]); v != "" {
			env[key] = v
			continue
		}
		switch {
		case strings.HasPrefix(key, "SERVICE_PASSWORD_"):
			env[key] = randomAlnum(24)
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

// DetectProxyPort returns the first Coolify SERVICE_URL_*_PORT / FQDN port, or "80".
func DetectProxyPort(raw, explicit string) string {
	if p := strings.TrimSpace(explicit); p != "" {
		return p
	}
	// Match full keys including multi-segment names (SERVICE_URL_MY_APP_3000).
	re := regexp.MustCompile(`SERVICE_(?:URL|FQDN)_[A-Z0-9_]+`)
	for _, key := range re.FindAllString(raw, -1) {
		if _, port, hasPort := parseServiceURLNamePort(key); hasPort {
			return port
		}
	}
	return "80"
}

// ExtractMagicEnv pulls already-expanded SERVICE_* values from prepared compose YAML text.
func ExtractMagicEnv(compose string) map[string]string {
	out := map[string]string{}
	re := regexp.MustCompile(`(?m)(SERVICE_(?:PASSWORD|USER|BASE64|HEX)_[A-Z0-9_]+)=([^\s"']+)`)
	for _, m := range re.FindAllStringSubmatch(compose, -1) {
		if len(m) == 3 {
			out[m[1]] = m[2]
		}
	}
	reQ := regexp.MustCompile(`(?m)(SERVICE_(?:PASSWORD|USER|BASE64|HEX)_[A-Z0-9_]+)="([^"]*)"`)
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
}

// persistMagicSecrets keeps SERVICE_PASSWORD/USER/BASE64/HEX in compose env so
// redeploys can reuse them via ExtractMagicEnv instead of regenerating.
func persistMagicSecrets(doc map[string]any, env map[string]string) {
	secrets := map[string]string{}
	for k, v := range env {
		if strings.HasPrefix(k, "SERVICE_PASSWORD_") ||
			strings.HasPrefix(k, "SERVICE_USER_") ||
			strings.HasPrefix(k, "SERVICE_BASE64_") ||
			strings.HasPrefix(k, "SERVICE_HEX_") {
			secrets[k] = v
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

func ensureExternalNetwork(doc map[string]any, network string) {
	services, _ := doc["services"].(map[string]any)
	if services == nil {
		return
	}
	for name, svcAny := range services {
		svc, ok := svcAny.(map[string]any)
		if !ok {
			continue
		}
		if _, has := svc["network_mode"]; has {
			continue
		}
		nets, _ := svc["networks"].([]any)
		found := false
		for _, n := range nets {
			if ns, ok := n.(string); ok && ns == network {
				found = true
				break
			}
		}
		if !found {
			nets = append(nets, network)
			svc["networks"] = nets
			services[name] = svc
		}
	}
	doc["services"] = services

	netSection, _ := doc["networks"].(map[string]any)
	if netSection == nil {
		netSection = map[string]any{}
	}
	if _, ok := netSection[network]; !ok {
		netSection[network] = map[string]any{"external": true}
	}
	doc["networks"] = netSection
}

// injectProxyLabels adds Traefik labels to the primary web service when FQDN is set.
func injectProxyLabels(doc map[string]any, opts PrepareOpts) {
	fqdn := strings.TrimSpace(opts.FQDN)
	if fqdn == "" {
		base := strings.TrimRight(opts.BaseURL, "/")
		fqdn = strings.TrimPrefix(strings.TrimPrefix(base, "https://"), "http://")
		fqdn = strings.Split(fqdn, "/")[0]
	}
	if fqdn == "" || fqdn == "127.0.0.1" || fqdn == "localhost" {
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
		fmt.Sprintf("traefik.http.routers.%s.rule", router):                      fmt.Sprintf("Host(`%s`)", fqdn),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", router):               "http",
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", router): port,
	}
	if opts.Network != "" {
		labels["traefik.docker.network"] = opts.Network
	}
	mergeLabels(svc, labels)
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
