import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import {
  Activity,
  AlertTriangle,
  CalendarClock,
  Gauge,
  GitBranch,
  GitPullRequest,
  HardDrive,
  History,
  Link2,
  RotateCcw,
  ScrollText,
  Server,
  Settings2,
  SlidersHorizontal,
  Tags,
  Terminal,
  Variable,
  Webhook,
  Wrench,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useConfirm } from '../components/ConfirmDialog'
import { DangerConfirmModal, DangerZoneCard } from '../components/DangerConfirmModal'
import { DomainsPanel, normalizeDomains } from '../components/DomainsPanel'
import { EnvVarsPanel } from '../components/EnvVarsPanel'
import { LinksMenu, LinksPanel } from '../components/LinksMenu'
import { MoveResourcePanel } from '../components/MoveResourcePanel'
import { PersistentStoragesPanel } from '../components/PersistentStoragesPanel'
import { ResourceTagsPanel } from '../components/ResourceTagsPanel'
import { ScheduledTasksPanel } from '../components/ScheduledTasksPanel'
import { ServerTerminal } from '../components/Terminal'
import { CodeEditor } from '../components/CodeEditor'
import { PageSkeleton } from '../components/ui/Skeleton'
import { useToast } from '../components/Toast'
import { api } from '../lib/api'
import { Btn, Input } from './Servers'

/** Coolify-style top IA — Configuration first, Links last. */
const TOP_TABS = [
  { id: 'configuration', label: 'Configuration', icon: Settings2 },
  { id: 'deployments', label: 'Deployments', icon: History },
  { id: 'logs', label: 'Logs', icon: ScrollText },
  { id: 'terminal', label: 'Terminal', icon: Terminal },
  { id: 'links', label: 'Links', icon: Link2 },
] as const

/** Coolify-style configuration sidebar (Dockfin design tokens). */
const SIDE_ITEMS = [
  { id: 'general', label: 'General', icon: Settings2 },
  { id: 'advanced', label: 'Advanced', icon: SlidersHorizontal },
  { id: 'environment', label: 'Environment Variables', icon: Variable },
  { id: 'storages', label: 'Persistent Storage', icon: HardDrive },
  { id: 'git', label: 'Git Source', icon: GitBranch },
  { id: 'servers', label: 'Servers', icon: Server },
  { id: 'tasks', label: 'Scheduled Tasks', icon: CalendarClock },
  { id: 'webhooks', label: 'Webhooks', icon: Webhook },
  { id: 'previews', label: 'Preview Deployments', icon: GitPullRequest },
  { id: 'rollback', label: 'Rollback', icon: RotateCcw },
  { id: 'limits', label: 'Resource Limits', icon: Gauge },
  { id: 'operations', label: 'Resource Operations', icon: Wrench },
  { id: 'metrics', label: 'Metrics', icon: Activity },
  { id: 'tags', label: 'Tags', icon: Tags },
  { id: 'danger', label: 'Danger Zone', icon: AlertTriangle },
] as const

function statusTone(status: string) {
  const s = (status || '').toLowerCase()
  if (s.includes('run') || s.includes('healthy')) return 'ok'
  if (s.includes('deploy') || s.includes('queue') || s.includes('progress')) return 'warn'
  if (s.includes('exit') || s.includes('stop') || s.includes('fail') || s.includes('error')) return 'bad'
  return 'muted'
}

function StatusText({ status }: { status: string }) {
  const tone = statusTone(status)
  const color =
    tone === 'ok'
      ? 'text-emerald-600 dark:text-emerald-400'
      : tone === 'warn'
        ? 'text-amber-600 dark:text-amber-400'
        : tone === 'bad'
          ? 'text-error-500'
          : 'text-gray-500 dark:text-gray-400'
  return <span className={`capitalize ${color}`}>{status || 'unknown'}</span>
}

export function ApplicationDetailPage() {
  const { appId, projectId, envId } = useParams({ strict: false }) as {
    appId: string
    projectId?: string
    envId?: string
  }
  const nav = useNavigate()
  const qc = useQueryClient()
  const toast = useToast()
  const confirm = useConfirm()
  const nested = Boolean(projectId && envId)

  const app = useQuery({ queryKey: ['application', appId], queryFn: () => api.application(appId) })
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const gitSources = useQuery({ queryKey: ['git-sources'], queryFn: api.gitSources })
  const keys = useQuery({ queryKey: ['keys'], queryFn: api.keys })
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

  const [topTab, setTopTab] = useState<(typeof TOP_TABS)[number]['id']>('configuration')
  const [side, setSide] = useState<(typeof SIDE_ITEMS)[number]['id']>('general')
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
    base_directory: '/',
    docker_compose_custom_build_command: '',
    docker_compose_custom_start_command: '',
    custom_docker_run_options: '',
    dockerfile_location: '/Dockerfile',
    dockerfile: '',
    dockerfile_target_build: '',
    docker_registry_image_name: '',
    docker_registry_image_tag: '',
    destination_id: '',
    git_source_id: '',
    private_key_id: '',
    is_build_server_enabled: false,
    is_force_https: true,
    is_preview_enabled: false,
    is_auto_deploy_enabled: true,
    is_git_submodules_enabled: false,
    is_preserve_repository_enabled: false,
    watch_paths: '',
    pre_deployment_command: '',
    post_deployment_command: '',
    custom_labels: '',
    http_basic_auth_username: '',
  })
  const [httpBasicAuthPassword, setHttpBasicAuthPassword] = useState('')
  const [clearHttpBasicAuth, setClearHttpBasicAuth] = useState(false)
  const [serviceDomains, setServiceDomains] = useState<Record<string, string>>({})
  const [showRawCompose, setShowRawCompose] = useState(true)
  const [loadComposeBusy, setLoadComposeBusy] = useState(false)
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
    setTopTab('configuration')
    setSide('general')
    setWebhookSecret(null)
    setHttpBasicAuthPassword('')
    setClearHttpBasicAuth(false)
    setCfg({
      name: '',
      description: '',
      fqdn: '',
      git_repository: '',
      git_branch: '',
      ports_exposes: '',
      docker_compose_location: '',
      compose_prepare: true,
      base_directory: '/',
      docker_compose_custom_build_command: '',
      docker_compose_custom_start_command: '',
      custom_docker_run_options: '',
      dockerfile_location: '/Dockerfile',
      dockerfile: '',
      dockerfile_target_build: '',
      docker_registry_image_name: '',
      docker_registry_image_tag: '',
      destination_id: '',
      git_source_id: '',
      private_key_id: '',
      is_build_server_enabled: false,
      is_force_https: true,
      is_preview_enabled: false,
      is_auto_deploy_enabled: true,
      is_git_submodules_enabled: false,
      is_preserve_repository_enabled: false,
      watch_paths: '',
      pre_deployment_command: '',
      post_deployment_command: '',
      custom_labels: '',
      http_basic_auth_username: '',
    })
    setServiceDomains({})
    setShowRawCompose(true)
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
    const updated = app.data
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
      base_directory: updated.base_directory || '/',
      docker_compose_custom_build_command: updated.docker_compose_custom_build_command || '',
      docker_compose_custom_start_command: updated.docker_compose_custom_start_command || '',
      custom_docker_run_options: updated.custom_docker_run_options || '',
      dockerfile_location: (updated as { dockerfile_location?: string }).dockerfile_location || '/Dockerfile',
      dockerfile: updated.dockerfile || '',
      dockerfile_target_build: updated.dockerfile_target_build || '',
      docker_registry_image_name: updated.docker_registry_image_name || '',
      docker_registry_image_tag: updated.docker_registry_image_tag || '',
      destination_id: updated.destination_id || '',
      git_source_id: updated.git_source_id || '',
      private_key_id: updated.private_key_id || '',
      is_build_server_enabled: Boolean(updated.is_build_server_enabled),
      is_force_https: updated.is_force_https !== false,
      is_preview_enabled: Boolean(updated.is_preview_enabled),
      is_auto_deploy_enabled: updated.is_auto_deploy_enabled !== false,
      is_git_submodules_enabled: Boolean(updated.is_git_submodules_enabled),
      is_preserve_repository_enabled: Boolean(updated.is_preserve_repository_enabled),
      watch_paths: updated.watch_paths || '',
      pre_deployment_command: updated.pre_deployment_command || '',
      post_deployment_command: updated.post_deployment_command || '',
      custom_labels: updated.custom_labels || '',
      http_basic_auth_username: updated.http_basic_auth_username || '',
    })
    setHttpBasicAuthPassword('')
    setClearHttpBasicAuth(false)
    const domains: Record<string, string> = {}
    const rawDomains = updated.docker_compose_domains || {}
    for (const [k, v] of Object.entries(rawDomains)) {
      domains[k] = typeof v === 'string' ? v : v?.domain || ''
    }
    for (const u of updated.compose_units || []) {
      if (u.domain && !domains[u.name]) domains[u.name] = u.domain
      if (!(u.name in domains) && !u.is_database) domains[u.name] = ''
    }
    setServiceDomains(domains)
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
  }, [app.data, appId])

  const activeDep = (deps.data?.deployments || []).find(
    (d) => d.status === 'queued' || d.status === 'in_progress',
  )

  const serverId = useMemo(() => {
    const destID = cfg.destination_id || app.data?.destination_id
    if (!destID) return ''
    return (dests.data?.destinations || []).find((d) => d.id === destID)?.server_id || ''
  }, [cfg.destination_id, app.data?.destination_id, dests.data])

  const sideItems = useMemo(() => {
    const inlineDockerfile = Boolean(app.data?.dockerfile || cfg.dockerfile)
    if (!inlineDockerfile) return SIDE_ITEMS
    return SIDE_ITEMS.filter(
      (item) => !['git', 'webhooks', 'previews', 'rollback'].includes(item.id),
    )
  }, [app.data?.dockerfile, cfg.dockerfile])

  const applyAppToForm = (updated: NonNullable<typeof app.data>) => {
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
      base_directory: updated.base_directory || '/',
      docker_compose_custom_build_command: updated.docker_compose_custom_build_command || '',
      docker_compose_custom_start_command: updated.docker_compose_custom_start_command || '',
      custom_docker_run_options: updated.custom_docker_run_options || '',
      dockerfile_location: (updated as { dockerfile_location?: string }).dockerfile_location || '/Dockerfile',
      dockerfile: updated.dockerfile || '',
      dockerfile_target_build: updated.dockerfile_target_build || '',
      docker_registry_image_name: updated.docker_registry_image_name || '',
      docker_registry_image_tag: updated.docker_registry_image_tag || '',
      destination_id: updated.destination_id || '',
      git_source_id: updated.git_source_id || '',
      private_key_id: updated.private_key_id || '',
      is_build_server_enabled: Boolean(updated.is_build_server_enabled),
      is_force_https: updated.is_force_https !== false,
      is_preview_enabled: Boolean(updated.is_preview_enabled),
      is_auto_deploy_enabled: updated.is_auto_deploy_enabled !== false,
      is_git_submodules_enabled: Boolean(updated.is_git_submodules_enabled),
      is_preserve_repository_enabled: Boolean(updated.is_preserve_repository_enabled),
      watch_paths: updated.watch_paths || '',
      pre_deployment_command: updated.pre_deployment_command || '',
      post_deployment_command: updated.post_deployment_command || '',
      custom_labels: updated.custom_labels || '',
      http_basic_auth_username: updated.http_basic_auth_username || '',
    })
    setHttpBasicAuthPassword('')
    setClearHttpBasicAuth(false)
    const domains: Record<string, string> = {}
    const rawDomains = updated.docker_compose_domains || {}
    for (const [k, v] of Object.entries(rawDomains)) {
      domains[k] = typeof v === 'string' ? v : v?.domain || ''
    }
    for (const u of updated.compose_units || []) {
      if (u.domain && !domains[u.name]) domains[u.name] = u.domain
      if (!(u.name in domains) && !u.is_database) domains[u.name] = ''
    }
    setServiceDomains(domains)
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

  const syncFromApp = (updated: NonNullable<typeof app.data>) => {
    applyAppToForm(updated)
  }

  const save = useMutation({
    mutationFn: (patch?: Partial<typeof cfg> & Record<string, unknown>) => {
      const docker_compose_domains = Object.fromEntries(
        Object.entries(serviceDomains).map(([k, v]) => [k, { domain: v }]),
      )
      const body: Record<string, unknown> = {
        ...cfg,
        docker_compose_domains,
        ...patch,
      }
      if (clearHttpBasicAuth) {
        body.clear_http_basic_auth = true
      } else if (httpBasicAuthPassword.trim()) {
        body.http_basic_auth_password = httpBasicAuthPassword
      }
      return api.updateApplication(appId, body)
    },
    onSuccess: (updated) => {
      void qc.invalidateQueries({ queryKey: ['application', appId] })
      syncFromApp(updated)
      toast.success('Saved')
    },
    onError: (e: Error) => toast.error(e.message || 'Save failed'),
  })

  const loadCompose = async () => {
    setLoadComposeBusy(true)
    try {
      const res = await api.loadComposeForApp(appId, {
        base_directory: cfg.base_directory,
        docker_compose_location: cfg.docker_compose_location,
      })
      const merged = {
        ...(res.application || ({} as NonNullable<typeof app.data>)),
        docker_compose_raw: res.docker_compose_raw,
        docker_compose: res.docker_compose,
        docker_compose_location: res.location,
        base_directory: res.base_directory,
        docker_compose_domains: res.docker_compose_domains,
        compose_units: (res.units || []).map((u) => ({
          name: u.name,
          image: u.image,
          is_database: /(?:^|\/)(postgres|postgresql|mysql|mariadb|mongo|mongodb|redis|memcached|rabbitmq|valkey)(?=[:@\/]|$)/i.test(
            u.image || '',
          ),
          domain: res.docker_compose_domains?.[u.name]?.domain || '',
        })),
        compose_volumes: res.volumes || [],
      } as NonNullable<typeof app.data>
      syncFromApp(merged)
      void qc.invalidateQueries({ queryKey: ['application', appId] })
      toast.success('Compose file loaded')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Load compose failed')
    } finally {
      setLoadComposeBusy(false)
    }
  }

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
      toast.success('Deploy queued')
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

  const startApp = useMutation({
    mutationFn: () => api.startApplication(appId),
    onSuccess: () => {
      toast.success('Started')
      void qc.invalidateQueries({ queryKey: ['application', appId] })
    },
    onError: (e: Error) => toast.error(e.message || 'Start failed'),
  })
  const stopApp = useMutation({
    mutationFn: () => api.stopApplication(appId),
    onSuccess: () => {
      toast.success('Stopped')
      void qc.invalidateQueries({ queryKey: ['application', appId] })
    },
    onError: (e: Error) => toast.error(e.message || 'Stop failed'),
  })
  const restartApp = useMutation({
    mutationFn: () => api.restartApplication(appId),
    onSuccess: () => {
      toast.success('Restarted')
      void qc.invalidateQueries({ queryKey: ['application', appId] })
    },
    onError: (e: Error) => toast.error(e.message || 'Restart failed'),
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
    mutationFn: (commit_sha?: string) => api.rollbackApplication(appId, true, commit_sha),
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
          <p className="mt-1 flex flex-wrap items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
            <span className="capitalize">{a.build_pack}</span>
            <span>·</span>
            <StatusText status={a.status} />
            {a.fqdn ? (
              <>
                <span>·</span>
                <span className="max-w-md truncate font-mono text-xs">{a.fqdn.split(',')[0]}</span>
              </>
            ) : null}
            {activeDep && (
              <span className="inline-flex items-center gap-1.5 rounded-full bg-brand-500/15 px-2 py-0.5 text-[11px] font-medium text-brand-600 dark:text-brand-300">
                <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-brand-400" />
                {activeDep.status}
              </span>
            )}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <LinksMenu links={a.links || []} />
          {activeDep && <Btn onClick={() => cancel.mutate(activeDep.id)}>Cancel</Btn>}
          <button
            type="button"
            title="Restart"
            className="inline-flex h-8 items-center gap-1.5 rounded-md bg-amber-500/15 px-2.5 text-xs font-medium text-amber-700 hover:bg-amber-500/25 dark:text-amber-300"
            disabled={restartApp.isPending || startApp.isPending || stopApp.isPending || deploy.isPending}
            onClick={() => restartApp.mutate()}
          >
            {restartApp.isPending ? 'Restarting…' : 'Restart'}
          </button>
          {(a.status || '').toLowerCase().includes('exit') ||
          (a.status || '').toLowerCase().includes('stop') ? (
            <button
              type="button"
              title="Start"
              className="inline-flex h-8 items-center gap-1.5 rounded-md bg-emerald-500/15 px-2.5 text-xs font-medium text-emerald-700 hover:bg-emerald-500/25 dark:text-emerald-300"
              disabled={startApp.isPending || restartApp.isPending || stopApp.isPending || deploy.isPending}
              onClick={() => startApp.mutate()}
            >
              {startApp.isPending ? 'Starting…' : 'Start'}
            </button>
          ) : (
            <button
              type="button"
              title="Stop"
              className="inline-flex h-8 items-center gap-1.5 rounded-md bg-error-500/15 px-2.5 text-xs font-medium text-error-500 hover:bg-error-500/25"
              disabled={stopApp.isPending || restartApp.isPending || startApp.isPending || deploy.isPending}
              onClick={() => {
                void (async () => {
                  if (
                    await confirm({
                      title: 'Stop application',
                      message: 'Stop this application container / compose stack?',
                      confirmLabel: 'Stop',
                      danger: true,
                    })
                  ) {
                    stopApp.mutate()
                  }
                })()
              }}
            >
              {stopApp.isPending ? 'Stopping…' : 'Stop'}
            </button>
          )}
          <Btn
            onClick={() => {
              setTopTab('configuration')
              setSide('general')
            }}
          >
            Configuration
          </Btn>
          <Btn primary onClick={() => deploy.mutate({})}>
            Redeploy
          </Btn>
        </div>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 dark:border-gray-800">
        <nav className="flex flex-wrap gap-1" role="tablist">
          {TOP_TABS.map((t) => {
            const Icon = t.icon
            const active = topTab === t.id
            return (
              <button
                key={t.id}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => setTopTab(t.id)}
                className={`relative inline-flex items-center gap-1.5 px-3 py-2.5 text-sm font-medium transition ${
                  active
                    ? 'text-gray-900 dark:text-white'
                    : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
                }`}
              >
                <Icon
                  className={`h-3.5 w-3.5 shrink-0 ${active ? 'opacity-100' : 'opacity-70'}`}
                  aria-hidden
                />
                {t.label}
                {active && (
                  <span className="absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-brand-500" />
                )}
              </button>
            )
          })}
        </nav>
      </div>

      {topTab === 'configuration' && (
        <div className="flex flex-col gap-6 md:flex-row">
          <aside className="w-full shrink-0 md:w-56">
            <nav className="space-y-0.5">
              {sideItems.map((item) => {
                const Icon = item.icon
                const active = side === item.id
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setSide(item.id)}
                    className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm ${
                      active
                        ? 'bg-brand-50 font-medium text-brand-700 dark:bg-brand-500/15 dark:text-brand-300'
                        : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-white/5'
                    }`}
                  >
                    <Icon
                      className={`h-3.5 w-3.5 shrink-0 ${
                        active
                          ? 'text-brand-600 dark:text-brand-400'
                          : 'text-gray-400 dark:text-gray-500'
                      }`}
                      aria-hidden
                    />
                    <span className="truncate">{item.label}</span>
                  </button>
                )
              })}
            </nav>
          </aside>
          <div className="min-w-0 flex-1 space-y-6">
            {side === 'general' && (
              <form
                className="space-y-6"
                onSubmit={(e) => {
                  e.preventDefault()
                  const fqdn = normalizeDomains(cfg.fqdn)
                  setCfg((c) => ({ ...c, fqdn }))
                  save.mutate({ fqdn })
                }}
              >
                <div>
                  <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <h2 className="text-sm font-semibold text-gray-900 dark:text-white">General</h2>
                      <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                        General configuration for your application.
                      </p>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <Btn primary type="submit" disabled={save.isPending}>
                        {save.isPending ? 'Saving…' : 'Save'}
                      </Btn>
                      {a.build_pack === 'dockercompose' ? (
                        <Btn type="button" disabled={loadComposeBusy} onClick={() => void loadCompose()}>
                          {loadComposeBusy
                            ? 'Loading…'
                            : a.docker_compose_raw
                              ? 'Reload Compose File'
                              : 'Load Compose File'}
                        </Btn>
                      ) : null}
                    </div>
                  </div>
                  <div className="panel-card grid gap-4 p-5 sm:grid-cols-2">
                    <Input label="Name" value={cfg.name} onChange={(v) => setCfg({ ...cfg, name: v })} />
                    <Input
                      label="Description"
                      value={cfg.description}
                      onChange={(v) => setCfg({ ...cfg, description: v })}
                      required={false}
                    />
                    <label className="block text-sm sm:col-span-2">
                      <span className="mb-1 block text-gray-500 dark:text-gray-400">Build Pack</span>
                      <input
                        readOnly
                        value={a.build_pack}
                        className="panel-field w-full cursor-default rounded-lg px-3 py-2 capitalize opacity-80"
                      />
                    </label>
                  </div>
                </div>

                {a.build_pack === 'dockercompose' &&
                (a.compose_units || []).some((u) => !u.is_database) &&
                cfg.compose_prepare ? (
                  <div className="panel-card space-y-4 p-5">
                    <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Domains</h3>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      One domain (or comma-separated list) per compose service. Use Generate Domain for
                      magic sslip/nip URLs.
                    </p>
                    {(a.compose_units || [])
                      .filter((u) => !u.is_database)
                      .map((u) => (
                        <div key={u.name} className="flex flex-wrap items-end gap-2">
                          <div className="min-w-0 flex-1">
                            <Input
                              label={`Domains for ${u.name}`}
                              value={serviceDomains[u.name] || ''}
                              onChange={(v) =>
                                setServiceDomains((d) => ({ ...d, [u.name]: v }))
                              }
                              required={false}
                            />
                          </div>
                          <Btn
                            type="button"
                            onClick={() => {
                              void api
                                .generateDomain({
                                  name: `${cfg.name || a.name}-${u.name}`,
                                  server_id: serverId || undefined,
                                  destination_id: cfg.destination_id || a.destination_id || undefined,
                                  resource_id: a.id,
                                })
                                .then((res) => {
                                  const url = res.fqdn || res.url || ''
                                  if (!url) throw new Error('No domain returned')
                                  setServiceDomains((d) => ({ ...d, [u.name]: url }))
                                  toast.success(`Domain for ${u.name}`)
                                })
                                .catch((err: Error) => toast.error(err.message || 'Generate failed'))
                            }}
                          >
                            Generate Domain
                          </Btn>
                        </div>
                      ))}
                  </div>
                ) : (
                  <div className="panel-card space-y-4 p-5">
                    <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Domains</h3>
                    <DomainsPanel
                      value={cfg.fqdn}
                      onChange={(v) => setCfg({ ...cfg, fqdn: v })}
                      onSave={(next) => {
                        setCfg((c) => ({ ...c, fqdn: next }))
                        save.mutate({ fqdn: next })
                      }}
                      saveBusy={save.isPending}
                      serverId={serverId || undefined}
                      destinationId={cfg.destination_id || a.destination_id || undefined}
                      resourceId={a.id}
                      resourceName={cfg.name || a.name}
                    />
                  </div>
                )}

                <div className="panel-card grid gap-4 p-5 sm:grid-cols-2">
                  <h3 className="text-sm font-semibold text-gray-900 dark:text-white sm:col-span-2">
                    Build
                  </h3>
                  {a.build_pack === 'dockercompose' ? (
                    <>
                      <Input
                        label="Base Directory"
                        value={cfg.base_directory}
                        onChange={(v) => setCfg({ ...cfg, base_directory: v })}
                        required={false}
                      />
                      <Input
                        label="Docker Compose Location"
                        value={cfg.docker_compose_location}
                        onChange={(v) => setCfg({ ...cfg, docker_compose_location: v })}
                        required={false}
                      />
                      <label className="flex items-center gap-3 text-sm sm:col-span-2">
                        <input
                          type="checkbox"
                          checked={cfg.is_preserve_repository_enabled}
                          onChange={(e) =>
                            setCfg({ ...cfg, is_preserve_repository_enabled: e.target.checked })
                          }
                        />
                        <span>Preserve Repository During Deployment</span>
                      </label>
                      <p className="text-xs text-gray-500 dark:text-gray-400 sm:col-span-2">
                        The following commands are for advanced use cases. Only modify them if you know
                        what you are doing.
                      </p>
                      <Input
                        label="Custom Build Command"
                        value={cfg.docker_compose_custom_build_command}
                        onChange={(v) =>
                          setCfg({ ...cfg, docker_compose_custom_build_command: v })
                        }
                        required={false}
                      />
                      <Input
                        label="Custom Start Command"
                        value={cfg.docker_compose_custom_start_command}
                        onChange={(v) =>
                          setCfg({ ...cfg, docker_compose_custom_start_command: v })
                        }
                        required={false}
                      />
                      <Input
                        label="Ports exposes (optional)"
                        value={cfg.ports_exposes}
                        onChange={(v) => setCfg({ ...cfg, ports_exposes: v })}
                        required={false}
                      />
                      <label className="block text-sm sm:col-span-2">
                        <span className="mb-1 block text-gray-500 dark:text-gray-400">Watch Paths</span>
                        <textarea
                          value={cfg.watch_paths}
                          onChange={(e) => setCfg({ ...cfg, watch_paths: e.target.value })}
                          rows={3}
                          placeholder="services/api/**"
                          className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
                        />
                      </label>
                    </>
                  ) : (
                    <>
                      <Input
                        label="Base Directory"
                        value={cfg.base_directory}
                        onChange={(v) => setCfg({ ...cfg, base_directory: v })}
                        required={false}
                      />
                      {a.build_pack === 'dockerfile' && !cfg.dockerfile ? (
                        <>
                          <Input
                            label="Dockerfile Location"
                            value={cfg.dockerfile_location}
                            onChange={(v) => setCfg({ ...cfg, dockerfile_location: v })}
                            required={false}
                          />
                          <Input
                            label="Docker Build Stage Target"
                            value={cfg.dockerfile_target_build}
                            onChange={(v) => setCfg({ ...cfg, dockerfile_target_build: v })}
                            required={false}
                          />
                        </>
                      ) : null}
                      {a.build_pack === 'dockerfile' && cfg.dockerfile ? (
                        <Input
                          label="Docker Build Stage Target"
                          value={cfg.dockerfile_target_build}
                          onChange={(v) => setCfg({ ...cfg, dockerfile_target_build: v })}
                          required={false}
                        />
                      ) : null}
                      <Input
                        label="Ports exposes"
                        value={cfg.ports_exposes}
                        onChange={(v) => setCfg({ ...cfg, ports_exposes: v })}
                        required={false}
                      />
                      <Input
                        label="Custom Docker Options"
                        value={cfg.custom_docker_run_options}
                        onChange={(v) => setCfg({ ...cfg, custom_docker_run_options: v })}
                        required={false}
                      />
                      <label className="block text-sm sm:col-span-2">
                        <span className="mb-1 block text-gray-500 dark:text-gray-400">Watch Paths</span>
                        <textarea
                          value={cfg.watch_paths}
                          onChange={(e) => setCfg({ ...cfg, watch_paths: e.target.value })}
                          rows={3}
                          placeholder="src/**"
                          className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
                        />
                      </label>
                    </>
                  )}
                  {a.build_pack === 'dockerimage' || (a.build_pack === 'dockerfile' && !cfg.dockerfile) ? (
                    <>
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
                    </>
                  ) : null}
                </div>

                {a.build_pack === 'dockerfile' && cfg.dockerfile ? (
                  <div className="panel-card space-y-3 p-5">
                    <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Dockerfile</h3>
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      Inline Dockerfile (no Git). ENV lines can be managed under Environment Variables.
                    </p>
                    <CodeEditor
                      language="dockerfile"
                      readOnly={false}
                      height="22rem"
                      value={cfg.dockerfile}
                      onChange={(v) => setCfg({ ...cfg, dockerfile: v })}
                      ariaLabel="Dockerfile content"
                    />
                  </div>
                ) : null}

                {a.build_pack === 'dockercompose' ? (
                  <div className="panel-card space-y-3 p-5">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <h3 className="text-sm font-semibold text-gray-900 dark:text-white">
                        Docker Compose
                      </h3>
                      {cfg.compose_prepare ? (
                        <Btn type="button" onClick={() => setShowRawCompose((v) => !v)}>
                          {showRawCompose ? 'Show Deployable Compose' : 'Show Raw Compose'}
                        </Btn>
                      ) : null}
                    </div>
                    {!a.docker_compose_raw && !a.docker_compose ? (
                      <p className="text-sm text-gray-500 dark:text-gray-400">
                        Click <strong>Load Compose File</strong> to fetch YAML from git and preview it
                        here.
                      </p>
                    ) : (
                      <>
                        <p className="text-xs text-gray-500 dark:text-gray-400">
                          {showRawCompose || !cfg.compose_prepare
                            ? 'Raw compose from the repository (edit in git).'
                            : 'Deployable compose after Dockfin adaptation (Traefik, network, ports).'}
                        </p>
                        <CodeEditor
                          language="yaml"
                          readOnly
                          height="26rem"
                          ariaLabel="Docker Compose YAML"
                          value={
                            showRawCompose || !cfg.compose_prepare
                              ? a.docker_compose_raw || ''
                              : a.docker_compose || a.docker_compose_raw || ''
                          }
                        />
                      </>
                    )}
                  </div>
                ) : null}

                {save.error && <p className="text-sm text-error-500">{save.error.message}</p>}
              </form>
            )}

            {side === 'advanced' && (
              <div className="space-y-6">
                <div>
                  <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Advanced</h2>
                  <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                    Health checks and deployment behaviour.
                  </p>
                </div>
                <form
                  className="panel-card space-y-4 p-5"
                  onSubmit={(e) => {
                    e.preventDefault()
                    save.mutate({})
                  }}
                >
                  <h3 className="text-sm font-medium text-gray-900 dark:text-white">Options</h3>
                  {a.build_pack === 'dockercompose' ? (
                    <fieldset className="space-y-3">
                      <legend className="text-sm text-gray-500 dark:text-gray-400">
                        Compose adaptation
                      </legend>
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
                            Traefik labels, proxy network, strip host ports.
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
                        <span>Don&apos;t modify — deploy compose as-is</span>
                      </label>
                    </fieldset>
                  ) : null}
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_auto_deploy_enabled}
                      onChange={(e) => setCfg({ ...cfg, is_auto_deploy_enabled: e.target.checked })}
                    />
                    <span>Auto deploy on git push / webhook</span>
                  </label>
                  <p className="ml-7 text-xs text-gray-500 dark:text-gray-400">
                    Skipped when every commit message contains [skip ci] or [skip cd].
                  </p>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_git_submodules_enabled}
                      onChange={(e) => setCfg({ ...cfg, is_git_submodules_enabled: e.target.checked })}
                    />
                    <span>Include Git submodules</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_force_https}
                      onChange={(e) => setCfg({ ...cfg, is_force_https: e.target.checked })}
                    />
                    <span>Force HTTPS redirects</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_preview_enabled}
                      onChange={(e) => setCfg({ ...cfg, is_preview_enabled: e.target.checked })}
                    />
                    <span>Enable preview deployments</span>
                  </label>
                  {a.build_pack !== 'dockercompose' ? (
                    <label className="flex items-center gap-3 text-sm">
                      <input
                        type="checkbox"
                        checked={cfg.is_build_server_enabled}
                        onChange={(e) => setCfg({ ...cfg, is_build_server_enabled: e.target.checked })}
                      />
                      <span>Build on dedicated build server</span>
                    </label>
                  ) : null}
                  <Btn primary type="submit" disabled={save.isPending}>
                    {save.isPending ? 'Saving…' : 'Save'}
                  </Btn>
                </form>

                <form
                  className="panel-card space-y-4 p-5"
                  onSubmit={(e) => {
                    e.preventDefault()
                    save.mutate({})
                  }}
                >
                  <h3 className="text-sm font-medium text-gray-900 dark:text-white">
                    Deployment commands
                  </h3>
                  <label className="block text-sm">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Pre-deployment command
                    </span>
                    <textarea
                      rows={3}
                      value={cfg.pre_deployment_command}
                      onChange={(e) => setCfg({ ...cfg, pre_deployment_command: e.target.value })}
                      className="panel-field w-full rounded-lg px-3 py-2 font-mono text-sm outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
                      placeholder="Runs in the app workdir after clone / before build"
                    />
                  </label>
                  <label className="block text-sm">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Post-deployment command
                    </span>
                    <textarea
                      rows={3}
                      value={cfg.post_deployment_command}
                      onChange={(e) => setCfg({ ...cfg, post_deployment_command: e.target.value })}
                      className="panel-field w-full rounded-lg px-3 py-2 font-mono text-sm outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
                      placeholder="Runs after successful cutover (docker exec when possible)"
                    />
                  </label>
                  <Btn primary type="submit" disabled={save.isPending}>
                    {save.isPending ? 'Saving…' : 'Save commands'}
                  </Btn>
                </form>

                <form
                  className="panel-card space-y-4 p-5"
                  onSubmit={(e) => {
                    e.preventDefault()
                    save.mutate({})
                  }}
                >
                  <h3 className="text-sm font-medium text-gray-900 dark:text-white">
                    HTTP Basic Auth &amp; Labels
                  </h3>
                  <Input
                    label="Basic auth username"
                    value={cfg.http_basic_auth_username}
                    onChange={(v) => setCfg({ ...cfg, http_basic_auth_username: v })}
                    required={false}
                  />
                  <label className="block text-sm">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Basic auth password
                      {a.has_http_basic_auth ? ' (set — leave blank to keep)' : ''}
                    </span>
                    <input
                      type="password"
                      autoComplete="new-password"
                      value={httpBasicAuthPassword}
                      onChange={(e) => setHttpBasicAuthPassword(e.target.value)}
                      className="panel-field w-full rounded-lg px-3 py-2 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
                    />
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={clearHttpBasicAuth}
                      onChange={(e) => setClearHttpBasicAuth(e.target.checked)}
                    />
                    <span>Clear HTTP basic auth</span>
                  </label>
                  <label className="block text-sm">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Custom labels (one key=value per line)
                    </span>
                    <textarea
                      rows={4}
                      value={cfg.custom_labels}
                      onChange={(e) => setCfg({ ...cfg, custom_labels: e.target.value })}
                      className="panel-field w-full rounded-lg px-3 py-2 font-mono text-sm outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
                    />
                  </label>
                  <Btn primary type="submit" disabled={save.isPending}>
                    {save.isPending ? 'Saving…' : 'Save auth & labels'}
                  </Btn>
                </form>

                {a.build_pack !== 'dockercompose' ? (
                <form
                  className="panel-card space-y-4 p-5"
                  onSubmit={(e) => {
                    e.preventDefault()
                    saveHealth.mutate()
                  }}
                >
                  <h3 className="text-sm font-medium text-gray-900 dark:text-white">Health Checks</h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    After deploy, Dockfin probes this HTTP endpoint inside the container.
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
                      onChange={(v) =>
                        setHealth({ ...health, health_check_retries: Number(v) || 10 })
                      }
                    />
                  </div>
                  {saveHealth.error && (
                    <p className="text-sm text-error-500">{saveHealth.error.message}</p>
                  )}
                  <Btn primary type="submit">
                    {saveHealth.isPending ? 'Saving…' : 'Save health checks'}
                  </Btn>
                </form>
                ) : (
                  <div className="panel-card p-5 text-sm text-gray-500 dark:text-gray-400">
                    Health checks for Docker Compose apps are defined in the compose file (or leave
                    adaptation mode to Dockfin Traefik routing).
                  </div>
                )}
              </div>
            )}

            {side === 'git' && (
              <form
                className="space-y-4"
                onSubmit={(e) => {
                  e.preventDefault()
                  save.mutate({})
                }}
              >
                <div>
                  <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Git Source</h2>
                  <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                    Repository and branch used for deployments.
                  </p>
                </div>
                <div className="panel-card grid gap-4 p-5 sm:grid-cols-2">
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
                  {(gitSources.data?.git_sources || []).length > 0 && (
                    <label className="block text-sm sm:col-span-2">
                      <span className="mb-1 block text-gray-500 dark:text-gray-400">GitHub App</span>
                      <select
                        value={cfg.git_source_id}
                        onChange={(e) => setCfg({ ...cfg, git_source_id: e.target.value })}
                        className="panel-field w-full rounded-lg px-3 py-2"
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
                  <label className="block text-sm sm:col-span-2">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Deploy key (SSH private key)
                    </span>
                    <select
                      value={cfg.private_key_id}
                      onChange={(e) => setCfg({ ...cfg, private_key_id: e.target.value })}
                      className="panel-field w-full rounded-lg px-3 py-2"
                    >
                      <option value="">None</option>
                      {(keys.data?.private_keys || []).map((k) => (
                        <option key={k.id} value={k.id}>
                          {k.name}
                        </option>
                      ))}
                    </select>
                    <span className="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                      Clear when using a GitHub App for private repos.
                    </span>
                  </label>
                </div>
                {save.error && <p className="text-sm text-error-500">{save.error.message}</p>}
                <Btn primary type="submit">
                  {save.isPending ? 'Saving…' : 'Save'}
                </Btn>
              </form>
            )}

            {side === 'servers' && (
              <form
                className="space-y-4"
                onSubmit={(e) => {
                  e.preventDefault()
                  save.mutate({})
                }}
              >
                <div>
                  <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Servers</h2>
                  <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                    Destination where this application is deployed.
                  </p>
                </div>
                <div className="panel-card space-y-4 p-5">
                  <label className="block text-sm">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">Destination</span>
                    <select
                      value={cfg.destination_id}
                      onChange={(e) => setCfg({ ...cfg, destination_id: e.target.value })}
                      className="panel-field w-full rounded-lg px-3 py-2"
                    >
                      <option value="">Select destination</option>
                      {(dests.data?.destinations || []).map((d) => (
                        <option key={d.id} value={d.id}>
                          {d.name} ({d.network})
                        </option>
                      ))}
                    </select>
                  </label>
                  {serverId ? (
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      Server ID:{' '}
                      <Link
                        to="/servers/$serverId"
                        params={{ serverId }}
                        className="font-mono text-brand-600 hover:underline dark:text-brand-400"
                      >
                        {serverId.slice(0, 8)}…
                      </Link>
                    </p>
                  ) : null}
                </div>
                <Btn primary type="submit">
                  {save.isPending ? 'Saving…' : 'Save'}
                </Btn>
              </form>
            )}

            {side === 'storages' && (
              <div className="space-y-4">
                <div>
                  <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                    Persistent Storage
                  </h2>
                  <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                    {a.build_pack === 'dockercompose'
                      ? 'Volumes come from your compose file. Edit YAML and Load Compose to refresh.'
                      : 'Persistent volumes mounted into the container on the next deploy.'}
                  </p>
                </div>
                {a.build_pack === 'dockercompose' ? (
                  <PersistentStoragesPanel
                    compose={a.docker_compose_raw || a.docker_compose || ''}
                    volumes={a.compose_volumes || []}
                  />
                ) : (
                  <PersistentStoragesPanel applicationId={appId} editable />
                )}
              </div>
            )}

            {side === 'metrics' && (
              <AppMetricsSection serverId={serverId} />
            )}

            {side === 'tags' && (
              <div className="space-y-4">
                <div>
                  <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Tags</h2>
                  <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                    Organize this application with tags.
                  </p>
                </div>
                <ResourceTagsPanel resourceType="application" resourceId={appId} />
              </div>
            )}

            {side === 'limits' && (
              <div>
                <form
                  className="space-y-4"
                  onSubmit={(e) => {
                    e.preventDefault()
                    saveLimits.mutate()
                  }}
                >
                  <div>
                    <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                      Resource Limits
                    </h2>
                    <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                      Docker resource limits applied on the next deploy. Leave empty for unlimited.
                    </p>
                  </div>
                  <div className="panel-card grid gap-4 p-5 sm:grid-cols-2">
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
                    <p className="text-xs text-gray-500 dark:text-gray-400 sm:col-span-2">
                      Examples: memory <code className="font-mono">512m</code> /{' '}
                      <code className="font-mono">1g</code>, CPUs{' '}
                      <code className="font-mono">0.5</code> / <code className="font-mono">2</code>.
                    </p>
                  </div>
                  {saveLimits.error && (
                    <p className="text-sm text-error-500">{saveLimits.error.message}</p>
                  )}
                  <Btn primary type="submit">
                    {saveLimits.isPending ? 'Saving…' : 'Save limits'}
                  </Btn>
                </form>
              </div>
            )}

            {side === 'environment' && (
              <div>
                <div className="mb-3">
                  <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                    Environment Variables
                  </h2>
                </div>
                <EnvVarsPanel
                  resourceType="application"
                  resourceId={appId}
                  title=""
                  previewTabs={Boolean(a.is_preview_enabled || cfg.is_preview_enabled)}
                />
              </div>
            )}

            {side === 'previews' && (
              <div>
                <div className="mb-3">
                  <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                    Preview Deployments
                  </h2>
                  <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                    Pull-request preview deployments for this application.
                    {!cfg.is_preview_enabled && (
                      <> Enable them under Advanced to accept PR webhooks.</>
                    )}{' '}
                    Preview-specific env vars live under Environment Variables → Preview.
                  </p>
                </div>
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
                                void (async () => {
                                  if (
                                    await confirm({
                                      title: 'Delete preview',
                                      message: `Delete preview for PR #${p.pull_request_id}?`,
                                      confirmLabel: 'Delete',
                                      danger: true,
                                    })
                                  ) {
                                    deletePreview.mutate(p.pull_request_id)
                                  }
                                })()
                              }}
                            >
                              Delete
                            </button>
                          </td>
                        </tr>
                      ))}
                      {!previews.data?.previews?.length && (
                        <tr>
                          <td
                            colSpan={6}
                            className="px-4 py-8 text-center text-gray-500 dark:text-gray-400"
                          >
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
            )}

            {side === 'webhooks' && (
              <div>
                <div className="mb-3">
                  <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Webhooks</h2>
                </div>
                <div className="panel-card space-y-4 p-5">
                  <div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">Git webhook URL</div>
                    <code className="mt-1 block break-all font-mono text-sm text-gray-900 dark:text-white">
                      {webhookUrl}
                    </code>
                  </div>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    Point GitHub, GitLab, Gitea, or Bitbucket webhooks here (optional{' '}
                    <code className="text-xs">?provider=github|gitlab|gitea|bitbucket</code>
                    ). Use the same secret for HMAC / GitLab token. Commits with{' '}
                    <code className="text-xs">[skip ci]</code> or{' '}
                    <code className="text-xs">[skip cd]</code> in every message skip auto-deploy;
                    PR/MR close events clean up preview deployments.
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
              </div>
            )}

            {side === 'tasks' && (
              <div>
                <ScheduledTasksPanel resourceType="application" resourceId={appId} />
              </div>
            )}

            {side === 'operations' && (
              <div className="space-y-4">
                <div>
                  <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                    Resource Operations
                  </h2>
                </div>
                <div className="panel-card space-y-3 p-5">
                  <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Deploy</h3>
                  <div className="flex flex-wrap gap-2">
                    <Btn primary onClick={() => deploy.mutate({})}>
                      Redeploy
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
            )}

            {side === 'rollback' && (
              <div className="space-y-4">
                <div>
                  <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Rollback</h2>
                  <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                    Redeploy a previous finished commit. This queues a new deployment with force rebuild.
                  </p>
                </div>
                <div className="panel-card overflow-hidden">
                  <table className="w-full text-left text-sm">
                    <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
                      <tr>
                        <th className="px-3 py-2">Commit</th>
                        <th className="px-3 py-2">Message</th>
                        <th className="px-3 py-2">When</th>
                        <th className="px-3 py-2" />
                      </tr>
                    </thead>
                    <tbody>
                      {(deps.data?.deployments || [])
                        .filter((d) => d.status === 'finished' && d.commit_sha)
                        .slice(0, 15)
                        .map((d, idx) => (
                          <tr key={d.id} className="border-t border-gray-200 dark:border-gray-800">
                            <td className="px-3 py-2 font-mono text-xs">{(d.commit_sha || '').slice(0, 8)}</td>
                            <td className="px-3 py-2">{d.commit_message || (idx === 0 ? 'Latest' : '—')}</td>
                            <td className="px-3 py-2 text-xs text-gray-500">
                              {d.created_at || ''}
                            </td>
                            <td className="px-3 py-2 text-right">
                              {idx === 0 ? (
                                <span className="text-xs text-gray-400">current</span>
                              ) : (
                                <Btn
                                  type="button"
                                  onClick={() => rollback.mutate(d.commit_sha)}
                                  disabled={rollback.isPending || !d.commit_sha}
                                >
                                  Rollback
                                </Btn>
                              )}
                            </td>
                          </tr>
                        ))}
                      {!((deps.data?.deployments || []).filter((d) => d.status === 'finished' && d.commit_sha).length) && (
                        <tr>
                          <td colSpan={4} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                            No finished deployments yet.
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
                {rollback.error && <p className="text-sm text-error-500">{rollback.error.message}</p>}
              </div>
            )}

            {side === 'danger' && (
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
                      This will stop your containers, delete related data on the server, and remove
                      the application from Dockfin. Beware — there is no coming back.
                    </p>
                    <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
                      Container status:{' '}
                      <span className="font-medium capitalize">{a.status || 'unknown'}</span>
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
            )}
          </div>
        </div>
      )}

      {topTab === 'deployments' && (
        <div>
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Deployments</h2>
            <Btn primary onClick={() => deploy.mutate({})}>
              Redeploy
            </Btn>
          </div>
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
                        Logs
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
        </div>
      )}

      {topTab === 'logs' && (
        <div className="space-y-6">
          <LiveContainerLogs appId={appId} isCompose={a.build_pack === 'dockercompose'} />
          <div className="space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                  Recent deployments
                </h2>
                <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                  Build / compose history — open a deployment for full deploy logs.
                </p>
              </div>
              <Btn primary onClick={() => deploy.mutate({})}>
                Redeploy
              </Btn>
            </div>
            <div className="panel-card divide-y divide-gray-200 dark:divide-gray-800">
              {(deps.data?.deployments || []).slice(0, 8).map((d) => (
                <button
                  key={d.id}
                  type="button"
                  className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left text-sm hover:bg-gray-50 dark:hover:bg-white/5"
                  onClick={() => openDeployment(d.id)}
                >
                  <span className="font-mono text-xs text-gray-500">{d.id.slice(0, 8)}…</span>
                  <span className="capitalize">{d.status}</span>
                  <span className="text-xs text-gray-500">{d.current_stage || '—'}</span>
                  <span className="text-brand-600 dark:text-brand-400">View logs →</span>
                </button>
              ))}
              {!deps.data?.deployments?.length && (
                <p className="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
                  No deployment logs yet. Click Redeploy to start one.
                </p>
              )}
            </div>
          </div>
        </div>
      )}

      {topTab === 'terminal' && (
        <div>
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
        </div>
      )}

      {topTab === 'links' && <LinksPanel links={a.links || []} />}
    </div>
  )
}

function AppMetricsSection({ serverId }: { serverId: string }) {
  const metrics = useQuery({
    queryKey: ['server-metrics', serverId],
    queryFn: () => api.serverMetrics(serverId, 60),
    enabled: Boolean(serverId),
    refetchInterval: 15000,
  })
  const list = metrics.data?.metrics || []
  const latest = list[list.length - 1]
  const cpu = list.map((m) => m.cpu_percent)
  const memPct = list.map((m) =>
    m.memory_total_bytes > 0 ? (m.memory_used_bytes / m.memory_total_bytes) * 100 : 0,
  )
  const diskPct = list.map((m) =>
    m.disk_total_bytes > 0 ? (m.disk_used_bytes / m.disk_total_bytes) * 100 : 0,
  )
  const fmtGiB = (n: number) => `${(n / 1024 ** 3).toFixed(1)} GiB`

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Metrics</h2>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
          Host metrics for the server running this application.
        </p>
      </div>
      {!serverId ? (
        <div className="panel-card p-5 text-sm text-gray-500 dark:text-gray-400">
          Select a destination under Servers first.
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="panel-card p-4">
              <div className="text-xs text-gray-500 dark:text-gray-400">CPU</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">
                {latest ? `${latest.cpu_percent.toFixed(1)}%` : '—'}
              </div>
              <MiniSpark values={cpu} color="#0d9488" />
            </div>
            <div className="panel-card p-4">
              <div className="text-xs text-gray-500 dark:text-gray-400">Memory</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">
                {latest && latest.memory_total_bytes
                  ? `${((latest.memory_used_bytes / latest.memory_total_bytes) * 100).toFixed(0)}%`
                  : '—'}
              </div>
              <div className="text-xs text-gray-500 dark:text-gray-400">
                {latest ? `${fmtGiB(latest.memory_used_bytes)} / ${fmtGiB(latest.memory_total_bytes)}` : ''}
              </div>
              <MiniSpark values={memPct} color="#2563eb" />
            </div>
            <div className="panel-card p-4">
              <div className="text-xs text-gray-500 dark:text-gray-400">Disk</div>
              <div className="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">
                {latest && latest.disk_total_bytes
                  ? `${((latest.disk_used_bytes / latest.disk_total_bytes) * 100).toFixed(0)}%`
                  : '—'}
              </div>
              <MiniSpark values={diskPct} color="#d97706" />
            </div>
          </div>
          <Link
            to="/servers/$serverId"
            params={{ serverId }}
            className="text-sm font-medium text-brand-600 hover:underline dark:text-brand-400"
          >
            Open server page →
          </Link>
          {metrics.error && <p className="text-sm text-error-500">{metrics.error.message}</p>}
          {!metrics.isLoading && !list.length && (
            <div className="panel-card p-6 text-center text-sm text-gray-500 dark:text-gray-400">
              No metrics yet. Ensure Sentinel is posting to the ingest endpoint.
            </div>
          )}
        </>
      )}
    </div>
  )
}

function MiniSpark({ values, color }: { values: number[]; color: string }) {
  if (!values.length) return <div className="mt-2 h-12" />
  const w = 280
  const h = 48
  const max = Math.max(100, ...values, 1)
  const pts = values
    .map((v, i) => {
      const x = values.length === 1 ? 0 : (i / (values.length - 1)) * w
      const y = h - (v / max) * (h - 4) - 2
      return `${x},${y}`
    })
    .join(' ')
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="mt-2 h-12 w-full" preserveAspectRatio="none">
      <polyline fill="none" stroke={color} strokeWidth="2" points={pts} />
    </svg>
  )
}

function LiveContainerLogs({ appId, isCompose }: { appId: string; isCompose: boolean }) {
  const [container, setContainer] = useState('')
  const [lines, setLines] = useState<string[]>([])
  const [status, setStatus] = useState<'connecting' | 'live' | 'ended' | 'error'>('connecting')
  const [error, setError] = useState('')
  const [nonce, setNonce] = useState(0)

  const containers = useQuery({
    queryKey: ['app-containers', appId],
    queryFn: () => api.applicationContainers(appId),
    enabled: isCompose,
  })

  useEffect(() => {
    if (!isCompose) {
      setContainer(`dockfin-${appId}`)
      return
    }
    const list = containers.data?.containers || []
    if (list.length && (!container || !list.includes(container))) {
      setContainer(list[0])
    }
  }, [isCompose, appId, containers.data, container])

  useEffect(() => {
    if (!container && isCompose) return
    const name = container || `dockfin-${appId}`
    setLines([])
    setStatus('connecting')
    setError('')
    const qs = new URLSearchParams({ tail: '200', container: name })
    const es = new EventSource(`/api/v1/applications/${appId}/logs/stream?${qs}`, {
      withCredentials: true,
    })
    es.addEventListener('log', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as { line?: string }
        setLines((prev) => [...prev.slice(-2000), data.line ?? (ev as MessageEvent).data])
        setStatus('live')
      } catch {
        setLines((prev) => [...prev.slice(-2000), (ev as MessageEvent).data])
        setStatus('live')
      }
    })
    es.addEventListener('done', () => {
      setStatus('ended')
      es.close()
    })
    es.onerror = () => {
      setStatus((s) => (s === 'live' ? 'ended' : 'error'))
      setError('Stream disconnected')
      es.close()
    }
    return () => es.close()
  }, [appId, container, isCompose, nonce])

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Live container logs</h2>
          <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
            Streaming <code className="font-mono text-xs">docker logs -f</code> from the destination.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {isCompose ? (
            <label className="text-sm">
              <span className="sr-only">Container</span>
              <select
                value={container}
                onChange={(e) => setContainer(e.target.value)}
                className="panel-field rounded-lg px-3 py-1.5 font-mono text-xs"
              >
                {(containers.data?.containers || []).map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
                {!containers.data?.containers?.length && (
                  <option value="">No containers</option>
                )}
              </select>
            </label>
          ) : null}
          <span
            className={`text-xs font-medium ${
              status === 'live'
                ? 'text-emerald-600 dark:text-emerald-400'
                : status === 'error'
                  ? 'text-error-500'
                  : 'text-gray-500'
            }`}
          >
            {status === 'connecting' ? 'Connecting…' : status === 'live' ? 'Live' : status === 'ended' ? 'Ended' : 'Error'}
          </span>
          <Btn type="button" onClick={() => setNonce((n) => n + 1)}>
            Reconnect
          </Btn>
        </div>
      </div>
      {error && status === 'error' ? (
        <p className="text-sm text-error-500">{error}</p>
      ) : null}
      <pre className="panel-card max-h-[28rem] overflow-auto p-4 font-mono text-xs leading-relaxed text-gray-800 dark:text-gray-200">
        {lines.length ? lines.join('\n') : 'Waiting for log lines…'}
      </pre>
    </div>
  )
}
