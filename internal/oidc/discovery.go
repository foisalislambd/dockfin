package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Document is a subset of OpenID Provider Metadata used for the panel login flow.
type Document struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JwksURI               string `json:"jwks_uri"`
}

var httpClient = &http.Client{
	Timeout: 8 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return ValidateIssuerURL(req.URL.String())
	},
}

// DiscoveryURL returns the well-known configuration URL for an issuer.
func DiscoveryURL(issuer string) string {
	u := strings.TrimRight(strings.TrimSpace(issuer), "/")
	if u == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(u), "/.well-known/openid-configuration") {
		return u
	}
	return u + "/.well-known/openid-configuration"
}

// ValidateIssuerURL requires an absolute http(s) URL. Non-local issuers must use HTTPS.
func ValidateIssuerURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("issuer URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("issuer URL must be an absolute URL")
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		if isCloudMetadataHost(u.Hostname()) {
			return fmt.Errorf("issuer URL must not point at cloud metadata")
		}
		return nil
	case "http":
		host := u.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return nil
		}
		return fmt.Errorf("issuer URL must use https (http is only allowed for localhost)")
	default:
		return fmt.Errorf("issuer URL scheme must be https")
	}
}

func isCloudMetadataHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "169.254.169.254" || h == "metadata.google.internal" || h == "metadata.goog" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
	}
	return false
}

func validateOIDCEndpoint(raw, name string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("oidc discovery missing %s", name)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("oidc %s is not an absolute URL", name)
	}
	if err := ValidateIssuerURL(u.Scheme + "://" + u.Host); err != nil {
		return fmt.Errorf("oidc %s: %w", name, err)
	}
	return nil
}

// Fetch loads and validates the OpenID Provider configuration.
func Fetch(ctx context.Context, issuer string) (*Document, error) {
	if err := ValidateIssuerURL(issuer); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DiscoveryURL(issuer), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("oidc discovery HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("oidc discovery: invalid json")
	}
	if err := doc.Validate(); err != nil {
		return nil, err
	}
	return &doc, nil
}

// Validate ensures the endpoints needed for authorization-code + userinfo are present.
func (d *Document) Validate() error {
	if d == nil {
		return fmt.Errorf("empty oidc discovery document")
	}
	if err := validateOIDCEndpoint(d.AuthorizationEndpoint, "authorization_endpoint"); err != nil {
		return err
	}
	if err := validateOIDCEndpoint(d.TokenEndpoint, "token_endpoint"); err != nil {
		return err
	}
	if err := validateOIDCEndpoint(d.UserinfoEndpoint, "userinfo_endpoint"); err != nil {
		return err
	}
	return nil
}
