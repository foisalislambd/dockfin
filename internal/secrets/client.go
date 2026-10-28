package secrets

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout = 10 * time.Second
	maxTimeout     = 30 * time.Second
	maxBody        = 8 << 20
)

// dopplerAPI is the Doppler API origin. Tests replace it with a local server.
var dopplerAPI = "https://api.doppler.com"

// Metadata is the non-secret half of an integration token.
type Metadata struct {
	BaseURL        string `json:"base_url"`
	ClientID       string `json:"client_id"`
	Namespace      string `json:"namespace"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// Settings selects the remote secret path for one resource.
type Settings struct {
	Project     string `json:"project"`
	Config      string `json:"config"`
	ProjectID   string `json:"project_id"`
	Environment string `json:"environment"`
	SecretPath  string `json:"secret_path"`
	Mount       string `json:"mount"`
	Path        string `json:"path"`
}

// Fetch loads secret values for a provider. Values stay in memory.
func Fetch(ctx context.Context, provider, token string, meta Metadata, settings Settings) (map[string]string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	timeout := timeoutOf(meta.TimeoutSeconds)
	switch provider {
	case "doppler":
		return fetchDoppler(ctx, token, settings, timeout)
	case "infisical":
		return fetchInfisical(ctx, token, meta, settings, timeout)
	case "vault":
		return fetchVault(ctx, token, meta, settings, timeout)
	default:
		return nil, fmt.Errorf("unsupported secret manager %q", provider)
	}
}

// Validate checks that the token can authenticate. It does not read secret values.
func Validate(ctx context.Context, provider, token string, meta Metadata) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	timeout := timeoutOf(meta.TimeoutSeconds)
	switch provider {
	case "doppler":
		_, err := doJSON(ctx, httpClient(timeout), http.MethodGet, dopplerAPI+"/v3/me", token, "bearer", nil, nil)
		if err != nil {
			return fmt.Errorf("doppler token was rejected")
		}
		return nil
	case "infisical":
		base, err := infisicalBase(meta)
		if err != nil {
			return err
		}
		if strings.TrimSpace(meta.ClientID) == "" || strings.TrimSpace(token) == "" {
			return fmt.Errorf("infisical client id and client secret are required")
		}
		if _, err := infisicalLogin(ctx, httpClient(timeout), base, meta.ClientID, token); err != nil {
			return fmt.Errorf("infisical login failed")
		}
		return nil
	case "vault":
		base, err := vaultBase(meta)
		if err != nil {
			return err
		}
		headers := vaultHeaders(token, meta.Namespace)
		if _, err := doJSON(ctx, httpClient(timeout), http.MethodGet, base+"/v1/auth/token/lookup-self", "", "", headers, nil); err != nil {
			return fmt.Errorf("vault token was rejected")
		}
		return nil
	default:
		return fmt.Errorf("unsupported secret manager %q", provider)
	}
}

// NormalizeMeta checks provider-specific metadata and fills defaults.
func NormalizeMeta(provider string, meta Metadata) (Metadata, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	meta.BaseURL = strings.TrimSpace(meta.BaseURL)
	meta.ClientID = strings.TrimSpace(meta.ClientID)
	meta.Namespace = strings.TrimSpace(meta.Namespace)
	if meta.TimeoutSeconds != 0 && (meta.TimeoutSeconds < 1 || meta.TimeoutSeconds > int(maxTimeout/time.Second)) {
		return meta, fmt.Errorf("timeout must be between 1 and %d seconds", int(maxTimeout/time.Second))
	}
	if strings.ContainsAny(meta.Namespace, "\r\n") {
		return meta, fmt.Errorf("namespace is invalid")
	}
	switch provider {
	case "doppler":
		meta.BaseURL = ""
		meta.ClientID = ""
		meta.Namespace = ""
		return meta, nil
	case "infisical":
		if meta.BaseURL == "" {
			meta.BaseURL = "https://app.infisical.com"
		}
		base, err := ValidateBaseURL(meta.BaseURL)
		if err != nil {
			return meta, err
		}
		meta.BaseURL = base
		if meta.ClientID == "" {
			return meta, fmt.Errorf("infisical client id is required")
		}
		meta.Namespace = ""
		return meta, nil
	case "vault":
		if meta.BaseURL == "" {
			return meta, fmt.Errorf("vault address is required")
		}
		base, err := ValidateBaseURL(meta.BaseURL)
		if err != nil {
			return meta, err
		}
		meta.BaseURL = base
		meta.ClientID = ""
		return meta, nil
	default:
		return meta, fmt.Errorf("unsupported secret manager %q", provider)
	}
}

// NormalizeSettings checks the per-resource source coordinates.
func NormalizeSettings(provider string, s Settings) (Settings, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	s.Project = strings.TrimSpace(s.Project)
	s.Config = strings.TrimSpace(s.Config)
	s.ProjectID = strings.TrimSpace(s.ProjectID)
	s.Environment = strings.TrimSpace(s.Environment)
	s.SecretPath = strings.TrimSpace(s.SecretPath)
	s.Mount = strings.Trim(strings.TrimSpace(s.Mount), "/")
	s.Path = strings.Trim(strings.TrimSpace(s.Path), "/")
	switch provider {
	case "doppler":
		s.ProjectID, s.Environment, s.SecretPath, s.Mount, s.Path = "", "", "", "", ""
		return s, nil
	case "infisical":
		if s.ProjectID == "" || s.Environment == "" {
			return s, fmt.Errorf("infisical project id and environment are required")
		}
		if s.SecretPath == "" {
			s.SecretPath = "/"
		}
		if !strings.HasPrefix(s.SecretPath, "/") || strings.Contains(s.SecretPath, "..") {
			return s, fmt.Errorf("infisical secret path must start with /")
		}
		s.Project, s.Config, s.Mount, s.Path = "", "", "", ""
		return s, nil
	case "vault":
		if s.Mount == "" {
			s.Mount = "secret"
		}
		if s.Path == "" || strings.Contains(s.Mount, "..") || strings.Contains(s.Path, "..") {
			return s, fmt.Errorf("vault mount and path are required")
		}
		if strings.ContainsAny(s.Mount+"/"+s.Path, "?#") {
			return s, fmt.Errorf("vault path is invalid")
		}
		s.Project, s.Config, s.ProjectID, s.Environment, s.SecretPath = "", "", "", "", ""
		return s, nil
	default:
		return s, fmt.Errorf("unsupported secret manager %q", provider)
	}
}

func fetchDoppler(ctx context.Context, token string, settings Settings, timeout time.Duration) (map[string]string, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("doppler token is required")
	}
	q := url.Values{"format": {"json"}}
	if settings.Project != "" {
		q.Set("project", settings.Project)
	}
	if settings.Config != "" {
		q.Set("config", settings.Config)
	}
	endpoint := dopplerAPI + "/v3/configs/config/secrets/download?" + q.Encode()
	body, err := doJSON(ctx, httpClient(timeout), http.MethodGet, endpoint, token, "bearer", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("doppler: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("doppler returned an unexpected payload")
	}
	return stringifyMap(raw), nil
}

func fetchInfisical(ctx context.Context, clientSecret string, meta Metadata, settings Settings, timeout time.Duration) (map[string]string, error) {
	base, err := infisicalBase(meta)
	if err != nil {
		return nil, err
	}
	settings, err = NormalizeSettings("infisical", settings)
	if err != nil {
		return nil, err
	}
	client := httpClient(timeout)
	access, err := infisicalLogin(ctx, client, base, meta.ClientID, clientSecret)
	if err != nil {
		return nil, err
	}
	q := url.Values{
		"projectId":              {settings.ProjectID},
		"environment":            {settings.Environment},
		"secretPath":             {settings.SecretPath},
		"includeImports":         {"true"},
		"expandSecretReferences": {"true"},
		"viewSecretValue":        {"true"},
	}
	body, err := doJSON(ctx, client, http.MethodGet, base+"/api/v4/secrets?"+q.Encode(), access, "bearer", nil, nil)
	if err != nil && strings.Contains(err.Error(), "HTTP 404") {
		q = url.Values{
			"workspaceId":            {settings.ProjectID},
			"environment":            {settings.Environment},
			"secretPath":             {settings.SecretPath},
			"include_imports":        {"true"},
			"expandSecretReferences": {"true"},
			"viewSecretValue":        {"true"},
		}
		body, err = doJSON(ctx, client, http.MethodGet, base+"/api/v3/secrets/raw?"+q.Encode(), access, "bearer", nil, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("infisical: %w", err)
	}
	var payload struct {
		Secrets []infisicalSecret `json:"secrets"`
		Imports []struct {
			Secrets []infisicalSecret `json:"secrets"`
		} `json:"imports"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("infisical returned an unexpected payload")
	}
	out := map[string]string{}
	for _, imp := range payload.Imports {
		for _, s := range imp.Secrets {
			if s.Key != "" {
				out[s.Key] = s.Value
			}
		}
	}
	for _, s := range payload.Secrets {
		if s.Key != "" {
			out[s.Key] = s.Value
		}
	}
	return out, nil
}

type infisicalSecret struct {
	Key   string
	Value string
}

func (s *infisicalSecret) UnmarshalJSON(b []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["secretKey"].(string); ok {
		s.Key = v
	}
	s.Value = asString(raw["secretValue"])
	return nil
}

func infisicalLogin(ctx context.Context, client *http.Client, base, clientID, clientSecret string) (string, error) {
	payload, _ := json.Marshal(map[string]string{"clientId": clientID, "clientSecret": clientSecret})
	body, err := doJSON(ctx, client, http.MethodPost, base+"/api/v1/auth/universal-auth/login", "", "", nil, payload)
	if err != nil {
		return "", fmt.Errorf("infisical login failed")
	}
	var resp struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.AccessToken == "" {
		return "", fmt.Errorf("infisical login failed")
	}
	return resp.AccessToken, nil
}

func fetchVault(ctx context.Context, token string, meta Metadata, settings Settings, timeout time.Duration) (map[string]string, error) {
	base, err := vaultBase(meta)
	if err != nil {
		return nil, err
	}
	settings, err = NormalizeSettings("vault", settings)
	if err != nil {
		return nil, err
	}
	endpoint := base + "/v1/" + url.PathEscape(settings.Mount) + "/data/" + escapePath(settings.Path)
	body, err := doJSON(ctx, httpClient(timeout), http.MethodGet, endpoint, "", "", vaultHeaders(token, meta.Namespace), nil)
	if err != nil {
		return nil, fmt.Errorf("vault: %w", err)
	}
	var payload struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("vault returned an unexpected payload")
	}
	return stringifyMap(payload.Data.Data), nil
}

func vaultHeaders(token, namespace string) map[string]string {
	h := map[string]string{"X-Vault-Token": token}
	if strings.TrimSpace(namespace) != "" {
		h["X-Vault-Namespace"] = namespace
	}
	return h
}

func infisicalBase(meta Metadata) (string, error) {
	if strings.TrimSpace(meta.BaseURL) == "" {
		meta.BaseURL = "https://app.infisical.com"
	}
	return ValidateBaseURL(meta.BaseURL)
}

func vaultBase(meta Metadata) (string, error) {
	if strings.TrimSpace(meta.BaseURL) == "" {
		return "", fmt.Errorf("vault address is required")
	}
	return ValidateBaseURL(meta.BaseURL)
}

func timeoutOf(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultTimeout
	}
	d := time.Duration(seconds) * time.Second
	if d > maxTimeout {
		return maxTimeout
	}
	return d
}

func stringifyMap(raw map[string]any) map[string]string {
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = asString(v)
	}
	return out
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func httpClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if blockedHostname(strings.ToLower(host)) {
				return nil, fmt.Errorf("blocked host")
			}
			var ips []net.IP
			if ip := net.ParseIP(host); ip != nil {
				ips = []net.IP{ip}
			} else {
				resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, err
				}
				for _, ip := range resolved {
					ips = append(ips, ip.IP)
				}
			}
			var last error
			for _, ip := range ips {
				if blockedIP(ip) {
					last = fmt.Errorf("blocked address")
					continue
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				last = err
			}
			if last == nil {
				last = fmt.Errorf("no address")
			}
			return nil, last
		},
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		ResponseHeaderTimeout: timeout,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func doJSON(ctx context.Context, client *http.Client, method, endpoint, token, scheme string, headers map[string]string, payload []byte) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		body = strings.NewReader(string(payload))
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch scheme {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed")
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("request failed")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("%s", providerError(res.StatusCode, raw))
	}
	return raw, nil
}

func providerError(status int, raw []byte) string {
	var payload map[string]any
	msg := ""
	if json.Unmarshal(raw, &payload) == nil {
		switch v := payload["message"].(type) {
		case string:
			msg = v
		}
		if msg == "" {
			if messages, ok := payload["messages"].([]any); ok && len(messages) > 0 {
				if s, ok := messages[0].(string); ok {
					msg = s
				}
			}
		}
		if msg == "" {
			if errors, ok := payload["errors"].([]any); ok && len(errors) > 0 {
				if s, ok := errors[0].(string); ok {
					msg = s
				}
			}
		}
	}
	msg = strings.TrimSpace(strings.ReplaceAll(msg, "\n", " "))
	if len(msg) > 180 {
		msg = msg[:180]
	}
	if msg == "" {
		return fmt.Sprintf("HTTP %d", status)
	}
	return fmt.Sprintf("HTTP %d: %s", status, msg)
}
