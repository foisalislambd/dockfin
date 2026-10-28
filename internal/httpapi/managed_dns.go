package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dockfin/dockfin/internal/dns"
	"github.com/dockfin/dockfin/internal/proxy"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type dnsLedger struct{ st *store.Store }

func (l dnsLedger) ListDNSZones(ctx context.Context, teamID uuid.UUID) ([]dns.Zone, error) {
	rows, err := l.st.ListDNSZones(ctx, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]dns.Zone, 0, len(rows))
	for _, z := range rows {
		out = append(out, dns.Zone{ID: z.ID, TokenID: z.TokenID, ProviderZoneID: z.ProviderZoneID, Name: z.Name})
	}
	return out, nil
}

func (l dnsLedger) DNSRecordByName(ctx context.Context, teamID uuid.UUID, name string) (*dns.Record, error) {
	row, err := l.st.DNSRecordByName(ctx, teamID, name)
	if err != nil || row == nil {
		return nil, err
	}
	rec := dnsRecord(*row)
	return &rec, nil
}

func (l dnsLedger) SaveDNSRecord(ctx context.Context, rec dns.Record) (dns.Record, error) {
	saved, err := l.st.SaveDNSRecord(ctx, store.DNSRecord{
		ID: rec.ID, TeamID: rec.TeamID, ZoneID: rec.ZoneID, TokenID: rec.TokenID,
		ProviderZoneID: rec.ProviderZoneID, ProviderRecordID: rec.ProviderRecordID,
		Type: rec.Type, Name: rec.Name, Content: rec.Content, Owned: rec.Owned, Comment: rec.Comment,
	})
	if err != nil {
		return dns.Record{}, err
	}
	return dnsRecord(saved), nil
}

func (l dnsLedger) AddDNSRef(ctx context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) error {
	return l.st.AddDNSRef(ctx, recordID, resourceType, resourceID)
}

func (l dnsLedger) RemoveDNSRef(ctx context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) (int, error) {
	return l.st.RemoveDNSRef(ctx, recordID, resourceType, resourceID)
}

func (l dnsLedger) OtherDNSRefs(ctx context.Context, recordID uuid.UUID, resourceType string, resourceID uuid.UUID) (int, error) {
	return l.st.OtherDNSRefs(ctx, recordID, resourceType, resourceID)
}

func (l dnsLedger) DeleteDNSRecord(ctx context.Context, id uuid.UUID) error {
	return l.st.DeleteDNSRecord(ctx, id)
}

func (l dnsLedger) DNSHostsForResource(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]string, error) {
	return l.st.DNSHostsForResource(ctx, teamID, resourceType, resourceID)
}

func dnsRecord(row store.DNSRecord) dns.Record {
	return dns.Record{
		ID: row.ID, TeamID: row.TeamID, ZoneID: row.ZoneID, TokenID: row.TokenID,
		ProviderZoneID: row.ProviderZoneID, ProviderRecordID: row.ProviderRecordID,
		Type: row.Type, Name: row.Name, Content: row.Content, Owned: row.Owned, Comment: row.Comment,
	}
}

func dnsHostList(fqdn string) []string {
	var out []string
	seen := map[string]bool{}
	for _, entry := range proxy.SplitDomainEntries(fqdn) {
		host := strings.ToLower(proxy.HostFromDomainEntry(entry))
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	return out
}

func (a *API) serverDNSTarget(ctx context.Context, teamID, serverID uuid.UUID) (string, string) {
	srv, err := a.Store.GetServer(ctx, teamID, serverID)
	if err != nil || srv == nil {
		return "", ""
	}
	if content, recType, ok := dns.PublicRecord(srv.PublicIP); ok {
		return content, recType
	}
	if content, recType, ok := dns.PublicRecord(srv.IP); ok {
		return content, recType
	}
	return "", ""
}

func (a *API) syncManagedDNS(ctx context.Context, teamID uuid.UUID, resourceType string, resourceID uuid.UUID, hosts []string, serverID *uuid.UUID) []string {
	if a == nil || a.Store == nil || resourceID == uuid.Nil {
		return nil
	}
	content, recType := "", ""
	if serverID != nil && *serverID != uuid.Nil {
		content, recType = a.serverDNSTarget(ctx, teamID, *serverID)
	}
	cache := map[uuid.UUID]dns.RecordAPI{}
	return dns.SyncHostnames(ctx, dnsLedger{a.Store}, dns.SyncInput{
		TeamID:       teamID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Hosts:        hosts,
		Content:      content,
		RecordType:   recType,
		ClientFor: func(tokenID uuid.UUID) (dns.RecordAPI, error) {
			if client, ok := cache[tokenID]; ok {
				return client, nil
			}
			enc, err := a.Store.GetCloudProviderTokenMaterial(ctx, teamID, tokenID)
			if err != nil {
				return nil, err
			}
			plain, err := a.Store.Box.DecryptString(enc)
			if err != nil {
				return nil, err
			}
			client := dns.NewCloudflare(plain)
			cache[tokenID] = client
			return client, nil
		},
	})
}

func (a *API) syncApplicationDNS(ctx context.Context, app *store.Application) []string {
	if app == nil {
		return nil
	}
	return a.syncManagedDNS(ctx, app.TeamID, "application", app.ID, dnsHostList(app.FQDN), a.resolveAppServerID(ctx, app))
}

func (a *API) syncServiceDNS(ctx context.Context, teamID uuid.UUID, svc *store.Service, hosts []string) []string {
	if svc == nil {
		return nil
	}
	var serverID *uuid.UUID
	if id, _, err := a.resolveServiceTarget(ctx, teamID, svc); err == nil && id != uuid.Nil {
		serverID = &id
	}
	return a.syncManagedDNS(ctx, teamID, "service", svc.ID, hosts, serverID)
}

func attachDNSWarnings(v any, warnings []string) any {
	if len(warnings) == 0 || v == nil {
		return v
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return v
	}
	m["dns_warnings"] = warnings
	return m
}

func (a *API) handleSyncDNSZones(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tokenID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	teamID := currentTeamID(r)
	tok, err := a.Store.GetCloudProviderToken(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if tok.Provider != "cloudflare" {
		writeError(w, http.StatusBadRequest, "zone sync is only available for Cloudflare tokens")
		return
	}
	enc, err := a.Store.GetCloudProviderTokenMaterial(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	plain, err := a.Store.Box.DecryptString(enc)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	zones, err := dns.NewCloudflare(plain).ListZones()
	if err != nil {
		writeError(w, http.StatusBadGateway, "cloudflare zone sync failed")
		return
	}
	in := make([]store.DNSZoneInput, 0, len(zones))
	for _, z := range zones {
		in = append(in, store.DNSZoneInput{ProviderZoneID: z.ID, Name: z.Name})
	}
	kept, retained, err := a.Store.ReplaceDNSZones(r.Context(), teamID, id, in)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	out := make([]map[string]string, 0, len(kept))
	for _, z := range kept {
		out = append(out, map[string]string{"id": z.ProviderZoneID, "name": z.Name})
	}
	if retained == nil {
		retained = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"zones": out, "retained": retained})
}
