package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/goolify/goolify/internal/config"
	"github.com/goolify/goolify/internal/store"
	"github.com/goolify/goolify/internal/terminal"
	"github.com/goolify/goolify/internal/worker"
	"github.com/goolify/goolify/internal/ws"
)

type ctxKey string

const (
	ctxUser    ctxKey = "user"
	ctxSession ctxKey = "session"
	ctxTeamID  ctxKey = "team_id"
	ctxRole    ctxKey = "role"
)

type API struct {
	Cfg       *config.Config
	Store     *store.Store
	Queue     *worker.Queue
	Hub       *ws.Hub
	Terminals *terminal.Manager
	SSH       interface{ Close() } // *sshx.Pool
	Logger    *slog.Logger
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   a.Cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Team-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "goolify"})
	})
	r.Get("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"version": "0.1.0",
			"name":    "Goolify",
			"license": "MIT",
		})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", a.handleRegister)
		r.Post("/auth/login", a.handleLogin)
		r.With(a.requireAuth).Post("/auth/logout", a.handleLogout)
		r.With(a.requireAuth).Get("/auth/me", a.handleMe)
		r.With(a.requireAuth).Post("/auth/switch-team", a.handleSwitchTeam)

		// Public ingress endpoints (no session cookie)
		r.Post("/webhooks/git/{appID}", a.handleGitWebhook)
		r.Get("/webhooks/github/app/callback", a.handleGitHubAppCallback)
		r.Post("/sentinel/metrics", a.handleSentinelMetrics)

		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)
			r.Use(a.requireTeam)
			r.Use(timeoutExceptSSE(120 * time.Second))

			r.Get("/teams", a.handleListTeams)

			r.Get("/team/members", a.handleListTeamMembers)
			r.Delete("/team/members/{userID}", a.handleRemoveTeamMember)
			r.Get("/team/invitations", a.handleListInvitations)
			r.Post("/team/invitations", a.handleCreateInvitation)
			r.Delete("/team/invitations/{inviteID}", a.handleDeleteInvitation)
			r.Post("/team/invitations/accept", a.handleAcceptInvitation)

			r.Route("/api-tokens", func(r chi.Router) {
				r.Get("/", a.handleListApiTokens)
				r.Post("/", a.handleCreateApiToken)
				r.Delete("/{tokenID}", a.handleDeleteApiToken)
			})

			r.Route("/private-keys", func(r chi.Router) {
				r.Get("/", a.handleListKeys)
				r.Post("/", a.handleCreateKey)
			})

			r.Route("/servers", func(r chi.Router) {
				r.Get("/", a.handleListServers)
				r.Post("/", a.handleCreateServer)
				r.Post("/bootstrap-self", a.handleBootstrapSelf)
				r.Get("/{serverID}", a.handleGetServer)
				r.Delete("/{serverID}", a.handleDeleteServer)
				r.Patch("/{serverID}/settings", a.handlePatchServerSettings)
				r.Post("/{serverID}/destinations", a.handleCreateDestination)
				r.Post("/{serverID}/terminal", a.handleCreateTerminal)
				r.Post("/{serverID}/validate", a.handleValidateServer)
				r.Post("/{serverID}/proxy/start", a.handleStartProxy)
				r.Post("/{serverID}/proxy/stop", a.handleStopProxy)
				r.Post("/{serverID}/exec", a.handleServerExec)
				r.Get("/{serverID}/metrics", a.handleListServerMetrics)
			})

			r.Get("/terminal/ws/{sessionID}", a.handleTerminalWS)

			r.Route("/git-sources", func(r chi.Router) {
				r.Get("/", a.handleListGitSources)
				r.Post("/", a.handleCreateGitSource)
				r.Get("/{sourceID}", a.handleGetGitSource)
				r.Delete("/{sourceID}", a.handleDeleteGitSource)
				r.Get("/{sourceID}/install-url", a.handleGitSourceInstallURL)
				r.Get("/{sourceID}/repositories", a.handleGitSourceRepositories)
			})

			r.Get("/destinations", a.handleListDestinations)
			r.Post("/domains/generate", a.handleGenerateDomain)

			r.Route("/projects", func(r chi.Router) {
				r.Get("/", a.handleListProjects)
				r.Post("/", a.handleCreateProject)
				r.Get("/{projectID}", a.handleGetProject)
				r.Patch("/{projectID}", a.handleUpdateProject)
				r.Delete("/{projectID}", a.handleDeleteProject)
				r.Get("/{projectID}/environments", a.handleListEnvironments)
				r.Post("/{projectID}/environments", a.handleCreateEnvironment)
				r.Get("/{projectID}/environments/{envID}", a.handleGetEnvironment)
				r.Patch("/{projectID}/environments/{envID}", a.handleUpdateEnvironment)
				r.Delete("/{projectID}/environments/{envID}", a.handleDeleteEnvironment)
			})

			r.Route("/applications", func(r chi.Router) {
				r.Get("/", a.handleListApplications)
				r.Post("/", a.handleCreateApplication)
				r.Get("/{appID}", a.handleGetApplication)
				r.Patch("/{appID}", a.handleUpdateApplication)
				r.Delete("/{appID}", a.handleDeleteApplication)
				r.Post("/{appID}/deploy", a.handleDeployApplication)
				r.Get("/{appID}/deployments", a.handleListDeployments)
				r.Post("/{appID}/webhook-secret", a.handleSetWebhookSecret)
				r.Post("/{appID}/rollback", a.handleRollbackApplication)
				r.Get("/{appID}/previews", a.handleListPreviews)
				r.Delete("/{appID}/previews/{prID}", a.handleDeletePreview)
			})

			r.Get("/deployments/{deploymentID}", a.handleGetDeployment)
			r.Post("/deployments/{deploymentID}/cancel", a.handleCancelDeployment)
			r.Get("/deployments/{deploymentID}/logs/stream", a.handleDeploymentLogStream)

			r.Route("/env-vars", func(r chi.Router) {
				r.Get("/", a.handleListEnvVars)
				r.Post("/", a.handleUpsertEnvVar)
				r.Delete("/{varID}", a.handleDeleteEnvVar)
			})
			r.Route("/shared-env-vars", func(r chi.Router) {
				r.Get("/", a.handleListSharedEnv)
				r.Post("/", a.handleUpsertSharedEnv)
			})

			r.Route("/databases", func(r chi.Router) {
				r.Get("/", a.handleListDatabases)
				r.Post("/", a.handleCreateDatabase)
				r.Get("/{dbID}", a.handleGetDatabase)
				r.Delete("/{dbID}", a.handleDeleteDatabase)
				r.Post("/{dbID}/start", a.handleStartDatabase)
				r.Post("/{dbID}/stop", a.handleStopDatabase)
				r.Get("/{dbID}/backups", a.handleListDatabaseBackups)
				r.Post("/{dbID}/backups", a.handleRunDatabaseBackup)
				r.Post("/{dbID}/backups/restore", a.handleRestoreDatabaseBackup)
			})

			r.Route("/services", func(r chi.Router) {
				r.Get("/", a.handleListServices)
				r.Post("/", a.handleCreateService)
				r.Get("/templates", a.handleListServiceTemplates)
				r.Get("/{serviceID}", a.handleGetService)
				r.Patch("/{serviceID}", a.handlePatchService)
				r.Delete("/{serviceID}", a.handleDeleteService)
				r.Post("/{serviceID}/deploy", a.handleDeployService)
				r.Post("/{serviceID}/stop", a.handleStopService)
				r.Post("/{serviceID}/restart", a.handleRestartService)
			})

			r.Route("/s3-storages", func(r chi.Router) {
				r.Get("/", a.handleListS3Storages)
				r.Post("/", a.handleCreateS3Storage)
				r.Get("/{storageID}", a.handleGetS3Storage)
				r.Delete("/{storageID}", a.handleDeleteS3Storage)
			})

			r.Route("/scheduled-backups", func(r chi.Router) {
				r.Get("/", a.handleListScheduledBackups)
				r.Post("/", a.handleCreateScheduledBackup)
			})

			r.Get("/notifications", a.handleListNotifications)
			r.Put("/notifications/{channel}", a.handleUpsertNotification)

			r.Get("/scheduled-tasks", a.handleListScheduledTasks)
			r.Post("/scheduled-tasks", a.handleCreateScheduledTask)
		})
	})

	a.mountWebOrRoot(r)

	return r
}

func timeoutExceptSSE(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/logs/stream") ||
				strings.Contains(r.URL.Path, "/terminal/ws/") ||
				(strings.HasSuffix(r.URL.Path, "/deploy") &&
					(r.URL.Query().Get("stream") == "1" || strings.Contains(r.Header.Get("Accept"), "text/event-stream"))) {
				next.ServeHTTP(w, r)
				return
			}
			chimw.Timeout(d)(next).ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := sessionToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		// Prefer session cookie / session bearer; fall back to API tokens (glfy_…).
		sess, err := a.Store.GetSession(r.Context(), token)
		if err == nil {
			user, err := a.Store.GetUserByID(r.Context(), sess.UserID)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			ctx := context.WithValue(r.Context(), ctxUser, user)
			ctx = context.WithValue(ctx, ctxSession, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		apiTok, user, err := a.Store.GetApiTokenByPlain(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		teamID := apiTok.TeamID
		synth := &store.Session{
			ID:            uuid.Nil,
			UserID:        user.ID,
			CurrentTeamID: &teamID,
			ExpiresAt:     time.Now().UTC().Add(24 * time.Hour),
		}
		ctx := context.WithValue(r.Context(), ctxUser, user)
		ctx = context.WithValue(ctx, ctxSession, synth)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) requireTeam(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Context().Value(ctxUser).(*store.User)
		sess := r.Context().Value(ctxSession).(*store.Session)
		teamID := sess.CurrentTeamID
		// Cookie/session auth may override team via header. API tokens are bound to one team.
		if sess.ID != uuid.Nil {
			if hdr := r.Header.Get("X-Team-ID"); hdr != "" {
				id, err := uuid.Parse(hdr)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid X-Team-ID")
					return
				}
				teamID = &id
			}
		}
		if teamID == nil {
			writeError(w, http.StatusBadRequest, "no team selected")
			return
		}
		role, err := a.Store.UserRoleOnTeam(r.Context(), user.ID, *teamID)
		if err != nil {
			writeError(w, http.StatusForbidden, "not a team member")
			return
		}
		ctx := context.WithValue(r.Context(), ctxTeamID, *teamID)
		ctx = context.WithValue(ctx, ctxRole, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func currentUser(r *http.Request) *store.User {
	return r.Context().Value(ctxUser).(*store.User)
}

func currentSession(r *http.Request) *store.Session {
	return r.Context().Value(ctxSession).(*store.Session)
}

func currentTeamID(r *http.Request) uuid.UUID {
	return r.Context().Value(ctxTeamID).(uuid.UUID)
}

func sessionToken(r *http.Request) string {
	if c, err := r.Cookie("goolify_session"); err == nil && c.Value != "" {
		return c.Value
	}
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && (auth[:7] == "Bearer " || auth[:7] == "bearer ") {
		return auth[7:]
	}
	return ""
}

func setSessionCookie(w http.ResponseWriter, cfg *config.Config, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "goolify_session",
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.CookieSecure,
	})
}

func clearSessionCookie(w http.ResponseWriter, cfg *config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     "goolify_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cfg.CookieSecure,
	})
}

func mapStoreErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrNotEmpty):
		writeError(w, http.StatusConflict, "has resources defined, please delete them first")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict")
	case errors.Is(err, store.ErrUnauthorized):
		writeError(w, http.StatusForbidden, "forbidden")
	default:
		slog.Error("request error", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
