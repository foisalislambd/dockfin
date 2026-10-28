package envvars

import "testing"

func TestVaultRefs(t *testing.T) {
	value := "postgres://{{vault.DB_USER}}:{{ vault.DB_PASS }}@db/{{team.NAME}}"
	if !ContainsVaultRef(value) {
		t.Fatal("expected vault refs")
	}
	keys := VaultKeys(value)
	if len(keys) != 2 || keys[0] != "DB_USER" || keys[1] != "DB_PASS" {
		t.Fatalf("keys: %#v", keys)
	}
	secrets := map[string]string{"DB_USER": "app"}
	missing := MissingVaultKeys(value, secrets)
	if len(missing) != 1 || missing[0] != "DB_PASS" {
		t.Fatalf("missing: %#v", missing)
	}
	secrets["DB_PASS"] = "s3cret"
	got := SubstituteVault(value, secrets)
	if got != "postgres://app:s3cret@db/{{team.NAME}}" {
		t.Fatalf("substitute: %q", got)
	}
}

func TestValidEnvKey(t *testing.T) {
	if !ValidEnvKey("DB_PASSWORD") || !ValidEnvKey("_A.B") {
		t.Fatal("expected valid keys")
	}
	if ValidEnvKey("db-password") || ValidEnvKey("1ABC") || ValidEnvKey("") {
		t.Fatal("expected invalid keys")
	}
}
