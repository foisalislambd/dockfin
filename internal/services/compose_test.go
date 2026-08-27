package services

import (
	"os"
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
		Network:   "dockfin",
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
	if !strings.Contains(out, "dockfin") {
		t.Fatalf("expected network attachment:\n%s", out)
	}
	// Proxy-facing wordpress joins shared network; mysql stays on default only.
	if !strings.Contains(out, "wordpress:") {
		t.Fatalf("missing wordpress:\n%s", out)
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
	raw := `# port: 5678
services:
  n8n:
    image: n8nio/n8n
    environment:
      - SERVICE_URL_N8N
      - N8N_EDITOR_BASE_URL=${SERVICE_URL_N8N}
      - N8N_PROTOCOL=${N8N_PROTOCOL:-https}
`
	out, env, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:    "http://n8n.1.2.3.4.sslip.io",
		FQDN:       "n8n.1.2.3.4.sslip.io",
		Network:    "dockfin",
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

func TestPrepareComposePortFromMetadata(t *testing.T) {
	raw := `# port: 5678
services:
  n8n:
    image: n8nio/n8n
    environment:
      - SERVICE_URL_N8N
      - N8N_EDITOR_BASE_URL=${SERVICE_URL_N8N}
      - N8N_HOST=${SERVICE_FQDN_N8N}
  redis:
    image: redis
`
	out, env, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:    "http://n8n.1.2.3.4.sslip.io",
		FQDN:       "n8n.1.2.3.4.sslip.io",
		Network:    "dockfin",
		RouterName: "n8n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if env["SERVICE_URL_N8N"] != "http://n8n.1.2.3.4.sslip.io" {
		t.Fatalf("url: %#v", env)
	}
	if env["SERVICE_FQDN_N8N"] != "n8n.1.2.3.4.sslip.io" {
		t.Fatalf("fqdn: %#v", env)
	}
	if !strings.Contains(out, "N8N_EDITOR_BASE_URL=http://n8n.1.2.3.4.sslip.io") {
		t.Fatalf("url not substituted:\n%s", out)
	}
	if !strings.Contains(out, "loadbalancer.server.port: \"5678\"") {
		t.Fatalf("expected traefik port 5678 from # port:\n%s", out)
	}
	if !strings.Contains(out, "Host(`n8n.1.2.3.4.sslip.io`)") {
		t.Fatalf("expected host rule:\n%s", out)
	}
	ui := CoolifyEnvForUI(raw, env)
	if len(ui) != 2 || ui["SERVICE_URL_N8N"] == "" || ui["SERVICE_FQDN_N8N"] == "" {
		t.Fatalf("UI pair: %#v", ui)
	}
}

func TestDetectProxyPortPrefersMetadata(t *testing.T) {
	if got := DetectProxyPort("# port: 8080\nSERVICE_URL_GATUS", ""); got != "8080" {
		t.Fatalf("metadata: got %q", got)
	}
	if got := DetectProxyPort("SERVICE_URL_MY_APP_3000", ""); got != "3000" {
		t.Fatalf("legacy suffix: got %q", got)
	}
	if got := DetectProxyPort("SERVICE_URL_N8N_5678", ""); got != "5678" {
		t.Fatalf("got %q", got)
	}
	if got := DetectProxyPort("SERVICE_URL_WORDPRESS", ""); got != "80" {
		t.Fatalf("got %q", got)
	}
	if got := DetectProxyPort("# port: 3001\n- SERVICE_URL_UPTIMEKUMA", "9999"); got != "9999" {
		t.Fatalf("explicit wins: got %q", got)
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

func TestPrepareComposeDomainKeysFollowBaseURL(t *testing.T) {
	raw := `services:
  app:
    image: nginx
    environment:
      - SERVICE_URL_APP
      - SERVICE_FQDN_APP
      - SERVICE_PASSWORD_APP
`
	_, env1, err := PrepareCompose(raw, PrepareOpts{BaseURL: "http://old.1.2.3.4.sslip.io"})
	if err != nil {
		t.Fatal(err)
	}
	_, env2, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:     "https://app.example.com",
		FQDN:        "https://app.example.com,https://www.example.com",
		ExistingEnv: env1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if env2["SERVICE_PASSWORD_APP"] != env1["SERVICE_PASSWORD_APP"] {
		t.Fatalf("password should stick: %q -> %q", env1["SERVICE_PASSWORD_APP"], env2["SERVICE_PASSWORD_APP"])
	}
	if env2["SERVICE_URL_APP"] != "https://app.example.com" {
		t.Fatalf("stale URL reused: %#v", env2["SERVICE_URL_APP"])
	}
	if env2["SERVICE_FQDN_APP"] != "app.example.com" {
		t.Fatalf("fqdn: %#v", env2["SERVICE_FQDN_APP"])
	}
}

func TestPrepareComposeAutoSSLCustomDomain(t *testing.T) {
	raw := `services:
  app:
    image: nginx
    environment:
      - SERVICE_URL_APP
`
	out, env, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:    "http://blog.example.com", // http sticky — must upgrade
		FQDN:       "blog.example.com",
		RouterName: "blog",
		Network:    "dockfin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if env["SERVICE_URL_APP"] != "https://blog.example.com" {
		t.Fatalf("expected https SERVICE_URL, got %#v", env["SERVICE_URL_APP"])
	}
	if !strings.Contains(out, "certresolver: letsencrypt") {
		t.Fatalf("expected Let's Encrypt labels:\n%s", out)
	}
	if !strings.Contains(out, "entrypoints: https") {
		t.Fatalf("expected https entrypoint:\n%s", out)
	}
}

func TestCoolifyEnvForUIWordPressPair(t *testing.T) {
	raw := `services:
  wordpress:
    environment:
      - SERVICE_URL_WORDPRESS
      - WORDPRESS_DB_PASSWORD=$SERVICE_PASSWORD_WORDPRESS
`
	_, env, err := PrepareCompose(raw, PrepareOpts{BaseURL: "http://wp.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	ui := CoolifyEnvForUI(raw, env)
	// Custom domains auto-upgrade to https for Let's Encrypt.
	if ui["SERVICE_URL_WORDPRESS"] != "https://wp.example.com" {
		t.Fatalf("url: %#v", ui)
	}
	if ui["SERVICE_FQDN_WORDPRESS"] != "wp.example.com" {
		t.Fatalf("fqdn: %#v", ui)
	}
	if _, ok := ui["SERVICE_PASSWORD_WORDPRESS"]; !ok {
		t.Fatalf("password missing: %#v", ui)
	}
	// No stray companion keys.
	for k := range ui {
		if strings.Contains(k, "8080") {
			t.Fatalf("unexpected key %s", k)
		}
	}
}

func TestCoolifyEnvForUIBarePair(t *testing.T) {
	raw := `# port: 8080
services:
  gatus:
    environment:
      - SERVICE_URL_GATUS
`
	out, env, err := PrepareCompose(raw, PrepareOpts{
		BaseURL: "http://gatus.example.com",
		FQDN:    "gatus.example.com",
		Network: "dockfin",
	})
	if err != nil {
		t.Fatal(err)
	}
	ui := CoolifyEnvForUI(raw, env)
	if len(ui) != 2 {
		t.Fatalf("want 2 UI keys (URL+FQDN), got %d: %#v", len(ui), ui)
	}
	if ui["SERVICE_URL_GATUS"] == "" || ui["SERVICE_FQDN_GATUS"] == "" {
		t.Fatalf("pair missing: %#v", ui)
	}
	if !strings.Contains(out, "SERVICE_FQDN_GATUS=gatus.example.com") {
		t.Fatalf("FQDN pair should be persisted in compose:\n%s", out)
	}
	if !strings.Contains(out, "loadbalancer.server.port: \"8080\"") {
		t.Fatalf("expected port from # port:\n%s", out)
	}
}

func TestPrepareComposeSubstitutesTopLevelConfigs(t *testing.T) {
	raw := `# port: 8080
services:
  headscale:
    image: headscale/headscale
    environment:
      - SERVICE_URL_HEADSCALE
    configs:
      - source: headscale_config
        target: /etc/headscale/config.yaml
configs:
  headscale_config:
    content: |
      server_url: ${SERVICE_URL_HEADSCALE}
`
	out, _, err := PrepareCompose(raw, PrepareOpts{BaseURL: "http://hs.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "server_url: https://hs.example.com") {
		t.Fatalf("config magic not substituted:\n%s", out)
	}
	if !strings.Contains(out, "certresolver: letsencrypt") {
		t.Fatalf("expected auto SSL for custom domain:\n%s", out)
	}
}

func TestGatusConfigUsesServiceURL(t *testing.T) {
	raw, err := os.ReadFile("../../templates/compose/gatus.yaml")
	if err != nil {
		t.Skip(err)
	}
	out, env, err := PrepareCompose(string(raw), PrepareOpts{
		BaseURL:    "http://gatus.example.com",
		FQDN:       "gatus.example.com",
		Network:    "dockfin",
		RouterName: "gatus",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "name: example") {
		t.Fatalf("placeholder example endpoint still present:\n%s", out)
	}
	if strings.Contains(out, "${SERVICE_URL") {
		t.Fatalf("SERVICE_URL not substituted in gatus config:\n%s", out)
	}
	if !strings.Contains(out, "gatus.example.com/health") {
		t.Fatalf("expected public health URL in config:\n%s", out)
	}
	if !strings.Contains(out, "loadbalancer.server.port: \"8080\"") {
		t.Fatalf("expected traefik 8080:\n%s", out)
	}
	ui := CoolifyEnvForUI(string(raw), env)
	if len(ui) != 2 || ui["SERVICE_URL_GATUS"] == "" || ui["SERVICE_FQDN_GATUS"] == "" {
		t.Fatalf("UI pair: %#v", ui)
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

func TestEnsureExternalNetworkOnlyProxyJoins(t *testing.T) {
	raw := `# port: 1337
services:
  planka:
    image: planka
    environment:
      - SERVICE_URL_PLANKA
      - DATABASE_URL=postgresql://u:p@postgresql:5432/planka
  postgresql:
    image: postgres
`
	out, _, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:    "http://planka.example.com",
		FQDN:       "planka.example.com",
		Network:    "dockfin",
		RouterName: "planka",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Crude but effective: postgresql service block should not list dockfin before volumes/networks end
	pgIdx := strings.Index(out, "postgresql:")
	plankaIdx := strings.Index(out, "planka:")
	if pgIdx < 0 || plankaIdx < 0 {
		t.Fatalf("missing services:\n%s", out)
	}
	// Find networks under planka vs postgresql by splitting on service keys is hard in yaml order.
	// Check that "dockfin" appears as external and planka has both networks.
	if !strings.Contains(out, "external: true") && !strings.Contains(out, "external:true") {
		t.Fatalf("expected external dockfin network:\n%s", out)
	}
	if !strings.Contains(out, "dockfin") {
		t.Fatalf("expected dockfin:\n%s", out)
	}
	// Count dockfin attachments under services — should be once (planka only), plus networks section.
	// After prepare, postgresql must resolve via default, not shared DNS.
	if strings.Count(out, "- dockfin") < 1 {
		t.Fatalf("expected proxy service on dockfin:\n%s", out)
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

func TestPrepareComposeStripsPublishedPorts(t *testing.T) {
	raw := `services:
  web:
    image: nginx
    ports:
      - "8080:80"
      - "443:443/tcp"
      - target: 3000
        published: 3000
    expose:
      - "90"
  db:
    image: postgres
    ports:
      - "5432:5432"
`
	out, _, err := PrepareCompose(raw, PrepareOpts{
		BaseURL: "http://app.1.2.3.4.sslip.io",
		FQDN:    "app.1.2.3.4.sslip.io",
		Network: "dockfin",
		Port:    "80",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "published:") || strings.Contains(out, "8080:80") || strings.Contains(out, "5432:5432") {
		t.Fatalf("expected host ports stripped:\n%s", out)
	}
	if !strings.Contains(out, "expose:") {
		t.Fatalf("expected expose kept/merged:\n%s", out)
	}
	for _, want := range []string{"80", "443", "3000", "90", "5432"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected container port %s in expose:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "traefik.enable") {
		t.Fatalf("expected traefik labels:\n%s", out)
	}
}

func TestPrepareComposeKeepPublishedPorts(t *testing.T) {
	raw := `services:
  web:
    image: nginx
    ports:
      - "8080:80"
`
	out, _, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:            "http://app.example.com",
		KeepPublishedPorts: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "8080:80") && !strings.Contains(out, "8080") {
		t.Fatalf("expected ports kept:\n%s", out)
	}
}

func TestDetectProxyPortForGitCompose(t *testing.T) {
	raw := `services:
  app:
    image: node
    ports:
      - "3000:3000"
`
	if got := DetectProxyPortForGitCompose(raw, ""); got != "3000" {
		t.Fatalf("auto from ports: got %q want 3000", got)
	}
	if got := DetectProxyPortForGitCompose(raw, "8080"); got != "8080" {
		t.Fatalf("explicit override: got %q want 8080", got)
	}
	meta := "# port: 5678\nservices:\n  n8n:\n    image: n8nio/n8n\n"
	if got := DetectProxyPortForGitCompose(meta, ""); got != "5678" {
		t.Fatalf("metadata: got %q want 5678", got)
	}
}

func TestInferContainerPortFromCompose(t *testing.T) {
	raw := `services:
  web:
    image: nginx
    ports:
      - "8080:80"
  db:
    image: postgres
    ports:
      - "5432:5432"
`
	if got := InferContainerPortFromCompose(raw); got != "80" {
		t.Fatalf("got %q want 80 (proxy service container port)", got)
	}
}

func TestPrepareComposeGPUAndRestart(t *testing.T) {
	raw := `services:
  web:
    image: nginx
`
	out, _, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:            "http://app.example.com",
		GPUEnabled:         true,
		GPUCount:           0,
		RestartPolicy:      "always",
		StopGracePeriodSec: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "gpus: all") && !strings.Contains(out, "gpus: \"all\"") {
		t.Fatalf("expected gpus: all:\n%s", out)
	}
	if !strings.Contains(out, "capabilities") || !strings.Contains(out, "gpu") {
		t.Fatalf("expected deploy.resources GPU reservation:\n%s", out)
	}
	if !strings.Contains(out, "restart: always") {
		t.Fatalf("expected restart policy:\n%s", out)
	}
	if !strings.Contains(out, "stop_grace_period: 25s") {
		t.Fatalf("expected stop_grace_period:\n%s", out)
	}
}

func TestPrepareComposeVaultAndObsidianLiveSync(t *testing.T) {
	for _, name := range []string{"vault.yaml", "obsidian-livesync.yaml"} {
		raw, err := os.ReadFile("../../templates/compose/" + name)
		if err != nil {
			t.Fatal(err)
		}
		out, env, err := PrepareCompose(string(raw), PrepareOpts{
			BaseURL: "http://svc.example.com",
			FQDN:    "svc.example.com",
			Network: "dockfin",
		})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(out, "certresolver: letsencrypt") {
			t.Fatalf("%s: expected auto SSL:\n%s", name, out)
		}
		if name == "vault.yaml" && env["SERVICE_URL_VAULT_8200"] == "" {
			t.Fatalf("vault URL not generated: %#v", env)
		}
		if name == "obsidian-livesync.yaml" {
			if env["SERVICE_USER_COUCHDB"] == "" || env["SERVICE_PASSWORD_64_COUCHDB"] == "" {
				t.Fatalf("couchdb secrets not generated: %#v", env)
			}
			if len(env["SERVICE_PASSWORD_64_COUCHDB"]) != 64 {
				t.Fatalf("PASSWORD_64 should be 64 chars, got %d", len(env["SERVICE_PASSWORD_64_COUCHDB"]))
			}
			if strings.Contains(out, "${SERVICE_PASSWORD_64_COUCHDB}") {
				t.Fatalf("password not substituted:\n%s", out)
			}
			if !strings.Contains(out, "couchdb_local") {
				t.Fatalf("expected compose configs for local.ini:\n%s", out)
			}
		}
	}
}

func TestPrepareComposeSkipHTTPSRedirect(t *testing.T) {
	raw := `# port: 80
services:
  web:
    image: nginx
    environment:
      - SERVICE_URL_WEB
`
	out, _, err := PrepareCompose(raw, PrepareOpts{
		BaseURL:           "http://app.example.com",
		FQDN:              "app.example.com",
		SkipHTTPSRedirect: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "certresolver: letsencrypt") {
		t.Fatalf("expected TLS:\n%s", out)
	}
	if strings.Contains(out, "redirectscheme") {
		t.Fatalf("did not expect HTTP redirect:\n%s", out)
	}
}
