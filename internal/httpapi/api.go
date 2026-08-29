package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dockfin/dockfin/internal/config"
	"github.com/dockfin/dockfin/internal/store"
	"github.com/dockfin/dockfin/internal/terminal"
	"github.com/dockfin/dockfin/internal/version"
	"github.com/dockfin/dockfin/internal/worker"
	"github.com/dockfin/dockfin/internal/ws"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
)

type ctxKey string

const (
	ctxUser    ctxKey = "user"
	ctxSession ctxKey = "session"
	ctxTeamID  ctxKey = "team_id"
	ctxRole    ctxKey = "role"
)

type API struct {
	Cfg        *config.Config
	Store      *store.Store
	Queue      *worker.Queue
	Hub        *ws.Hub
	Terminals  *terminal.Manager
	SSH        interface{ Close() } // *sshx.Pool
	Logger     *slog.Logger
	TaskRunner TaskRunner
}

// TaskRunner runs scheduled tasks on demand (wired to the scheduler runner).
type TaskRunner interface {
	ExecuteTaskNow(ctx context.Context, t store.ScheduledTaskRow, execID uuid.UUID)
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	// RealIP only when explicitly trusted — otherwise clients can spoof
	// X-Forwarded-For and bypass login rate limits / API IP allowlists.
	if a.Cfg != nil && a.Cfg.TrustProxy {
		r.Use(chimw.RealIP)
	}
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc:  a.corsAllowOrigin,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Team-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", a.handleHealth)
	r.Get("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"version": version.Version,
			"name":    "Dockfin",
			"license": "MIT",
		})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", a.handleRegister)
		r.Get("/auth/registration", a.handleRegistrationStatus)
		r.Post("/auth/login", a.handleLogin)
		r.Post("/auth/login/2fa", a.handleLogin2FA)
		r.Post("/auth/forgot-password", a.handleForgotPassword)
		r.Post("/auth/reset-password", a.handleResetPassword)
		r.With(a.requireAuth).Post("/auth/logout", a.handleLogout)
		r.With(a.requireAuth).Get("/auth/me", a.handleMe)
		r.With(a.requireAuth).Post("/auth/switch-team", a.handleSwitchTeam)
		r.With(a.requireAuth).Post("/auth/2fa/setup", a.handleTOTPSetup)
		r.With(a.requireAuth).Post("/auth/2fa/enable", a.handleTOTPEnable)
		r.With(a.requireAuth).Post("/auth/2fa/disable", a.handleTOTPDisable)
		r.With(a.requireAuth).Get("/auth/2fa/status", a.handleTOTPStatus)

		// Public OAuth login endpoints (no session cookie yet).
		r.Get("/auth/oauth/providers", a.handleOauthProviders)
		r.Get("/auth/oauth/{provider}/start", a.handleOauthStart)
		r.Get("/auth/oauth/{provider}/callback", a.handleOauthCallback)

		// Public ingress endpoints (no session cookie)
		r.Post("/webhooks/git/{appID}", a.handleGitWebhook)
		r.Get("/webhooks/github/app/callback", a.handleGitHubAppCallback)
		r.Get("/webhooks/github/app/manifest", a.handleGitHubAppManifestCallback)
		r.Post("/webhooks/github/app/events", a.handleGitHubAppEvents)
		r.Post("/sentinel/metrics", a.handleSentinelMetrics)

		// Coolify-compatible deploy webhook (auth via Bearer API token or resource secret)
		r.Get("/deploy", a.handleDeployByUUID)
		r.Post("/deploy", a.handleDeployByUUID)
		r.Get("/webhooks/deploy/services/{serviceID}", a.handleServiceDeployWebhook)
		r.Post("/webhooks/deploy/services/{serviceID}", a.handleServiceDeployWebhook)

		r.Get("/invitations/preview", a.handlePreviewInvitation)

		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)
			r.Use(a.enforceAPITokenPolicy)
			r.Use(a.requireTeam)
			r.Use(a.auditMutations)
			r.Use(timeoutExceptSSE(120 * time.Second))

			r.Get("/teams", a.handleListTeams)
			r.Post("/teams", a.handleCreateTeam)

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
				r.Post("/generate", a.handleGenerateKey)
				r.Post("/cleanup-unused", a.handleCleanupUnusedKeys)
				r.Get("/{keyID}", a.handleGetKey)
				r.Patch("/{keyID}", a.handleUpdateKey)
				r.Delete("/{keyID}", a.handleDeleteKey)
			})

			r.Route("/cloud-tokens", func(r chi.Router) {
				r.Get("/", a.handleListCloudTokens)
				r.Post("/", a.handleCreateCloudToken)
				r.Get("/{tokenID}", a.handleGetCloudToken)
				r.Patch("/{tokenID}", a.handleUpdateCloudToken)
				r.Delete("/{tokenID}", a.handleDeleteCloudToken)
				r.Post("/{tokenID}/validate", a.handleValidateCloudToken)
				r.Get("/{tokenID}/defaults", a.handleCloudProviderDefaults)
				r.With(a.requireAdmin).Post("/{tokenID}/provision", a.handleProvisionServer)
			})

			r.Route("/cloud-init-scripts", func(r chi.Router) {
				r.Get("/", a.handleListCloudInitScripts)
				r.Post("/", a.handleCreateCloudInitScript)
				r.Get("/{scriptID}", a.handleGetCloudInitScript)
				r.Patch("/{scriptID}", a.handleUpdateCloudInitScript)
				r.Delete("/{scriptID}", a.handleDeleteCloudInitScript)
			})

			r.Route("/servers", func(r chi.Router) {
				r.Get("/", a.handleListServers)
				r.Post("/", a.handleCreateServer)
				r.Post("/bootstrap-self", a.handleBootstrapSelf)
				r.With(a.requireAdmin).Post("/provision", a.handleProvisionServer)
				r.Get("/{serverID}", a.handleGetServer)
				r.With(a.requireAdmin).Delete("/{serverID}", a.handleDeleteServer)
				r.Patch("/{serverID}/settings", a.handlePatchServerSettings)
				r.Post("/{serverID}/destinations", a.handleCreateDestination)
				r.Post("/{serverID}/terminal", a.handleCreateTerminal)
				r.Post("/{serverID}/validate", a.handleValidateServer)
				r.Post("/{serverID}/proxy/start", a.handleStartProxy)
				r.Post("/{serverID}/proxy/stop", a.handleStopProxy)
				r.Get("/{serverID}/proxy/logs", a.handleProxyLogs)
				r.Get("/{serverID}/proxy/dynamic", a.handleListProxyDynamic)
				r.Put("/{serverID}/proxy/dynamic", a.handleUpsertProxyDynamic)
				r.Delete("/{serverID}/proxy/dynamic/{configID}", a.handleDeleteProxyDynamic)
				r.Get("/{serverID}/ops", a.handleGetServerOps)
				r.Patch("/{serverID}/ops", a.handlePatchServerOps)
				r.Post("/{serverID}/sentinel/rotate-token", a.handleRotateSentinelToken)
				r.Post("/{serverID}/sentinel/{action}", a.handleSentinelAction)
				r.Get("/{serverID}/sentinel/logs", a.handleSentinelLogs)
				r.Post("/{serverID}/cloudflare-tunnel/{action}", a.handleCloudflareTunnelAction)
				r.With(a.requireAdmin).Post("/{serverID}/patches/check", a.handleCheckServerPatches)
				r.Post("/{serverID}/docker-cleanup", a.handleRunDockerCleanup)
				r.Get("/{serverID}/docker-cleanup/executions", a.handleListDockerCleanupExecutions)
				r.With(a.requireAdmin).Post("/{serverID}/exec", a.handleServerExec)
				r.Get("/{serverID}/metrics", a.handleListServerMetrics)
			})

			r.Get("/terminal/ws/{sessionID}", a.handleTerminalWS)

			r.Route("/git-sources", func(r chi.Router) {
				r.Get("/", a.handleListGitSources)
				r.Post("/", a.handleCreateGitSource)
				r.Get("/{sourceID}", a.handleGetGitSource)
				r.Patch("/{sourceID}", a.handleUpdateGitSource)
				r.Delete("/{sourceID}", a.handleDeleteGitSource)
				r.Get("/{sourceID}/install-url", a.handleGitSourceInstallURL)
				r.Get("/{sourceID}/manifest", a.handleGitSourceManifest)
				r.Get("/{sourceID}/repositories", a.handleGitSourceRepositories)
				r.Get("/{sourceID}/repositories/{owner}/{repo}/branches", a.handleGitSourceBranches)
				r.Get("/{sourceID}/applications", a.handleGitSourceApps)
			})

			r.Get("/destinations", a.handleListDestinations)
			r.Post("/domains/generate", a.handleGenerateDomain)
			r.Post("/domains/check", a.handleCheckDomainDNS)
			r.Post("/domains/cloudflare", a.handleCloudflareDNS)
			r.Get("/audit-logs", a.handleListAuditLogs)

			r.Route("/projects", func(r chi.Router) {
				r.Get("/", a.handleListProjects)
				r.Post("/", a.handleCreateProject)
				r.Get("/{projectID}", a.handleGetProject)
				r.Patch("/{projectID}", a.handleUpdateProject)
				r.With(a.requireAdmin).Delete("/{projectID}", a.handleDeleteProject)
				r.Get("/{projectID}/environments", a.handleListEnvironments)
				r.Post("/{projectID}/environments", a.handleCreateEnvironment)
				r.Get("/{projectID}/environments/{envID}", a.handleGetEnvironment)
				r.Patch("/{projectID}/environments/{envID}", a.handleUpdateEnvironment)
				r.With(a.requireAdmin).Delete("/{projectID}/environments/{envID}", a.handleDeleteEnvironment)
				r.Post("/{projectID}/environments/{envID}/clone", a.handleCloneEnvironment)
				r.Get("/{projectID}/environments/{envID}/tags", a.handleListEnvironmentTags)
			})

			r.Route("/applications", func(r chi.Router) {
				r.Get("/", a.handleListApplications)
				r.Post("/", a.handleCreateApplication)
				r.Post("/detect-compose", a.handleDetectCompose)
				r.Get("/{appID}", a.handleGetApplication)
				r.Patch("/{appID}", a.handleUpdateApplication)
				r.With(a.requireAdmin).Delete("/{appID}", a.handleDeleteApplication)
				r.Post("/{appID}/deploy", a.handleDeployApplication)
				r.Post("/{appID}/start", a.handleStartApplication)
				r.Post("/{appID}/stop", a.handleStopApplication)
				r.Post("/{appID}/restart", a.handleRestartApplication)
				r.Get("/{appID}/containers", a.handleListApplicationContainers)
				r.Get("/{appID}/logs/stream", a.handleApplicationLogsStream)
				r.Get("/{appID}/volumes", a.handleListAppVolumes)
				r.Post("/{appID}/volumes", a.handleUpsertAppVolume)
				r.Delete("/{appID}/volumes/{volumeID}", a.handleDeleteAppVolume)
				r.Get("/{appID}/backups", a.handleListApplicationBackups)
				r.Post("/{appID}/backups", a.handleRunApplicationBackup)
				r.Post("/{appID}/backups/restore", a.handleRestoreApplicationBackup)
				r.Post("/{appID}/clone", a.handleCloneApplication)
				r.Post("/{appID}/stop-cleanup", a.handleStopAndCleanupApplication)
				r.Get("/{appID}/images", a.handleListServerImages)
				r.Get("/{appID}/additional-destinations", a.handleListAdditionalDestinations)
				r.Put("/{appID}/additional-destinations", a.handleSetAdditionalDestinations)
				r.Get("/{appID}/metrics", a.handleApplicationMetrics)
				r.Post("/{appID}/detect-compose", a.handleDetectComposeForApp)
				r.Post("/{appID}/load-compose", a.handleLoadComposeForApp)
				r.Get("/{appID}/deployments", a.handleListDeployments)
				r.Post("/{appID}/webhook-secret", a.handleSetWebhookSecret)
				r.Post("/{appID}/rollback", a.handleRollbackApplication)
				r.Get("/{appID}/previews", a.handleListPreviews)
				r.Post("/{appID}/previews/deploy", a.handleManualPreviewDeploy)
				r.With(a.requireAdmin).Delete("/{appID}/previews/{prID}", a.handleDeletePreview)
			})

			r.Get("/environments/{envID}", a.handleGetEnvironmentByID)

			r.Get("/deployments/{deploymentID}", a.handleGetDeployment)
			r.Post("/deployments/{deploymentID}/cancel", a.handleCancelDeployment)
			r.Get("/deployments/{deploymentID}/logs/stream", a.handleDeploymentLogStream)

			r.Route("/env-vars", func(r chi.Router) {
				r.Get("/", a.handleListEnvVars)
				r.Post("/", a.handleUpsertEnvVar)
				r.Post("/{varID}/lock", a.handleLockEnvVar)
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
				r.Patch("/{dbID}", a.handleUpdateDatabase)
				r.With(a.requireAdmin).Delete("/{dbID}", a.handleDeleteDatabase)
				r.Post("/{dbID}/start", a.handleStartDatabase)
				r.Post("/{dbID}/stop", a.handleStopDatabase)
				r.Get("/{dbID}/logs/stream", a.handleDatabaseLogsStream)
				r.Get("/{dbID}/metrics", a.handleDatabaseMetrics)
				r.Get("/{dbID}/backups", a.handleListDatabaseBackups)
				r.Post("/{dbID}/backups", a.handleRunDatabaseBackup)
				r.Post("/{dbID}/backups/restore", a.handleRestoreDatabaseBackup)
				r.Post("/{dbID}/backups/import", a.handleImportDatabaseBackup)
			})

			r.Route("/services", func(r chi.Router) {
				r.Get("/", a.handleListServices)
				r.Post("/", a.handleCreateService)
				r.Get("/templates", a.handleListServiceTemplates)
				r.Get("/{serviceID}", a.handleGetService)
				r.Patch("/{serviceID}", a.handlePatchService)
				r.With(a.requireAdmin).Delete("/{serviceID}", a.handleDeleteService)
				r.Post("/{serviceID}/deploy", a.handleDeployService)
				r.Post("/{serviceID}/stop", a.handleStopService)
				r.Post("/{serviceID}/restart", a.handleRestartService)
				r.Get("/{serviceID}/webhook", a.handleGetServiceWebhookInfo)
				r.Post("/{serviceID}/webhook-secret", a.handleSetServiceWebhookSecret)
				r.Get("/{serviceID}/containers", a.handleListServiceContainers)
				r.Get("/{serviceID}/logs/stream", a.handleServiceLogsStream)
				r.Get("/{serviceID}/backups", a.handleListServiceBackups)
				r.Post("/{serviceID}/backups", a.handleRunServiceBackup)
				r.Post("/{serviceID}/backups/restore", a.handleRestoreServiceBackup)
			})

			r.Get("/mcp", a.handleMCPProbe)
			r.Post("/mcp", a.handleMCP)

			r.Route("/s3-storages", func(r chi.Router) {
				r.Get("/", a.handleListS3Storages)
				r.Post("/", a.handleCreateS3Storage)
				r.Get("/{storageID}", a.handleGetS3Storage)
				r.Delete("/{storageID}", a.handleDeleteS3Storage)
			})

			r.Route("/docker-registries", func(r chi.Router) {
				r.Get("/", a.handleListDockerRegistries)
				r.Post("/", a.handleCreateDockerRegistry)
				r.Patch("/{registryID}", a.handleUpdateDockerRegistry)
				r.Delete("/{registryID}", a.handleDeleteDockerRegistry)
			})

			r.Route("/scheduled-backups", func(r chi.Router) {
				r.Get("/", a.handleListScheduledBackups)
				r.Post("/", a.handleCreateScheduledBackup)
				r.Patch("/{backupID}", a.handlePatchScheduledBackup)
				r.Delete("/{backupID}", a.handleDeleteScheduledBackup)
			})

			r.Get("/notifications", a.handleListNotifications)
			r.Put("/notifications/{channel}", a.handleUpsertNotification)
			r.Post("/notifications/{channel}/test", a.handleTestNotification)

			r.Get("/settings", a.handleGetInstanceSettings)
			r.Patch("/settings", a.handlePatchInstanceSettings)
			r.Get("/settings/oauth", a.handleListOauthSettings)
			r.Patch("/settings/oauth/{provider}", a.handlePatchOauthSetting)
			r.Get("/settings/backup", a.handleGetInstanceBackup)
			r.Post("/settings/backup/configure", a.handleConfigureInstanceBackup)
			r.Patch("/settings/backup", a.handlePatchInstanceBackup)
			r.Post("/settings/backup/run", a.handleRunInstanceBackup)
			r.Post("/settings/backup/restore", a.handleRestoreInstanceBackup)
			r.Get("/settings/backup/executions", a.handleListInstanceBackupExecutions)

			r.Get("/scheduled-tasks", a.handleListScheduledTasks)
			r.Post("/scheduled-tasks", a.handleCreateScheduledTask)
			r.Patch("/scheduled-tasks/{taskID}", a.handlePatchScheduledTask)
			r.Delete("/scheduled-tasks/{taskID}", a.handleDeleteScheduledTask)
			r.Get("/scheduled-tasks/{taskID}/executions", a.handleListScheduledTaskExecutions)
			r.Post("/scheduled-tasks/{taskID}/run", a.handleRunScheduledTask)
			r.Get("/scheduled-task-executions", a.handleListScheduledTaskExecutionsForTeam)
			r.Get("/docker-cleanup-executions", a.handleListDockerCleanupExecutionsForTeam)

			r.Route("/tags", func(r chi.Router) {
				r.Get("/", a.handleListTags)
				r.Post("/", a.handleCreateTag)
				r.Delete("/{tagID}", a.handleDeleteTag)
				r.Get("/{tagID}/resources", a.handleListTagResources)
				r.Post("/attach", a.handleAttachTag)
				r.Delete("/{tagID}/attach", a.handleDetachTag)
			})
			r.Post("/resources/move", a.handleMoveResource)
			r.Get("/resource-tags", a.handleListResourceTags)
		})
	})

	a.mountWebOrRoot(r)

	return r
}

func timeoutExceptSSE(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/logs/stream") ||
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

const maxJSONBody = 1 << 20 // 1 MiB

func handleHealthStatus(a *API, ctx context.Context) (int, map[string]any) {
	body := map[string]any{"status": "ok", "service": "dockfin"}
	if a == nil || a.Store == nil || a.Store.Pool == nil {
		return http.StatusOK, body
	}
	if ctx == nil {
		ctx = context.Background()
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := a.Store.Pool.Ping(pingCtx); err != nil {
		body["status"] = "unhealthy"
		body["error"] = "database unavailable"
		return http.StatusServiceUnavailable, body
	}
	return http.StatusOK, body
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	code, body := handleHealthStatus(a, r.Context())
	writeJSON(w, code, body)
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
	if r.Body == nil {
		return io.EOF
	}
	limited := http.MaxBytesReader(nil, r.Body, maxJSONBody)
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// decodeJSONOptional treats an empty body as success; malformed JSON is still an error.
func decodeJSONOptional(r *http.Request, v any) error {
	err := decodeJSON(r, v)
	if err == nil || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := sessionToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		// Prefer session cookie / session bearer; fall back to API tokens (dfin_…).
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
		ctx = withAPIAbilities(ctx, apiTok.Abilities)
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

func isTeamAdmin(r *http.Request) bool {
	role, _ := r.Context().Value(ctxRole).(string)
	return role == "owner" || role == "admin"
}

// requireAdmin restricts the route to team owners and admins.
func (a *API) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isTeamAdmin(r) {
			writeError(w, http.StatusForbidden, "admin or owner role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) corsAllowOrigin(r *http.Request, origin string) bool {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return true
	}
	if a.Cfg != nil {
		for _, o := range a.Cfg.CORSOrigins {
			o = strings.TrimRight(strings.TrimSpace(o), "/")
			// Never reflect arbitrary Origin when credentials are enabled.
			if o == "*" {
				continue
			}
			if strings.EqualFold(o, origin) {
				return true
			}
		}
	}
	// Settings → Domain (custom panel URL) is a valid browser origin.
	if a.Store != nil {
		if st, err := a.Store.GetInstanceSettings(r.Context()); err == nil {
			u := strings.TrimRight(strings.TrimSpace(st.PublicURL), "/")
			if u != "" && strings.EqualFold(u, origin) {
				return true
			}
		}
	}
	return false
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
	// Prefer Authorization Bearer over the session cookie so API clients
	// (and deploy webhooks) are not shadowed by a stale browser cookie.
	if t := bearerAuthToken(r); t != "" {
		return t
	}
	if c, err := r.Cookie("dockfin_session"); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}

func bearerAuthToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, cfg *config.Config, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "dockfin_session",
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecureForRequest(r, cfg),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     "dockfin_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecureForRequest(r, cfg),
	})
}

// cookieSecureForRequest uses TLS termination on this process or explicit config.
// X-Forwarded-Proto is intentionally ignored: with chi RealIP, clients can spoof
// X-Forwarded-For to a private IP and force Secure cookies (breaking HTTP logins).
func cookieSecureForRequest(r *http.Request, cfg *config.Config) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	return cfg != nil && cfg.CookieSecure
}

func mapStoreErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, store.ErrNotEmpty):
		writeError(w, http.StatusConflict, "has resources defined, please delete them first")
	case errors.Is(err, store.ErrConflict):
		msg := err.Error()
		if msg == "" || msg == store.ErrConflict.Error() {
			msg = "conflict"
		}
		writeError(w, http.StatusConflict, msg)
	case errors.Is(err, store.ErrEnvLocked):
		writeError(w, http.StatusConflict, "environment variable is locked")
	case errors.Is(err, store.ErrUnauthorized):
		writeError(w, http.StatusForbidden, "forbidden")
	default:
		slog.Error("request error", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
