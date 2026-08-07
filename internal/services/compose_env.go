package services

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ComposeEnvRef is a docker-compose ${VAR} reference surfaced in Environment Variables.
type ComposeEnvRef struct {
	Key     string
	Value   string // default from :- / - ; empty for bare ${VAR} or :?
	Comment string // error text from :? / ?
}

// ComposeEnvForUI extracts env keys from compose `environment` and `build.args`
// (Coolify applicationParser): ${VAR}, ${VAR:-default}, ${VAR:?msg}, $VAR.
// Magic SERVICE_* keys are excluded (handled by CoolifyEnvForUI).
func ComposeEnvForUI(raw string) map[string]ComposeEnvRef {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil || doc == nil {
		return nil
	}
	services, _ := doc["services"].(map[string]any)
	if services == nil {
		return nil
	}
	out := map[string]ComposeEnvRef{}
	for _, svcAny := range services {
		svc, ok := svcAny.(map[string]any)
		if !ok {
			continue
		}
		for _, entry := range environmentEntries(svc["environment"]) {
			mergeComposeEnvEntry(out, entry)
		}
		if build, ok := svc["build"].(map[string]any); ok {
			for _, entry := range environmentEntries(build["args"]) {
				mergeComposeEnvEntry(out, entry)
			}
		}
	}
	return out
}

type composeEnvEntry struct {
	Key   string // compose map key when known (list bare KEY, or map key)
	Value string // raw value string (may contain ${...})
}

func mergeComposeEnvEntry(dst map[string]ComposeEnvRef, entry composeEnvEntry) {
	val := strings.TrimSpace(entry.Value)
	// Coolify: KEY: $SERVICE_… creates the compose KEY so the UI shows the wrapper.
	if entry.Key != "" && strings.HasPrefix(val, "$SERVICE_") && isEnvIdent(entry.Key) && !strings.HasPrefix(entry.Key, "SERVICE_") {
		mergeComposeEnvRefs(dst, []ComposeEnvRef{{Key: entry.Key, Value: val}})
	}
	mergeComposeEnvRefs(dst, extractComposeVarRefs(val))
	if entry.Key != "" && (val == "" || val == "${"+entry.Key+"}" || val == "$"+entry.Key) {
		// Bare / self-ref KEY — ensure the key exists even if value parse missed it.
		if isEnvIdent(entry.Key) && !strings.HasPrefix(entry.Key, "SERVICE_") {
			mergeComposeEnvRefs(dst, []ComposeEnvRef{{Key: entry.Key}})
		}
	}
}

func mergeComposeEnvRefs(dst map[string]ComposeEnvRef, refs []ComposeEnvRef) {
	for _, r := range refs {
		if r.Key == "" || strings.HasPrefix(r.Key, "SERVICE_") {
			continue
		}
		if !isEnvIdent(r.Key) {
			continue
		}
		cur, ok := dst[r.Key]
		if !ok {
			dst[r.Key] = r
			continue
		}
		// Prefer a non-empty default / comment if we see a richer ref later.
		if cur.Value == "" && r.Value != "" {
			cur.Value = r.Value
		}
		if cur.Comment == "" && r.Comment != "" {
			cur.Comment = r.Comment
		}
		dst[r.Key] = cur
	}
}

// environmentEntries flattens compose environment / args (map or list) to key/value pairs.
func environmentEntries(envAny any) []composeEnvEntry {
	var out []composeEnvEntry
	switch env := envAny.(type) {
	case map[string]any:
		for k, v := range env {
			ks := strings.TrimSpace(k)
			if ks == "" {
				continue
			}
			switch t := v.(type) {
			case nil:
				out = append(out, composeEnvEntry{Key: ks, Value: "${" + ks + "}"})
			case string:
				if strings.TrimSpace(t) == "" {
					out = append(out, composeEnvEntry{Key: ks, Value: "${" + ks + "}"})
				} else {
					out = append(out, composeEnvEntry{Key: ks, Value: t})
				}
			case int, int64, float64, bool:
				continue
			default:
				if s := stringifyYAMLScalar(v); s != "" && strings.Contains(s, "$") {
					out = append(out, composeEnvEntry{Key: ks, Value: s})
				}
			}
		}
	case []any:
		for _, item := range env {
			switch t := item.(type) {
			case string:
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				if i := strings.IndexByte(t, '='); i >= 0 {
					out = append(out, composeEnvEntry{Key: strings.TrimSpace(t[:i]), Value: t[i+1:]})
				} else {
					out = append(out, composeEnvEntry{Key: t, Value: "${" + t + "}"})
				}
			case map[string]any:
				for k, v := range t {
					ks := strings.TrimSpace(k)
					if ks == "" {
						continue
					}
					if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
						out = append(out, composeEnvEntry{Key: ks, Value: s})
					} else {
						out = append(out, composeEnvEntry{Key: ks, Value: "${" + ks + "}"})
					}
				}
			}
		}
	}
	return out
}

func stringifyYAMLScalar(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case float64:
		return fmt.Sprintf("%v", t)
	default:
		return ""
	}
}

func extractComposeVarRefs(s string) []ComposeEnvRef {
	if s == "" || !strings.Contains(s, "$") {
		return nil
	}
	var out []ComposeEnvRef
	for i := 0; i < len(s); {
		if s[i] != '$' {
			i++
			continue
		}
		if i+1 < len(s) && s[i+1] == '{' {
			content, end, ok := extractBalancedBraceContent(s, i+1)
			if !ok {
				i++
				continue
			}
			ref := parseComposeVarContent(content)
			if ref.Key != "" {
				out = append(out, ref)
				if strings.Contains(ref.Value, "${") {
					out = append(out, extractComposeVarRefs(ref.Value)...)
				}
			}
			i = end
			continue
		}
		if i+1 < len(s) && isEnvIdentStart(s[i+1]) {
			j := i + 1
			for j < len(s) && isEnvIdentByte(s[j]) {
				j++
			}
			out = append(out, ComposeEnvRef{Key: s[i+1 : j]})
			i = j
			continue
		}
		i++
	}
	return out
}

// extractBalancedBraceContent expects s[open] == '{'; returns inner content and index after '}'.
func extractBalancedBraceContent(s string, open int) (content string, end int, ok bool) {
	if open >= len(s) || s[open] != '{' {
		return "", open, false
	}
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[open+1 : i], i + 1, true
			}
		}
	}
	return "", open, false
}

func parseComposeVarContent(content string) ComposeEnvRef {
	content = strings.TrimSpace(content)
	if content == "" {
		return ComposeEnvRef{}
	}
	name, op, def, hasOp := splitComposeVarOperator(content)
	name = strings.TrimSpace(name)
	if name == "" || !isEnvIdent(name) {
		return ComposeEnvRef{}
	}
	ref := ComposeEnvRef{Key: name}
	if !hasOp {
		return ref
	}
	switch op {
	case ":-", "-":
		ref.Value = def
	case ":?", "?":
		ref.Comment = strings.TrimSpace(def)
		// Required vars get empty value — user must fill (error text is not a secret).
	}
	return ref
}

// splitComposeVarOperator finds :- / :? / - / ? at brace depth 0 (Coolify order).
func splitComposeVarOperator(content string) (name, op, def string, ok bool) {
	depth := 0
	for i := 0; i < len(content); i++ {
		c := content[i]
		if c == '{' {
			depth++
			continue
		}
		if c == '}' {
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth != 0 {
			continue
		}
		if i+1 < len(content) && c == ':' && content[i+1] == '-' {
			return content[:i], ":-", content[i+2:], true
		}
		if i+1 < len(content) && c == ':' && content[i+1] == '?' {
			return content[:i], ":?", content[i+2:], true
		}
		if c == '-' {
			return content[:i], "-", content[i+1:], true
		}
		if c == '?' {
			return content[:i], "?", content[i+1:], true
		}
	}
	return content, "", "", false
}

func isEnvIdent(s string) bool {
	if s == "" || !isEnvIdentStart(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isEnvIdentByte(s[i]) {
			return false
		}
	}
	return true
}

func isEnvIdentStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isEnvIdentByte(c byte) bool {
	return isEnvIdentStart(c) || (c >= '0' && c <= '9')
}
