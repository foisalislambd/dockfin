package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/dockfin/dockfin/internal/config"
	"github.com/dockfin/dockfin/internal/crypto"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/oauth2"
)

const oauthStateCookie = "dockfin_oauth_state"

func (a *API) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	team, err := a.Store.CreateTeam(r.Context(), currentUser(r).ID, body.Name, body.Description)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"team": team})
}

func (a *API) handleOauthProviders(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListEnabledOauthProviders(r.Context())
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": list})
}

func (a *API) handleOauthStart(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "provider")))
	m, err := a.Store.GetOauthSettingMaterial(r.Context(), provider)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "unknown provider")
			return
		}
		mapStoreErr(w, err)
		return
	}
	if !m.Enabled {
		writeError(w, http.StatusNotFound, "provider not enabled")
		return
	}
	spec, err := buildOauthProviderSpec(provider, m)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	conf := &oauth2.Config{
		ClientID:     m.ClientID,
		ClientSecret: m.ClientSecret,
		Endpoint:     oauth2.Endpoint{AuthURL: spec.AuthURL, TokenURL: spec.TokenURL},
		RedirectURL:  oauthRedirectURI(a.Cfg, m, provider),
		Scopes:       spec.Scopes,
	}
	state, err := crypto.RandomToken(24)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecureForRequest(r, a.Cfg),
	})
	http.Redirect(w, r, conf.AuthCodeURL(state), http.StatusFound)
}

func (a *API) handleOauthCallback(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "provider")))
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "code and state required")
		return
	}
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || cookie.Value == "" || cookie.Value != state {
		writeError(w, http.StatusBadRequest, "invalid or expired state")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecureForRequest(r, a.Cfg),
	})

	m, err := a.Store.GetOauthSettingMaterial(r.Context(), provider)
	if err != nil || !m.Enabled {
		writeError(w, http.StatusNotFound, "provider not enabled")
		return
	}
	spec, err := buildOauthProviderSpec(provider, m)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	conf := &oauth2.Config{
		ClientID:     m.ClientID,
		ClientSecret: m.ClientSecret,
		Endpoint:     oauth2.Endpoint{AuthURL: spec.AuthURL, TokenURL: spec.TokenURL},
		RedirectURL:  oauthRedirectURI(a.Cfg, m, provider),
		Scopes:       spec.Scopes,
	}
	tok, err := conf.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oauth exchange failed: "+err.Error())
		return
	}
	profile, err := fetchOauthProfile(r.Context(), conf, tok, spec.UserInfoURL, provider)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch profile: "+err.Error())
		return
	}
	if profile.ID == "" {
		writeError(w, http.StatusBadGateway, "provider did not return a user id")
		return
	}
	if profile.Email == "" {
		writeError(w, http.StatusBadGateway, "provider did not return an email address")
		return
	}

	user, err := a.Store.FindOauthAccountUser(r.Context(), provider, profile.ID)
	if err != nil && err != store.ErrNotFound {
		mapStoreErr(w, err)
		return
	}
	if user == nil {
		existing, _, existErr := a.Store.GetUserByEmail(r.Context(), profile.Email)
		switch {
		case existErr == nil:
			user = existing
		case existErr == store.ErrNotFound:
			enabled, regErr := a.Store.RegistrationEnabled(r.Context())
			if regErr != nil {
				mapStoreErr(w, regErr)
				return
			}
			if !enabled {
				writeError(w, http.StatusForbidden, "registration disabled")
				return
			}
			name := profile.Name
			if name == "" {
				name = profile.Email
			}
			newUser, _, createErr := a.Store.CreateUserOAuth(r.Context(), profile.Email, name)
			if createErr != nil {
				mapStoreErr(w, createErr)
				return
			}
			user = newUser
		default:
			mapStoreErr(w, existErr)
			return
		}
		if linkErr := a.Store.LinkOauthAccount(r.Context(), user.ID, provider, profile.ID, profile.Email); linkErr != nil {
			mapStoreErr(w, linkErr)
			return
		}
	}

	if has, err := a.Store.UserHasTOTP(r.Context(), user.ID); err == nil && has {
		challenge, err := a.Store.CreateAuthChallenge(r.Context(), user.ID, "totp", 5*time.Minute)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		http.Redirect(w, r, "/login?challenge_id="+challenge.ID.String(), http.StatusFound)
		return
	}

	if _, _, err := a.issueSession(w, r, user); err != nil {
		mapStoreErr(w, err)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// --- provider configuration ---

type oauthProviderSpec struct {
	AuthURL     string
	TokenURL    string
	UserInfoURL string
	Scopes      []string
}

func buildOauthProviderSpec(provider string, m *store.OauthMaterial) (*oauthProviderSpec, error) {
	switch provider {
	case "github":
		return &oauthProviderSpec{
			AuthURL:     "https://github.com/login/oauth/authorize",
			TokenURL:    "https://github.com/login/oauth/access_token",
			UserInfoURL: "https://api.github.com/user",
			Scopes:      []string{"read:user", "user:email"},
		}, nil
	case "gitlab":
		base := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/")
		if base == "" {
			base = "https://gitlab.com"
		}
		return &oauthProviderSpec{
			AuthURL:     base + "/oauth/authorize",
			TokenURL:    base + "/oauth/token",
			UserInfoURL: base + "/api/v4/user",
			Scopes:      []string{"read_user"},
		}, nil
	case "bitbucket":
		return &oauthProviderSpec{
			AuthURL:     "https://bitbucket.org/site/oauth2/authorize",
			TokenURL:    "https://bitbucket.org/site/oauth2/access_token",
			UserInfoURL: "https://api.bitbucket.org/2.0/user",
			Scopes:      []string{"account", "email"},
		}, nil
	case "google":
		return &oauthProviderSpec{
			AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:    "https://oauth2.googleapis.com/token",
			UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
			Scopes:      []string{"openid", "email", "profile"},
		}, nil
	case "azure":
		tenant := strings.TrimSpace(m.Tenant)
		if tenant == "" {
			tenant = "common"
		}
		return &oauthProviderSpec{
			AuthURL:     fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", tenant),
			TokenURL:    fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenant),
			UserInfoURL: "https://graph.microsoft.com/v1.0/me",
			Scopes:      []string{"openid", "email", "profile", "User.Read"},
		}, nil
	case "discord":
		return &oauthProviderSpec{
			AuthURL:     "https://discord.com/api/oauth2/authorize",
			TokenURL:    "https://discord.com/api/oauth2/token",
			UserInfoURL: "https://discord.com/api/users/@me",
			Scopes:      []string{"identify", "email"},
		}, nil
	case "authentik", "clerk", "infomaniak", "zitadel":
		base := strings.TrimRight(strings.TrimSpace(m.BaseURL), "/")
		if base == "" {
			return nil, fmt.Errorf("base_url is not configured for %s", provider)
		}
		// Generic OIDC-style endpoints; providers exposing a discovery document
		// at this base_url are expected to follow this convention.
		return &oauthProviderSpec{
			AuthURL:     base + "/oauth/authorize",
			TokenURL:    base + "/oauth/token",
			UserInfoURL: base + "/oauth/userinfo",
			Scopes:      []string{"openid", "email", "profile"},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported oauth provider %q", provider)
	}
}

func oauthRedirectURI(cfg *config.Config, m *store.OauthMaterial, provider string) string {
	if u := strings.TrimSpace(m.RedirectURI); u != "" {
		return u
	}
	base := ""
	if cfg != nil {
		base = strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/")
	}
	return base + "/api/v1/auth/oauth/" + provider + "/callback"
}

// --- profile fetching ---

type oauthProfile struct {
	ID    string
	Email string
	Name  string
}

func fetchOauthProfile(ctx context.Context, conf *oauth2.Config, tok *oauth2.Token, userInfoURL, provider string) (*oauthProfile, error) {
	client := conf.Client(ctx, tok)
	raw, err := fetchOauthJSON(ctx, client, userInfoURL, provider)
	if err != nil {
		return nil, err
	}

	profile := &oauthProfile{}
	switch provider {
	case "github":
		profile.ID = rawOauthStr(raw, "id")
		profile.Name = rawOauthStr(raw, "name")
		profile.Email = rawOauthStr(raw, "email")
		if profile.Name == "" {
			profile.Name = rawOauthStr(raw, "login")
		}
		if profile.Email == "" {
			profile.Email = fetchGithubPrimaryEmail(ctx, client)
		}
	case "gitlab":
		profile.ID = rawOauthStr(raw, "id")
		profile.Email = rawOauthStr(raw, "email")
		profile.Name = rawOauthStr(raw, "name")
	case "bitbucket":
		profile.ID = rawOauthStr(raw, "uuid")
		profile.Name = rawOauthStr(raw, "display_name")
		profile.Email = fetchBitbucketPrimaryEmail(ctx, client)
	case "discord":
		profile.ID = rawOauthStr(raw, "id")
		profile.Email = rawOauthStr(raw, "email")
		profile.Name = rawOauthStr(raw, "username")
	case "google":
		profile.ID = rawOauthStr(raw, "sub")
		profile.Email = rawOauthStr(raw, "email")
		profile.Name = rawOauthStr(raw, "name")
	case "azure":
		profile.ID = rawOauthStr(raw, "id")
		profile.Email = rawOauthStr(raw, "mail")
		if profile.Email == "" {
			profile.Email = rawOauthStr(raw, "userPrincipalName")
		}
		profile.Name = rawOauthStr(raw, "displayName")
	default: // generic OIDC-style userinfo (authentik, clerk, infomaniak, zitadel, ...)
		profile.ID = rawOauthStr(raw, "sub")
		profile.Email = rawOauthStr(raw, "email")
		profile.Name = rawOauthStr(raw, "name")
	}
	return profile, nil
}

func fetchOauthJSON(ctx context.Context, client *http.Client, url, provider string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if provider == "github" {
		req.Header.Set("Accept", "application/vnd.github+json")
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func rawOauthStr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func fetchGithubPrimaryEmail(ctx context.Context, client *http.Client) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return ""
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(res.Body).Decode(&emails); err != nil {
		return ""
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email
		}
	}
	if len(emails) > 0 {
		return emails[0].Email
	}
	return ""
}

func fetchBitbucketPrimaryEmail(ctx context.Context, client *http.Client) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.bitbucket.org/2.0/user/emails", nil)
	if err != nil {
		return ""
	}
	res, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return ""
	}
	var payload struct {
		Values []struct {
			Email       string `json:"email"`
			IsPrimary   bool   `json:"is_primary"`
			IsConfirmed bool   `json:"is_confirmed"`
		} `json:"values"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ""
	}
	for _, e := range payload.Values {
		if e.IsPrimary && e.IsConfirmed {
			return e.Email
		}
	}
	for _, e := range payload.Values {
		if e.IsConfirmed {
			return e.Email
		}
	}
	if len(payload.Values) > 0 {
		return payload.Values[0].Email
	}
	return ""
}
