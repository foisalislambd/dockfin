package services

import (
	"strings"
	"testing"
)

func TestComposeEnvForUIGlobalUSDStyle(t *testing.T) {
	raw := `
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-globalusd}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?Set POSTGRES_PASSWORD in .env}
      POSTGRES_DB: ${POSTGRES_DB:-globalusd}
  backend:
    build:
      context: ./backend
      args:
        NEXT_PUBLIC_API_URL: ${NEXT_PUBLIC_API_URL:?Set NEXT_PUBLIC_API_URL in .env}
    environment:
      DATABASE_URL: postgresql://${POSTGRES_USER:-globalusd}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB:-globalusd}
      JWT_SECRET: ${JWT_SECRET:?Set JWT_SECRET in .env}
      CORS_ORIGIN: ${CORS_ORIGIN:?Set CORS_ORIGIN in .env}
      NODE_ENV: production
      PORT: 4000
`
	got := ComposeEnvForUI(raw)
	wantKeys := []string{
		"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB",
		"JWT_SECRET", "CORS_ORIGIN", "NEXT_PUBLIC_API_URL",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Fatalf("missing key %s in %#v", k, got)
		}
	}
	if got["POSTGRES_USER"].Value != "globalusd" {
		t.Fatalf("POSTGRES_USER default: %#v", got["POSTGRES_USER"])
	}
	if got["POSTGRES_PASSWORD"].Value != "" {
		t.Fatalf(":? should not use error text as value: %#v", got["POSTGRES_PASSWORD"])
	}
	if !strings.Contains(got["POSTGRES_PASSWORD"].Comment, "POSTGRES_PASSWORD") {
		t.Fatalf(":? comment: %#v", got["POSTGRES_PASSWORD"])
	}
	if _, ok := got["NODE_ENV"]; ok {
		t.Fatalf("hardcoded NODE_ENV should not become a UI env var")
	}
	if _, ok := got["PORT"]; ok {
		t.Fatalf("hardcoded PORT should not become a UI env var")
	}
}

func TestComposeEnvForUISkipsServiceMagic(t *testing.T) {
	raw := `
services:
  app:
    environment:
      - SERVICE_PASSWORD_APP
      - WORDPRESS_DB_PASSWORD=$SERVICE_PASSWORD_APP
      - DB=${SERVICE_PASSWORD_DB:-x}
`
	got := ComposeEnvForUI(raw)
	if _, ok := got["SERVICE_PASSWORD_APP"]; ok {
		t.Fatal("SERVICE_* should be excluded")
	}
	if _, ok := got["SERVICE_PASSWORD_DB"]; ok {
		t.Fatal("SERVICE_* should be excluded")
	}
	if got["WORDPRESS_DB_PASSWORD"].Value != "$SERVICE_PASSWORD_APP" {
		t.Fatalf("wrapper key: %#v", got["WORDPRESS_DB_PASSWORD"])
	}
}

func TestComposeEnvForUIBareListKey(t *testing.T) {
	raw := `
services:
  app:
    environment:
      - FOO
      - BAR=baz
      - BAZ=${BAZ:-qux}
`
	got := ComposeEnvForUI(raw)
	if _, ok := got["FOO"]; !ok {
		t.Fatal("bare list FOO")
	}
	if _, ok := got["BAZ"]; !ok || got["BAZ"].Value != "qux" {
		t.Fatalf("BAZ: %#v", got["BAZ"])
	}
	if _, ok := got["BAR"]; ok {
		t.Fatal("hardcoded BAR=baz should not create UI var")
	}
}

func TestComposeEnvForUINestedInURL(t *testing.T) {
	// Coolify skips values that don't start with `$`; we still extract nested ${VAR}.
	raw := `
services:
  api:
    environment:
      DATABASE_URL: postgresql://${DB_USER:-app}:${DB_PASS:?required}@db:5432/${DB_NAME:-app}
      NODE_ENV: production
`
	got := ComposeEnvForUI(raw)
	if got["DB_USER"].Value != "app" {
		t.Fatalf("DB_USER: %#v", got["DB_USER"])
	}
	if got["DB_PASS"].Value != "" || got["DB_PASS"].Comment == "" {
		t.Fatalf("DB_PASS: %#v", got["DB_PASS"])
	}
	if got["DB_NAME"].Value != "app" {
		t.Fatalf("DB_NAME: %#v", got["DB_NAME"])
	}
	if _, ok := got["DATABASE_URL"]; ok {
		t.Fatal("DATABASE_URL itself is not a ${} name")
	}
	if _, ok := got["NODE_ENV"]; ok {
		t.Fatal("hardcoded skipped")
	}
}

func TestSplitComposeVarOperatorOrder(t *testing.T) {
	name, op, def, ok := splitComposeVarOperator("FOO:-bar-baz")
	if !ok || name != "FOO" || op != ":-" || def != "bar-baz" {
		t.Fatalf("got %q %q %q %v", name, op, def, ok)
	}
	name, op, def, ok = splitComposeVarOperator("FOO:?must set")
	if !ok || name != "FOO" || op != ":?" || def != "must set" {
		t.Fatalf("got %q %q %q %v", name, op, def, ok)
	}
}
