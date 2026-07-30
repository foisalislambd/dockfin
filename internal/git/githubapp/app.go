package githubapp

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type App struct {
	AppID         string
	ClientID      string
	PrivateKeyPEM string
	HTMLURL       string // e.g. https://github.com
	APIURL        string // e.g. https://api.github.com
	Name          string // GitHub App slug for install URL
}

func (a *App) apiBase() string {
	if a.APIURL != "" {
		return strings.TrimRight(a.APIURL, "/")
	}
	return "https://api.github.com"
}

func (a *App) htmlBase() string {
	if a.HTMLURL != "" {
		return strings.TrimRight(a.HTMLURL, "/")
	}
	return "https://github.com"
}

func (a *App) signedJWT() (string, error) {
	block, _ := pem.Decode([]byte(a.PrivateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("invalid private key PEM")
	}
	var signer any
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		signer = k
	} else if k8, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		signer = k8
	} else {
		return "", fmt.Errorf("unsupported private key")
	}
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": a.AppID,
	})
	return tok.SignedString(signer)
}

func (a *App) InstallationToken(installationID string) (string, error) {
	jwtStr, err := a.signedJWT()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, a.apiBase()+"/app/installations/"+url.PathEscape(installationID)+"/access_tokens", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtStr)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("installation token: %s %s", resp.Status, string(body))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.Token, nil
}

func (a *App) InstallURL(state string) string {
	name := a.Name
	if name == "" {
		name = "app"
	}
	u := a.htmlBase() + "/apps/" + url.PathEscape(name) + "/installations/new"
	if state != "" {
		u += "?state=" + url.QueryEscape(state)
	}
	return u
}

func (a *App) ListRepos(installationID string, page int) ([]map[string]any, error) {
	token, err := a.InstallationToken(installationID)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/installation/repositories?per_page=50&page=%d", a.apiBase(), page), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list repos: %s %s", resp.Status, string(body))
	}
	var out struct {
		Repositories []map[string]any `json:"repositories"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Repositories, nil
}

func CloneURL(repoHTTPS, token string) string {
	repoHTTPS = strings.TrimSpace(repoHTTPS)
	if token == "" {
		return repoHTTPS
	}
	u, err := url.Parse(repoHTTPS)
	if err != nil {
		return repoHTTPS
	}
	u.User = url.UserPassword("x-access-token", token)
	return u.String()
}
