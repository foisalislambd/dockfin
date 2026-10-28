package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/database"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/tlscerts"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	caLifetime   = 10 * 365 * 24 * time.Hour
	leafLifetime = 365 * 24 * time.Hour
)

func (a *API) prepareDatabaseTLS(ctx context.Context, teamID, serverID uuid.UUID, db *store.Database) (*database.TLSMaterial, error) {
	if !database.SSLSupported(db.Engine) {
		return nil, errString("ssl is not supported for " + db.Engine)
	}
	ca, err := a.ensureServerCA(ctx, teamID, serverID)
	if err != nil {
		return nil, err
	}
	leaf, err := a.ensureDatabaseLeaf(ctx, teamID, serverID, db, ca)
	if err != nil {
		return nil, err
	}
	return &database.TLSMaterial{
		CACert: []byte(ca.CertPEM),
		Cert:   []byte(leaf.CertPEM),
		Key:    []byte(leaf.KeyPEM),
	}, nil
}

func (a *API) ensureServerCA(ctx context.Context, teamID, serverID uuid.UUID) (*store.SSLMaterial, error) {
	existing, err := a.Store.GetServerCA(ctx, teamID, serverID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.ValidUntil.After(time.Now().Add(30*24*time.Hour)) {
		return existing, nil
	}
	made, err := tlscerts.GenerateCA("Dockfin CA", caLifetime)
	if err != nil {
		return nil, err
	}
	certEnc, err := a.Store.Box.EncryptString(made.CertPEM)
	if err != nil {
		return nil, err
	}
	keyEnc, err := a.Store.Box.EncryptString(made.KeyPEM)
	if err != nil {
		return nil, err
	}
	if _, err := a.Store.SaveServerCA(ctx, teamID, serverID, certEnc, keyEnc, made.CommonName, made.NotAfter); err != nil {
		return nil, err
	}
	return a.Store.GetServerCA(ctx, teamID, serverID)
}

func (a *API) ensureDatabaseLeaf(ctx context.Context, teamID, serverID uuid.UUID, db *store.Database, ca *store.SSLMaterial) (*store.SSLMaterial, error) {
	existing, err := a.Store.GetResourceCertificate(ctx, teamID, "database", db.ID)
	if err != nil {
		return nil, err
	}
	ips := serverCertIPs(ctx, a, teamID, serverID)
	dnsNames := []string{db.ID.String(), database.ContainerName(db.ID.String())}
	if existing != nil && existing.ServerID == serverID && existing.ValidUntil.After(time.Now().Add(7*24*time.Hour)) {
		if err := tlscerts.LeafCovers(existing.CertPEM, ca.CertPEM, dnsNames, ips); err == nil {
			return existing, nil
		}
	}
	made, err := tlscerts.GenerateLeaf(tlscerts.Material{CertPEM: ca.CertPEM, KeyPEM: ca.KeyPEM}, db.ID.String(), dnsNames, ips, leafLifetime)
	if err != nil {
		return nil, err
	}
	certEnc, err := a.Store.Box.EncryptString(made.CertPEM)
	if err != nil {
		return nil, err
	}
	keyEnc, err := a.Store.Box.EncryptString(made.KeyPEM)
	if err != nil {
		return nil, err
	}
	sans := []string{"DNS:" + db.ID.String(), "DNS:" + database.ContainerName(db.ID.String())}
	for _, ip := range ips {
		sans = append(sans, "IP:"+ip.String())
	}
	if _, err := a.Store.SaveResourceCertificate(ctx, teamID, serverID, db.ID, "database", certEnc, keyEnc, made.CommonName, sans, made.NotAfter); err != nil {
		return nil, err
	}
	return a.Store.GetResourceCertificate(ctx, teamID, "database", db.ID)
}

func serverCertIPs(ctx context.Context, a *API, teamID, serverID uuid.UUID) []net.IP {
	srv, err := a.Store.GetServer(ctx, teamID, serverID)
	if err != nil || srv == nil {
		return nil
	}
	var out []net.IP
	seen := map[string]bool{}
	for _, raw := range []string{srv.PublicIP, srv.IP} {
		ip := net.ParseIP(strings.TrimSpace(raw))
		if ip == nil {
			continue
		}
		key := ip.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ip)
	}
	return out
}

func (a *API) handleGetDatabaseTLS(w http.ResponseWriter, r *http.Request) {
	db, _, ok := a.databaseForTLS(w, r)
	if !ok {
		return
	}
	out := map[string]any{"enabled": db.EnableSSL, "supported": database.SSLSupported(db.Engine)}
	cert, err := a.Store.GetResourceCertificate(r.Context(), db.TeamID, "database", db.ID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if cert != nil {
		out["common_name"] = cert.CommonName
		out["valid_until"] = cert.ValidUntil
		out["subject_alternative_names"] = cert.SANs
	}
	if db.DestinationID != nil {
		if dest, err := a.Store.GetDestination(r.Context(), db.TeamID, *db.DestinationID); err == nil {
			if ca, err := a.Store.GetServerCA(r.Context(), db.TeamID, dest.ServerID); err == nil && ca != nil {
				out["ca_common_name"] = ca.CommonName
				out["ca_valid_until"] = ca.ValidUntil
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleGetDatabaseCA(w http.ResponseWriter, r *http.Request) {
	db, dest, ok := a.databaseForTLS(w, r)
	if !ok {
		return
	}
	if dest == nil {
		writeError(w, http.StatusBadRequest, "database has no destination")
		return
	}
	ca, err := a.Store.GetServerCA(r.Context(), db.TeamID, dest.ServerID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if ca == nil {
		writeError(w, http.StatusNotFound, "CA certificate is created the first time SSL starts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"common_name": ca.CommonName,
		"certificate": ca.CertPEM,
	})
}

func (a *API) handleRegenerateDatabaseTLS(w http.ResponseWriter, r *http.Request) {
	db, _, ok := a.databaseForTLS(w, r)
	if !ok {
		return
	}
	if err := a.Store.DeleteResourceCertificate(r.Context(), db.TeamID, "database", db.ID); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
}

func (a *API) databaseForTLS(w http.ResponseWriter, r *http.Request) (*store.Database, *store.Destination, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "dbID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return nil, nil, false
	}
	teamID := currentTeamID(r)
	db, err := a.Store.GetDatabase(r.Context(), teamID, id)
	if err != nil {
		mapStoreErr(w, err)
		return nil, nil, false
	}
	if db.DestinationID == nil {
		return db, nil, true
	}
	dest, err := a.Store.GetDestination(r.Context(), teamID, *db.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return nil, nil, false
	}
	return db, dest, true
}

type errString string

func (e errString) Error() string { return string(e) }
