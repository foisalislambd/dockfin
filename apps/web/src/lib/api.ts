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
  servers: () => request<{ servers: Server[] }>('/api/v1/servers'),
  createServer: (body: CreateServerBody) =>
    request<Server>('/api/v1/servers', { method: 'POST', body: JSON.stringify(body) }),
  validateServer: (id: string) =>
    request(`/api/v1/servers/${id}/validate`, { method: 'POST' }),
  startProxy: (id: string) =>
    request(`/api/v1/servers/${id}/proxy/start`, { method: 'POST' }),
  keys: () => request<{ private_keys: Key[] }>('/api/v1/private-keys'),
  createKey: (name: string, private_key: string) =>
    request<Key>('/api/v1/private-keys', {
      method: 'POST',
      body: JSON.stringify({ name, private_key }),
    }),
  projects: () => request<{ projects: Project[] }>('/api/v1/projects'),
  createProject: (name: string, description = '') =>
    request<{ project: Project; environment: Environment }>('/api/v1/projects', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    }),
  environments: (projectId: string) =>
    request<{ environments: Environment[] }>(`/api/v1/projects/${projectId}/environments`),
  applications: (environment_id?: string) =>
    request<{ applications: Application[] }>(
      `/api/v1/applications${environment_id ? `?environment_id=${environment_id}` : ''}`,
    ),
  application: (id: string) => request<Application>(`/api/v1/applications/${id}`),
  createApplication: (body: Record<string, unknown>) =>
    request<Application>('/api/v1/applications', { method: 'POST', body: JSON.stringify(body) }),
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
  deployments: (appId: string) =>
    request<{ deployments: Deployment[] }>(`/api/v1/applications/${appId}/deployments`),
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
  databases: () => request<{ databases: Database[] }>('/api/v1/databases'),
  createDatabase: (body: Record<string, unknown>) =>
    request<{ database: Database; password: string }>('/api/v1/databases', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  startDatabase: (id: string) =>
    request(`/api/v1/databases/${id}/start`, { method: 'POST' }),
  stopDatabase: (id: string) =>
    request(`/api/v1/databases/${id}/stop`, { method: 'POST' }),
  services: () => request<{ services: Service[] }>('/api/v1/services'),
  templates: () => request<{ templates: Template[] }>('/api/v1/services/templates'),
  createService: (body: Record<string, unknown>) =>
    request('/api/v1/services', { method: 'POST', body: JSON.stringify(body) }),
  deployService: (id: string) =>
    request(`/api/v1/services/${id}/deploy`, { method: 'POST' }),
  destinations: () => request<{ destinations: Destination[] }>('/api/v1/destinations'),
  notifications: () => request<{ notifications: NotificationSetting[] }>('/api/v1/notifications'),
  upsertNotification: (channel: string, body: { enabled: boolean; config: unknown; events?: string[] }) =>
    request<{ status: string }>(`/api/v1/notifications/${channel}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
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
}
export type Key = { id: string; name: string; fingerprint: string; public_key: string }
export type Project = { id: string; name: string; description: string }
export type Environment = { id: string; name: string; project_id: string }
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
export type Database = { id: string; name: string; engine: string; status: string }
export type Service = { id: string; name: string; service_type: string; status: string }
export type Template = { type: string; name: string; description: string }
export type Destination = { id: string; name: string; server_id: string; network: string }
export type NotificationSetting = {
  id: string
  channel: string
  enabled: boolean
  events: string[]
  created_at: string
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
