package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/store"
)

// resolveServerForDomain returns the server used for magic/wildcard FQDN generation.
func (a *API) resolveServerForDomain(ctx context.Context, teamID uuid.UUID, serverID, destinationID *uuid.UUID) (*store.Server, error) {
	switch {
	case serverID != nil:
		return a.Store.GetServer(ctx, teamID, *serverID)
	case destinationID != nil:
		dest, err := a.Store.GetDestination(ctx, teamID, *destinationID)
		if err != nil {
			return nil, err
		}
		return a.Store.GetServer(ctx, teamID, dest.ServerID)
	default:
		return nil, store.ErrNotFound
	}
}

// generateResourceFQDN builds a free sslip.io/nip.io (or wildcard) hostname.
func generateResourceFQDN(name string, id uuid.UUID, srv *store.Server) string {
	if srv == nil {
		return ""
	}
	magicIP := proxy.PreferMagicIP(srv.IP, srv.PublicIP)
	return proxy.GenerateFQDN(name, id, magicIP, srv.WildcardDomain, srv.MagicDomain)
}

func (a *API) handleGenerateDomain(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          string `json:"name"`
		ServerID      string `json:"server_id"`
		DestinationID string `json:"destination_id"`
		ResourceID    string `json:"resource_id"` // optional; random UUID used if empty
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	teamID := currentTeamID(r)
	var serverID, destID *uuid.UUID
	if body.ServerID != "" {
		id, err := uuid.Parse(body.ServerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid server_id")
			return
		}
		serverID = &id
	}
	if body.DestinationID != "" {
		id, err := uuid.Parse(body.DestinationID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid destination_id")
			return
		}
		destID = &id
	}
	srv, err := a.resolveServerForDomain(r.Context(), teamID, serverID, destID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	rid := uuid.New()
	if body.ResourceID != "" {
		if id, err := uuid.Parse(body.ResourceID); err == nil {
			rid = id
		}
	}
	fqdn := generateResourceFQDN(body.Name, rid, srv)
	if fqdn == "" {
		writeError(w, http.StatusBadRequest, "cannot generate domain: server has no usable IP")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"fqdn": fqdn,
		"url":  proxy.PublicURL(fqdn),
	})
}

func (a *API) handleCheckDomainDNS(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domains       string `json:"domains"`
		ServerID      string `json:"server_id"`
		DestinationID string `json:"destination_id"`
		ExpectedIP    string `json:"expected_ip"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	domains := strings.TrimSpace(body.Domains)
	if domains == "" {
		writeError(w, http.StatusBadRequest, "domains required")
		return
	}

	teamID := currentTeamID(r)
	expectIP := strings.TrimSpace(body.ExpectedIP)
	if expectIP == "" {
		var serverID, destID *uuid.UUID
		if body.ServerID != "" {
			id, err := uuid.Parse(body.ServerID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid server_id")
				return
			}
			serverID = &id
		}
		if body.DestinationID != "" {
			id, err := uuid.Parse(body.DestinationID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid destination_id")
				return
			}
			destID = &id
		}
		if srv, err := a.resolveServerForDomain(r.Context(), teamID, serverID, destID); err == nil {
			// A-record checks need a real IPv4 — not sslip dashed IPv6 from PreferMagicIP.
			expectIP = proxy.PreferPublicIPv4(srv.IP, srv.PublicIP)
		}
	}

	resolvers := []string{"1.1.1.1"}
	validationOn := true
	if st, err := a.Store.GetInstanceSettings(r.Context()); err == nil {
		validationOn = st.IsDNSValidationEnabled
		resolvers = proxy.ParseDNSResolvers(st.CustomDNSServers)
		if expectIP == "" {
			expectIP = proxy.PreferPublicIPv4("", st.PublicIPv4)
		}
	}

	hosts := proxy.HostsFromDomainList(domains)
	if len(hosts) == 0 {
		if h := proxy.HostFromDomainEntry(domains); h != "" {
			hosts = []string{h}
		}
	}

	results := make([]proxy.DNSCheckResult, 0, len(hosts))
	allOK := true
	for _, h := range hosts {
		if !validationOn {
			results = append(results, proxy.DNSCheckResult{
				Host:           h,
				ExpectedIP:     expectIP,
				Matched:        true,
				SkipValidation: true,
				Resolvers:      resolvers,
			})
			continue
		}
		rlt := proxy.CheckDomainDNS(r.Context(), h, expectIP, resolvers)
		if !rlt.Matched && !rlt.SkipValidation {
			allOK = false
		}
		results = append(results, rlt)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                 allOK,
		"validation_enabled": validationOn,
		"expected_ip":        expectIP,
		"resolvers":          resolvers,
		"results":            results,
		"instructions":       dnsInstructions(hosts, expectIP),
	})
}

func dnsInstructions(hosts []string, serverIP string) []map[string]string {
	var out []map[string]string
	seenZone := map[string]bool{}
	for _, host := range hosts {
		if proxy.IsMagicDomainHost(host) {
			continue
		}
		out = append(out, map[string]string{
			"type":  "A",
			"name":  proxy.DNSRecordName(host),
			"value": serverIP,
			"host":  host,
		})
		// Coolify-style wildcard so future subdomains need no extra DNS.
		parts := strings.Split(host, ".")
		var labels []string
		for _, p := range parts {
			if p != "" {
				labels = append(labels, p)
			}
		}
		if len(labels) >= 2 {
			zone := strings.Join(labels[len(labels)-2:], ".")
			if !seenZone[zone] {
				seenZone[zone] = true
				out = append(out, map[string]string{
					"type":  "A",
					"name":  "*",
					"value": serverIP,
					"host":  "*." + zone,
				})
			}
		}
	}
	return out
}
