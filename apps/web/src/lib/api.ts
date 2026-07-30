const API_BASE = ''

export type User = { id: string; email: string; name: string }
export type Team = { id: string; name: string; personal: boolean; role?: string }

export const LAST_ENV_KEY = 'goolify:last_environment_id'

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
    if (res.status === 401 && !path.includes('/auth/login') && !path.includes('/auth/register')) {
      window.dispatchEvent(new CustomEvent('goolify:unauthorized'))
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
    request<{ user: User; team: Team; token: string }>('/api/v1/auth/register', {
      method: 'POST',
      body: JSON.stringify({ name, email, password }),
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
  createKey: (name: string, private_key: string) =>
    request<Key>('/api/v1/private-keys', {
      method: 'POST',
      body: JSON.stringify({ name, private_key }),
    }),

  projects: () => request<{ projects: Project[] }>('/api/v1/projects'),
  getProject: (id: string) => request<Project>(`/api/v1/projects/${id}`),
  createProject: (name: string, description = '') =>
    request<{ project: Project; environment: Environment }>('/api/v1/projects', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    }),
  environments: (projectId: string) =>
    request<{ environments: Environment[] }>(`/api/v1/projects/${projectId}/environments`),
  createEnvironment: (projectId: string, name: string, description = '') =>
    request<Environment>(`/api/v1/projects/${projectId}/environments`, {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    }),

  applications: (environment_id?: string) =>
    request<{ applications: Application[] }>(
      `/api/v1/applications${environment_id ? `?environment_id=${environment_id}` : ''}`,
    ),
  application: (id: string) => request<Application>(`/api/v1/applications/${id}`),
  createApplication: (body: Record<string, unknown>) =>
    request<Application>('/api/v1/applications', { method: 'POST', body: JSON.stringify(body) }),
  deleteApplication: (id: string) =>
    request<{ status: string }>(`/api/v1/applications/${id}`, { method: 'DELETE' }),
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
  rollbackApplication: (id: string, force_rebuild = true) =>
    request<Deployment>(`/api/v1/applications/${id}/rollback`, {
      method: 'POST',
      body: JSON.stringify({ force_rebuild }),
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
    comment?: string
  }) => request<EnvVar>('/api/v1/env-vars', { method: 'POST', body: JSON.stringify(body) }),
  deleteEnvVar: (id: string) =>
    request(`/api/v1/env-vars/${id}`, { method: 'DELETE' }),
  serverExec: (id: string, command: string) =>
    request<{ stdout?: string; stderr?: string; output?: string; error?: string; exit_error?: boolean }>(
      `/api/v1/servers/${id}/exec`,
      { method: 'POST', body: JSON.stringify({ command }) },
    ),

  sharedEnvVars: (scope_type = 'team', scope_id?: string) =>
    request<{ shared_environment_variables: SharedEnvVar[] }>(
      `/api/v1/shared-env-vars?scope_type=${encodeURIComponent(scope_type)}${scope_id ? `&scope_id=${scope_id}` : ''}`,
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
  deleteDatabase: (id: string) =>
    request<{ status: string }>(`/api/v1/databases/${id}`, { method: 'DELETE' }),

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
  templates: () => request<{ templates: Template[] }>('/api/v1/services/templates'),
  createService: (body: Record<string, unknown>) =>
    request<Service>('/api/v1/services', { method: 'POST', body: JSON.stringify(body) }),
  deployService: (id: string) =>
    request(`/api/v1/services/${id}/deploy`, { method: 'POST' }),
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
  }) =>
    request<{ id: string }>('/api/v1/scheduled-tasks', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
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
    app_id: string
    slug?: string
    private_key: string
    client_id?: string
    html_url?: string
    api_url?: string
  }) =>
    request<GitSource>('/api/v1/git-sources', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  deleteGitSource: (id: string) =>
    request<{ status: string }>(`/api/v1/git-sources/${id}`, { method: 'DELETE' }),
  gitSourceInstallURL: (id: string) =>
    request<{ install_url: string; state: string }>(`/api/v1/git-sources/${id}/install-url`),
  gitSourceRepositories: (id: string, page = 1) =>
    request<{ repositories: Record<string, unknown>[] }>(
      `/api/v1/git-sources/${id}/repositories?page=${page}`,
    ),
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
  app_id: string
  installation_id?: string
  client_id?: string
  html_url: string
  api_url: string
  is_public: boolean
  created_at: string
}
export type Key = { id: string; name: string; fingerprint: string; public_key: string }
export type Project = { id: string; name: string; description: string }
export type Environment = { id: string; name: string; project_id: string; description?: string }
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
  docker_registry_image_name?: string
  docker_registry_image_tag?: string
  destination_id?: string | null
  git_source_id?: string | null
  is_build_server_enabled?: boolean
  links?: { label: string; url: string }[]
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
  id: string
  channel: string
  enabled: boolean
  events: string[]
  created_at: string
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
  enabled: boolean
  created_at: string
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
