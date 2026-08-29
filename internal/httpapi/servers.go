package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/sshdial"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
)

func (a *API) handleListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := a.Store.ListPrivateKeys(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if keys == nil {
		keys = []store.PrivateKey{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"private_keys": keys})
}

func (a *API) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		PrivateKey  string `json:"private_key"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == "" || body.PrivateKey == "" {
		writeError(w, http.StatusBadRequest, "name and private_key required")
		return
	}
	signer, err := ssh.ParsePrivateKey([]byte(body.PrivateKey))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid private key")
		return
	}
	pub := string(ssh.MarshalAuthorizedKey(signer.PublicKey()))
	sum := sha256.Sum256(signer.PublicKey().Marshal())
	fp := "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
	enc, err := a.Store.Box.EncryptString(body.PrivateKey)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	key, err := a.Store.CreatePrivateKey(r.Context(), currentTeamID(r), body.Name, body.Description, pub, enc, fp)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, key)
}

func (a *API) handleListServers(w http.ResponseWriter, r *http.Request) {
	servers, err := a.Store.ListServers(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": servers})
}

func (a *API) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		IP           string `json:"ip"`
		Port         int    `json:"port"`
		UserName     string `json:"user_name"`
		PrivateKeyID string `json:"private_key_id"`
		ProxyType    string `json:"proxy_type"`
		JumpHostID   string `json:"jump_host_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == "" || body.IP == "" {
		writeError(w, http.StatusBadRequest, "name and ip required")
		return
	}
	if body.Port == 0 {
		body.Port = 22
	}
	if body.UserName == "" {
		body.UserName = "root"
	}
	if body.ProxyType == "" {
		body.ProxyType = "traefik"
	}
	switch body.ProxyType {
	case "traefik", "caddy", "none":
	default:
		writeError(w, http.StatusBadRequest, "proxy_type must be traefik, caddy, or none")
		return
	}
	var keyID *uuid.UUID
	if body.PrivateKeyID != "" {
		id, err := uuid.Parse(body.PrivateKeyID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid private_key_id")
			return
		}
		keyID = &id
	}
	teamID := currentTeamID(r)
	srv, err := a.Store.CreateServer(r.Context(), teamID, keyID, body.Name, body.Description, body.IP, body.UserName, body.Port, body.ProxyType)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if strings.TrimSpace(body.JumpHostID) != "" {
		jid, err := uuid.Parse(body.JumpHostID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid jump_host_id")
			return
		}
		if err := a.Store.SetServerJumpHost(r.Context(), teamID, srv.ID, &jid); err != nil {
			if errors.Is(err, store.ErrConflict) {
				writeConflictDetail(w, err)
				return
			}
			mapStoreErr(w, err)
			return
		}
		srv, _ = a.Store.GetServer(r.Context(), teamID, srv.ID)
	}
	writeJSON(w, http.StatusCreated, srv)
}

func (a *API) handleGetServer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	srv, err := a.Store.GetServer(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, srv)
}

func (a *API) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := a.Store.DeleteServer(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) dialServer(r *http.Request, serverID uuid.UUID) (*ssh.Client, error) {
	if a.Queue == nil || a.Queue.SSH == nil {
		return nil, fmt.Errorf("ssh pool unavailable")
	}
	return sshdial.DialClient(r.Context(), a.Store, a.Queue.SSH, currentTeamID(r), serverID)
}

func (a *API) handleValidateServer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	srv, err := a.Store.GetServer(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if srv.JumpHostID == nil {
		if err := sshx.TCPReachable(srv.IP, srv.Port, 5*time.Second); err != nil {
			_ = a.Store.UpdateServerStatus(r.Context(), id, false, false, "", "unknown")
			writeJSON(w, http.StatusOK, map[string]any{"reachable": false, "usable": false, "error": err.Error()})
			return
		}
	}
	client, err := a.dialServer(r, id)
	if err != nil {
		_ = a.Store.UpdateServerStatus(r.Context(), id, true, false, "", "unknown")
		writeJSON(w, http.StatusOK, map[string]any{"reachable": true, "usable": false, "error": err.Error()})
		return
	}
	if err := sshx.EnsureDataDirs(client); err != nil {
		_ = a.Store.UpdateServerStatus(r.Context(), id, true, false, "", "unknown")
		writeJSON(w, http.StatusOK, map[string]any{"reachable": true, "usable": false, "error": err.Error()})
		return
	}
	version, err := sshx.ValidateDocker(client)
	if err != nil {
		_ = a.Store.UpdateServerStatus(r.Context(), id, true, false, "", "unknown")
		writeJSON(w, http.StatusOK, map[string]any{"reachable": true, "usable": false, "error": err.Error()})
		return
	}
	_ = sshx.EnsureNetwork(client, "dockfin")
	_ = proxy.EnsureProxyJoinedResourceNetworks(client)
	proxyStatus := proxy.ProxyStatus(client)

	publicIP := strings.TrimSpace(srv.PublicIP)
	// Only auto-fill when public_ip is missing/unusable. Do not overwrite a good
	// value just because SSH IP is loopback (Validate used to fight manual Public IP).
	needDetect := publicIP == "" || proxy.IsUnusableMagicIP(publicIP)
	if needDetect {
		if detected := detectServerPublicIP(client); detected != "" {
			publicIP = detected
			_ = a.Store.SetServerPublicIP(r.Context(), teamID, id, publicIP)
			srv.PublicIP = publicIP
		} else if publicIP == "" && !proxy.IsUnusableMagicIP(srv.IP) {
			// SSH IP is already usable for magic DNS (non-loopback).
			publicIP = srv.IP
			_ = a.Store.SetServerPublicIP(r.Context(), teamID, id, publicIP)
			srv.PublicIP = publicIP
		}
	}

	_ = a.Store.UpdateServerStatus(r.Context(), id, true, true, version, proxyStatus)
	writeJSON(w, http.StatusOK, map[string]any{
		"reachable": true, "usable": true, "docker_version": version, "proxy_status": proxyStatus,
		"public_ip": publicIP,
		"magic_ip":  proxy.PreferMagicIP(srv.IP, publicIP),
	})
}

func detectServerPublicIP(client *ssh.Client) string {
	out, _, err := sshx.RunArgs(client, "sh", "-c",
		`curl -4 -fsS --max-time 5 https://api.ipify.org 2>/dev/null || curl -4 -fsS --max-time 5 https://ifconfig.me/ip 2>/dev/null || curl -4 -fsS --max-time 5 https://icanhazip.com 2>/dev/null`)
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(out)
	if proxy.IsUnusableMagicIP(ip) {
		return ""
	}
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

func (a *API) handleStartProxy(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	srv, err := a.Store.GetServer(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	client, err := a.dialServer(r, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	network := "dockfin"
	if dests, err := a.Store.ListDestinations(r.Context(), teamID, &id); err == nil && len(dests) > 0 && dests[0].Network != "" {
		network = dests[0].Network
	}
	if err := proxy.StartProxy(client, srv.ProxyType, a.Cfg.TraefikImage, a.Cfg.CaddyImage, network, a.Cfg.AcmeEmail); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "disabled") || strings.Contains(msg, "unsupported") {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		writeError(w, http.StatusInternalServerError, msg)
		return
	}
	_ = proxy.EnsureProxyJoinedResourceNetworks(client)
	_ = a.Store.UpdateServerProxyStatus(r.Context(), id, "running")
	writeJSON(w, http.StatusOK, map[string]string{"status": "running", "proxy_type": srv.ProxyType})
}

func (a *API) handleStopProxy(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	client, err := a.dialServer(r, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := proxy.StopProxy(client); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = a.Store.UpdateServerProxyStatus(r.Context(), id, "exited")
	writeJSON(w, http.StatusOK, map[string]string{"status": "exited"})
}

func (a *API) handleListDestinations(w http.ResponseWriter, r *http.Request) {
	var serverID *uuid.UUID
	if s := r.URL.Query().Get("server_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid server_id")
			return
		}
		serverID = &id
	}
	dests, err := a.Store.ListDestinations(r.Context(), currentTeamID(r), serverID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"destinations": dests})
}

func (a *API) handlePatchServerSettings(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		IsBuildServer   *bool   `json:"is_build_server"`
		IsSwarmManager  *bool   `json:"is_swarm_manager"`
		WildcardDomain  *string `json:"wildcard_domain"`
		MagicDomain     *string `json:"magic_domain"`
		PublicIP        *string `json:"public_ip"`
		JumpHostID      *string `json:"jump_host_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetServer(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	if body.IsBuildServer != nil {
		if err := a.Store.SetServerBuildServer(r.Context(), teamID, id, *body.IsBuildServer); err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	if body.IsSwarmManager != nil {
		if err := a.Store.SetServerSwarmManager(r.Context(), teamID, id, *body.IsSwarmManager); err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	if body.PublicIP != nil {
		ip := strings.TrimSpace(*body.PublicIP)
		if ip != "" && proxy.IsUnusableMagicIP(ip) {
			writeError(w, http.StatusBadRequest, "public_ip cannot be localhost/loopback — use the server's reachable public IP")
			return
		}
		if err := a.Store.SetServerPublicIP(r.Context(), teamID, id, ip); err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	if body.JumpHostID != nil {
		raw := strings.TrimSpace(*body.JumpHostID)
		var jumpID *uuid.UUID
		if raw != "" {
			idj, err := uuid.Parse(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid jump_host_id")
				return
			}
			jumpID = &idj
		}
		if err := a.Store.SetServerJumpHost(r.Context(), teamID, id, jumpID); err != nil {
			if errors.Is(err, store.ErrConflict) {
				writeConflictDetail(w, err)
				return
			}
			mapStoreErr(w, err)
			return
		}
	}
	if body.WildcardDomain != nil || body.MagicDomain != nil {
		srv, err := a.Store.GetServer(r.Context(), teamID, id)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		wildcard := srv.WildcardDomain
		magic := srv.MagicDomain
		if body.WildcardDomain != nil {
			wildcard = *body.WildcardDomain
		}
		if body.MagicDomain != nil {
			magic = *body.MagicDomain
		}
		if err := a.Store.SetServerDomainSettings(r.Context(), teamID, id, wildcard, magic); err != nil {
			mapStoreErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (a *API) handleCreateDestination(w http.ResponseWriter, r *http.Request) {
	serverID, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Network string `json:"network"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if body.Kind == "" {
		body.Kind = "standalone"
	}
	switch body.Kind {
	case "standalone", "swarm":
	default:
		writeError(w, http.StatusBadRequest, "kind must be standalone or swarm")
		return
	}
	dest, err := a.Store.CreateDestination(r.Context(), currentTeamID(r), serverID, body.Name, body.Kind, body.Network)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dest)
}

