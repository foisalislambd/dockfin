package httpapi

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		category, summary := classifyAudit(r.Method, r.URL.Path, action, rtype)
		email := ""
		if u, ok := r.Context().Value(ctxUser).(*store.User); ok && u != nil {
			email = u.Email
		}
		_ = a.Store.InsertAuditLog(context.Background(), store.AuditLog{
			TeamID:       teamID,
			UserID:       uid,
			ActorEmail:   email,
			Method:       r.Method,
			Path:         r.URL.Path,
			Action:       action,
			Category:     category,
			Summary:      summary,
			ResourceType: rtype,
			ResourceID:   rid,
			StatusCode:   sw.code,
			IP:           a.rateLimitIP(r),
			UserAgent:    r.UserAgent(),
		})
	})
}

func (a *API) recordAuth(r *http.Request, user *store.User, email, action, summary string, status int) {
	if a == nil || a.Store == nil || r == nil {
		return
	}
	row := store.AuditLog{
		Method:       r.Method,
		Path:         r.URL.Path,
		Action:       action,
		Category:     "auth",
		Summary:      summary,
		ResourceType: "auth",
		ActorEmail:   strings.TrimSpace(email),
		StatusCode:   status,
		IP:           a.rateLimitIP(r),
		UserAgent:    r.UserAgent(),
	}
	if user != nil {
		id := user.ID
		row.UserID = &id
		if row.ActorEmail == "" {
			row.ActorEmail = user.Email
		}
		teams, err := a.Store.ListTeamsForUser(r.Context(), user.ID)
		if err == nil && len(teams) > 0 {
			for _, team := range teams {
				row.TeamID = team.ID
				_ = a.Store.InsertAuditLog(context.Background(), row)
			}
			return
		}
	}
	_ = a.Store.InsertAuditLog(context.Background(), row)
}

func (a *API) startMaintenance() {
	if a == nil {
		return
	}
	a.maintOnce.Do(func() {
		if a.Store == nil {
			return
		}
		go func() {
			a.pruneMaintenance()
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				a.pruneMaintenance()
			}
		}()
	})
}

func (a *API) pruneMaintenance() {
	if a == nil || a.Store == nil {
		return
	}
	days := 90
	if a.Cfg != nil {
		days = a.Cfg.AuditRetentionDays
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := a.Store.PruneAuditLogs(ctx, days); err != nil && a.Logger != nil {
		a.Logger.Warn("audit retention prune failed", "error", err.Error())
	}
	if a.Cfg != nil && a.Cfg.RateLimitStore == "postgres" {
		if err := a.Store.PruneRateLimitHits(ctx); err != nil && a.Logger != nil {
			a.Logger.Warn("rate limit prune failed", "error", err.Error())
		}
	}
}

func (a *API) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	if !isTeamAdmin(r) {
		writeError(w, http.StatusForbidden, "admin or owner role required")
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	page, err := a.Store.QueryAuditLogs(r.Context(), store.AuditQuery{
		TeamID:              currentTeamID(r),
		Action:              q.Get("action"),
		Category:            q.Get("category"),
		ResourceType:        q.Get("resource_type"),
		Query:               q.Get("q"),
		Limit:               limit,
		Offset:              offset,
		IncludeUnscopedAuth: true,
	})
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"audit_logs": page.Logs,
		"total":      page.Total,
		"limit":      page.Limit,
		"offset":     page.Offset,
	})
}

func classifyAudit(method, path, action, resourceType string) (category, summary string) {
	p := strings.ToLower(path)
	switch {
	case strings.Contains(p, "/auth/"):
		return "auth", "Authentication " + action
	case strings.HasSuffix(p, "/deploy"):
		return "deploy", "Queued a deployment"
	case strings.HasSuffix(p, "/image-upgrade"):
		return "deploy", "Queued an image upgrade or rebuild"
	case strings.HasSuffix(p, "/candidates"):
		return "resource", "Checked newer image tags"
	case strings.HasSuffix(p, "/transfer"):
		return "access", "Transferred a server to another team"
	case strings.HasSuffix(p, "/rollback"):
		return "deploy", "Rolled back a deployment"
	case strings.HasSuffix(p, "/freeze"):
		return "deploy", "Updated the deploy freeze"
	case strings.Contains(p, "/team/members") && method == http.MethodPatch:
		return "access", "Changed a member role"
	case strings.Contains(p, "/team/members") && method == http.MethodDelete:
		return "access", "Removed a team member"
	case strings.Contains(p, "/invitations"):
		return "access", "Updated a team invitation"
	case strings.Contains(p, "/settings"):
		return "settings", "Updated instance settings"
	case method == http.MethodDelete:
		return "resource", "Deleted " + strings.TrimSpace(resourceType)
	case method == http.MethodPost:
		return "resource", "Created " + strings.TrimSpace(resourceType)
	case method == http.MethodPut || method == http.MethodPatch:
		return "resource", "Updated " + strings.TrimSpace(resourceType)
	default:
		return "mutation", strings.TrimSpace(method + " " + resourceType)
	}
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
