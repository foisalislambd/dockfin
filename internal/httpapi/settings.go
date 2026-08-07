package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/bootstrap"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
)

func writeConflictDetail(w http.ResponseWriter, err error) {
	msg := err.Error()
	msg = strings.TrimPrefix(msg, "conflict: ")
	if msg == "" || msg == "conflict" {
		msg = "invalid settings"
	}
	writeError(w, http.StatusBadRequest, msg)
}

func (a *API) handleGetInstanceSettings(w http.ResponseWriter, r *http.Request) {
	st, err := a.Store.GetInstanceSettings(r.Context())
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if st.PublicURL == "" && a.Cfg != nil && a.Cfg.PublicURL != "" {
		st.PublicURL = a.Cfg.PublicURL
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": st})
}

func (a *API) handlePatchInstanceSettings(w http.ResponseWriter, r *http.Request) {
	role := r.Context().Value(ctxRole).(string)
	if role != "owner" && role != "admin" {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	var patch store.InstanceSettingsPatch
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	prev, _ := a.Store.GetInstanceSettings(r.Context())
	st, err := a.Store.UpdateInstanceSettings(r.Context(), patch)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeConflictDetail(w, err)
			return
		}
		mapStoreErr(w, err)
		return
	}
	// When instance Domain (public_url) changes, route it through Traefik → panel.
	if patch.PublicURL != nil && (prev == nil || prev.PublicURL != st.PublicURL) {
		a.rememberCORSOrigin(st.PublicURL)
		if syncErr := a.syncPanelRoute(r.Context(), currentTeamID(r), st.PublicURL); syncErr != nil {
			if a.Logger != nil {
				a.Logger.Warn("panel route sync", slog.String("error", syncErr.Error()), slog.String("public_url", st.PublicURL))
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"settings":            st,
				"panel_route_warning": syncErr.Error(),
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": st})
}

func (a *API) rememberCORSOrigin(publicURL string) {
	u := strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if u == "" || a.Cfg == nil {
		return
	}
	for _, o := range a.Cfg.CORSOrigins {
		if strings.EqualFold(strings.TrimRight(strings.TrimSpace(o), "/"), u) {
			return
		}
	}
	a.Cfg.CORSOrigins = append(a.Cfg.CORSOrigins, u)
}

func (a *API) syncPanelRoute(ctx context.Context, teamID uuid.UUID, publicURL string) error {
	client, err := a.dialSelfSSH(ctx, teamID)
	if err != nil {
		return err
	}
	return proxy.SyncPanelRoute(client, publicURL, "http://dockfin:8000")
}

func (a *API) dialSelfSSH(ctx context.Context, teamID uuid.UUID) (*ssh.Client, error) {
	if a.Queue == nil || a.Queue.SSH == nil {
		return nil, fmt.Errorf("ssh pool not configured")
	}
	srv, err := a.findSelfServer(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("no local server — register/bootstrap first: %w", err)
	}
	if srv.PrivateKeyID == nil {
		return nil, fmt.Errorf("server has no private key")
	}
	enc, err := a.Store.GetPrivateKeyMaterial(ctx, teamID, *srv.PrivateKeyID)
	if err != nil {
		return nil, err
	}
	priv, err := a.Store.Box.DecryptString(enc)
	if err != nil {
		return nil, err
	}
	res, err := a.Queue.SSH.Dial(sshx.Target{
		Host:                srv.IP,
		Port:                srv.Port,
		User:                srv.UserName,
		PrivateKey:          []byte(priv),
		ExpectedFingerprint: srv.HostKeyFingerprint,
		ExpectedKeyType:     srv.HostKeyType,
	})
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}
	if res.IsNewHost {
		_ = a.Store.UpdateServerHostKey(ctx, srv.ID, res.Fingerprint, res.KeyType)
	}
	return res.Client, nil
}

func (a *API) findSelfServer(ctx context.Context, teamID uuid.UUID) (*store.Server, error) {
	servers, err := a.Store.ListServers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	pub := a.resolvedPublicIP()
	for i := range servers {
		s := &servers[i]
		if s.Name == bootstrap.DefaultServerName {
			return s, nil
		}
		if pub != "" && (s.IP == pub || s.PublicIP == pub) {
			return s, nil
		}
	}
	if len(servers) == 1 {
		return &servers[0], nil
	}
	return nil, store.ErrNotFound
}

func (a *API) handleListOauthSettings(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListOauthSettings(r.Context())
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.OauthSetting{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"oauth_settings": list})
}

func (a *API) handlePatchOauthSetting(w http.ResponseWriter, r *http.Request) {
	role := r.Context().Value(ctxRole).(string)
	if role != "owner" && role != "admin" {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	provider := chi.URLParam(r, "provider")
	var patch store.OauthSettingPatch
	if err := decodeJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	row, err := a.Store.UpdateOauthSetting(r.Context(), provider, patch)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeConflictDetail(w, err)
			return
		}
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"oauth_setting": row})
}
