package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/dockfin/dockfin/internal/imagescan"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *API) handleImageScanPolicy(w http.ResponseWriter, r *http.Request) {
	p, err := a.Store.ImageScanPolicy(r.Context())
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *API) handlePutImageScanPolicy(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	var body struct {
		Enabled bool   `json:"enabled"`
		Gate    string `json:"gate"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := a.Store.SetImageScanPolicy(r.Context(), body.Enabled, body.Gate); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, store.ImageScanPolicy{Enabled: body.Enabled, Gate: body.Gate})
}

func (a *API) handleListImageScans(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListImageScans(r.Context(), currentTeamID(r), 50)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scans": list})
}

func (a *API) handleGetImageScan(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "scanID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scan")
		return
	}
	sc, err := a.Store.GetImageScan(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (a *API) handleIgnoreVuln(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	var body struct {
		VulnID string `json:"vuln_id"`
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil || strings.TrimSpace(body.VulnID) == "" {
		writeError(w, http.StatusBadRequest, "vuln_id required")
		return
	}
	if err := a.Store.IgnoreImageVuln(r.Context(), currentTeamID(r), strings.TrimSpace(body.VulnID), body.Reason); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
}

func (a *API) handleDeleteVulnIgnore(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	id := chi.URLParam(r, "vulnID")
	if err := a.Store.DeleteImageVulnIgnore(r.Context(), currentTeamID(r), id); err != nil {
		mapStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleScanApplication(w http.ResponseWriter, r *http.Request) {
	appID, err := uuid.Parse(chi.URLParam(r, "appID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid application")
		return
	}
	app, err := a.Store.GetApplication(r.Context(), currentTeamID(r), appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	var body struct {
		Image string `json:"image"`
	}
	_ = decodeJSON(r, &body)
	ref := strings.TrimSpace(body.Image)
	if ref == "" {
		ref = imagescan.ApplicationImage(app)
	}
	if ref == "" && app.DockerComposeRaw != "" {
		refs := imagescan.ComposeImages(app.DockerComposeRaw)
		if len(refs) > 0 {
			ref = refs[0]
		}
	}
	if app.DestinationID == nil {
		writeError(w, http.StatusBadRequest, "application has no destination")
		return
	}
	dest, err := a.Store.GetDestination(r.Context(), currentTeamID(r), *app.DestinationID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	sc, err := a.scanOnServer(r, dest.ServerID, "application", app.ID, ref)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (a *API) handleScanService(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service")
		return
	}
	svc, err := a.Store.GetService(r.Context(), currentTeamID(r), id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	refs := imagescan.ServiceImages(svc)
	if len(refs) == 0 {
		writeError(w, http.StatusBadRequest, "service compose has no image")
		return
	}
	var serverID uuid.UUID
	switch {
	case svc.ServerID != nil:
		serverID = *svc.ServerID
	case svc.DestinationID != nil:
		dest, err := a.Store.GetDestination(r.Context(), currentTeamID(r), *svc.DestinationID)
		if err != nil {
			mapStoreErr(w, err)
			return
		}
		serverID = dest.ServerID
	default:
		writeError(w, http.StatusBadRequest, "service has no server")
		return
	}
	var last *store.ImageScan
	for _, ref := range refs {
		sc, err := a.scanOnServer(r, serverID, "service", svc.ID, ref)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		last = sc
	}
	writeJSON(w, http.StatusOK, last)
}

func (a *API) scanOnServer(r *http.Request, serverID uuid.UUID, kind string, resourceID uuid.UUID, imageRef string) (*store.ImageScan, error) {
	if !imagescan.ValidImageRef(imageRef) {
		return nil, errors.New("invalid image reference")
	}
	client, err := a.dialServer(r, serverID)
	if err != nil {
		return nil, err
	}
	ignored, err := a.Store.ImageScanIgnores(r.Context(), currentTeamID(r))
	if err != nil {
		return nil, err
	}
	raw, runErr := imagescan.Run(client, imageRef)
	scan := store.ImageScan{
		TeamID: currentTeamID(r), ResourceType: kind, ResourceID: resourceID,
		ServerID: &serverID, ImageRef: imageRef, Status: "completed", Findings: []byte("[]"),
	}
	if runErr != nil {
		scan.Status = "failed"
		scan.Error = runErr.Error()
	} else {
		sum, err := imagescan.ParseGrype(raw, ignored)
		if err != nil {
			scan.Status = "failed"
			scan.Error = err.Error()
		} else {
			scan.Critical, scan.High, scan.Medium, scan.Low, scan.Unknown = sum.Critical, sum.High, sum.Medium, sum.Low, sum.Unknown
			b, _ := json.Marshal(sum.Findings)
			scan.Findings = b
		}
	}
	saved, err := a.Store.InsertImageScan(r.Context(), scan)
	if err != nil {
		return nil, err
	}
	if runErr != nil {
		return saved, runErr
	}
	return saved, nil
}

func (a *API) handleExportBundle(w http.ResponseWriter, r *http.Request) {
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
		Passphrase string `json:"passphrase"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	sealed, err := a.Store.ExportSecretBundle(r.Context(), currentTeamID(r), serverID, body.Passphrase)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"bundle": sealed})
}

func (a *API) handleImportBundle(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	var body struct {
		Passphrase string `json:"passphrase"`
		Bundle     string `json:"bundle"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	res, err := a.Store.ImportSecretBundle(r.Context(), currentTeamID(r), body.Passphrase, body.Bundle)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, res)
}
