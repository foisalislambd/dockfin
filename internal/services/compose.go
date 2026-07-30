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
	ServiceID string
	Network   string // optional Docker network to attach (external)
	BaseURL   string // e.g. http://app.example.com — used for SERVICE_URL_*
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
	for key := range keys {
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
			env[key] = base64.RawStdEncoding.EncodeToString([]byte(randomAlnum(18)))
		case strings.HasPrefix(key, "SERVICE_HEX_"):
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			env[key] = hex.EncodeToString(b)
		}
	}

	for key := range keys {
		if strings.HasPrefix(key, "SERVICE_URL_") {
			rest := strings.TrimPrefix(key, "SERVICE_URL_")
			fqKey := "SERVICE_FQDN_" + rest
			if _, ok := env[fqKey]; !ok {
				env[fqKey] = fqdn
			}
		}
		if strings.HasPrefix(key, "SERVICE_FQDN_") {
			rest := strings.TrimPrefix(key, "SERVICE_FQDN_")
			urlKey := "SERVICE_URL_" + rest
			if _, ok := env[urlKey]; !ok {
				env[urlKey] = baseURL
			}
		}
	}
	return env
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
