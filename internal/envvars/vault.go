package envvars

import "regexp"

// vaultRef matches Coolify's provider-neutral remote secret syntax.
// The namespace is "vault" for every provider so it stays distinct from
// {{team.KEY}} / {{project.KEY}} shared-variable references.
var vaultRef = regexp.MustCompile(`\{\{\s*vault\.([A-Za-z0-9_]+)\s*\}\}`)

// envKey matches keys that can be written as shell environment variables.
var envKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

// ContainsVaultRef reports whether value has at least one {{vault.KEY}} reference.
func ContainsVaultRef(value string) bool {
	return vaultRef.MatchString(value)
}

// VaultKeys returns referenced remote key names, unique, in order of appearance.
func VaultKeys(value string) []string {
	matches := vaultRef.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, m := range matches {
		if _, ok := seen[m[1]]; ok {
			continue
		}
		seen[m[1]] = struct{}{}
		out = append(out, m[1])
	}
	return out
}

// MissingVaultKeys returns referenced keys that are absent from secrets.
func MissingVaultKeys(value string, secrets map[string]string) []string {
	var missing []string
	for _, key := range VaultKeys(value) {
		if _, ok := secrets[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

// SubstituteVault replaces {{vault.KEY}} with secrets[KEY].
// Keys missing from secrets are left unchanged. Callers must fail first
// when MissingVaultKeys is non-empty so a placeholder is never deployed.
func SubstituteVault(value string, secrets map[string]string) string {
	return vaultRef.ReplaceAllStringFunc(value, func(m string) string {
		parts := vaultRef.FindStringSubmatch(m)
		if len(parts) != 2 {
			return m
		}
		if v, ok := secrets[parts[1]]; ok {
			return v
		}
		return m
	})
}

// ValidEnvKey reports whether key can be stored as an environment variable name.
func ValidEnvKey(key string) bool {
	return envKey.MatchString(key)
}

// vaultEnvKey is a name that is both a shell env key and a {{vault.KEY}} token.
// Dots are valid in env names but cannot be written inside {{vault.KEY}}.
var vaultEnvKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidVaultEnvKey reports whether key can be imported as {{vault.KEY}}.
func ValidVaultEnvKey(key string) bool {
	return vaultEnvKey.MatchString(key)
}
