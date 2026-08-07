package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/bootstrap"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/sshx"
	"github.com/dockfin/dockfin/internal/store"
	"golang.org/x/crypto/ssh"
)

// BootstrapSelfResult is returned when this VPS is registered as a deploy server.
type BootstrapSelfResult struct {
	Server       *store.Server `json:"server"`
	PrivateKeyID string        `json:"private_key_id"`
	PublicIP     string        `json:"public_ip"`
	Validated    bool          `json:"validated"`
	ProxyStarted bool          `json:"proxy_started"`
	Message      string        `json:"message,omitempty"`
}

// bootstrapSelfServer adds this VPS as a normal server using its public IP
// (SSH ip + public_ip), same shape as a manually added remote host.
func (a *API) bootstrapSelfServer(ctx context.Context, teamID uuid.UUID, startProxy bool) (*BootstrapSelfResult, error) {
	publicIP := a.resolvedPublicIP()
	if publicIP == "" {
		return nil, fmt.Errorf("could not detect public IP — set DOCKFIN_PUBLIC_IP")
	}

	servers, err := a.Store.ListServers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	for i := range servers {
		s := &servers[i]
		if !isSelfServer(s, publicIP) {
			continue
		}
		// Already present — ensure public_ip + finish validate/proxy if needed.
		if strings.TrimSpace(s.PublicIP) == "" || proxy.IsUnusableMagicIP(s.PublicIP) {
			_ = a.Store.SetServerPublicIP(ctx, teamID, s.ID, publicIP)
			s.PublicIP = publicIP
		}
		result := &BootstrapSelfResult{
			Server:       s,
			PublicIP:     s.PublicIP,
			Message:      "already registered",
		}
		if s.PrivateKeyID != nil {
			result.PrivateKeyID = s.PrivateKeyID.String()
		}
		needValidate := !s.IsUsable || (startProxy && s.ProxyStatus != "running")
		if needValidate {
			validated, proxyOK, valErr := a.validateAndMaybeProxy(ctx, teamID, s.ID, startProxy)
			result.Validated = validated
			result.ProxyStarted = proxyOK
			if valErr != nil {
				result.Message = "already registered: " + valErr.Error()
				a.warnBootstrap("bootstrap self re-validate", valErr, publicIP)
			} else if fresh, err := a.Store.GetServer(ctx, teamID, s.ID); err == nil {
				result.Server = fresh
				result.Message = "already registered"
			}
		} else {
			result.Validated = s.IsUsable
			result.ProxyStarted = s.ProxyStatus == "running"
		}
		return result, nil
	}

	privPEM, pubAuth, err := bootstrap.EnsureKeyPair(a.Cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("ssh key: %w", err)
	}
	sshUser := a.Cfg.BootstrapSSHUser
	if sshUser == "" {
		sshUser = bootstrap.DefaultSSHUser
	}
	sshPort := a.Cfg.BootstrapSSHPort
	if sshPort <= 0 {
		sshPort = bootstrap.DefaultSSHPort
	}

	signer, err := ssh.ParsePrivateKey([]byte(privPEM))
	if err != nil {
		return nil, fmt.Errorf("parse bootstrap key: %w", err)
	}
	sum := sha256.Sum256(signer.PublicKey().Marshal())
	fp := "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])

	key, err := a.ensureBootstrapPrivateKey(ctx, teamID, privPEM, pubAuth, fp)
	if err != nil {
		return nil, err
	}
	// Authorize the key we will actually SSH with (DB row may reuse an older localhost key).
	authPub := strings.TrimSpace(key.PublicKey)
	if authPub == "" {
		authPub = pubAuth
	}
	if err := bootstrap.AuthorizePublicKey(sshUser, authPub); err != nil {
		return nil, fmt.Errorf("authorize ssh key for %s: %w", sshUser, err)
	}

	// Register like a real remote server: SSH target = public IP, public_ip = same.
	srv, err := a.Store.CreateServer(ctx, teamID, &key.ID, bootstrap.DefaultServerName,
		"This VPS (install host)", publicIP, sshUser, sshPort, "traefik")
	if err != nil {
		return nil, err
	}
	// CreateServer copies non-loopback ip → public_ip; set explicitly for clarity.
	if err := a.Store.SetServerPublicIP(ctx, teamID, srv.ID, publicIP); err != nil {
		return nil, err
	}
	srv.PublicIP = publicIP

	result := &BootstrapSelfResult{
		Server:       srv,
		PrivateKeyID: key.ID.String(),
		PublicIP:     publicIP,
	}

	validated, proxyOK, valErr := a.validateAndMaybeProxy(ctx, teamID, srv.ID, startProxy)
	result.Validated = validated
	result.ProxyStarted = proxyOK
	if valErr != nil {
		// Server row still created — caller can re-validate. Do not fail the whole bootstrap.
		result.Message = valErr.Error()
		a.warnBootstrap("bootstrap self validate", valErr, publicIP)
	} else {
		result.Message = "registered"
		if fresh, err := a.Store.GetServer(ctx, teamID, srv.ID); err == nil {
			result.Server = fresh
		}
	}
	return result, nil
}

func isSelfServer(s *store.Server, publicIP string) bool {
	if s.Name == bootstrap.DefaultServerName {
		return true
	}
	if publicIP != "" && s.IP == publicIP {
		return true
	}
	if publicIP != "" && s.PublicIP == publicIP {
		return true
	}
	return false
}

func (a *API) ensureBootstrapPrivateKey(ctx context.Context, teamID uuid.UUID, privPEM, pubAuth, fp string) (*store.PrivateKey, error) {
	keys, err := a.Store.ListPrivateKeys(ctx, teamID)
	if err != nil {
		return nil, err
	}
	for i := range keys {
		k := &keys[i]
		if k.Name == "localhost" || (fp != "" && k.Fingerprint == fp) {
			return k, nil
		}
	}
	enc, err := a.Store.Box.EncryptString(privPEM)
	if err != nil {
		return nil, err
	}
	return a.Store.CreatePrivateKey(ctx, teamID, "localhost", "Auto-generated for this VPS", pubAuth, enc, fp)
}

func (a *API) warnBootstrap(msg string, err error, publicIP string) {
	if a.Logger == nil || err == nil {
		return
	}
	a.Logger.Warn(msg, slog.String("error", err.Error()), slog.String("public_ip", publicIP))
}

func (a *API) resolvedPublicIP() string {
	return bootstrap.ResolvePublicIP(a.Cfg.PublicIP)
}

func (a *API) validateAndMaybeProxy(ctx context.Context, teamID, serverID uuid.UUID, startProxy bool) (validated, proxyStarted bool, err error) {
	if a.Queue == nil || a.Queue.SSH == nil {
		return false, false, fmt.Errorf("ssh pool not configured")
	}
	srv, err := a.Store.GetServer(ctx, teamID, serverID)
	if err != nil {
		return false, false, err
	}
	if err := sshx.TCPReachable(srv.IP, srv.Port, 8*time.Second); err != nil {
		_ = a.Store.UpdateServerStatus(ctx, serverID, false, false, "", "unknown")
		return false, false, fmt.Errorf("ssh not reachable at %s:%d: %w", srv.IP, srv.Port, err)
	}
	if srv.PrivateKeyID == nil {
		return false, false, fmt.Errorf("server has no private key")
	}
	enc, err := a.Store.GetPrivateKeyMaterial(ctx, teamID, *srv.PrivateKeyID)
	if err != nil {
		return false, false, err
	}
	priv, err := a.Store.Box.DecryptString(enc)
	if err != nil {
		return false, false, err
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
		_ = a.Store.UpdateServerStatus(ctx, serverID, true, false, "", "unknown")
		return false, false, fmt.Errorf("ssh dial: %w", err)
	}
	client := res.Client
	if res.IsNewHost {
		_ = a.Store.UpdateServerHostKey(ctx, serverID, res.Fingerprint, res.KeyType)
	}
	if err := sshx.EnsureDataDirs(client); err != nil {
		_ = a.Store.UpdateServerStatus(ctx, serverID, true, false, "", "unknown")
		return false, false, err
	}
	version, err := sshx.ValidateDocker(client)
	if err != nil {
		_ = a.Store.UpdateServerStatus(ctx, serverID, true, false, "", "unknown")
		return false, false, err
	}
	_ = sshx.EnsureNetwork(client, "dockfin")
	proxyStatus := proxy.ProxyStatus(client)
	_ = a.Store.UpdateServerStatus(ctx, serverID, true, true, version, proxyStatus)
	validated = true

	if !startProxy {
		return validated, false, nil
	}
	if proxyStatus == "running" {
		_ = a.Store.UpdateServerProxyStatus(ctx, serverID, "running")
		proxyStarted = true
	} else {
		network := "dockfin"
		if dests, err := a.Store.ListDestinations(ctx, teamID, &serverID); err == nil && len(dests) > 0 && dests[0].Network != "" {
			network = dests[0].Network
		}
		if err := proxy.StartProxy(client, srv.ProxyType, a.Cfg.TraefikImage, a.Cfg.CaddyImage, network, a.Cfg.AcmeEmail); err != nil {
			return validated, false, fmt.Errorf("start proxy: %w", err)
		}
		_ = a.Store.UpdateServerProxyStatus(ctx, serverID, "running")
		proxyStarted = true
	}
	// Publish Settings → Domain on Traefik when proxy is up (Coolify-style panel FQDN).
	if st, err := a.Store.GetInstanceSettings(ctx); err == nil && strings.TrimSpace(st.PublicURL) != "" {
		if syncErr := proxy.SyncPanelRoute(client, st.PublicURL, "http://dockfin:8000"); syncErr != nil {
			a.warnBootstrap("panel route sync", syncErr, srv.PublicIP)
		}
	}
	return validated, proxyStarted, nil
}

func (a *API) handleBootstrapSelf(w http.ResponseWriter, r *http.Request) {
	if !a.Cfg.BootstrapSelf {
		writeError(w, http.StatusForbidden, "bootstrap disabled (DOCKFIN_BOOTSTRAP_SELF=0)")
		return
	}
	var body struct {
		StartProxy *bool `json:"start_proxy"`
	}
	_ = decodeJSON(r, &body)
	startProxy := true
	if body.StartProxy != nil {
		startProxy = *body.StartProxy
	}
	result, err := a.bootstrapSelfServer(r.Context(), currentTeamID(r), startProxy)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	status := http.StatusCreated
	if strings.HasPrefix(result.Message, "already") {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}
