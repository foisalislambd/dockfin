import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { DangerConfirmModal, DangerZoneCard } from '../components/DangerConfirmModal'
import { EnvVarsPanel } from '../components/EnvVarsPanel'
import { LinksMenu, LinksPanel } from '../components/LinksMenu'
import { MoveResourcePanel } from '../components/MoveResourcePanel'
import { ScheduledTasksPanel } from '../components/ScheduledTasksPanel'
import { ServerTerminal } from '../components/Terminal'
import { PageSkeleton } from '../components/ui/Skeleton'
import { Meta, ResourceTabs, TabPanel } from '../components/ui/tabs'
import { api } from '../lib/api'
import { Btn, Input } from './Servers'

const APP_TABS = [
  { id: 'links', label: 'Links' },
  { id: 'configuration', label: 'Configuration' },
  { id: 'health', label: 'Health Checks' },
  { id: 'limits', label: 'Resource Limits' },
  { id: 'environment', label: 'Environment Variables' },
  { id: 'deployments', label: 'Deployments' },
  { id: 'previews', label: 'Previews' },
  { id: 'terminal', label: 'Terminal' },
  { id: 'tasks', label: 'Scheduled Tasks' },
  { id: 'webhooks', label: 'Webhooks' },
  { id: 'operations', label: 'Resource Operations' },
  { id: 'rollback', label: 'Rollback' },
  { id: 'danger', label: 'Danger Zone' },
]

export function ApplicationDetailPage() {
  const { appId, projectId, envId } = useParams({ strict: false }) as {
    appId: string
    projectId?: string
    envId?: string
  }
  const nav = useNavigate()
  const qc = useQueryClient()
  const nested = Boolean(projectId && envId)

  const app = useQuery({ queryKey: ['application', appId], queryFn: () => api.application(appId) })
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const gitSources = useQuery({ queryKey: ['git-sources'], queryFn: api.gitSources })
  const deps = useQuery({
    queryKey: ['deployments', appId],
    queryFn: () => api.deployments(appId),
    refetchInterval: (q) => {
      const list = q.state.data?.deployments || []
      const busy = list.some((d) => d.status === 'queued' || d.status === 'in_progress')
      return busy ? 3000 : false
    },
  })
  const previews = useQuery({
    queryKey: ['previews', appId],
    queryFn: () => api.listPreviews(appId),
    enabled: Boolean(appId),
  })

  const [tab, setTab] = useState('links')
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [cfg, setCfg] = useState({
    name: '',
    description: '',
    fqdn: '',
    git_repository: '',
    git_branch: '',
    ports_exposes: '',
    docker_compose_location: '',
    compose_prepare: true,
    docker_registry_image_name: '',
    docker_registry_image_tag: '',
    destination_id: '',
    git_source_id: '',
    is_build_server_enabled: false,
    is_force_https: true,
    is_preview_enabled: false,
  })
  const [health, setHealth] = useState({
    health_check_enabled: false,
    health_check_path: '/',
    health_check_port: '' as string,
    health_check_method: 'GET',
    health_check_return_code: 200,
    health_check_interval: 5,
    health_check_timeout: 5,
    health_check_retries: 10,
  })
  const [limits, setLimits] = useState({ limits_memory: '', limits_cpus: '' })
  const [webhookSecret, setWebhookSecret] = useState<string | null>(null)

  useEffect(() => {
    setTab('configuration')
    setWebhookSecret(null)
    setCfg({
      name: '',
      description: '',
      fqdn: '',
      git_repository: '',
      git_branch: '',
      ports_exposes: '',
      docker_compose_location: '',
      compose_prepare: true,
      docker_registry_image_name: '',
      docker_registry_image_tag: '',
      destination_id: '',
      git_source_id: '',
      is_build_server_enabled: false,
      is_force_https: true,
      is_preview_enabled: false,
    })
    setHealth({
      health_check_enabled: false,
      health_check_path: '/',
      health_check_port: '',
      health_check_method: 'GET',
      health_check_return_code: 200,
      health_check_interval: 5,
      health_check_timeout: 5,
      health_check_retries: 10,
    })
    setLimits({ limits_memory: '', limits_cpus: '' })
  }, [appId])

  useEffect(() => {
    if (!app.data || app.data.id !== appId) return
    setCfg({
      name: app.data.name || '',
      description: app.data.description || '',
      fqdn: app.data.fqdn || '',
      git_repository: app.data.git_repository || '',
      git_branch: app.data.git_branch || 'main',
      ports_exposes:
        app.data.build_pack === 'dockercompose'
          ? app.data.ports_exposes || ''
          : app.data.ports_exposes || '80',
      docker_compose_location: app.data.docker_compose_location || '',
      compose_prepare: app.data.compose_prepare !== false,
      docker_registry_image_name: app.data.docker_registry_image_name || '',
      docker_registry_image_tag: app.data.docker_registry_image_tag || '',
      destination_id: app.data.destination_id || '',
      git_source_id: app.data.git_source_id || '',
      is_build_server_enabled: Boolean(app.data.is_build_server_enabled),
      is_force_https: app.data.is_force_https !== false,
      is_preview_enabled: Boolean(app.data.is_preview_enabled),
    })
    setHealth({
      health_check_enabled: Boolean(app.data.health_check_enabled),
      health_check_path: app.data.health_check_path || '/',
      health_check_port:
        app.data.health_check_port != null ? String(app.data.health_check_port) : '',
      health_check_method: app.data.health_check_method || 'GET',
      health_check_return_code: app.data.health_check_return_code ?? 200,
      health_check_interval: app.data.health_check_interval ?? 5,
      health_check_timeout: app.data.health_check_timeout ?? 5,
      health_check_retries: app.data.health_check_retries ?? 10,
    })
    setLimits({
      limits_memory: app.data.limits_memory || '',
      limits_cpus: app.data.limits_cpus || '',
    })
  }, [app.data, appId])

  const activeDep = (deps.data?.deployments || []).find(
    (d) => d.status === 'queued' || d.status === 'in_progress',
  )

  const serverId = useMemo(() => {
    const destID = cfg.destination_id || app.data?.destination_id
    if (!destID) return ''
    return (dests.data?.destinations || []).find((d) => d.id === destID)?.server_id || ''
  }, [cfg.destination_id, app.data?.destination_id, dests.data])

  const syncFromApp = (updated: NonNullable<typeof app.data>) => {
    setCfg({
      name: updated.name || '',
      description: updated.description || '',
      fqdn: updated.fqdn || '',
      git_repository: updated.git_repository || '',
      git_branch: updated.git_branch || 'main',
      ports_exposes:
        updated.build_pack === 'dockercompose'
          ? updated.ports_exposes || ''
          : updated.ports_exposes || '80',
      docker_compose_location: updated.docker_compose_location || '',
      compose_prepare: updated.compose_prepare !== false,
      docker_registry_image_name: updated.docker_registry_image_name || '',
      docker_registry_image_tag: updated.docker_registry_image_tag || '',
      destination_id: updated.destination_id || '',
      git_source_id: updated.git_source_id || '',
      is_build_server_enabled: Boolean(updated.is_build_server_enabled),
      is_force_https: updated.is_force_https !== false,
      is_preview_enabled: Boolean(updated.is_preview_enabled),
    })
    setHealth({
      health_check_enabled: Boolean(updated.health_check_enabled),
      health_check_path: updated.health_check_path || '/',
      health_check_port: updated.health_check_port != null ? String(updated.health_check_port) : '',
      health_check_method: updated.health_check_method || 'GET',
      health_check_return_code: updated.health_check_return_code ?? 200,
      health_check_interval: updated.health_check_interval ?? 5,
      health_check_timeout: updated.health_check_timeout ?? 5,
      health_check_retries: updated.health_check_retries ?? 10,
    })
    setLimits({
      limits_memory: updated.limits_memory || '',
      limits_cpus: updated.limits_cpus || '',
    })
  }

  const save = useMutation({
    mutationFn: () => api.updateApplication(appId, cfg),
    onSuccess: (updated) => {
      void qc.invalidateQueries({ queryKey: ['application', appId] })
      syncFromApp(updated)
    },
  })

  const saveHealth = useMutation({
    mutationFn: () =>
      api.updateApplication(appId, {
        health_check_enabled: health.health_check_enabled,
        health_check_path: health.health_check_path,
        health_check_port: health.health_check_port ? Number(health.health_check_port) : 0,
        health_check_method: health.health_check_method,
        health_check_return_code: health.health_check_return_code,
        health_check_interval: health.health_check_interval,
        health_check_timeout: health.health_check_timeout,
        health_check_retries: health.health_check_retries,
      }),
    onSuccess: (updated) => {
      void qc.invalidateQueries({ queryKey: ['application', appId] })
      syncFromApp(updated)
    },
  })

  const saveLimits = useMutation({
    mutationFn: () => api.updateApplication(appId, limits),
    onSuccess: (updated) => {
      void qc.invalidateQueries({ queryKey: ['application', appId] })
      syncFromApp(updated)
    },
  })

  const deploy = useMutation({
    mutationFn: (vars: { force?: boolean } = {}) => api.deployApplication(appId, Boolean(vars.force)),
    onSuccess: (dep) => {
      void qc.invalidateQueries({ queryKey: ['deployments', appId] })
      void qc.invalidateQueries({ queryKey: ['application', appId] })
      if (nested && projectId && envId) {
        void nav({
          to: '/projects/$projectId/environments/$envId/applications/$appId/deployments/$deploymentId',
          params: { projectId, envId, appId, deploymentId: dep.id },
        })
      } else {
        void nav({
          to: '/applications/$appId/deployments/$deploymentId',
          params: { appId, deploymentId: dep.id },
        })
      }
    },
  })

  const remove = useMutation({
    mutationFn: (body: Parameters<typeof api.deleteApplication>[1]) => api.deleteApplication(appId, body),
    onSuccess: () => {
      setDeleteOpen(false)
      void qc.invalidateQueries({ queryKey: ['applications'] })
      if (nested && projectId && envId) {
        void nav({ to: '/projects/$projectId/environments/$envId', params: { projectId, envId } })
      } else {
        void nav({ to: '/projects' })
      }
    },
  })

  const cancel = useMutation({
    mutationFn: (id: string) => api.cancelDeployment(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['deployments', appId] }),
  })

  const rollback = useMutation({
    mutationFn: () => api.rollbackApplication(appId),
    onSuccess: (dep) => {
      void qc.invalidateQueries({ queryKey: ['deployments', appId] })
      if (nested && projectId && envId) {
        void nav({
          to: '/projects/$projectId/environments/$envId/applications/$appId/deployments/$deploymentId',
          params: { projectId, envId, appId, deploymentId: dep.id },
        })
      } else {
        void nav({
          to: '/applications/$appId/deployments/$deploymentId',
          params: { appId, deploymentId: dep.id },
        })
      }
    },
  })

  const webhook = useMutation({
    mutationFn: () => api.setWebhookSecret(appId),
    onSuccess: (data) => setWebhookSecret(data.secret),
  })

  const deletePreview = useMutation({
    mutationFn: (prId: number) => api.deletePreview(appId, prId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['previews', appId] }),
  })

  if (app.isLoading) {
    return <PageSkeleton cards={3} />
  }

  const backLink =
    nested && projectId && envId ? (
      <Link
        to="/projects/$projectId/environments/$envId"
        params={{ projectId, envId }}
        className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
      >
        ← Resources
      </Link>
    ) : (
      <Link
        to="/projects"
        className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
      >
        ← Projects
      </Link>
    )

  if (app.error || !app.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{app.error?.message || 'Application not found'}</p>
        {backLink}
      </div>
    )
  }

  const a = app.data
  const webhookUrl =
    typeof window !== 'undefined'
      ? `${window.location.origin}/api/v1/webhooks/git/${appId}`
      : `/api/v1/webhooks/git/${appId}`

  const openDeployment = (deploymentId: string) => {
    if (nested && projectId && envId) {
      void nav({
        to: '/projects/$projectId/environments/$envId/applications/$appId/deployments/$deploymentId',
        params: { projectId, envId, appId, deploymentId },
      })
    } else {
      void nav({
        to: '/applications/$appId/deployments/$deploymentId',
        params: { appId, deploymentId },
      })
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          {backLink}
          <h1 className="mt-2 text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
            {a.name}
          </h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {a.build_pack} · {a.status}
            {a.fqdn ? ` · ${a.fqdn}` : ''}
          </p>
        </div>
        <div className="flex gap-2">
          <LinksMenu links={a.links || []} />
          {activeDep && <Btn onClick={() => cancel.mutate(activeDep.id)}>Cancel deploy</Btn>}
          <Btn primary onClick={() => deploy.mutate({})}>
            Deploy
          </Btn>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        <Meta label="Status" value={a.status} />
        <Meta label="Build pack" value={a.build_pack} />
        <Meta label="FQDN" value={a.fqdn || '—'} />
      </div>

      <ResourceTabs tabs={APP_TABS} active={tab} onChange={setTab} />

      {tab === 'links' && (
        <TabPanel>
          <LinksPanel links={a.links || []} />
        </TabPanel>
      )}

      {tab === 'configuration' && (
        <TabPanel>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              save.mutate()
            }}
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <Input label="Name" value={cfg.name} onChange={(v) => setCfg({ ...cfg, name: v })} />
              <div className="space-y-2">
                <Input
                  label="FQDN"
                  value={cfg.fqdn}
                  onChange={(v) => setCfg({ ...cfg, fqdn: v })}
                  required={false}
                />
                <button
                  type="button"
                  className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400"
                  onClick={() => {
                    void api
                      .generateDomain({
                        name: cfg.name || a.name,
                        destination_id: cfg.destination_id || a.destination_id || undefined,
                        resource_id: a.id,
                      })
                      .then((d) => setCfg((c) => ({ ...c, fqdn: d.fqdn })))
                      .catch(() => undefined)
                  }}
                >
                  Generate free domain (sslip.io / nip.io)
                </button>
                {cfg.fqdn ? (
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    Multiple hosts: comma-separated. Optional path/port:{" "}
                    <code className="font-mono">example.com:8080</code>
                  </p>
                ) : null}
              </div>
              <Input
                label="Description"
                value={cfg.description}
                onChange={(v) => setCfg({ ...cfg, description: v })}
                required={false}
              />
              <Input
                label="Ports exposes"
                value={cfg.ports_exposes}
                onChange={(v) => setCfg({ ...cfg, ports_exposes: v })}
                required={false}
              />
              {a.build_pack === 'dockercompose' ? (
                <p className="-mt-2 text-xs text-gray-500 dark:text-gray-400 sm:col-span-2">
                  For Compose, leave empty to auto-detect the container port from the compose file.
                  Set a value only to override Traefik&apos;s target port.
                </p>
              ) : null}
              {a.build_pack === 'dockercompose' ? (
                <>
                  <Input
                    label="Compose file path"
                    value={cfg.docker_compose_location}
                    onChange={(v) => setCfg({ ...cfg, docker_compose_location: v })}
                    required={false}
                  />
                  <div className="sm:col-span-2 -mt-2 flex flex-wrap items-center gap-3">
                    <button
                      type="button"
                      className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400"
                      onClick={() => {
                        void api
                          .detectComposeForApp(appId, true)
                          .then((d) => {
                            setCfg((c) => ({ ...c, docker_compose_location: d.location }))
                            void qc.invalidateQueries({ queryKey: ['application', appId] })
                          })
                          .catch(() => undefined)
                      }}
                    >
                      Auto-detect from repository
                    </button>
                    <span className="text-xs text-gray-500 dark:text-gray-400">
                      Empty path = auto-find on deploy (
                      <code className="font-mono">docker-compose.yml</code> /{' '}
                      <code className="font-mono">compose.yaml</code>, depth 3).
                    </span>
                  </div>
                  <fieldset className="space-y-3 sm:col-span-2">
                    <legend className="text-sm text-gray-500 dark:text-gray-400">
                      Compose adaptation
                    </legend>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      Adapt adds Traefik labels, joins the proxy network, and removes host port
                      mappings so you do not conflict with ports 80/443.
                    </p>
                    <label className="flex items-start gap-3 text-sm">
                      <input
                        type="radio"
                        className="mt-1"
                        name="compose_prepare"
                        checked={cfg.compose_prepare}
                        onChange={() => setCfg({ ...cfg, compose_prepare: true })}
                      />
                      <span>
                        Adapt for Dockfin (recommended)
                        <span className="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">
                          Auto-fix compose for Dockfin proxy on each deploy.
                        </span>
                      </span>
                    </label>
                    <label className="flex items-start gap-3 text-sm">
                      <input
                        type="radio"
                        className="mt-1"
                        name="compose_prepare"
                        checked={!cfg.compose_prepare}
                        onChange={() => setCfg({ ...cfg, compose_prepare: false })}
                      />
                      <span>
                        Don&apos;t modify
                        <span className="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">
                          Use the repository compose file as-is.
                        </span>
                      </span>
                    </label>
                  </fieldset>
                </>
              ) : null}
              <Input
                label="Git repository"
                value={cfg.git_repository}
                onChange={(v) => setCfg({ ...cfg, git_repository: v })}
                required={false}
              />
              <Input
                label="Git branch"
                value={cfg.git_branch}
                onChange={(v) => setCfg({ ...cfg, git_branch: v })}
                required={false}
              />
              <Input
                label="Registry image"
                value={cfg.docker_registry_image_name}
                onChange={(v) => setCfg({ ...cfg, docker_registry_image_name: v })}
                required={false}
              />
              <Input
                label="Image tag"
                value={cfg.docker_registry_image_tag}
                onChange={(v) => setCfg({ ...cfg, docker_registry_image_tag: v })}
                required={false}
              />
              <label className="block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">Destination</span>
                <select
                  value={cfg.destination_id}
                  onChange={(e) => setCfg({ ...cfg, destination_id: e.target.value })}
                  className="w-full panel-field rounded-lg px-3 py-2"
                >
                  <option value="">Select destination</option>
                  {(dests.data?.destinations || []).map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.name}
                    </option>
                  ))}
                </select>
              </label>
              {(gitSources.data?.git_sources || []).length > 0 && (
                <label className="block text-sm">
                  <span className="mb-1 block text-gray-500 dark:text-gray-400">Git source</span>
                  <select
                    value={cfg.git_source_id}
                    onChange={(e) => setCfg({ ...cfg, git_source_id: e.target.value })}
                    className="w-full panel-field rounded-lg px-3 py-2"
                  >
                    <option value="">None (public HTTPS clone)</option>
                    {(gitSources.data?.git_sources || []).map((gs) => (
                      <option key={gs.id} value={gs.id}>
                        {gs.name}
                        {gs.installation_id ? '' : ' (not installed)'}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              <label className="flex items-center gap-3 text-sm sm:col-span-2">
                <input
                  type="checkbox"
                  checked={cfg.is_force_https}
                  onChange={(e) => setCfg({ ...cfg, is_force_https: e.target.checked })}
                />
                <span>Force HTTPS redirects</span>
              </label>
              <label className="flex items-center gap-3 text-sm sm:col-span-2">
                <input
                  type="checkbox"
                  checked={cfg.is_preview_enabled}
                  onChange={(e) => setCfg({ ...cfg, is_preview_enabled: e.target.checked })}
                />
                <span>
                  Enable preview deployments
                  <span className="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">
                    Allows PR preview environments when webhooks include pull request events.
                  </span>
                </span>
              </label>
              <label className="flex items-center gap-3 text-sm sm:col-span-2">
                <input
                  type="checkbox"
                  checked={cfg.is_build_server_enabled}
                  onChange={(e) => setCfg({ ...cfg, is_build_server_enabled: e.target.checked })}
                />
                <span>
                  Build on dedicated build server
                  <span className="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">
                    Requires at least one server marked as a build server.
                  </span>
                </span>
              </label>
            </div>
            {save.error && <p className="text-sm text-error-500">{save.error.message}</p>}
            <Btn primary type="submit">
              {save.isPending ? 'Saving…' : 'Save'}
            </Btn>
          </form>
        </TabPanel>
      )}

      {tab === 'health' && (
        <TabPanel>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              saveHealth.mutate()
            }}
          >
            <p className="text-sm text-gray-500 dark:text-gray-400">
              After deploy, Dockfin probes this HTTP endpoint inside the container (path / port /
              expected status). Retries use the interval below until healthy or exhausted.
            </p>
            <label className="flex items-center gap-3 text-sm">
              <input
                type="checkbox"
                checked={health.health_check_enabled}
                onChange={(e) => setHealth({ ...health, health_check_enabled: e.target.checked })}
              />
              Enable health checks
            </label>
            <div className="grid gap-4 sm:grid-cols-2">
              <Input
                label="Path"
                value={health.health_check_path}
                onChange={(v) => setHealth({ ...health, health_check_path: v })}
              />
              <Input
                label="Port (optional)"
                value={health.health_check_port}
                onChange={(v) => setHealth({ ...health, health_check_port: v })}
                required={false}
              />
              <label className="block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">Method</span>
                <select
                  value={health.health_check_method}
                  onChange={(e) => setHealth({ ...health, health_check_method: e.target.value })}
                  className="panel-field w-full rounded-lg px-3 py-2"
                >
                  {['GET', 'HEAD', 'POST'].map((m) => (
                    <option key={m} value={m}>
                      {m}
                    </option>
                  ))}
                </select>
              </label>
              <Input
                label="Expected status"
                value={String(health.health_check_return_code)}
                onChange={(v) =>
                  setHealth({ ...health, health_check_return_code: Number(v) || 200 })
                }
              />
              <Input
                label="Interval (s)"
                value={String(health.health_check_interval)}
                onChange={(v) =>
                  setHealth({ ...health, health_check_interval: Number(v) || 5 })
                }
              />
              <Input
                label="Timeout (s)"
                value={String(health.health_check_timeout)}
                onChange={(v) => setHealth({ ...health, health_check_timeout: Number(v) || 5 })}
              />
              <Input
                label="Retries"
                value={String(health.health_check_retries)}
                onChange={(v) => setHealth({ ...health, health_check_retries: Number(v) || 10 })}
              />
            </div>
            {saveHealth.error && <p className="text-sm text-error-500">{saveHealth.error.message}</p>}
            <Btn primary type="submit">
              {saveHealth.isPending ? 'Saving…' : 'Save health checks'}
            </Btn>
          </form>
        </TabPanel>
      )}

      {tab === 'limits' && (
        <TabPanel>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              saveLimits.mutate()
            }}
          >
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Docker resource limits applied on the next deploy. Leave empty for unlimited.
            </p>
            <div className="grid gap-4 sm:grid-cols-2">
              <Input
                label="Memory limit"
                value={limits.limits_memory}
                onChange={(v) => setLimits({ ...limits, limits_memory: v })}
                required={false}
              />
              <Input
                label="CPU limit"
                value={limits.limits_cpus}
                onChange={(v) => setLimits({ ...limits, limits_cpus: v })}
                required={false}
              />
            </div>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Examples: memory <code className="font-mono">512m</code> /{' '}
              <code className="font-mono">1g</code>, CPUs <code className="font-mono">0.5</code> /{' '}
              <code className="font-mono">2</code>.
            </p>
            {saveLimits.error && <p className="text-sm text-error-500">{saveLimits.error.message}</p>}
            <Btn primary type="submit">
              {saveLimits.isPending ? 'Saving…' : 'Save limits'}
            </Btn>
          </form>
        </TabPanel>
      )}

      {tab === 'environment' && (
        <TabPanel>
          <EnvVarsPanel resourceType="application" resourceId={appId} title="" />
        </TabPanel>
      )}

      {tab === 'deployments' && (
        <TabPanel>
          <div className="panel-card overflow-hidden">
            <table className="w-full text-left text-sm">
              <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
                <tr>
                  <th className="px-3 py-2">ID</th>
                  <th className="px-3 py-2">Status</th>
                  <th className="px-3 py-2">Stage</th>
                  <th className="px-3 py-2">Created</th>
                  <th className="px-3 py-2">Actions</th>
                </tr>
              </thead>
              <tbody>
                {(deps.data?.deployments || []).map((d) => (
                  <tr key={d.id} className="border-t border-gray-200 dark:border-gray-800">
                    <td className="px-3 py-2 font-mono text-xs">{d.id.slice(0, 8)}…</td>
                    <td className="px-3 py-2">{d.status}</td>
                    <td className="px-3 py-2">{d.current_stage || '—'}</td>
                    <td className="px-3 py-2 text-gray-500 dark:text-gray-400">
                      {new Date(d.created_at).toLocaleString()}
                    </td>
                    <td className="space-x-3 px-3 py-2">
                      <button
                        type="button"
                        className="text-brand-600 dark:text-brand-400"
                        onClick={() => openDeployment(d.id)}
                      >
                        Open
                      </button>
                      {(d.status === 'queued' || d.status === 'in_progress') && (
                        <button
                          type="button"
                          className="text-error-500"
                          onClick={() => cancel.mutate(d.id)}
                        >
                          Cancel
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
                {!deps.data?.deployments?.length && (
                  <tr>
                    <td colSpan={5} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                      No deployments yet.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </TabPanel>
      )}

      {tab === 'previews' && (
        <TabPanel>
          <div className="space-y-4">
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Pull-request preview deployments for this application.
              {!cfg.is_preview_enabled && (
                <>
                  {' '}
                  Enable “preview deployments” under Configuration to accept PR webhooks.
                </>
              )}
            </p>
            <div className="panel-card overflow-hidden">
              <table className="w-full text-left text-sm">
                <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
                  <tr>
                    <th className="px-3 py-2">PR</th>
                    <th className="px-3 py-2">Title</th>
                    <th className="px-3 py-2">Branch</th>
                    <th className="px-3 py-2">FQDN</th>
                    <th className="px-3 py-2">Status</th>
                    <th className="px-3 py-2" />
                  </tr>
                </thead>
                <tbody>
                  {(previews.data?.previews || []).map((p) => (
                    <tr key={p.id} className="border-t border-gray-200 dark:border-gray-800">
                      <td className="px-3 py-2 font-mono text-xs">#{p.pull_request_id}</td>
                      <td className="px-3 py-2">{p.pull_request_title || '—'}</td>
                      <td className="px-3 py-2 font-mono text-xs">{p.git_branch || '—'}</td>
                      <td className="px-3 py-2 font-mono text-xs">{p.fqdn || '—'}</td>
                      <td className="px-3 py-2">{p.status}</td>
                      <td className="px-3 py-2 text-right">
                        <button
                          type="button"
                          className="text-error-500"
                          onClick={() => {
                            if (confirm(`Delete preview for PR #${p.pull_request_id}?`)) {
                              deletePreview.mutate(p.pull_request_id)
                            }
                          }}
                        >
                          Delete
                        </button>
                      </td>
                    </tr>
                  ))}
                  {!previews.data?.previews?.length && (
                    <tr>
                      <td colSpan={6} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                        No preview deployments yet.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
            {deletePreview.error && (
              <p className="text-sm text-error-500">{deletePreview.error.message}</p>
            )}
          </div>
        </TabPanel>
      )}

      {tab === 'terminal' && (
        <TabPanel>
          {serverId ? (
            <ServerTerminal
              serverId={serverId}
              defaultContainer={`dockfin-${appId}`}
              containerOptions={[`dockfin-${appId}`]}
              hideHostShell
            />
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Assign a destination so the application container terminal can connect.
            </p>
          )}
        </TabPanel>
      )}

      {tab === 'webhooks' && (
        <TabPanel>
          <div className="panel-card space-y-4 p-5">
            <div>
              <div className="text-xs text-gray-500 dark:text-gray-400">Git webhook URL</div>
              <code className="mt-1 block break-all font-mono text-sm text-gray-900 dark:text-white">
                {webhookUrl}
              </code>
            </div>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Point GitHub/GitLab webhook here. Generate a secret and configure the same value on your
              provider (HMAC).
            </p>
            <Btn primary onClick={() => webhook.mutate()}>
              {webhook.isPending ? 'Generating…' : 'Generate webhook secret'}
            </Btn>
            {webhookSecret && (
              <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-500/30 dark:bg-amber-500/10">
                <p className="text-xs font-medium text-amber-800 dark:text-amber-200">
                  Copy now — shown once
                </p>
                <code className="mt-1 block break-all font-mono text-sm">{webhookSecret}</code>
              </div>
            )}
            {webhook.error && <p className="text-sm text-error-500">{webhook.error.message}</p>}
          </div>
        </TabPanel>
      )}

      {tab === 'tasks' && (
        <TabPanel>
          <ScheduledTasksPanel resourceType="application" resourceId={appId} />
        </TabPanel>
      )}

      {tab === 'operations' && (
        <TabPanel>
          <div className="space-y-4">
            <div className="panel-card space-y-3 p-5">
              <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Deploy</h2>
              <div className="flex flex-wrap gap-2">
                <Btn primary onClick={() => deploy.mutate({})}>
                  Deploy
                </Btn>
                <Btn onClick={() => deploy.mutate({ force: true })}>Force rebuild</Btn>
              </div>
            </div>
            <MoveResourcePanel
              resourceType="application"
              resourceId={appId}
              currentEnvironmentId={a.environment_id}
              projectId={projectId}
            />
          </div>
        </TabPanel>
      )}

      {tab === 'rollback' && (
        <TabPanel>
          <div className="panel-card space-y-4 p-5">
            <p className="text-sm text-gray-600 dark:text-gray-300">
              Redeploy the last finished commit (or force an image rebuild). This queues a new
              deployment.
            </p>
            <Btn primary onClick={() => rollback.mutate()}>
              {rollback.isPending ? 'Queuing…' : 'Rollback / redeploy'}
            </Btn>
            {rollback.error && <p className="text-sm text-error-500">{rollback.error.message}</p>}
          </div>
        </TabPanel>
      )}

      {tab === 'danger' && (
        <TabPanel>
          <div className="space-y-4">
            <div className="panel-card space-y-4 border-error-200 p-5 dark:border-error-500/30">
              <h2 className="text-sm font-semibold text-error-500">Force rebuild</h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Queue a deployment with force rebuild enabled.
              </p>
              <Btn onClick={() => deploy.mutate({ force: true })}>Force rebuild deploy</Btn>
              {deploy.error && <p className="text-sm text-error-500">{deploy.error.message}</p>}
            </div>

            <DangerZoneCard>
              <div>
                <h3 className="text-sm font-semibold text-error-500">Delete Resource</h3>
                <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  This will stop your containers, delete related data on the server, and remove the
                  application from Dockfin. Beware — there is no coming back.
                </p>
                <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
                  Container status:{' '}
                  <span className="font-medium capitalize">{a.status || 'unknown'}</span>
                  {a.status === 'running'
                    ? ' — container will be stopped and removed.'
                    : a.status === 'exited' || a.status === 'stopped'
                      ? ' — container is already stopped; it will still be removed.'
                      : ' — Dockfin will best-effort remove any matching container.'}
                </p>
              </div>
              <Btn type="button" onClick={() => setDeleteOpen(true)}>
                Delete
              </Btn>
              {remove.error && <p className="text-sm text-error-500">{remove.error.message}</p>}
            </DangerZoneCard>

            <DangerConfirmModal
              open={deleteOpen}
              onClose={() => setDeleteOpen(false)}
              title="Confirm Resource Deletion?"
              resourceLabel="Resource Name"
              expectedName={a.name}
              statusLine={
                a.status === 'running'
                  ? `Container is currently RUNNING (${a.status}). Deleting will stop and remove it.`
                  : `Current status: ${a.status || 'unknown'}.`
              }
              actions={[
                'Permanently delete all containers of this resource.',
                'Remove the application record, env vars, and scheduled jobs from Dockfin.',
              ]}
              requirePassword
              showResourceCheckboxes
              confirmButtonLabel="Delete"
              busy={remove.isPending}
              error={remove.error?.message}
              onConfirm={(payload) => remove.mutate(payload)}
            />
          </div>
        </TabPanel>
      )}
    </div>
  )
}
