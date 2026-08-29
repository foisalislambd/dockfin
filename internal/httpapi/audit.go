package httpapi

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/dockfin/dockfin/internal/store"
	"github.com/google/uuid"
)

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

func (a *API) auditMutations(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(sw, r)

		teamID := currentTeamID(r)
		var uid *uuid.UUID
		if u, ok := r.Context().Value(ctxUser).(*store.User); ok && u != nil {
			id := u.ID
			uid = &id
		}
		action, rtype, rid := auditFromPath(r.Method, r.URL.Path)
		_ = a.Store.InsertAuditLog(context.Background(), store.AuditLog{
			TeamID:       teamID,
			UserID:       uid,
			Method:       r.Method,
			Path:         r.URL.Path,
			Action:       action,
			ResourceType: rtype,
			ResourceID:   rid,
			StatusCode:   sw.code,
			IP:           a.rateLimitIP(r),
			UserAgent:    r.UserAgent(),
		})
	})
}

func (a *API) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	list, err := a.Store.ListAuditLogs(r.Context(), currentTeamID(r), 100)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	if list == nil {
		list = []store.AuditLog{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"audit_logs": list})
}

func auditFromPath(method, path string) (action, resourceType, resourceID string) {
	action = method
	path = strings.TrimPrefix(path, "/api/v1")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return action, "", ""
	}
	resourceType = parts[0]
	if len(parts) >= 2 && looksUUID(parts[1]) {
		resourceID = parts[1]
	}
	if len(parts) >= 3 && !looksUUID(parts[len(parts)-1]) {
		action = parts[len(parts)-1]
	}
	return action, resourceType, resourceID
}

func looksUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
