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
