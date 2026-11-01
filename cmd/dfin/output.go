package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func humanizeJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	return strings.TrimRight(humanizeValue(v, 0), "\n")
}

func humanizeValue(v any, indent int) string {
	pad := strings.Repeat("  ", indent)
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sortStrings(keys)
		var b strings.Builder
		for _, k := range keys {
			child := t[k]
			switch child.(type) {
			case map[string]any, []any:
				fmt.Fprintf(&b, "%s%s:\n%s\n", pad, k, humanizeValue(child, indent+1))
			default:
				fmt.Fprintf(&b, "%s%s: %s\n", pad, k, scalar(child))
			}
		}
		return strings.TrimRight(b.String(), "\n")
	case []any:
		if len(t) == 0 {
			return pad + "(none)"
		}
		var b strings.Builder
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				if name, id := named(m); name != "" {
					line := name
					if id != "" {
						line += "  " + id
					}
					fmt.Fprintf(&b, "%s- %s\n", pad, line)
					continue
				}
			}
			fmt.Fprintf(&b, "%s- %s\n", pad, strings.TrimSpace(humanizeValue(item, indent+1)))
		}
		return strings.TrimRight(b.String(), "\n")
	default:
		return pad + scalar(v)
	}
}

func named(m map[string]any) (name, id string) {
	name = scalar(first(m, "name", "key", "host", "email"))
	id = scalar(first(m, "id"))
	if name == "" || name == "—" {
		return "", ""
	}
	return name, id
}

func first(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && scalar(v) != "" && scalar(v) != "—" {
			return v
		}
	}
	return nil
}

func scalar(v any) string {
	switch t := v.(type) {
	case nil:
		return "—"
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
