package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env              string
	HTTPAddr         string
	DatabaseURL      string
	MasterKey        string
	CORSOrigins      []string
	PublicURL        string
	DataDir          string
	WebDir           string
	CookieSecure     bool
	SessionTTL       time.Duration
	TraefikImage     string
	CaddyImage       string
	AcmeEmail        string
	BootstrapSelf    bool   // auto-register this VPS as a server on first register
	PublicIP         string // optional override for bootstrap / magic DNS
	BootstrapSSHUser string
	BootstrapSSHPort int
	TrustProxy       bool // honor X-Forwarded-* / RealIP only when true
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Env:              getenv("DOCKFIN_ENV", "development"),
		HTTPAddr:         getenv("DOCKFIN_HTTP_ADDR", ":8000"),
		DatabaseURL:      getenv("DOCKFIN_DATABASE_URL", "postgres://dockfin:dockfin@localhost:5432/dockfin?sslmode=disable"),
		MasterKey:        getenv("DOCKFIN_MASTER_KEY", ""),
		PublicURL:        getenv("DOCKFIN_PUBLIC_URL", "http://localhost:8000"),
		DataDir:          getenv("DOCKFIN_DATA_DIR", "./data"),
		WebDir:           getenv("DOCKFIN_WEB_DIR", ""),
		SessionTTL:       7 * 24 * time.Hour,
		TraefikImage:     getenv("DOCKFIN_TRAEFIK_IMAGE", "traefik:v3.6"),
		CaddyImage:       getenv("DOCKFIN_CADDY_IMAGE", "lucaslorentz/caddy-docker-proxy:2.9-alpine"),
		AcmeEmail:        getenv("DOCKFIN_ACME_EMAIL", "admin@sslip.io"),
		PublicIP:         strings.TrimSpace(getenv("DOCKFIN_PUBLIC_IP", "")),
		BootstrapSSHUser: getenv("DOCKFIN_BOOTSTRAP_SSH_USER", "root"),
		BootstrapSSHPort: 22,
	}
	cfg.BootstrapSelf = parseBool(getenv("DOCKFIN_BOOTSTRAP_SELF", "1"), true)
	cfg.TrustProxy = parseBool(getenv("DOCKFIN_TRUST_PROXY", "0"), false)
	if p := getenv("DOCKFIN_BOOTSTRAP_SSH_PORT", ""); p != "" {
		if n, err := fmt.Sscanf(p, "%d", &cfg.BootstrapSSHPort); n != 1 || err != nil || cfg.BootstrapSSHPort <= 0 {
			cfg.BootstrapSSHPort = 22
		}
	}

	origins := getenv("DOCKFIN_CORS_ORIGINS", "http://localhost:5173")
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, o)
		}
	}
	if cfg.PublicURL != "" {
		cfg.CORSOrigins = append(cfg.CORSOrigins, cfg.PublicURL)
	}

	// Secure cookies only over HTTPS. HTTP VPS IPs (http://x.x.x.x:8000) must not set Secure.
	switch strings.ToLower(strings.TrimSpace(getenv("DOCKFIN_COOKIE_SECURE", ""))) {
	case "1", "true", "yes":
		cfg.CookieSecure = true
	case "0", "false", "no":
		cfg.CookieSecure = false
	default:
		cfg.CookieSecure = strings.HasPrefix(strings.ToLower(cfg.PublicURL), "https://")
	}

	if len(cfg.MasterKey) < 32 {
		return nil, fmt.Errorf("DOCKFIN_MASTER_KEY must be at least 32 characters")
	}
	return cfg, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func parseBool(v string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func (c *Config) IsDev() bool { return c.Env == "development" }
