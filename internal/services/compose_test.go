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
		BaseURL:   "http://wp.example.com",
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
	if env["SERVICE_URL_WORDPRESS"] != "http://wp.example.com" {
		t.Fatalf("url env: %#v", env["SERVICE_URL_WORDPRESS"])
	}
	if env["SERVICE_PASSWORD_WORDPRESS"] == "" || env["SERVICE_PASSWORD_ROOT"] == "" {
		t.Fatalf("passwords missing: %#v", env)
	}
	if !strings.Contains(out, "SERVICE_URL_WORDPRESS=http://wp.example.com") {
		t.Fatalf("bare SERVICE_URL not expanded to KEY=value:\n%s", out)
	}
	if !strings.Contains(out, "goolify") {
		t.Fatalf("expected network attachment:\n%s", out)
	}
	if !strings.Contains(out, "services:") {
		t.Fatalf("missing services:\n%s", out)
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
