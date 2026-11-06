package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"

	"github.com/dockfin/dockfin/internal/cloud"
	"github.com/dockfin/dockfin/internal/git"
	"github.com/dockfin/dockfin/internal/scan"
	"github.com/dockfin/dockfin/internal/store"
)

func (a *API) enforceProjectACL(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isTeamAdmin(r) {
			next.ServeHTTP(w, r)
			return
		}
		scope, err := a.Store.MemberAccess(r.Context(), currentTeamID(r), currentUser(r).ID, "")
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		if scope.Unrestricted {
			next.ServeHTTP(w, r)
			return
		}
		kind, id, ok := projectResource(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		projectID, found, err := a.Store.ProjectIDForResource(r.Context(), currentTeamID(r), kind, id)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		write := r.Method != http.MethodGet && r.Method != http.MethodHead
		if !scope.Allows(projectID, write, r.Method == http.MethodDelete) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func projectResource(path string) (string, uuid.UUID, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// /api/v1/{kind}/{id}
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "v1" {
		return "", uuid.Nil, false
	}
	var kind string
	switch parts[2] {
	case "projects":
		kind = "project"
	case "applications":
		kind = "application"
	case "services":
		kind = "service"
	case "databases":
		kind = "database"
	case "environments":
		kind = "environment"
	case "deployments":
		kind = "deployment"
	default:
		return "", uuid.Nil, false
	}
	id, err := uuid.Parse(parts[3])
	if err != nil {
		return "", uuid.Nil, false
	}
	return kind, id, true
}

func (a *API) scopedProjects(r *http.Request, projects []store.Project) []store.Project {
	if isTeamAdmin(r) {
		return projects
	}
	scope, err := a.Store.MemberAccess(r.Context(), currentTeamID(r), currentUser(r).ID, "")
	if err != nil || scope.Unrestricted {
		return projects
	}
	var out []store.Project
	for _, p := range projects {
		if _, ok := scope.Grants[p.ID]; ok {
			out = append(out, p)
		}
	}
	if out == nil {
		out = []store.Project{}
	}
	return out
}

func (a *API) envAllowed(r *http.Request) (func(uuid.UUID) bool, error) {
	allowAll := func(uuid.UUID) bool { return true }
	if isTeamAdmin(r) {
		return allowAll, nil
	}
	scope, err := a.Store.MemberAccess(r.Context(), currentTeamID(r), currentUser(r).ID, "")
	if err != nil {
		return nil, err
	}
	if scope.Unrestricted {
		return allowAll, nil
	}
	return func(envID uuid.UUID) bool {
		projectID, ok := scope.EnvProject[envID]
		if !ok {
			return false
		}
		_, granted := scope.Grants[projectID]
		return granted
	}, nil
}

func (a *API) handlePutMemberProjects(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user")
		return
	}
	var body struct {
		AccessScope string               `json:"access_scope"`
		Projects    []store.ProjectGrant `json:"projects"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := a.Store.SetProjectAccess(r.Context(), currentTeamID(r), userID, body.AccessScope, body.Projects); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			mapStoreErr(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	grants, err := a.Store.ListProjectGrants(r.Context(), currentTeamID(r), userID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_scope": body.AccessScope, "projects": grants})
}

func (a *API) handleGetMemberProjects(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user")
		return
	}
	grants, err := a.Store.ListProjectGrants(r.Context(), currentTeamID(r), userID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": grants})
}

func (a *API) handleFleetMetrics(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Store.FleetMetrics(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": rows})
}

func (a *API) handleTransferServer(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	serverID, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server")
		return
	}
	var body struct {
		TeamID string `json:"team_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	target, err := uuid.Parse(body.TeamID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "team_id required")
		return
	}
	role, err := a.Store.UserRoleOnTeam(r.Context(), currentUser(r).ID, target)
	if err != nil || (role != "owner" && role != "admin") {
		writeError(w, http.StatusForbidden, "you must be an owner or admin of the target team")
		return
	}
	if err := a.Store.TransferServer(r.Context(), currentTeamID(r), target, serverID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			mapStoreErr(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "transferred"})
}

func (a *API) handleExportServer(w http.ResponseWriter, r *http.Request) {
	serverID, err := uuid.Parse(chi.URLParam(r, "serverID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid server")
		return
	}
	exp, err := a.Store.ExportServer(r.Context(), currentTeamID(r), serverID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exp)
}

func (a *API) handleImportServer(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	var body struct {
		PrivateKeyID string             `json:"private_key_id"`
		Server       store.ServerExport `json:"server"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var keyID *uuid.UUID
	if strings.TrimSpace(body.PrivateKeyID) != "" {
		id, err := uuid.Parse(body.PrivateKeyID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid private_key_id")
			return
		}
		if _, err := a.Store.GetPrivateKey(r.Context(), currentTeamID(r), id); err != nil {
			mapStoreErr(w, err)
			return
		}
		keyID = &id
	}
	srv, err := a.Store.ImportServer(r.Context(), currentTeamID(r), keyID, body.Server)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"server": srv})
}

func (a *API) handleGitScan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repository string `json:"repository"`
		Branch     string `json:"branch"`
		Token      string `json:"token"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Repository) == "" {
		writeError(w, http.StatusBadRequest, "repository required")
		return
	}
	repo, err := scan.ParseRepo(body.Repository)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	paths, err := scan.ListPaths(r.Context(), repo, body.Branch, strings.TrimSpace(body.Token))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	result := scan.Detect(paths)
	writeJSON(w, http.StatusOK, map[string]any{"repository": repo, "scan": result, "files": len(paths)})
}

func (a *API) hostinger(r *http.Request, tokenID uuid.UUID) (cloud.Hostinger, error) {
	tok, err := a.Store.GetCloudProviderToken(r.Context(), currentTeamID(r), tokenID)
	if err != nil {
		return cloud.Hostinger{}, err
	}
	if tok.Provider != "hostinger" {
		return cloud.Hostinger{}, fmt.Errorf("token is not a Hostinger token")
	}
	enc, err := a.Store.GetCloudProviderTokenMaterial(r.Context(), currentTeamID(r), tokenID)
	if err != nil {
		return cloud.Hostinger{}, err
	}
	plain, err := a.Store.Box.DecryptString(enc)
	if err != nil {
		return cloud.Hostinger{}, err
	}
	return cloud.Hostinger{Token: plain}, nil
}

func (a *API) handleHostingerOptions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid token")
		return
	}
	h, err := a.hostinger(r, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	ctx := r.Context()
	vms, err := h.VirtualMachines(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	centers, err := h.DataCenters(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	templates, err := h.Templates(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	catalog, err := h.Catalog(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"virtual_machines": vms,
		"data_centers":     centers,
		"templates":        templates,
		"catalog":          catalog,
		"limits": []string{
			"Purchasing charges the Hostinger account.",
			"An order without a virtual machine id is waiting for payment in hPanel.",
			"HTTP 429 means the Hostinger rate limit was hit.",
			"A hostname is sent only when it contains a dot.",
			"Only KVM VPS plans are listed. Game panel plans are excluded.",
		},
	})
}

func (a *API) handleHostingerLink(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	tokenID, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid token")
		return
	}
	var body struct {
		VirtualMachineID int    `json:"virtual_machine_id"`
		Name             string `json:"name"`
		PrivateKeyID     string `json:"private_key_id"`
		ProxyType        string `json:"proxy_type"`
	}
	if err := decodeJSON(r, &body); err != nil || body.VirtualMachineID == 0 {
		writeError(w, http.StatusBadRequest, "virtual_machine_id required")
		return
	}
	h, err := a.hostinger(r, tokenID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	vms, err := h.VirtualMachines(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var found *cloud.VirtualMachine
	for i := range vms {
		if vms[i].ID == body.VirtualMachineID {
			found = &vms[i]
			break
		}
	}
	if found == nil || found.PublicIP() == "" {
		writeError(w, http.StatusBadRequest, "virtual machine has no public IP yet")
		return
	}
	var keyID *uuid.UUID
	if strings.TrimSpace(body.PrivateKeyID) != "" {
		id, err := uuid.Parse(body.PrivateKeyID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid private_key_id")
			return
		}
		if _, err := a.Store.GetPrivateKey(r.Context(), currentTeamID(r), id); err != nil {
			mapStoreErr(w, err)
			return
		}
		keyID = &id
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = fmt.Sprintf("hostinger-%d", found.ID)
	}
	proxy := body.ProxyType
	if proxy == "" {
		proxy = "traefik"
	}
	srv, err := a.Store.CreateServer(r.Context(), currentTeamID(r), keyID, name, "Linked from Hostinger", found.PublicIP(), "root", 22, proxy)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	_ = a.Store.SetHostingerVM(r.Context(), currentTeamID(r), srv.ID, fmt.Sprint(found.ID))
	writeJSON(w, http.StatusCreated, map[string]any{"server": srv})
}

func (a *API) handleHostingerPurchase(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	tokenID, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid token")
		return
	}
	var body struct {
		Confirm      string `json:"confirm"`
		ItemID       string `json:"item_id"`
		DataCenterID int    `json:"data_center_id"`
		TemplateID   int    `json:"template_id"`
		Hostname     string `json:"hostname"`
		Name         string `json:"name"`
		PrivateKeyID string `json:"private_key_id"`
		ProxyType    string `json:"proxy_type"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Confirm != "PURCHASE" {
		writeError(w, http.StatusBadRequest, "type PURCHASE to confirm this charges the Hostinger account")
		return
	}
	keyID, err := uuid.Parse(body.PrivateKeyID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "private_key_id required")
		return
	}
	key, err := a.Store.GetPrivateKey(r.Context(), currentTeamID(r), keyID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	h, err := a.hostinger(r, tokenID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	vm, err := h.Purchase(r.Context(), cloud.PurchaseOrder{
		ItemID:       body.ItemID,
		DataCenterID: body.DataCenterID,
		TemplateID:   body.TemplateID,
		Hostname:     body.Hostname,
		PublicKey:    key.PublicKey,
		KeyName:      key.Name,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if vm.PublicIP() == "" {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":             "provisioning",
			"virtual_machine_id": vm.ID,
			"message":            "Hostinger created the VPS. Link it once a public IP is assigned.",
		})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = fmt.Sprintf("hostinger-%d", vm.ID)
	}
	proxy := body.ProxyType
	if proxy == "" {
		proxy = "traefik"
	}
	srv, err := a.Store.CreateServer(r.Context(), currentTeamID(r), &keyID, name, "Purchased from Hostinger", vm.PublicIP(), "root", 22, proxy)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	_ = a.Store.SetHostingerVM(r.Context(), currentTeamID(r), srv.ID, fmt.Sprint(vm.ID))
	writeJSON(w, http.StatusCreated, map[string]any{"server": srv, "virtual_machine_id": vm.ID})
}

func (a *API) handleCreateGitLabSystemHook(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	var body struct {
		GitLabURL string `json:"gitlab_url"`
		Token     string `json:"token"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	base := strings.TrimRight(strings.TrimSpace(body.GitLabURL), "/")
	if base == "" {
		base = "https://gitlab.com"
	}
	if !strings.HasPrefix(base, "https://") {
		writeError(w, http.StatusBadRequest, "gitlab_url must use https")
		return
	}
	pat := strings.TrimSpace(body.Token)
	if pat == "" {
		writeError(w, http.StatusBadRequest, "token required")
		return
	}
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create hook secret")
		return
	}
	plainSecret := hex.EncodeToString(secret)
	hookURL := strings.TrimRight(a.Cfg.PublicURL, "/") + "/api/v1/webhooks/gitlab/system"
	payload, _ := json.Marshal(map[string]any{
		"url":                     hookURL,
		"token":                   plainSecret,
		"push_events":             true,
		"merge_requests_events":   true,
		"enable_ssl_verification": strings.HasPrefix(strings.ToLower(a.Cfg.PublicURL), "https://"),
	})
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, base+"/api/v4/hooks", bytes.NewReader(payload))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Header.Set("PRIVATE-TOKEN", pat)
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("gitlab returned HTTP %d", res.StatusCode))
		return
	}
	var created struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(raw, &created)
	var hookID *int64
	if created.ID != 0 {
		hookID = &created.ID
	}
	enc, err := a.Store.Box.EncryptString(pat)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	hook, err := a.Store.CreateGitLabSystemHook(r.Context(), currentTeamID(r), base, store.HashToken(plainSecret), enc, hookID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"hook": hook})
}

func (a *API) handleListGitLabSystemHooks(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListGitLabSystemHooks(r.Context(), currentTeamID(r))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hooks": list})
}

func (a *API) handleDeleteGitLabSystemHook(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "hookID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid hook")
		return
	}
	if hook, enc, err := a.Store.GitLabSystemHook(r.Context(), currentTeamID(r), id); err == nil && hook.HookID != nil && enc != "" {
		if pat, err := a.Store.Box.DecryptString(enc); err == nil {
			req, err := http.NewRequestWithContext(r.Context(), http.MethodDelete, hook.GitLabURL+"/api/v4/hooks/"+fmt.Sprint(*hook.HookID), nil)
			if err == nil {
				req.Header.Set("PRIVATE-TOKEN", pat)
				res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
				if err == nil {
					res.Body.Close()
				}
			}
		}
	}
	if err := a.Store.DeleteGitLabSystemHook(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) handleGitLabSystemHook(w http.ResponseWriter, r *http.Request) {
	secret := r.Header.Get("X-Gitlab-Token")
	if secret == "" {
		secret = r.URL.Query().Get("token")
	}
	if secret == "" {
		writeError(w, http.StatusUnauthorized, "missing hook token")
		return
	}
	hook, enc, err := a.Store.GitLabSystemHookByToken(r.Context(), store.HashToken(secret))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unknown hook")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	body, err = git.NormalizeGitLabSystemHook(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var probe struct {
		Project struct {
			ID                int    `json:"id"`
			PathWithNamespace string `json:"path_with_namespace"`
		} `json:"project"`
	}
	_ = json.Unmarshal(body, &probe)
	if probe.Project.PathWithNamespace == "" && probe.Project.ID != 0 && enc != "" {
		if pat, err := a.Store.Box.DecryptString(enc); err == nil {
			if path := gitlabProjectPath(r.Context(), hook.GitLabURL, pat, probe.Project.ID); path != "" {
				var doc map[string]any
				if json.Unmarshal(body, &doc) == nil {
					proj, _ := doc["project"].(map[string]any)
					if proj == nil {
						proj = map[string]any{"id": probe.Project.ID}
						doc["project"] = proj
					}
					proj["path_with_namespace"] = path
					if next, err := json.Marshal(doc); err == nil {
						body = next
					}
				}
			}
		}
	}
	event, err := git.ParseWebhook("gitlab", r, body)
	if err != nil || event == nil || event.RepoFullName == "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ignored"})
		return
	}
	ids, err := a.Store.ApplicationsForRepo(r.Context(), hook.TeamID, event.RepoFullName)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	var results []webhookActionResult
	for _, id := range ids {
		app, err := a.Store.GetApplication(r.Context(), hook.TeamID, id)
		if err != nil {
			continue
		}
		results = append(results, a.processWebhookEvent(r.Context(), app, event))
	}
	if results == nil {
		results = []webhookActionResult{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func gitlabProjectPath(ctx context.Context, base, token string, id int) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/api/v4/projects/"+fmt.Sprint(id), nil)
	if err != nil {
		return ""
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	res, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ""
	}
	var doc struct {
		PathWithNamespace string `json:"path_with_namespace"`
	}
	_ = json.Unmarshal(raw, &doc)
	return doc.PathWithNamespace
}

type waUser struct {
	id    uuid.UUID
	name  string
	email string
	creds []webauthn.Credential
}

func (u waUser) WebAuthnID() []byte                         { return u.id[:] }
func (u waUser) WebAuthnName() string                       { return u.email }
func (u waUser) WebAuthnDisplayName() string                { return u.name }
func (u waUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func (a *API) webAuthn() (*webauthn.WebAuthn, error) {
	raw := strings.TrimSpace(a.Cfg.PublicURL)
	if raw == "" {
		raw = "http://localhost:8000"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("DOCKFIN_PUBLIC_URL is not a valid origin")
	}
	return webauthn.New(&webauthn.Config{
		RPDisplayName: "Dockfin",
		RPID:          u.Hostname(),
		RPOrigins:     []string{strings.TrimRight(raw, "/")},
	})
}

func (a *API) passkeysOn(ctx context.Context) bool {
	on, err := a.Store.PasskeysEnabled(ctx)
	return err == nil && on
}

func (a *API) handlePasskeysEnabled(w http.ResponseWriter, r *http.Request) {
	on, err := a.Store.PasskeysEnabled(r.Context())
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": on})
}

func (a *API) handlePutPasskeysEnabled(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := a.Store.SetPasskeysEnabled(r.Context(), body.Enabled); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": body.Enabled})
}

func (a *API) handleListPasskeys(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListPasskeys(r.Context(), currentUser(r).ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	type row struct {
		ID         uuid.UUID  `json:"id"`
		Name       string     `json:"name"`
		CreatedAt  time.Time  `json:"created_at"`
		LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	}
	out := make([]row, 0, len(list))
	for _, p := range list {
		out = append(out, row{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt, LastUsedAt: p.LastUsedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"passkeys": out})
}

func (a *API) credentialsFor(ctx context.Context, userID uuid.UUID) ([]webauthn.Credential, error) {
	list, err := a.Store.ListPasskeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	var creds []webauthn.Credential
	for _, p := range list {
		var c webauthn.Credential
		if json.Unmarshal(p.Credential, &c) == nil {
			creds = append(creds, c)
		}
	}
	return creds, nil
}

func (a *API) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	if !a.passkeysOn(r.Context()) {
		writeError(w, http.StatusForbidden, "passkeys are disabled")
		return
	}
	wa, err := a.webAuthn()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user := currentUser(r)
	creds, err := a.credentialsFor(r.Context(), user.ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	creation, session, err := wa.BeginRegistration(waUser{id: user.ID, name: user.Name, email: user.Email, creds: creds})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	raw, _ := json.Marshal(session)
	uid := user.ID
	sid, err := a.Store.SaveWebAuthnSession(r.Context(), &uid, "register", raw, 5*time.Minute)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session_id": sid, "options": creation})
}

func (a *API) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	if !a.passkeysOn(r.Context()) {
		writeError(w, http.StatusForbidden, "passkeys are disabled")
		return
	}
	var body struct {
		SessionID  uuid.UUID       `json:"session_id"`
		Name       string          `json:"name"`
		Credential json.RawMessage `json:"credential"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	raw, userID, err := a.Store.TakeWebAuthnSession(r.Context(), body.SessionID, "register")
	if err != nil || userID == nil || *userID != currentUser(r).ID {
		writeError(w, http.StatusBadRequest, "registration session expired")
		return
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(raw, &session); err != nil {
		writeError(w, http.StatusBadRequest, "registration session expired")
		return
	}
	wa, err := a.webAuthn()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user := currentUser(r)
	creds, _ := a.credentialsFor(r.Context(), user.ID)
	req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewReader(body.Credential))
	req.Header.Set("Content-Type", "application/json")
	cred, err := wa.FinishRegistration(waUser{id: user.ID, name: user.Name, email: user.Email, creds: creds}, session, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	blob, _ := json.Marshal(cred)
	saved, err := a.Store.SavePasskey(r.Context(), user.ID, body.Name, cred.ID, blob)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": saved.ID, "name": saved.Name})
}

func (a *API) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	if !a.passkeysOn(r.Context()) {
		writeError(w, http.StatusForbidden, "passkeys are disabled")
		return
	}
	wa, err := a.webAuthn()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	assert, session, err := wa.BeginDiscoverableLogin()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	raw, _ := json.Marshal(session)
	sid, err := a.Store.SaveWebAuthnSession(r.Context(), nil, "login", raw, 5*time.Minute)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session_id": sid, "options": assert})
}

func (a *API) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	ip := a.rateLimitIP(r)
	if !globalLoginLimiter.allow(ip) {
		writeError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	if !a.passkeysOn(r.Context()) {
		writeError(w, http.StatusForbidden, "passkeys are disabled")
		return
	}
	var body struct {
		SessionID  uuid.UUID       `json:"session_id"`
		Credential json.RawMessage `json:"credential"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	raw, _, err := a.Store.TakeWebAuthnSession(r.Context(), body.SessionID, "login")
	if err != nil {
		writeError(w, http.StatusBadRequest, "login session expired")
		return
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(raw, &session); err != nil {
		writeError(w, http.StatusBadRequest, "login session expired")
		return
	}
	wa, err := a.webAuthn()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewReader(body.Credential))
	req.Header.Set("Content-Type", "application/json")
	user, cred, err := wa.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
		if len(userHandle) != 16 {
			return nil, fmt.Errorf("unknown passkey")
		}
		id, err := uuid.FromBytes(userHandle)
		if err != nil {
			return nil, err
		}
		account, err := a.Store.GetUserByID(r.Context(), id)
		if err != nil {
			return nil, err
		}
		creds, err := a.credentialsFor(r.Context(), id)
		if err != nil {
			return nil, err
		}
		return waUser{id: account.ID, name: account.Name, email: account.Email, creds: creds}, nil
	}, session, req)
	if err != nil {
		globalLoginLimiter.fail(ip)
		writeError(w, http.StatusUnauthorized, "passkey was rejected")
		return
	}
	account := user.(waUser)
	if cred != nil {
		if row, err := a.Store.PasskeyByCredentialID(r.Context(), cred.ID); err == nil {
			blob, _ := json.Marshal(cred)
			_ = a.Store.TouchPasskey(r.Context(), row.ID, blob)
		}
	}
	globalLoginLimiter.success(ip)
	full, err := a.Store.GetUserByID(r.Context(), account.id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	hasTOTP, err := a.Store.UserHasTOTP(r.Context(), full.ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if hasTOTP {
		challenge, err := a.Store.CreateAuthChallenge(r.Context(), full.ID, "totp", 5*time.Minute)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "2fa_required", "challenge_id": challenge.ID})
		return
	}
	token, teams, err := a.issueSession(w, r, full)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	a.recordAuth(r, full, full.Email, "login", "Signed in with a passkey", http.StatusOK)
	writeJSON(w, http.StatusOK, map[string]any{"user": full, "team": teams[0], "teams": teams, "token": token})
}

func (a *API) handleRenamePasskey(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "passkeyID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid passkey")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if err := a.Store.RenamePasskey(r.Context(), currentUser(r).ID, id, body.Name); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleDeletePasskey(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "passkeyID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid passkey")
		return
	}
	if err := a.Store.DeletePasskey(r.Context(), currentUser(r).ID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
