package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env            string
	HTTPAddr       string
	DatabaseURL    string
	RedisURL       string
	MasterKey      string
	SessionSecret  string
	CORSOrigins    []string
	PublicURL      string
	DataDir        string
	WebDir         string
	CookieSecure   bool
	SessionTTL     time.Duration
	TraefikImage   string
	CaddyImage     string
	AcmeEmail      string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Env:           getenv("GOOLIFY_ENV", "development"),
		HTTPAddr:      getenv("GOOLIFY_HTTP_ADDR", ":8080"),
		DatabaseURL:   getenv("GOOLIFY_DATABASE_URL", "postgres://goolify:goolify@localhost:5432/goolify?sslmode=disable"),
		RedisURL:      getenv("GOOLIFY_REDIS_URL", "redis://localhost:6379/0"),
		MasterKey:     getenv("GOOLIFY_MASTER_KEY", ""),
		SessionSecret: getenv("GOOLIFY_SESSION_SECRET", ""),
		PublicURL:     getenv("GOOLIFY_PUBLIC_URL", "http://localhost:8080"),
		DataDir:       getenv("GOOLIFY_DATA_DIR", "./data"),
		WebDir:        getenv("GOOLIFY_WEB_DIR", ""),
		SessionTTL:    7 * 24 * time.Hour,
		TraefikImage:  getenv("GOOLIFY_TRAEFIK_IMAGE", "traefik:v3.6"),
		CaddyImage:    getenv("GOOLIFY_CADDY_IMAGE", "lucaslorentz/caddy-docker-proxy:2.9-alpine"),
		AcmeEmail:     getenv("GOOLIFY_ACME_EMAIL", "admin@sslip.io"),
	}

	origins := getenv("GOOLIFY_CORS_ORIGINS", "http://localhost:5173")
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, o)
		}
	}
	if cfg.PublicURL != "" {
		cfg.CORSOrigins = append(cfg.CORSOrigins, cfg.PublicURL)
	}

	// Secure cookies only over HTTPS. HTTP VPS IPs (http://x.x.x.x:8080) must not set Secure.
	switch strings.ToLower(strings.TrimSpace(getenv("GOOLIFY_COOKIE_SECURE", ""))) {
	case "1", "true", "yes":
		cfg.CookieSecure = true
	case "0", "false", "no":
		cfg.CookieSecure = false
	default:
		cfg.CookieSecure = strings.HasPrefix(strings.ToLower(cfg.PublicURL), "https://")
	}

	if len(cfg.MasterKey) < 32 {
		return nil, fmt.Errorf("GOOLIFY_MASTER_KEY must be at least 32 characters")
	}
	if len(cfg.SessionSecret) < 32 {
		return nil, fmt.Errorf("GOOLIFY_SESSION_SECRET must be at least 32 characters")
	}
	return cfg, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func (c *Config) IsDev() bool { return c.Env == "development" }
