package envvars

import (
	"regexp"
)

var refPattern = regexp.MustCompile(`\{\{(team|project|environment|server)\.([A-Za-z0-9_]+)\}\}`)

// Resolve replaces {{team.KEY}} / {{project.KEY}} / {{environment.KEY}} / {{server.KEY}} placeholders.
func Resolve(value string, scopes map[string]map[string]string) string {
	return refPattern.ReplaceAllStringFunc(value, func(m string) string {
		parts := refPattern.FindStringSubmatch(m)
		if len(parts) != 3 {
			return m
		}
		scope, key := parts[1], parts[2]
		if vals, ok := scopes[scope]; ok {
			if v, ok := vals[key]; ok {
				return v
			}
		}
		return m
	})
}
