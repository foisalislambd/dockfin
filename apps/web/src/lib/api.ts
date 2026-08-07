const API_BASE = ''

export type User = { id: string; email: string; name: string }
export type Team = { id: string; name: string; personal: boolean; role?: string }

export type DeleteResourceBody = {
  confirmation_name?: string
  password?: string
  delete_volumes?: boolean
  delete_configurations?: boolean
  delete_networks?: boolean
  docker_cleanup?: boolean
}

export const LAST_ENV_KEY = 'dockfin:last_environment_id'

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  if (!headers.has('Content-Type') && init?.body) {
    headers.set('Content-Type', 'application/json')
  }
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: 'include',
    headers,
  })
  if (!res.ok) {
    let msg = res.statusText
    try {
      const body = await res.json()
      msg = body.error || msg
    } catch {
      /* ignore */
    }
    // Don't emit for login/register/me — AuthProvider owns bootstrap; me 401 is expected when logged out
    if (
      res.status === 401 &&
      !path.includes('/auth/login') &&
      !path.includes('/auth/register') &&
      !path.includes('/auth/me')
    ) {
      window.dispatchEvent(new CustomEvent('dockfin:unauthorized'))
    }
    throw new ApiError(res.status, msg)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export type CreateServerBody = {
  name: string
  ip: string
  port?: number
  user_name?: string
  private_key_id?: string
  proxy_type?: string
  description?: string
}

export const api = {
  health: () => request<{ status: string }>('/health'),
  version: () => request<{ version: string; name: string; license?: string }>('/api/v1/version'),
  me: () => request<{ user: User; team: Team; teams: Team[] }>('/api/v1/auth/me'),
  login: (email: string, password: string) =>
    request<{ user: User; team: Team; token: string }>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),
  register: (name: string, email: string, password: string) =>
    request<{
      user: User
      team: Team
      token: string
      server?: Server
      bootstrap?: {
        server: Server
        public_ip?: string
        validated?: boolean
        proxy_started?: boolean
        message?: string
      }
      bootstrap_error?: string
    }>('/api/v1/auth/register', {
      method: 'POST',
      body: JSON.stringify({ name, email, password }),
    }),
  bootstrapSelf: (start_proxy = true) =>
    request<{
      server: Server
      private_key_id: string
      public_ip: string
      validated: boolean
      proxy_started: boolean
      message?: string
    }>('/api/v1/servers/bootstrap-self', {
      method: 'POST',
      body: JSON.stringify({ start_proxy }),
    }),
  logout: () => request('/api/v1/auth/logout', { method: 'POST' }),
  switchTeam: (team_id: string) =>
    request<{ status: string }>('/api/v1/auth/switch-team', {
      method: 'POST',
      body: JSON.stringify({ team_id }),
    }),
  teams: () => request<{ teams: Team[] }>('/api/v1/teams'),

  servers: () => request<{ servers: Server[] }>('/api/v1/servers'),
  getServer: (id: string) => request<Server>(`/api/v1/servers/${id}`),
  createServer: (body: CreateServerBody) =>
    request<Server>('/api/v1/servers', { method: 'POST', body: JSON.stringify(body) }),
  deleteServer: (id: string) =>
    request<{ status: string }>(`/api/v1/servers/${id}`, { method: 'DELETE' }),
  validateServer: (id: string) =>
    request(`/api/v1/servers/${id}/validate`, { method: 'POST' }),
  startProxy: (id: string) =>
    request(`/api/v1/servers/${id}/proxy/start`, { method: 'POST' }),
  stopProxy: (id: string) =>
    request(`/api/v1/servers/${id}/proxy/stop`, { method: 'POST' }),

  keys: () => request<{ private_keys: Key[] }>('/api/v1/private-keys'),
  getKey: (id: string) => request<Key>(`/api/v1/private-keys/${id}`),
  createKey: (name: string, private_key: string, description = '') =>
    request<Key>('/api/v1/private-keys', {
      method: 'POST',
      body: JSON.stringify({ name, private_key, description }),
    }),
  generateKey: (type: 'ed25519' | 'rsa' = 'ed25519', name = '', description = '') =>
    request<Key>('/api/v1/private-keys/generate', {
      method: 'POST',
      body: JSON.stringify({ type, name, description }),
    }),
  updateKey: (id: string, name: string, description = '') =>
    request<Key>(`/api/v1/private-keys/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ name, description }),
    }),
  deleteKey: (id: string) =>
    request<{ status: string }>(`/api/v1/private-keys/${id}`, { method: 'DELETE' }),
  cleanupUnusedKeys: () =>
    request<{ status: string; deleted: number }>('/api/v1/private-keys/cleanup-unused', {
      method: 'POST',
    }),

  cloudTokens: () => request<{ cloud_tokens: CloudProviderToken[] }>('/api/v1/cloud-tokens'),
  getCloudToken: (id: string) => request<CloudProviderToken>(`/api/v1/cloud-tokens/${id}`),
  createCloudToken: (body: {
    provider: 'hetzner' | 'digitalocean' | 'vultr'
    name: string
    description?: string
    token: string
    validate?: boolean
  }) =>
    request<CloudProviderToken>('/api/v1/cloud-tokens', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  updateCloudToken: (
    id: string,
    body: { name: string; description?: string; token?: string },
  ) =>
    request<CloudProviderToken>(`/api/v1/cloud-tokens/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  deleteCloudToken: (id: string) =>
    request<{ status: string }>(`/api/v1/cloud-tokens/${id}`, { method: 'DELETE' }),
  validateCloudToken: (id: string) =>
    request<{ status: string }>(`/api/v1/cloud-tokens/${id}/validate`, { method: 'POST' }),

  cloudInitScripts: () =>
    request<{ cloud_init_scripts: CloudInitScript[] }>('/api/v1/cloud-init-scripts'),
  getCloudInitScript: (id: string) =>
    request<CloudInitScript>(`/api/v1/cloud-init-scripts/${id}`),
  createCloudInitScript: (name: string, script: string) =>
    request<CloudInitScript>('/api/v1/cloud-init-scripts', {
      method: 'POST',
      body: JSON.stringify({ name, script }),
    }),
  updateCloudInitScript: (id: string, name: string, script: string) =>
    request<CloudInitScript>(`/api/v1/cloud-init-scripts/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ name, script }),
    }),
  deleteCloudInitScript: (id: string) =>
    request<{ status: string }>(`/api/v1/cloud-init-scripts/${id}`, { method: 'DELETE' }),

  projects: () => request<{ projects: Project[] }>('/api/v1/projects'),
  getProject: (id: string) => request<Project>(`/api/v1/projects/${id}`),
  createProject: (name: string, description = '') =>
    request<{ project: Project; environment: Environment }>('/api/v1/projects', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    }),
  updateProject: (id: string, name: string, description = '') =>
    request<Project>(`/api/v1/projects/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ name, description }),
    }),
  deleteProject: (id: string, body?: DeleteResourceBody) =>
    request<{ status: string }>(`/api/v1/projects/${id}`, {
      method: 'DELETE',
      body: body ? JSON.stringify(body) : undefined,
    }),
  environments: (projectId: string) =>
    request<{ environments: Environment[] }>(`/api/v1/projects/${projectId}/environments`),
  getEnvironment: (projectId: string, envId: string) =>
    request<Environment>(`/api/v1/projects/${projectId}/environments/${envId}`),
  getEnvironmentById: (envId: string) => request<Environment>(`/api/v1/environments/${envId}`),
  moveResource: (body: {
    resource_type: 'application' | 'database' | 'service'
    resource_id: string
    environment_id: string
  }) =>
    request<{ status: string; environment_id: string }>('/api/v1/resources/move', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  createEnvironment: (projectId: string, name: string, description = '') =>
    request<Environment>(`/api/v1/projects/${projectId}/environments`, {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    }),
  updateEnvironment: (projectId: string, envId: string, name: string, description = '') =>
    request<Environment>(`/api/v1/projects/${projectId}/environments/${envId}`, {
      method: 'PATCH',
      body: JSON.stringify({ name, description }),
    }),
  deleteEnvironment: (projectId: string, envId: string, body?: DeleteResourceBody) =>
    request<{ status: string }>(`/api/v1/projects/${projectId}/environments/${envId}`, {
      method: 'DELETE',
      body: body ? JSON.stringify(body) : undefined,
    }),
  cloneEnvironment: (projectId: string, envId: string, name: string, description = '') =>
    request<{
      environment: Environment
      applications: number
      databases: number
      services: number
    }>(`/api/v1/projects/${projectId}/environments/${envId}/clone`, {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    }),
  environmentTags: (projectId: string, envId: string) =>
    request<{
      resource_tags: Array<{ resource_type: string; resource_id: string; tags: Tag[] }>
    }>(`/api/v1/projects/${projectId}/environments/${envId}/tags`),

  tags: () => request<{ tags: Tag[] }>('/api/v1/tags'),
  createTag: (name: string, color = '#14b8a6') =>
    request<Tag>('/api/v1/tags', { method: 'POST', body: JSON.stringify({ name, color }) }),
  deleteTag: (id: string) => request<{ status: string }>(`/api/v1/tags/${id}`, { method: 'DELETE' }),
  attachTag: (body: {
    tag_id?: string
    name?: string
    color?: string
    resource_type: string
    resource_id: string
  }) => request<{ tags: Tag[] }>('/api/v1/tags/attach', { method: 'POST', body: JSON.stringify(body) }),
  detachTag: (tagId: string, resource_type: string, resource_id: string) =>
    request<{ status: string }>(
      `/api/v1/tags/${tagId}/attach?resource_type=${encodeURIComponent(resource_type)}&resource_id=${resource_id}`,
      { method: 'DELETE' },
    ),

  applications: (environment_id?: string) =>
    request<{ applications: Application[] }>(
      `/api/v1/applications${environment_id ? `?environment_id=${environment_id}` : ''}`,
    ),
  application: (id: string) => request<Application>(`/api/v1/applications/${id}`),
  createApplication: (body: Record<string, unknown>) =>
    request<Application>('/api/v1/applications', { method: 'POST', body: JSON.stringify(body) }),
  deleteApplication: (id: string, body?: DeleteResourceBody) =>
    request<{ status: string }>(`/api/v1/applications/${id}`, {
      method: 'DELETE',
      body: body ? JSON.stringify(body) : undefined,
    }),
  updateApplication: (id: string, body: Record<string, unknown>) =>
    request<Application>(`/api/v1/applications/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  deployApplication: (id: string, force_rebuild = false) =>
    request<Deployment>(`/api/v1/applications/${id}/deploy`, {
      method: 'POST',
      body: JSON.stringify({ force_rebuild }),
    }),
  detectCompose: (body: {
    git_repository: string
    git_branch?: string
    git_source_id?: string
    private_key_id?: string
  }) =>
    request<{ location: string; candidates: string[] }>('/api/v1/applications/detect-compose', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  detectComposeForApp: (id: string, save = true) =>
    request<{ location: string; candidates: string[]; saved?: boolean }>(
      `/api/v1/applications/${id}/detect-compose`,
      { method: 'POST', body: JSON.stringify({ save }) },
    ),
  rollbackApplication: (id: string, force_rebuild = true) =>
    request<Deployment>(`/api/v1/applications/${id}/rollback`, {
      method: 'POST',
      body: JSON.stringify({ force_rebuild }),
    }),
  listPreviews: (appId: string) =>
    request<{ previews: ApplicationPreview[] }>(`/api/v1/applications/${appId}/previews`),
  deletePreview: (appId: string, prId: number) =>
    request<{ status: string }>(`/api/v1/applications/${appId}/previews/${prId}`, {
      method: 'DELETE',
    }),
  setWebhookSecret: (id: string, secret = '') =>
    request<{ secret: string }>(`/api/v1/applications/${id}/webhook-secret`, {
      method: 'POST',
      body: JSON.stringify({ secret }),
    }),
  deployments: (appId: string) =>
    request<{ deployments: Deployment[] }>(`/api/v1/applications/${appId}/deployments`),
  getDeployment: (id: string) => request<Deployment>(`/api/v1/deployments/${id}`),
  cancelDeployment: (id: string) =>
    request<{ status: string }>(`/api/v1/deployments/${id}/cancel`, { method: 'POST' }),

  envVars: (resourceType: string, resourceId: string, reveal = false) =>
    request<{ environment_variables: EnvVar[] }>(
      `/api/v1/env-vars?resource_type=${encodeURIComponent(resourceType)}&resource_id=${resourceId}${reveal ? '&reveal=1' : ''}`,
    ),
  upsertEnvVar: (body: {
    resource_type: string
    resource_id: string
    key: string
    value: string
    is_runtime?: boolean
    is_buildtime?: boolean
    is_literal?: boolean
    is_multiline?: boolean
    is_locked?: boolean
    comment?: string
    keep_value?: boolean
  }) => request<EnvVar>('/api/v1/env-vars', { method: 'POST', body: JSON.stringify(body) }),
  lockEnvVar: (id: string, locked: boolean) =>
    request<EnvVar>(`/api/v1/env-vars/${id}/lock`, {
      method: 'POST',
      body: JSON.stringify({ locked }),
    }),
  deleteEnvVar: (id: string) =>
    request(`/api/v1/env-vars/${id}`, { method: 'DELETE' }),
  serverExec: (id: string, command: string) =>
    request<{ stdout?: string; stderr?: string; output?: string; error?: string; exit_error?: boolean }>(
      `/api/v1/servers/${id}/exec`,
      { method: 'POST', body: JSON.stringify({ command }) },
    ),

  sharedEnvVars: (scope_type = 'team', scope_id?: string, reveal = false) =>
    request<{ shared_environment_variables: SharedEnvVar[] }>(
      `/api/v1/shared-env-vars?scope_type=${encodeURIComponent(scope_type)}${scope_id ? `&scope_id=${scope_id}` : ''}${reveal ? '&reveal=1' : ''}`,
    ),
  upsertSharedEnvVar: (body: {
    scope_type: string
    scope_id?: string
    key: string
    value: string
    is_literal?: boolean
  }) =>
    request<SharedEnvVar>('/api/v1/shared-env-vars', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  databases: (environment_id?: string) =>
    request<{ databases: Database[] }>(
      `/api/v1/databases${environment_id ? `?environment_id=${environment_id}` : ''}`,
    ),
  getDatabase: (id: string) => request<Database>(`/api/v1/databases/${id}`),
  createDatabase: (body: Record<string, unknown>) =>
    request<{ database: Database; password: string }>('/api/v1/databases', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  startDatabase: (id: string) =>
    request(`/api/v1/databases/${id}/start`, { method: 'POST' }),
  stopDatabase: (id: string) =>
    request(`/api/v1/databases/${id}/stop`, { method: 'POST' }),
  deleteDatabase: (id: string, body?: DeleteResourceBody) =>
    request<{ status: string }>(`/api/v1/databases/${id}`, {
      method: 'DELETE',
      body: body ? JSON.stringify(body) : undefined,
    }),

  services: (environment_id?: string) =>
    request<{ services: Service[] }>(
      `/api/v1/services${environment_id ? `?environment_id=${environment_id}` : ''}`,
    ),
  getService: (id: string) => request<Service>(`/api/v1/services/${id}`),
  updateService: (id: string, body: { name?: string; description?: string }) =>
    request<Service>(`/api/v1/services/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  stopService: (id: string) =>
    request<Service>(`/api/v1/services/${id}/stop`, { method: 'POST' }),
  restartService: (id: string) =>
    request<Service>(`/api/v1/services/${id}/restart`, { method: 'POST' }),
  deleteService: (id: string, body?: DeleteResourceBody) =>
    request<{ status: string }>(`/api/v1/services/${id}`, {
      method: 'DELETE',
      body: body ? JSON.stringify(body) : undefined,
    }),
  templates: () => request<{ templates: Template[] }>('/api/v1/services/templates'),
  createService: (body: Record<string, unknown>) =>
    request<Service>('/api/v1/services', { method: 'POST', body: JSON.stringify(body) }),
  deployService: (id: string) =>
    request(`/api/v1/services/${id}/deploy`, { method: 'POST' }),
  serviceWebhook: (id: string) =>
    request<{
      uuid: string
      has_secret: boolean
      deploy_url: string
      deploy_webhook_url: string
    }>(`/api/v1/services/${id}/webhook`),
  setServiceWebhookSecret: (id: string, secret = '') =>
    request<{ secret: string }>(`/api/v1/services/${id}/webhook-secret`, {
      method: 'POST',
      body: JSON.stringify({ secret }),
    }),
  /** Live SSE deploy — calls onLine for each event; resolves with final status. */
  deployServiceStream: async (
    id: string,
    onLine: (ev: { stage: string; line: string; status?: string }) => void,
    signal?: AbortSignal,
  ): Promise<{ status: string }> => {
    const res = await fetch(`${API_BASE}/api/v1/services/${id}/deploy?stream=1`, {
      method: 'POST',
      credentials: 'include',
      headers: { Accept: 'text/event-stream' },
      signal,
    })
    if (!res.ok || !res.body) {
      let msg = res.statusText
      try {
        const body = await res.json()
        msg = body.error || msg
      } catch {
        /* ignore */
      }
      throw new ApiError(res.status, msg)
    }
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let finalStatus = 'running'
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const parts = buffer.split('\n\n')
      buffer = parts.pop() || ''
      for (const chunk of parts) {
        const dataLine = chunk
          .split('\n')
          .map((l) => l.trim())
          .find((l) => l.startsWith('data:'))
        if (!dataLine) continue
        const raw = dataLine.replace(/^data:\s?/, '')
        try {
          const ev = JSON.parse(raw) as { stage?: string; line?: string; status?: string }
          onLine({
            stage: ev.stage || '',
            line: ev.line || raw,
            status: ev.status,
          })
          if (ev.status === 'failed') {
            finalStatus = 'failed'
            throw new ApiError(500, ev.line || 'Deploy failed')
          }
          if (ev.status === 'running' || ev.stage === 'done') {
            finalStatus = 'running'
          }
        } catch (e) {
          if (e instanceof ApiError) throw e
          onLine({ stage: '', line: raw })
        }
      }
    }
    return { status: finalStatus }
  },

  destinations: () => request<{ destinations: Destination[] }>('/api/v1/destinations'),

  s3Storages: () => request<{ s3_storages: S3Storage[] }>('/api/v1/s3-storages'),
  createS3Storage: (body: {
    name: string
    endpoint: string
    bucket: string
    region?: string
    access_key: string
    secret_key: string
    path_style?: boolean
  }) =>
    request<S3Storage>('/api/v1/s3-storages', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  deleteS3Storage: (id: string) =>
    request<{ status: string }>(`/api/v1/s3-storages/${id}`, { method: 'DELETE' }),

  notifications: () => request<{ notifications: NotificationSetting[] }>('/api/v1/notifications'),
  upsertNotification: (channel: string, body: { enabled: boolean; config: unknown; events?: string[] }) =>
    request<{ status: string }>(`/api/v1/notifications/${channel}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  testNotification: (channel: string, body?: { email?: string }) =>
    request<{ status: string }>(`/api/v1/notifications/${channel}/test`, {
      method: 'POST',
      body: JSON.stringify(body || {}),
    }),

  instanceSettings: () => request<{ settings: InstanceSettings }>('/api/v1/settings'),
  patchInstanceSettings: (body: Partial<InstanceSettingsPatch>) =>
    request<{ settings: InstanceSettings }>('/api/v1/settings', {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  oauthSettings: () => request<{ oauth_settings: OauthSetting[] }>('/api/v1/settings/oauth'),
  patchOauthSetting: (provider: string, body: Partial<OauthSettingPatch>) =>
    request<{ oauth_setting: OauthSetting }>(`/api/v1/settings/oauth/${provider}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  instanceBackup: () =>
    request<{
      backup: InstanceBackupConfig
      runtime: {
        container: string
        detected_container: string
        data_dir: string
        backup_dir: string
        db_password_set: boolean
      }
      executions: BackupExecution[]
    }>('/api/v1/settings/backup'),
  configureInstanceBackup: () =>
    request<{ backup: InstanceBackupConfig; status: string }>('/api/v1/settings/backup/configure', {
      method: 'POST',
    }),
  patchInstanceBackup: (body: Partial<InstanceBackupPatch>) =>
    request<{ backup: InstanceBackupConfig }>('/api/v1/settings/backup', {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  runInstanceBackup: () =>
    request<{ execution: BackupExecution }>('/api/v1/settings/backup/run', { method: 'POST' }),
  instanceBackupExecutions: () =>
    request<{ executions: BackupExecution[] }>('/api/v1/settings/backup/executions'),

  teamMembers: () => request<{ members: TeamMember[] }>('/api/v1/team/members'),
  removeTeamMember: (userId: string) =>
    request<{ status: string }>(`/api/v1/team/members/${userId}`, { method: 'DELETE' }),
  teamInvitations: () => request<{ invitations: TeamInvitation[] }>('/api/v1/team/invitations'),
  createInvitation: (email: string, role = 'member') =>
    request<TeamInvitation>('/api/v1/team/invitations', {
      method: 'POST',
      body: JSON.stringify({ email, role }),
    }),
  deleteInvitation: (id: string) =>
    request<{ status: string }>(`/api/v1/team/invitations/${id}`, { method: 'DELETE' }),
  acceptInvitation: (token: string) =>
    request<{ status: string; team: Team }>('/api/v1/team/invitations/accept', {
      method: 'POST',
      body: JSON.stringify({ token }),
    }),

  apiTokens: () => request<{ api_tokens: ApiToken[] }>('/api/v1/api-tokens'),
  createApiToken: (name: string, abilities: string[] = ['*'], expires_in_days?: number) =>
    request<{ api_token: ApiToken; token: string }>('/api/v1/api-tokens', {
      method: 'POST',
      body: JSON.stringify({ name, abilities, expires_in_days }),
    }),
  deleteApiToken: (id: string) =>
    request<{ status: string }>(`/api/v1/api-tokens/${id}`, { method: 'DELETE' }),

  scheduledBackups: () => request<{ scheduled_backups: ScheduledBackup[] }>('/api/v1/scheduled-backups'),
  createScheduledBackup: (body: {
    resource_type: string
    resource_id: string
    s3_storage_id?: string
    frequency?: string
    retention?: number
  }) =>
    request<ScheduledBackup>('/api/v1/scheduled-backups', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  databaseBackups: (dbId: string) =>
    request<{ backup_executions: BackupExecution[] }>(`/api/v1/databases/${dbId}/backups`),
  runDatabaseBackup: (dbId: string) =>
    request<BackupExecution>(`/api/v1/databases/${dbId}/backups`, { method: 'POST' }),
  restoreDatabaseBackup: (dbId: string, body: { execution_id?: string; filename?: string }) =>
    request<{ status: string; filename: string }>(`/api/v1/databases/${dbId}/backups/restore`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  scheduledTasks: (params?: { resource_type?: string; resource_id?: string }) => {
    const q = new URLSearchParams()
    if (params?.resource_type) q.set('resource_type', params.resource_type)
    if (params?.resource_id) q.set('resource_id', params.resource_id)
    const qs = q.toString()
    return request<{ scheduled_tasks: ScheduledTask[] }>(
      `/api/v1/scheduled-tasks${qs ? `?${qs}` : ''}`,
    )
  },
  createScheduledTask: (body: {
    resource_type?: string
    resource_id: string
    name: string
    command: string
    frequency: string
    container_name?: string
  }) =>
    request<ScheduledTask>('/api/v1/scheduled-tasks', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  patchScheduledTask: (
    id: string,
    body: {
      name?: string
      command?: string
      frequency?: string
      container_name?: string
      enabled?: boolean
    },
  ) =>
    request<ScheduledTask>(`/api/v1/scheduled-tasks/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  deleteScheduledTask: (id: string) =>
    request<{ status: string }>(`/api/v1/scheduled-tasks/${id}`, { method: 'DELETE' }),
  runScheduledTask: (id: string) =>
    request<{ execution_id: string; status: string }>(`/api/v1/scheduled-tasks/${id}/run`, {
      method: 'POST',
    }),
  scheduledTaskExecutions: (id: string) =>
    request<{ executions: ScheduledTaskExecution[] }>(
      `/api/v1/scheduled-tasks/${id}/executions`,
    ),
  serverMetrics: (id: string, limit = 60) =>
    request<{ metrics: ServerMetric[] }>(`/api/v1/servers/${id}/metrics?limit=${limit}`),

  patchServerSettings: (
    id: string,
    body: {
      is_build_server?: boolean
      is_swarm_manager?: boolean
      wildcard_domain?: string
      magic_domain?: string
      public_ip?: string
    },
  ) =>
    request<{ status: string }>(`/api/v1/servers/${id}/settings`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  generateDomain: (body: {
    name: string
    destination_id?: string
    server_id?: string
    resource_id?: string
  }) =>
    request<{ fqdn: string; url: string }>('/api/v1/domains/generate', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  createDestination: (
    serverId: string,
    body: { name: string; kind?: string; network?: string },
  ) =>
    request<Destination>(`/api/v1/servers/${serverId}/destinations`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  createTerminal: (serverId: string, container?: string) =>
    request<{ session_id: string }>(`/api/v1/servers/${serverId}/terminal`, {
      method: 'POST',
      body: JSON.stringify({ container: container || '' }),
    }),

  gitSources: () => request<{ git_sources: GitSource[] }>('/api/v1/git-sources'),
  getGitSource: (id: string) => request<GitSource>(`/api/v1/git-sources/${id}`),
  createGitSource: (body: {
    name: string
    provider?: string
    organization?: string
    app_id?: string
    slug?: string
    private_key?: string
    client_id?: string
    client_secret?: string
    webhook_secret?: string
    html_url?: string
    api_url?: string
    custom_user?: string
    custom_port?: number
  }) =>
    request<GitSource>('/api/v1/git-sources', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  updateGitSource: (id: string, body: Record<string, unknown>) =>
    request<GitSource>(`/api/v1/git-sources/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  deleteGitSource: (id: string) =>
    request<{ status: string }>(`/api/v1/git-sources/${id}`, { method: 'DELETE' }),
  gitSourceInstallURL: (id: string) =>
    request<{ install_url: string; state: string }>(`/api/v1/git-sources/${id}/install-url`),
  gitSourceManifest: (
    id: string,
    opts?: { endpoint?: string; preview?: boolean },
  ) => {
    const q = new URLSearchParams()
    if (opts?.endpoint) q.set('endpoint', opts.endpoint)
    if (opts?.preview === false) q.set('preview', '0')
    const qs = q.toString()
    return request<{
      state: string
      action_url: string
      manifest: Record<string, unknown>
      endpoint: string
    }>(`/api/v1/git-sources/${id}/manifest${qs ? `?${qs}` : ''}`)
  },
  gitSourceRepositories: (id: string, page = 1) =>
    request<{ repositories: Record<string, unknown>[] }>(
      `/api/v1/git-sources/${id}/repositories?page=${page}`,
    ),
  gitSourceBranches: (id: string, owner: string, repo: string) =>
    request<{ branches: string[] }>(
      `/api/v1/git-sources/${id}/repositories/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/branches`,
    ),
  gitSourceApplications: (id: string) =>
    request<{
      applications: Array<{
        id: string
        name: string
        environment_id: string
        project_id: string
        project_name: string
        environment_name: string
        build_pack: string
      }>
    }>(`/api/v1/git-sources/${id}/applications`),
}

export type Server = {
  id: string
  name: string
  ip: string
  port: number
  user_name: string
  is_reachable: boolean
  is_usable: boolean
  docker_version: string
  proxy_type: string
  proxy_status: string
  description?: string
  is_build_server?: boolean
  is_swarm_manager?: boolean
  wildcard_domain?: string
  magic_domain?: string
  public_ip?: string
}
export type GitSource = {
  id: string
  name: string
  provider: string
  organization?: string
  app_id: string
  installation_id?: string
  client_id?: string
  html_url: string
  api_url: string
  custom_user?: string
  custom_port?: number
  is_public: boolean
  is_system_wide?: boolean
  has_private_key?: boolean
  configured?: boolean
  installed?: boolean
  applications_count?: number
  created_at: string
}
export type Key = {
  id: string
  name: string
  description?: string
  fingerprint: string
  public_key: string
  in_use?: boolean
  created_at?: string
}
export type CloudProviderToken = {
  id: string
  provider: 'hetzner' | 'digitalocean' | 'vultr' | string
  name: string
  description: string
  created_at: string
  updated_at?: string
}
export type CloudInitScript = {
  id: string
  name: string
  script?: string
  created_at: string
  updated_at?: string
}
export type Project = {
  id: string
  name: string
  description: string
  is_empty?: boolean
  created_at?: string
}
export type Environment = {
  id: string
  name: string
  project_id: string
  description?: string
  is_empty?: boolean
}
export type Tag = {
  id: string
  team_id: string
  name: string
  color: string
  created_at: string
}
export type Service = {
  id: string
  name: string
  service_type: string
  status: string
  environment_id: string
  description?: string
  destination_id?: string | null
  server_id?: string | null
  fqdn?: string
  docker_compose?: string
  docker_compose_raw?: string
  links?: { label: string; url: string }[]
  units?: ServiceUnit[]
  volumes?: Array<{
    service: string
    name: string
    mount_path: string
    host_path?: string
    type: string
  }>
}
export type ServiceUnit = {
  name: string
  image: string
  status?: string
  links?: { label: string; url: string }[]
}
export type Application = {
  id: string
  name: string
  status: string
  build_pack: string
  fqdn: string
  environment_id: string
  description?: string
  git_repository?: string
  git_branch?: string
  ports_exposes?: string
  docker_compose_location?: string
  compose_prepare?: boolean
  docker_registry_image_name?: string
  docker_registry_image_tag?: string
  destination_id?: string | null
  git_source_id?: string | null
  private_key_id?: string | null
  is_build_server_enabled?: boolean
  is_force_https?: boolean
  is_preview_enabled?: boolean
  health_check_enabled?: boolean
  health_check_path?: string
  health_check_port?: number | null
  health_check_method?: string
  health_check_return_code?: number
  health_check_interval?: number
  health_check_timeout?: number
  health_check_retries?: number
  limits_memory?: string
  limits_cpus?: string
  links?: { label: string; url: string }[]
}
export type ApplicationPreview = {
  id: string
  application_id: string
  pull_request_id: number
  pull_request_title: string
  git_branch: string
  fqdn: string
  status: string
  created_at: string
}
export type Deployment = {
  id: string
  status: string
  current_stage: string
  created_at: string
  error_message: string
  commit_sha?: string
  commit_message?: string
}
export type EnvVar = {
  id: string
  key: string
  value?: string
  is_runtime: boolean
  is_buildtime: boolean
  is_literal: boolean
  is_multiline?: boolean
  is_locked?: boolean
  comment: string
}
export type SharedEnvVar = {
  id: string
  scope_type: string
  scope_id?: string | null
  key: string
  value?: string
  is_literal: boolean
}
export type Database = {
  id: string
  name: string
  engine: string
  status: string
  environment_id: string
  description?: string
  image?: string
  is_public?: boolean
  public_port?: number | null
  destination_id?: string | null
}
export type Template = {
  type: string
  name: string
  description: string
  category?: string
  logo?: string
}
export type Destination = {
  id: string
  name: string
  server_id: string
  network: string
  kind?: string
}
export type S3Storage = {
  id: string
  name: string
  endpoint: string
  bucket: string
  region: string
  path_style: boolean
  created_at: string
}
export type NotificationSetting = {
  id?: string
  channel: string
  enabled: boolean
  events: string[]
  config?: Record<string, unknown>
  created_at?: string
}
export type TeamMember = {
  user_id: string
  email: string
  name: string
  role: string
  created_at: string
}
export type TeamInvitation = {
  id: string
  team_id: string
  email: string
  role: string
  token?: string
  invited_by: string
  expires_at: string
  created_at: string
}
export type ApiToken = {
  id: string
  name: string
  token_prefix: string
  abilities: string[]
  last_used_at?: string | null
  expires_at?: string | null
  created_at: string
}
export type ScheduledBackup = {
  id: string
  resource_type: string
  resource_id: string
  s3_storage_id?: string | null
  frequency: string
  enabled: boolean
  retention: number
  created_at: string
}
export type BackupExecution = {
  id: string
  resource_type: string
  resource_id: string
  status: string
  size_bytes: number
  filename: string
  error_message: string
  s3_uploaded?: boolean
  s3_key?: string
  started_at: string
  finished_at?: string | null
}
export type ScheduledTask = {
  id: string
  resource_type: string
  resource_id: string
  name: string
  command: string
  frequency: string
  container_name?: string
  enabled: boolean
  created_at: string
  updated_at?: string
}
export type ScheduledTaskExecution = {
  id: string
  status: string
  output: string
  started_at: string
  finished_at?: string | null
}
export type InstanceSettings = {
  id: number
  public_url: string
  instance_name: string
  instance_timezone: string
  public_ipv4: string
  public_ipv6: string
  is_registration_enabled: boolean
  do_not_track: boolean
  is_dns_validation_enabled: boolean
  custom_dns_servers: string
  is_api_enabled: boolean
  allowed_ips: string
  webhook_allowed_internal_hosts: string
  webhook_allow_localhost: boolean
  is_mcp_server_enabled: boolean
  disable_two_step_confirmation: boolean
  is_sponsorship_popup_enabled: boolean
  update_channel: string
  is_auto_update_enabled: boolean
  auto_update_frequency: string
  update_check_frequency: string
  docker_registry_url: string
  smtp_enabled: boolean
  smtp_from_name: string
  smtp_from_address: string
  smtp_host: string
  smtp_port: number
  smtp_encryption: string
  smtp_username?: string
  smtp_password_set: boolean
  smtp_timeout?: number | null
  resend_enabled: boolean
  resend_api_key_set: boolean
  updated_at: string
}
export type InstanceSettingsPatch = {
  public_url?: string
  instance_name?: string
  instance_timezone?: string
  public_ipv4?: string
  public_ipv6?: string
  is_registration_enabled?: boolean
  do_not_track?: boolean
  is_dns_validation_enabled?: boolean
  custom_dns_servers?: string
  is_api_enabled?: boolean
  allowed_ips?: string
  webhook_allowed_internal_hosts?: string
  webhook_allow_localhost?: boolean
  is_mcp_server_enabled?: boolean
  disable_two_step_confirmation?: boolean
  is_sponsorship_popup_enabled?: boolean
  update_channel?: string
  is_auto_update_enabled?: boolean
  auto_update_frequency?: string
  update_check_frequency?: string
  docker_registry_url?: string
  smtp_enabled?: boolean
  smtp_from_name?: string
  smtp_from_address?: string
  smtp_host?: string
  smtp_port?: number
  smtp_encryption?: string
  smtp_username?: string
  smtp_password?: string
  smtp_timeout?: number | null
  resend_enabled?: boolean
  resend_api_key?: string
}
export type OauthSetting = {
  id: string
  provider: string
  enabled: boolean
  client_id: string
  client_secret_set: boolean
  redirect_uri: string
  tenant: string
  base_url: string
  updated_at: string
}
export type OauthSettingPatch = {
  enabled?: boolean
  client_id?: string
  client_secret?: string
  redirect_uri?: string
  tenant?: string
  base_url?: string
}
export type InstanceBackupConfig = {
  configured: boolean
  enabled: boolean
  frequency: string
  retention: number
  description: string
  db_user: string
  db_name: string
  container: string
  name: string
  uuid: string
}
export type InstanceBackupPatch = {
  enabled?: boolean
  frequency?: string
  retention?: number
  description?: string
  container?: string
  db_user?: string
  db_name?: string
}
export type ServerMetric = {
  cpu_percent: number
  memory_used_bytes: number
  memory_total_bytes: number
  disk_used_bytes: number
  disk_total_bytes: number
  recorded_at: string
}

/** Fetch all environments across projects for dropdowns. */
export async function fetchAllEnvironments(): Promise<
  { id: string; name: string; project_id: string; project_name: string }[]
> {
  const { projects } = await api.projects()
  const nested = await Promise.all(
    (projects || []).map(async (p) => {
      const { environments } = await api.environments(p.id)
      return (environments || []).map((e) => ({
        id: e.id,
        name: e.name,
        project_id: p.id,
        project_name: p.name,
      }))
    }),
  )
  return nested.flat()
}
