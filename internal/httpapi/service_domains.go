package httpapi

import (
	"net/http"
	"strings"

	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/services"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type serviceDomainBody struct {
	UnitName     string `json:"unit_name"`
	Host         string `json:"host"`
	Scheme       string `json:"scheme"`
	InternalPort *int   `json:"internal_port"`
	ForceHTTPS   *bool  `json:"force_https"`
	Redirect     string `json:"redirect"`
	NoIndex      *bool  `json:"noindex"`
}

func defaultServiceUnit(prepared, raw string) string {
	src := prepared
	if strings.TrimSpace(src) == "" {
		src = raw
	}
	units := services.ParseComposeUnits(src)
	var apps []string
	for _, u := range units {
		if services.IsDatabaseImage(u.Image) {
			continue
		}
		apps = append(apps, u.Name)
	}
	if len(apps) == 1 {
		return apps[0]
	}
	if len(units) == 1 {
		return units[0].Name
	}
	return ""
}

func normalizeDomainHost(raw string) (string, bool) {
	h := strings.TrimSpace(raw)
	h = strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://")
	h = strings.TrimSuffix(h, "/")
	if i := strings.Index(h, "/"); i >= 0 {
		h = h[:i]
	}
	if i := strings.Index(h, ":"); i >= 0 {
		h = h[:i]
	}
	h = strings.ToLower(strings.TrimSpace(h))
	if h == "" || !isValidHostnameList(h) {
		return "", false
	}
	return h, true
}

func domainInputFromBody(body serviceDomainBody, fallback store.ServiceDomain) (store.ServiceDomainInput, string) {
	host, ok := normalizeDomainHost(body.Host)
	if body.Host == "" {
		host, ok = normalizeDomainHost(fallback.Host)
	}
	if !ok {
		return store.ServiceDomainInput{}, "invalid host"
	}
	scheme := strings.ToLower(strings.TrimSpace(body.Scheme))
	if scheme == "" {
		scheme = fallback.Scheme
	}
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	if proxy.IsMagicDomainHost(host) {
		scheme = "http"
	} else {
		// Custom hosts always get HTTPS at deploy (Let's Encrypt). Storing http
		// here would disagree with the hostname written onto the service.
		scheme = "https"
	}
	redirect := strings.ToLower(strings.TrimSpace(body.Redirect))
	if redirect == "" {
		redirect = fallback.Redirect
	}
	if redirect != "www" && redirect != "non-www" {
		redirect = "both"
	}
	if proxy.IsMagicDomainHost(host) {
		redirect = "both"
	}
	force := fallback.ForceHTTPS
	if body.ForceHTTPS != nil {
		force = *body.ForceHTTPS
	}
	if proxy.IsMagicDomainHost(host) {
		force = false
	}
	noindex := fallback.NoIndex
	if body.NoIndex != nil {
		noindex = *body.NoIndex
	}
	var port *int
	if body.InternalPort != nil {
		if *body.InternalPort == 0 {
			port = nil
		} else if *body.InternalPort < 1 || *body.InternalPort > 65535 {
			return store.ServiceDomainInput{}, "internal_port must be between 1 and 65535"
		} else {
			p := *body.InternalPort
			port = &p
		}
	} else {
		port = fallback.InternalPort
	}
	unit := strings.TrimSpace(body.UnitName)
	if unit == "" {
		unit = fallback.UnitName
	}
	return store.ServiceDomainInput{
		UnitName:     unit,
		Host:         host,
		Scheme:       scheme,
		InternalPort: port,
		ForceHTTPS:   force,
		Redirect:     redirect,
		NoIndex:      noindex,
	}, ""
}

func (a *API) syncDomainsOntoService(r *http.Request, teamID, serviceID uuid.UUID) {
	list, err := a.Store.ListServiceDomains(r.Context(), teamID, serviceID)
	if err != nil {
		return
	}
	published := services.PublishedServiceDomains(list)
	hosts := make([]string, 0, len(published))
	force := len(list) == 0
	for _, d := range published {
		hosts = append(hosts, d.Host)
		if !proxy.IsMagicDomainHost(d.Host) && d.ForceHTTPS {
			force = true
		}
	}
	fqdn := proxy.NormalizeDomains(strings.Join(hosts, ","))
	_ = a.Store.UpdateServiceFQDN(r.Context(), serviceID, fqdn)
	_ = a.Store.UpdateServiceCompose(r.Context(), serviceID, "")
	_ = a.Store.SetServiceForceHTTPS(r.Context(), teamID, serviceID, force)
	a.rewriteServiceDomainEnv(r.Context(), teamID, serviceID, fqdn)
}

func (a *API) handleListServiceDomains(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	svc, err := a.Store.GetService(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	list, err := a.Store.ListServiceDomains(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	unit := defaultServiceUnit(svc.DockerCompose, svc.DockerComposeRaw)
	if len(list) == 0 && strings.TrimSpace(svc.FQDN) != "" {
		for _, part := range proxy.SplitDomainEntries(svc.FQDN) {
			host := proxy.HostFromDomainEntry(part)
			if host == "" {
				continue
			}
			scheme := "https"
			force := svc.IsForceHTTPS
			if proxy.IsMagicDomainHost(host) || strings.HasPrefix(strings.ToLower(part), "http://") {
				scheme = "http"
				if proxy.IsMagicDomainHost(host) {
					force = false
				}
			}
			created, err := a.Store.CreateServiceDomain(r.Context(), teamID, id, store.ServiceDomainInput{
				UnitName:   unit,
				Host:       host,
				Scheme:     scheme,
				ForceHTTPS: force,
				Redirect:   "both",
			})
			if err == nil && created != nil {
				list = append(list, *created)
			}
		}
	}
	if unit != "" {
		for i := range list {
			if strings.TrimSpace(list[i].UnitName) != "" {
				continue
			}
			updated, err := a.Store.UpdateServiceDomain(r.Context(), teamID, id, list[i].ID, store.ServiceDomainInput{
				UnitName:     unit,
				Host:         list[i].Host,
				Scheme:       list[i].Scheme,
				InternalPort: list[i].InternalPort,
				ForceHTTPS:   list[i].ForceHTTPS,
				Redirect:     list[i].Redirect,
				NoIndex:      list[i].NoIndex,
			})
			if err == nil && updated != nil {
				list[i] = *updated
			}
		}
	}
	if list == nil {
		list = []store.ServiceDomain{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"domains": list})
}

func (a *API) handleCreateServiceDomain(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body serviceDomainBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	teamID := currentTeamID(r)
	if _, err := a.Store.GetService(r.Context(), teamID, id); err != nil {
		mapStoreErr(w, err)
		return
	}
	fallback := store.ServiceDomain{Scheme: "https", ForceHTTPS: true, Redirect: "both"}
	in, msg := domainInputFromBody(body, fallback)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	created, err := a.Store.CreateServiceDomain(r.Context(), teamID, id, in)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	a.syncDomainsOntoService(r, teamID, id)
	writeJSON(w, http.StatusCreated, created)
}

func (a *API) handlePatchServiceDomain(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	domainID, err := uuid.Parse(chi.URLParam(r, "domainID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}
	var body serviceDomainBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	teamID := currentTeamID(r)
	current, err := a.Store.GetServiceDomain(r.Context(), teamID, id, domainID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if body.Host == "" {
		body.Host = current.Host
	}
	if body.Scheme == "" {
		body.Scheme = current.Scheme
	}
	if body.Redirect == "" {
		body.Redirect = current.Redirect
	}
	if body.UnitName == "" {
		body.UnitName = current.UnitName
	}
	in, msg := domainInputFromBody(body, *current)
	if msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	updated, err := a.Store.UpdateServiceDomain(r.Context(), teamID, id, domainID, in)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	a.syncDomainsOntoService(r, teamID, id)
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) handleDeleteServiceDomain(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	domainID, err := uuid.Parse(chi.URLParam(r, "domainID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain id")
		return
	}
	teamID := currentTeamID(r)
	if err := a.Store.DeleteServiceDomain(r.Context(), teamID, id, domainID); err != nil {
		mapStoreErr(w, err)
		return
	}
	a.syncDomainsOntoService(r, teamID, id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
