package services

import (
	"strings"
	"testing"
)

func TestPrepareComposeAddsVolumesAndMagicEnv(t *testing.T) {
	raw := `# logo: svgs/wordpress.svg
services:
  wordpress:
    image: wordpress:latest
    volumes:
      - wordpress-files:/var/www/html
    environment:
      - SERVICE_URL_WORDPRESS
      - WORDPRESS_DB_HOST=mysql
      - WORDPRESS_DB_USER=$SERVICE_USER_WORDPRESS
      - WORDPRESS_DB_PASSWORD=$SERVICE_PASSWORD_WORDPRESS
      - WORDPRESS_DB_NAME=wordpress
    depends_on:
      - mysql
  mysql:
    image: mysql:8
    volumes:
      - mysql-data:/var/lib/mysql
    environment:
      - MYSQL_ROOT_PASSWORD=$SERVICE_PASSWORD_ROOT
      - MYSQL_DATABASE=wordpress
      - MYSQL_USER=$SERVICE_USER_WORDPRESS
      - MYSQL_PASSWORD=$SERVICE_PASSWORD_WORDPRESS
`
	out, env, err := PrepareCompose(raw, PrepareOpts{
		ServiceID: "test",
		Network:   "goolify",
		BaseURL:   "https://wp.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wordpress-files:") && !strings.Contains(out, "wordpress-files: null") && !strings.Contains(out, "wordpress-files:\n") {
		// yaml.v3 emits "wordpress-files: null" or "wordpress-files:"
		if !strings.Contains(out, "volumes:") {
			t.Fatalf("missing volumes section:\n%s", out)
		}
		if !strings.Contains(out, "wordpress-files") {
			t.Fatalf("missing wordpress-files volume:\n%s", out)
		}
	}
	if !strings.Contains(out, "mysql-data") {
		t.Fatalf("missing mysql-data volume:\n%s", out)
	}
	if strings.Contains(out, "$SERVICE_PASSWORD_WORDPRESS") {
		t.Fatalf("password not substituted:\n%s", out)
	}
	if strings.Contains(out, "$SERVICE_USER_WORDPRESS") {
		t.Fatalf("user not substituted:\n%s", out)
	}
	if env["SERVICE_URL_WORDPRESS"] != "https://wp.example.com" {
		t.Fatalf("url env: %#v", env["SERVICE_URL_WORDPRESS"])
	}
	if env["SERVICE_PASSWORD_WORDPRESS"] == "" || env["SERVICE_PASSWORD_ROOT"] == "" {
		t.Fatalf("passwords missing: %#v", env)
	}
	if !strings.Contains(out, "SERVICE_URL_WORDPRESS=https://wp.example.com") {
		t.Fatalf("bare SERVICE_URL not expanded to KEY=value:\n%s", out)
	}
	if !strings.Contains(out, "goolify") {
		t.Fatalf("expected network attachment:\n%s", out)
	}
	if !strings.Contains(out, "services:") {
		t.Fatalf("missing services:\n%s", out)
	}
	if !strings.Contains(out, "traefik.enable") {
		t.Fatalf("expected traefik labels when BaseURL host is set:\n%s", out)
	}
	if !strings.Contains(out, "Host(`wp.example.com`)") {
		t.Fatalf("expected Host rule:\n%s", out)
	}
	if !strings.Contains(out, "entrypoints: https") && !strings.Contains(out, `entrypoints: "https"`) && !strings.Contains(out, "entrypoints: https") {
		if !strings.Contains(out, "certresolver: letsencrypt") {
			t.Fatalf("expected https/letsencrypt labels:\n%s", out)
		}
	}
	if !strings.Contains(out, "loadbalancer.server.port") {
		t.Fatalf("expected traefik port:\n%s", out)
	}
}

func TestPrepareComposeMagicDomainHTTP(t *testing.T) {
	raw := `services:
  n8n:
    image: n8nio/n8n
    environment:
      - SERVICE_URL_N8N_5678
      - N8N_EDITOR_BASE_URL=${SERVICE_URL_N8N}
      - N8N_PROTOCOL=${N8N_PROTOCOL:-https}
`
	out, env, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:    "http://n8n.1.2.3.4.sslip.io",
		FQDN:       "n8n.1.2.3.4.sslip.io",
		Network:    "goolify",
		RouterName: "n8n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if env["SERVICE_URL_N8N"] != "http://n8n.1.2.3.4.sslip.io" {
		t.Fatalf("url: %#v", env["SERVICE_URL_N8N"])
	}
	if env["N8N_PROTOCOL"] != "http" || env["N8N_SECURE_COOKIE"] != "false" {
		t.Fatalf("http compat: %#v", env)
	}
	if !strings.Contains(out, "N8N_PROTOCOL=http") {
		t.Fatalf("expected N8N_PROTOCOL=http baked:\n%s", out)
	}
	if !strings.Contains(out, "N8N_SECURE_COOKIE=false") {
		t.Fatalf("expected N8N_SECURE_COOKIE=false:\n%s", out)
	}
	if strings.Contains(out, "certresolver") || strings.Contains(out, "entrypoints: https") {
		t.Fatalf("magic http must not use TLS labels:\n%s", out)
	}
	if !strings.Contains(out, "entrypoints: http") && !strings.Contains(out, `entrypoints: "http"`) {
		t.Fatalf("expected http entrypoint:\n%s", out)
	}
}

func TestPrepareComposePortSuffixAndCompanionKeys(t *testing.T) {
	raw := `services:
  n8n:
    image: n8nio/n8n
    environment:
      - SERVICE_URL_N8N_5678
      - N8N_EDITOR_BASE_URL=${SERVICE_URL_N8N}
      - N8N_HOST=${SERVICE_FQDN_N8N}
  redis:
    image: redis
`
	out, env, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:    "http://n8n.1.2.3.4.sslip.io",
		FQDN:       "n8n.1.2.3.4.sslip.io",
		Network:    "goolify",
		RouterName: "n8n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if env["SERVICE_URL_N8N_5678"] != "http://n8n.1.2.3.4.sslip.io" {
		t.Fatalf("port url: %#v", env["SERVICE_URL_N8N_5678"])
	}
	if env["SERVICE_URL_N8N"] != "http://n8n.1.2.3.4.sslip.io" {
		t.Fatalf("companion url missing: %#v", env)
	}
	if env["SERVICE_FQDN_N8N"] != "n8n.1.2.3.4.sslip.io" {
		t.Fatalf("companion fqdn missing: %#v", env)
	}
	if !strings.Contains(out, "N8N_EDITOR_BASE_URL=http://n8n.1.2.3.4.sslip.io") {
		t.Fatalf("companion not substituted:\n%s", out)
	}
	if !strings.Contains(out, "loadbalancer.server.port: \"5678\"") {
		t.Fatalf("expected traefik port 5678:\n%s", out)
	}
	if !strings.Contains(out, "Host(`n8n.1.2.3.4.sslip.io`)") {
		t.Fatalf("expected host rule:\n%s", out)
	}
}

func TestDetectProxyPortMultiSegmentName(t *testing.T) {
	if got := DetectProxyPort("SERVICE_URL_MY_APP_3000", ""); got != "3000" {
		t.Fatalf("got %q", got)
	}
	if got := DetectProxyPort("SERVICE_URL_N8N_5678", ""); got != "5678" {
		t.Fatalf("got %q", got)
	}
	if got := DetectProxyPort("SERVICE_URL_WORDPRESS", ""); got != "80" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractMagicEnvPreservesPasswords(t *testing.T) {
	raw := `services:
  wordpress:
    image: wordpress
    environment:
      - WORDPRESS_DB_PASSWORD=$SERVICE_PASSWORD_WORDPRESS
  mysql:
    image: mysql
    environment:
      - MYSQL_PASSWORD=$SERVICE_PASSWORD_WORDPRESS
`
	out1, env1, err := PrepareCompose(raw, PrepareOpts{BaseURL: "http://127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	pass := env1["SERVICE_PASSWORD_WORDPRESS"]
	if pass == "" {
		t.Fatal("missing password")
	}
	if !strings.Contains(out1, "SERVICE_PASSWORD_WORDPRESS="+pass) {
		t.Fatalf("password not persisted in compose:\n%s", out1)
	}
	out2, env2, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:     "http://wp.1.2.3.4.sslip.io",
		FQDN:        "wp.1.2.3.4.sslip.io",
		ExistingEnv: ExtractMagicEnv(out1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if env2["SERVICE_PASSWORD_WORDPRESS"] != pass {
		t.Fatalf("password regenerated: %q -> %q", pass, env2["SERVICE_PASSWORD_WORDPRESS"])
	}
	if !strings.Contains(out2, "WORDPRESS_DB_PASSWORD="+pass) {
		t.Fatalf("substituted password lost:\n%s", out2)
	}
	if !strings.Contains(out2, "Host(`wp.1.2.3.4.sslip.io`)") {
		t.Fatalf("expected domain labels:\n%s", out2)
	}
}

func TestPickProxyServiceExactOverContains(t *testing.T) {
	got := pickProxyService(map[string]any{
		"webhook": map[string]any{"image": "x"},
		"app":     map[string]any{"image": "y"},
	})
	if got != "app" {
		t.Fatalf("got %q want app", got)
	}
	got = pickProxyService(map[string]any{
		"webhook": map[string]any{"image": "x"},
		"mysql":   map[string]any{"image": "y"},
	})
	if got == "webhook" {
		// "web" must not match webhook via contains
		t.Fatalf("webhook should not win via 'web' substring, got %q", got)
	}
}

func TestPrepareComposeSkipsLocalhostProxy(t *testing.T) {
	raw := `services:
  app:
    image: nginx
`
	out, _, err := PrepareCompose(raw, PrepareOpts{BaseURL: "http://127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "traefik.enable") {
		t.Fatalf("should not inject traefik for localhost:\n%s", out)
	}
}

func TestNamedVolumeSkipsBindMounts(t *testing.T) {
	if got := namedVolumeFromMount("./data:/app"); got != "" {
		t.Fatalf("bind mount should be empty, got %q", got)
	}
	if got := namedVolumeFromMount("/var/lib:/app"); got != "" {
		t.Fatalf("abs bind should be empty, got %q", got)
	}
	if got := namedVolumeFromMount("db-data:/var/lib/mysql"); got != "db-data" {
		t.Fatalf("got %q", got)
	}
}
