package services

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	dockerfileEnvLine = regexp.MustCompile(`(?i)^\s*ENV\s+(.+)$`)
	dockerfileExpose  = regexp.MustCompile(`(?i)^\s*EXPOSE\s+(\d+)`)
)

// PortFromDockerfile returns the first EXPOSE port, or 0 if none.
func PortFromDockerfile(dockerfile string) int {
	for _, line := range strings.Split(dockerfile, "\n") {
		m := dockerfileExpose.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err == nil && n > 0 && n <= 65535 {
			return n
		}
	}
	return 0
}

// EnvFromDockerfile extracts KEY=value pairs from ENV instructions.
// Supports both `ENV KEY=value` and `ENV KEY value` forms; skips empty keys.
func EnvFromDockerfile(dockerfile string) map[string]string {
	out := map[string]string{}
	for _, raw := range strings.Split(dockerfile, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := dockerfileEnvLine.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		rest := strings.TrimSpace(m[1])
		// Multi-var: ENV A=1 B=2
		if strings.Contains(rest, "=") {
			for _, part := range splitDockerfileEnvParts(rest) {
				k, v, ok := strings.Cut(part, "=")
				k = strings.TrimSpace(k)
				if !ok || k == "" {
					continue
				}
				out[k] = unquoteDockerfileValue(strings.TrimSpace(v))
			}
			continue
		}
		// ENV KEY value
		fields := strings.Fields(rest)
		if len(fields) >= 2 {
			out[fields[0]] = unquoteDockerfileValue(strings.Join(fields[1:], " "))
		}
	}
	return out
}

func splitDockerfileEnvParts(s string) []string {
	var parts []string
	var b strings.Builder
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote != 0 {
			b.WriteByte(c)
			if c == inQuote && (i == 0 || s[i-1] != '\\') {
				inQuote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = c
			b.WriteByte(c)
			continue
		}
		if c == ' ' || c == '\t' {
			if b.Len() > 0 {
				parts = append(parts, b.String())
				b.Reset()
			}
			continue
		}
		b.WriteByte(c)
	}
	if b.Len() > 0 {
		parts = append(parts, b.String())
	}
	return parts
}

func unquoteDockerfileValue(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

// InjectDockerfileBuildArgs inserts missing `ARG KEY` lines after the first FROM
// so `docker build --build-arg KEY=…` can consume Dockfin build-time env vars
// (Coolify-style). Existing ARG declarations are left untouched.
func InjectDockerfileBuildArgs(dockerfile string, keys []string) string {
	if strings.TrimSpace(dockerfile) == "" || len(keys) == 0 {
		return dockerfile
	}
	existing := map[string]struct{}{}
	for _, raw := range strings.Split(dockerfile, "\n") {
		line := strings.TrimSpace(raw)
		if len(line) < 4 || !strings.EqualFold(line[:4], "ARG ") {
			continue
		}
		rest := strings.TrimSpace(line[4:])
		k, _, _ := strings.Cut(rest, "=")
		k = strings.TrimSpace(k)
		if k != "" {
			existing[k] = struct{}{}
		}
	}
	var toAdd []string
	seen := map[string]struct{}{}
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := existing[k]; ok {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		toAdd = append(toAdd, "ARG "+k)
	}
	if len(toAdd) == 0 {
		return dockerfile
	}
	lines := strings.Split(dockerfile, "\n")
	out := make([]string, 0, len(lines)+len(toAdd))
	inserted := false
	for _, line := range lines {
		out = append(out, line)
		trim := strings.TrimSpace(line)
		if !inserted && len(trim) >= 5 && strings.EqualFold(trim[:5], "FROM ") {
			out = append(out, toAdd...)
			inserted = true
		}
	}
	if !inserted {
		out = append(toAdd, out...)
	}
	return strings.Join(out, "\n")
}
