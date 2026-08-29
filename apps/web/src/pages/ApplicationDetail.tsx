import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams, useSearch } from '@tanstack/react-router'
import {
  Activity,
  AlertTriangle,
  Archive,
  CalendarClock,
  Gauge,
  ExternalLink,
  GitBranch,
  GitPullRequest,
  HardDrive,
  History,
  LayoutDashboard,
  Rocket,
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
import { AppOverview, DeploymentRows, primaryVisitUrl } from '../components/AppOverview'
import { ConfigSideNav } from '../components/ConfigSideNav'
import { DangerConfirmModal, DangerZoneCard } from '../components/DangerConfirmModal'
import { DomainsPanel, domainsWantAutoHttps, normalizeDomains } from '../components/DomainsPanel'
import { EnvVarsPanel } from '../components/EnvVarsPanel'
import { LinksMenu } from '../components/LinksMenu'
import { MoveResourcePanel } from '../components/MoveResourcePanel'
import { PersistentStoragesPanel } from '../components/PersistentStoragesPanel'
import { ResourceSetupBanner, type SetupCheck } from '../components/ResourceSetupBanner'
import { ResourceSwitcher } from '../components/ResourceSwitcher'
import { BackLink } from '../components/BackLink'
import { ServiceLogo, logoForApplication } from '../components/ServiceLogo'
import { StatusBadge } from '../components/StatusBadge'
import { ResourceTagsPanel } from '../components/ResourceTagsPanel'
import { ScheduledTasksPanel } from '../components/ScheduledTasksPanel'
import { ServerTerminal } from '../components/Terminal'
import { CodeEditor } from '../components/CodeEditor'
import { LiveLogViewer } from '../components/LiveLogViewer'
import { DetailPageSkeleton, PanelSkeleton, TableSkeleton } from '../components/ui/Skeleton'
import { DetailMoreItem, DetailMoreMenu } from '../components/DetailMoreMenu'
import { useToast } from '../components/Toast'
import { api, fetchAllEnvironments } from '../lib/api'
import { deployBlockFromEnv, emptyUserEnvVars } from '../lib/env-readiness'
import { gentleRefetchInterval } from '../lib/poll'
import { useLogStream } from '../lib/useLogStream'
import { Btn, Input } from './Servers'

/** Vercel-style IA — Overview first, Settings last. */
const TOP_TABS = [
  { id: 'overview', label: 'Overview', icon: LayoutDashboard },
  { id: 'deployments', label: 'Deployments', icon: History },
  { id: 'logs', label: 'Logs', icon: ScrollText },
  { id: 'terminal', label: 'Terminal', icon: Terminal },
  { id: 'backups', label: 'Backups', icon: Archive },
  { id: 'configuration', label: 'Settings', icon: Settings2 },
] as const

type AppCfg = {
  name: string
  description: string
  build_pack: string
  fqdn: string
  git_repository: string
  git_branch: string
  ports_exposes: string
  ports_mappings: string
  custom_network_aliases: string
  install_command: string
  build_command: string
  start_command: string
  publish_directory: string
  custom_nginx_configuration: string
  preview_url_template: string
  docker_compose_location: string
  compose_prepare: boolean
  base_directory: string
  docker_compose_custom_build_command: string
  docker_compose_custom_start_command: string
  custom_docker_run_options: string
  dockerfile_location: string
  dockerfile: string
  dockerfile_target_build: string
  docker_registry_image_name: string
  docker_registry_image_tag: string
  docker_registry_id: string
  destination_id: string
  git_source_id: string
  private_key_id: string
  is_build_server_enabled: boolean
  is_force_https: boolean
  is_preview_enabled: boolean
  is_auto_deploy_enabled: boolean
  is_git_submodules_enabled: boolean
  is_preserve_repository_enabled: boolean
  is_disable_build_cache: boolean
  is_git_shallow_clone_enabled: boolean
  is_git_lfs_enabled: boolean
  is_gpu_enabled: boolean
  gpu_count: number
  custom_docker_stop_timeout: number
  custom_docker_restart_policy: string
  redirect: string
  watch_paths: string
  pre_deployment_command: string
  post_deployment_command: string
  custom_labels: string
  http_basic_auth_username: string
  is_spa: boolean
  inject_build_args_to_dockerfile: boolean
  use_build_secrets: boolean
  include_source_commit_in_build: boolean
  docker_images_to_keep: number
  is_consistent_container_name_enabled: boolean
  custom_internal_name: string
  is_gzip_enabled: boolean
  is_stripprefix_enabled: boolean
  is_log_drain_enabled: boolean
  is_debug_enabled: boolean
  is_env_sorting_enabled: boolean
  is_pr_deployments_public_enabled: boolean
  skip_rebuild_if_unchanged: boolean
  gpu_driver: string
  gpu_device_ids: string
  gpu_options: string
  custom_docker_max_restart_count: number
  pre_deployment_command_container: string
  post_deployment_command_container: string
  is_swarm_only_worker_nodes: boolean
  is_include_timestamps: boolean
  logs_line_limit: number
  swarm_replicas: number
  swarm_placement_constraints: string
}

function emptyAppCfg(): AppCfg {
  return {
    name: '',
    description: '',
    build_pack: 'dockerfile',
    fqdn: '',
    git_repository: '',
    git_branch: '',
    ports_exposes: '',
    ports_mappings: '',
    custom_network_aliases: '',
    install_command: '',
    build_command: '',
    start_command: '',
    publish_directory: '',
    custom_nginx_configuration: '',
    preview_url_template: '{{pr_id}}.{{domain}}',
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
    docker_registry_id: '',
    destination_id: '',
    git_source_id: '',
    private_key_id: '',
    is_build_server_enabled: false,
    is_force_https: true,
    is_preview_enabled: false,
    is_auto_deploy_enabled: true,
    is_git_submodules_enabled: false,
    is_preserve_repository_enabled: false,
    is_disable_build_cache: false,
    is_git_shallow_clone_enabled: true,
    is_git_lfs_enabled: false,
    is_gpu_enabled: false,
    gpu_count: 0,
    custom_docker_stop_timeout: 0,
    custom_docker_restart_policy: 'unless-stopped',
    redirect: 'both',
    watch_paths: '',
    pre_deployment_command: '',
    post_deployment_command: '',
    custom_labels: '',
    http_basic_auth_username: '',
    is_spa: false,
    inject_build_args_to_dockerfile: true,
    use_build_secrets: false,
    include_source_commit_in_build: false,
    docker_images_to_keep: 5,
    is_consistent_container_name_enabled: false,
    custom_internal_name: '',
    is_gzip_enabled: true,
    is_stripprefix_enabled: true,
    is_log_drain_enabled: false,
    is_debug_enabled: false,
    is_env_sorting_enabled: true,
    is_pr_deployments_public_enabled: false,
    skip_rebuild_if_unchanged: true,
    gpu_driver: 'nvidia',
    gpu_device_ids: '',
    gpu_options: '',
    custom_docker_max_restart_count: 0,
    pre_deployment_command_container: '',
    post_deployment_command_container: '',
    is_swarm_only_worker_nodes: false,
    is_include_timestamps: false,
    logs_line_limit: 1000,
    swarm_replicas: 1,
    swarm_placement_constraints: '',
  }
}

function appCfgFromData(updated: {
  name?: string
  description?: string
  build_pack?: string
  fqdn?: string
  git_repository?: string
  git_branch?: string
  ports_exposes?: string
  ports_mappings?: string
  custom_network_aliases?: string
  install_command?: string
  build_command?: string
  start_command?: string
  publish_directory?: string
  custom_nginx_configuration?: string
  preview_url_template?: string
  docker_compose_location?: string
  compose_prepare?: boolean
  base_directory?: string
  docker_compose_custom_build_command?: string
  docker_compose_custom_start_command?: string
  custom_docker_run_options?: string
  dockerfile_location?: string
  dockerfile?: string
  dockerfile_target_build?: string
  docker_registry_image_name?: string
  docker_registry_image_tag?: string
  docker_registry_id?: string | null
  destination_id?: string | null
  git_source_id?: string | null
  private_key_id?: string | null
  is_build_server_enabled?: boolean
  is_force_https?: boolean
  is_preview_enabled?: boolean
  is_auto_deploy_enabled?: boolean
  is_git_submodules_enabled?: boolean
  is_preserve_repository_enabled?: boolean
  is_disable_build_cache?: boolean
  is_git_shallow_clone_enabled?: boolean
  is_git_lfs_enabled?: boolean
  is_gpu_enabled?: boolean
  gpu_count?: number
  custom_docker_stop_timeout?: number
  custom_docker_restart_policy?: string
  redirect?: string
  watch_paths?: string
  pre_deployment_command?: string
  post_deployment_command?: string
  custom_labels?: string
  http_basic_auth_username?: string
  is_spa?: boolean
  inject_build_args_to_dockerfile?: boolean
  use_build_secrets?: boolean
  include_source_commit_in_build?: boolean
  docker_images_to_keep?: number
  is_consistent_container_name_enabled?: boolean
  custom_internal_name?: string
  is_gzip_enabled?: boolean
  is_stripprefix_enabled?: boolean
  is_log_drain_enabled?: boolean
  is_debug_enabled?: boolean
  is_env_sorting_enabled?: boolean
  is_pr_deployments_public_enabled?: boolean
  skip_rebuild_if_unchanged?: boolean
  gpu_driver?: string
  gpu_device_ids?: string
  gpu_options?: string
  custom_docker_max_restart_count?: number
  pre_deployment_command_container?: string
  post_deployment_command_container?: string
  is_swarm_only_worker_nodes?: boolean
  is_include_timestamps?: boolean
  logs_line_limit?: number
  swarm_replicas?: number
  swarm_placement_constraints?: string
}): AppCfg {
  return {
    name: updated.name || '',
    description: updated.description || '',
    build_pack: updated.build_pack || 'dockerfile',
    fqdn: updated.fqdn || '',
    git_repository: updated.git_repository || '',
    git_branch: updated.git_branch || 'main',
    ports_exposes:
      updated.build_pack === 'dockercompose'
        ? updated.ports_exposes || ''
        : updated.ports_exposes || '80',
    ports_mappings: updated.ports_mappings || '',
    custom_network_aliases: updated.custom_network_aliases || '',
    install_command: updated.install_command || '',
    build_command: updated.build_command || '',
    start_command: updated.start_command || '',
    publish_directory: updated.publish_directory || '',
    custom_nginx_configuration: updated.custom_nginx_configuration || '',
    preview_url_template: updated.preview_url_template || '{{pr_id}}.{{domain}}',
    docker_compose_location: updated.docker_compose_location || '',
    compose_prepare: updated.compose_prepare !== false,
    base_directory: updated.base_directory || '/',
    docker_compose_custom_build_command: updated.docker_compose_custom_build_command || '',
    docker_compose_custom_start_command: updated.docker_compose_custom_start_command || '',
    custom_docker_run_options: updated.custom_docker_run_options || '',
    dockerfile_location: updated.dockerfile_location || '/Dockerfile',
    dockerfile: updated.dockerfile || '',
    dockerfile_target_build: updated.dockerfile_target_build || '',
    docker_registry_image_name: updated.docker_registry_image_name || '',
    docker_registry_image_tag: updated.docker_registry_image_tag || '',
    docker_registry_id: updated.docker_registry_id || '',
    destination_id: updated.destination_id || '',
    git_source_id: updated.git_source_id || '',
    private_key_id: updated.private_key_id || '',
    is_build_server_enabled: Boolean(updated.is_build_server_enabled),
    is_force_https: updated.is_force_https !== false,
    is_preview_enabled: Boolean(updated.is_preview_enabled),
    is_auto_deploy_enabled: updated.is_auto_deploy_enabled !== false,
    is_git_submodules_enabled: Boolean(updated.is_git_submodules_enabled),
    is_preserve_repository_enabled: Boolean(updated.is_preserve_repository_enabled),
    is_disable_build_cache: Boolean(updated.is_disable_build_cache),
    is_git_shallow_clone_enabled: updated.is_git_shallow_clone_enabled !== false,
    is_git_lfs_enabled: Boolean(updated.is_git_lfs_enabled),
    is_gpu_enabled: Boolean(updated.is_gpu_enabled),
    gpu_count: updated.gpu_count ?? 0,
    custom_docker_stop_timeout: updated.custom_docker_stop_timeout ?? 0,
    custom_docker_restart_policy: updated.custom_docker_restart_policy || 'unless-stopped',
    redirect: updated.redirect || 'both',
    watch_paths: updated.watch_paths || '',
    pre_deployment_command: updated.pre_deployment_command || '',
    post_deployment_command: updated.post_deployment_command || '',
    custom_labels: updated.custom_labels || '',
    http_basic_auth_username: updated.http_basic_auth_username || '',
    is_spa: Boolean(updated.is_spa),
    inject_build_args_to_dockerfile: updated.inject_build_args_to_dockerfile !== false,
    use_build_secrets: Boolean(updated.use_build_secrets),
    include_source_commit_in_build: Boolean(updated.include_source_commit_in_build),
    docker_images_to_keep: updated.docker_images_to_keep ?? 5,
    is_consistent_container_name_enabled: Boolean(updated.is_consistent_container_name_enabled),
    custom_internal_name: updated.custom_internal_name || '',
    is_gzip_enabled: updated.is_gzip_enabled !== false,
    is_stripprefix_enabled: updated.is_stripprefix_enabled !== false,
    is_log_drain_enabled: Boolean(updated.is_log_drain_enabled),
    is_debug_enabled: Boolean(updated.is_debug_enabled),
    is_env_sorting_enabled: updated.is_env_sorting_enabled !== false,
    is_pr_deployments_public_enabled: Boolean(updated.is_pr_deployments_public_enabled),
    skip_rebuild_if_unchanged: updated.skip_rebuild_if_unchanged !== false,
    gpu_driver: updated.gpu_driver || 'nvidia',
    gpu_device_ids: updated.gpu_device_ids || '',
    gpu_options: updated.gpu_options || '',
    custom_docker_max_restart_count: updated.custom_docker_max_restart_count ?? 0,
    pre_deployment_command_container: updated.pre_deployment_command_container || '',
    post_deployment_command_container: updated.post_deployment_command_container || '',
    is_swarm_only_worker_nodes: Boolean(updated.is_swarm_only_worker_nodes),
    is_include_timestamps: Boolean(updated.is_include_timestamps),
    logs_line_limit: updated.logs_line_limit ?? 1000,
    swarm_replicas: updated.swarm_replicas ?? 1,
    swarm_placement_constraints: updated.swarm_placement_constraints || '',
  }
}

function healthFromApp(updated: {
  health_check_enabled?: boolean
  health_check_path?: string
  health_check_port?: number | null
  health_check_method?: string
  health_check_return_code?: number
  health_check_interval?: number
  health_check_timeout?: number
  health_check_retries?: number
  health_check_host?: string
  health_check_scheme?: string
  health_check_response_text?: string
  health_check_start_period?: number
  health_check_type?: string
  health_check_command?: string
}) {
  return {
    health_check_enabled: Boolean(updated.health_check_enabled),
    health_check_path: updated.health_check_path || '/',
    health_check_port: updated.health_check_port != null ? String(updated.health_check_port) : '',
    health_check_method: updated.health_check_method || 'GET',
    health_check_return_code: updated.health_check_return_code ?? 200,
    health_check_interval: updated.health_check_interval ?? 5,
    health_check_timeout: updated.health_check_timeout ?? 5,
    health_check_retries: updated.health_check_retries ?? 10,
    health_check_host: updated.health_check_host || 'localhost',
    health_check_scheme: updated.health_check_scheme || 'http',
    health_check_response_text: updated.health_check_response_text || '',
    health_check_start_period: updated.health_check_start_period ?? 5,
    health_check_type: updated.health_check_type || 'http',
    health_check_command: updated.health_check_command || '',
  }
}

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

const APP_SIDE_GROUPS = [
  { label: 'Setup', ids: ['general', 'advanced', 'environment'] },
  { label: 'Source', ids: ['git', 'servers', 'previews', 'webhooks'] },
  { label: 'Runtime', ids: ['storages', 'tasks', 'limits', 'metrics', 'tags'] },
  { label: 'Manage', ids: ['rollback', 'operations', 'danger'] },
] as const

type TopTabId = (typeof TOP_TABS)[number]['id']
type SideId = (typeof SIDE_ITEMS)[number]['id']

function isTopTabId(v: string | undefined): v is TopTabId {
  return !!v && TOP_TABS.some((t) => t.id === v)
}

function isSideId(v: string | undefined): v is SideId {
  return !!v && SIDE_ITEMS.some((t) => t.id === v)
}

/** Persisted in ?tab=&side= so refresh keeps the active panels. */
export type ApplicationDetailSearch = {
  tab?: TopTabId
  side?: SideId
}

export function parseApplicationDetailSearch(s: Record<string, unknown>): ApplicationDetailSearch {
  const tab = typeof s.tab === 'string' && isTopTabId(s.tab) ? s.tab : undefined
  const side = typeof s.side === 'string' && isSideId(s.side) ? s.side : undefined
  return { tab, side }
}

export function ApplicationDetailPage() {
  const { appId, projectId, envId } = useParams({ strict: false }) as {
    appId: string
    projectId?: string
    envId?: string
  }
  const nav = useNavigate()
  const search = useSearch({ strict: false }) as ApplicationDetailSearch
  const topTab: TopTabId = isTopTabId(search.tab) ? search.tab : 'overview'
  const sideRaw: SideId = isSideId(search.side) ? search.side : 'general'
  const qc = useQueryClient()
  const toast = useToast()
  const confirm = useConfirm()
  const nested = Boolean(projectId && envId)

  const setAppNav = (next: { tab?: TopTabId; side?: SideId }) => {
    const tab = next.tab ?? topTab
    const nextSide = next.side ?? sideRaw
    void nav({
      // Omit cleared keys (TanStack drops missing keys / undefined).
      search: ((prev: Record<string, unknown>) => {
        const { tab: _t, side: _s, ...rest } = prev
        const out: Record<string, unknown> = { ...rest }
        if (tab !== 'overview') {
          out.tab = tab
        }
        // Keep side in the URL even on other top tabs so Configuration restores it.
        if (nextSide !== 'general') {
          out.side = nextSide
        }
        return out
      }) as never,
      replace: true,
    })
  }

  const app = useQuery({ queryKey: ['application', appId], queryFn: () => api.application(appId) })
  const allEnvs = useQuery({
    queryKey: ['all-environments'],
    queryFn: fetchAllEnvironments,
  })
  const resolvedProjectId =
    projectId ||
    (app.data?.environment_id
      ? allEnvs.data?.find((e) => e.id === app.data.environment_id)?.project_id
      : undefined) ||
    ''
  const resolvedEnvId = envId || app.data?.environment_id || ''
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const gitSources = useQuery({ queryKey: ['git-sources'], queryFn: api.gitSources })
  const keys = useQuery({ queryKey: ['keys'], queryFn: api.keys })
  const envVarsQ = useQuery({
    queryKey: ['env-vars', 'application', appId, 'prod'],
    queryFn: () => api.envVars('application', appId, true, false),
    enabled: Boolean(appId),
  })
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

  const [deleteOpen, setDeleteOpen] = useState(false)
  const [cfg, setCfg] = useState<AppCfg>(emptyAppCfg)
  const [extraDestIds, setExtraDestIds] = useState<string[]>([])
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
    health_check_host: 'localhost',
    health_check_scheme: 'http',
    health_check_response_text: '',
    health_check_start_period: 5,
    health_check_type: 'http',
    health_check_command: '',
  })
  const [limits, setLimits] = useState({ limits_memory: '', limits_cpus: '' })
  const [webhookSecret, setWebhookSecret] = useState<string | null>(null)
  const [previewDeploy, setPreviewDeploy] = useState({
    pull_request_id: '',
    pull_request_title: '',
    git_branch: '',
  })

  const registries = useQuery({ queryKey: ['docker-registries'], queryFn: api.dockerRegistries })
  const extraDests = useQuery({
    queryKey: ['app-additional-destinations', appId],
    queryFn: () => api.additionalDestinations(appId),
    enabled: Boolean(appId),
  })
  const appContainers = useQuery({
    queryKey: ['app-containers', appId],
    queryFn: () => api.applicationContainers(appId),
    enabled: Boolean(appId) && topTab === 'terminal',
  })

  useEffect(() => {
    setWebhookSecret(null)
    setHttpBasicAuthPassword('')
    setClearHttpBasicAuth(false)
    setCfg(emptyAppCfg())
    setExtraDestIds([])
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
      health_check_host: 'localhost',
      health_check_scheme: 'http',
      health_check_response_text: '',
      health_check_start_period: 5,
      health_check_type: 'http',
      health_check_command: '',
    })
    setLimits({ limits_memory: '', limits_cpus: '' })
  }, [appId])

  useEffect(() => {
    if (extraDests.data?.destination_ids) {
      setExtraDestIds(extraDests.data.destination_ids.map(String))
    }
  }, [extraDests.data])

  useEffect(() => {
    if (!app.data || app.data.id !== appId) return
    const updated = app.data
    setCfg(appCfgFromData(updated))
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
    setHealth(healthFromApp(updated))
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
    const base = !inlineDockerfile
      ? SIDE_ITEMS
      : SIDE_ITEMS.filter((item) => !['git', 'webhooks', 'previews', 'rollback'].includes(item.id))
    const emptyCount = emptyUserEnvVars(envVarsQ.data?.environment_variables).length
    return base.map((item) =>
      item.id === 'environment' && emptyCount ? { ...item, badge: emptyCount } : item,
    )
  }, [app.data?.dockerfile, cfg.dockerfile, envVarsQ.data?.environment_variables])

  // If URL points at a hidden side item (e.g. Git on inline Dockerfile), fall back.
  const side: SideId = sideItems.some((i) => i.id === sideRaw) ? sideRaw : 'general'

  const applyAppToForm = (updated: NonNullable<typeof app.data>) => {
    setCfg(appCfgFromData(updated))
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
    setHealth(healthFromApp(updated))
    setLimits({
      limits_memory: updated.limits_memory || '',
      limits_cpus: updated.limits_cpus || '',
    })
  }

  const syncFromApp = (updated: NonNullable<typeof app.data>) => {
    applyAppToForm(updated)
  }

  const save = useMutation({
    mutationFn: (patch?: Partial<AppCfg> & Record<string, unknown>) => {
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

  const saveExtraDests = useMutation({
    mutationFn: (ids: string[]) => api.setAdditionalDestinations(appId, ids),
    onSuccess: (res) => {
      setExtraDestIds(res.destination_ids.map(String))
      void qc.invalidateQueries({ queryKey: ['app-additional-destinations', appId] })
    },
    onError: (e: Error) => toast.error(e.message || 'Failed to save additional destinations'),
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
      void qc.invalidateQueries({ queryKey: ['env-vars', 'application', appId] })
      toast.success('Compose file loaded — environment variables updated')
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
        health_check_host: health.health_check_host,
        health_check_scheme: health.health_check_scheme,
        health_check_response_text: health.health_check_response_text,
        health_check_start_period: health.health_check_start_period,
        health_check_type: health.health_check_type,
        health_check_command: health.health_check_command,
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
    onError: (e: Error) => toast.error(e.message || 'Deploy failed'),
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

  const deployPreview = useMutation({
    mutationFn: (body: {
      pull_request_id: number
      pull_request_title?: string
      git_branch?: string
    }) => api.deployPreview(appId, body),
    onSuccess: () => {
      toast.success('Preview deploy queued')
      void qc.invalidateQueries({ queryKey: ['previews', appId] })
      void qc.invalidateQueries({ queryKey: ['deployments', appId] })
    },
    onError: (e: Error) => toast.error(e.message || 'Preview deploy failed'),
  })

  const stopCleanup = useMutation({
    mutationFn: () => api.stopCleanupApplication(appId),
    onSuccess: () => {
      toast.success('Stopped and cleaned up Docker resources')
      void qc.invalidateQueries({ queryKey: ['application', appId] })
    },
    onError: (e: Error) => toast.error(e.message || 'Stop + cleanup failed'),
  })

  const cloneApp = useMutation({
    mutationFn: (name: string) => api.cloneApplication(appId, { name: name || undefined }),
    onSuccess: (created) => {
      toast.success(`Cloned as ${created.name}`)
      void qc.invalidateQueries({ queryKey: ['applications'] })
      const envMeta = allEnvs.data?.find((e) => e.id === created.environment_id)
      if (envMeta?.project_id && created.environment_id) {
        void nav({
          to: '/projects/$projectId/environments/$envId/applications/$appId',
          params: {
            projectId: envMeta.project_id,
            envId: created.environment_id,
            appId: created.id,
          },
        })
      }
    },
    onError: (e: Error) => toast.error(e.message || 'Clone failed'),
  })

  const emptyEnv = useMemo(
    () => emptyUserEnvVars(envVarsQ.data?.environment_variables),
    [envVarsQ.data?.environment_variables],
  )

  const requestDeploy = (vars: { force?: boolean } = {}) => {
    const gate = deployBlockFromEnv(envVarsQ)
    if (gate.block) {
      toast.warning(gate.message || 'Finish setup before deploying.')
      if (gate.empty.length) setAppNav({ tab: 'configuration', side: 'environment' })
      return
    }
    const dest = cfg.destination_id || app.data?.destination_id
    if (!dest) {
      toast.warning('Choose a destination server before deploying.')
      setAppNav({ tab: 'configuration', side: 'servers' })
      return
    }
    deploy.mutate(vars)
  }

  const appImages = useQuery({
    queryKey: ['app-images', appId],
    queryFn: () => api.listApplicationImages(appId),
    enabled: side === 'rollback' && Boolean(appId),
  })

  if (app.isLoading) {
    return <DetailPageSkeleton withSideNav />
  }

  const crumbs =
    resolvedProjectId && resolvedEnvId ? (
      <BackLink
        label="Resources"
        to="/projects/$projectId/environments/$envId"
        params={{ projectId: resolvedProjectId, envId: resolvedEnvId }}
      />
    ) : (
      <BackLink label="Projects" to="/projects" />
    )

  if (app.error || !app.data) {
    return (
      <div className="space-y-4">
        {crumbs}
        <p className="text-error-500">{app.error?.message || 'Application not found'}</p>
      </div>
    )
  }

  const a = app.data
  const httpsRedirectApplies = domainsWantAutoHttps(a.fqdn || '')
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

  const deployments = deps.data?.deployments || []
  const latestDep =
    activeDep ||
    deployments.find((d) => d.status === 'finished') ||
    deployments[0]
  const visitUrl = primaryVisitUrl(a)
  const visitHref = visitUrl
  const destMeta = (dests.data?.destinations || []).find(
    (d) => d.id === (cfg.destination_id || a.destination_id),
  )
  const setupChecks: SetupCheck[] = [
    {
      id: 'env',
      ok: !envVarsQ.isError && (!envVarsQ.isSuccess || emptyEnv.length === 0),
      title: 'Environment variables',
      hint: envVarsQ.isError
        ? 'Could not load environment variables.'
        : emptyEnv.length === 1
          ? `${emptyEnv[0]?.key} still needs a value.`
          : `${emptyEnv.length} variables still need a value.`,
      actionLabel: 'Fill now',
      onAction: () => setAppNav({ tab: 'configuration', side: 'environment' }),
    },
    {
      id: 'dest',
      ok: Boolean(cfg.destination_id || a.destination_id),
      title: 'Destination server',
      hint: 'Pick where this application should run.',
      actionLabel: 'Choose',
      onAction: () => setAppNav({ tab: 'configuration', side: 'servers' }),
    },
    {
      id: 'domain',
      ok:
        Boolean((cfg.fqdn || a.fqdn || '').trim()) ||
        Object.values(serviceDomains).some((v) => String(v).trim()) ||
        Boolean((a.links || []).length),
      title: 'Public domain',
      hint: 'Add a magic or custom domain so Traefik can route traffic.',
      actionLabel: 'Set domain',
      onAction: () => setAppNav({ tab: 'configuration', side: 'general' }),
    },
  ]

  return (
    <div className="space-y-5">
      {crumbs}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 items-center gap-3">
          <ServiceLogo
            src={logoForApplication(a.build_pack, a.git_source_id)}
            name={a.name}
            className="h-10 w-10"
          />
          <div className="min-w-0">
            <h1 className="flex flex-wrap items-center gap-2 text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
              {a.name}
              <ResourceSwitcher
                kind="application"
                currentId={appId}
                environmentId={a.environment_id || envId}
                projectId={resolvedProjectId}
              />
            </h1>
            <div className="mt-1.5 flex flex-wrap items-center gap-2">
              <StatusBadge status={activeDep ? activeDep.status : a.status} />
              <span className="text-xs capitalize text-gray-500 dark:text-gray-400">{a.build_pack}</span>
              {emptyEnv.length ? (
                <button
                  type="button"
                  onClick={() => setAppNav({ tab: 'configuration', side: 'environment' })}
                  className="rounded-full bg-amber-500/15 px-2 py-0.5 text-[11px] font-medium text-amber-800 hover:bg-amber-500/25 dark:text-amber-300"
                >
                  {emptyEnv.length} env {emptyEnv.length === 1 ? 'var' : 'vars'} to fill
                </button>
              ) : null}
            </div>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {visitHref ? (
            <a
              href={visitHref}
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-8 items-center gap-1.5 rounded-md border border-gray-200 bg-white px-2.5 text-xs font-medium text-gray-800 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-100 dark:hover:bg-gray-800"
            >
              Visit
              <ExternalLink className="h-3.5 w-3.5 opacity-70" />
            </a>
          ) : null}
          <LinksMenu links={a.links || []} />
          {activeDep ? <Btn onClick={() => cancel.mutate(activeDep.id)}>Cancel deploy</Btn> : null}
          <Btn primary onClick={() => requestDeploy({})} disabled={deploy.isPending}>
            <span className="inline-flex items-center gap-1.5">
              <Rocket className="h-3.5 w-3.5" />
              {deploy.isPending ? 'Queueing…' : 'Redeploy'}
            </span>
          </Btn>
          <DetailMoreMenu>
            <DetailMoreItem
              disabled={restartApp.isPending || startApp.isPending || stopApp.isPending || deploy.isPending}
              onClick={() => restartApp.mutate()}
            >
              {restartApp.isPending ? 'Restarting…' : 'Restart'}
            </DetailMoreItem>
            {(a.status || '').toLowerCase().includes('exit') ||
            (a.status || '').toLowerCase().includes('stop') ? (
              <DetailMoreItem
                disabled={startApp.isPending || restartApp.isPending || stopApp.isPending || deploy.isPending}
                onClick={() => startApp.mutate()}
              >
                {startApp.isPending ? 'Starting…' : 'Start'}
              </DetailMoreItem>
            ) : (
              <DetailMoreItem
                danger
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
              </DetailMoreItem>
            )}
          </DetailMoreMenu>
        </div>
      </div>

      <div className="sticky top-0 z-20 -mx-3 border-b border-gray-200 bg-white/90 px-3 backdrop-blur dark:border-gray-800 dark:bg-gray-950/90 sm:-mx-5 sm:px-5 lg:-mx-6 lg:px-6">
        <nav className="flex flex-nowrap gap-1 overflow-x-auto" role="tablist">
          {TOP_TABS.map((t) => {
            const Icon = t.icon
            const active = topTab === t.id
            return (
              <button
                key={t.id}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => setAppNav({ tab: t.id })}
                className={`relative inline-flex shrink-0 items-center gap-1.5 px-3 py-2.5 text-sm font-medium transition ${
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
                {t.id === 'deployments' && activeDep ? (
                  <span className="h-1.5 w-1.5 rounded-full bg-amber-500" aria-hidden />
                ) : null}
                {active && (
                  <span className="absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-brand-500" />
                )}
              </button>
            )
          })}
        </nav>
      </div>

      {topTab === 'overview' && (
        <div className="space-y-6">
          <ResourceSetupBanner checks={setupChecks} />
          <AppOverview
            app={a}
            logoSrc={logoForApplication(a.build_pack, a.git_source_id)}
            latest={latestDep}
            recent={deployments.slice(0, 8)}
            destination={destMeta}
            emptyEnvCount={emptyEnv.length}
            envTotal={(envVarsQ.data?.environment_variables || []).length}
            links={a.links || []}
            onOpenDeployment={openDeployment}
            onCancelDeployment={(id) => cancel.mutate(id)}
            onRedeploy={() => requestDeploy({})}
            onOpenSettings={(side) => setAppNav({ tab: 'configuration', side })}
            onViewAllDeployments={() => setAppNav({ tab: 'deployments' })}
            deployBusy={deploy.isPending}
            showGit={sideItems.some((i) => i.id === 'git')}
          />
        </div>
      )}

      {topTab === 'configuration' && (
        <div className="flex flex-col gap-6 md:flex-row">
          <ConfigSideNav
            items={sideItems}
            groups={APP_SIDE_GROUPS}
            active={side}
            onSelect={(id) => setAppNav({ tab: 'configuration', side: id })}
          />
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
                      <select
                        value={cfg.build_pack}
                        onChange={(e) => setCfg({ ...cfg, build_pack: e.target.value })}
                        className="panel-field w-full rounded-lg px-3 py-2 text-sm capitalize"
                      >
                        {['dockerfile', 'dockercompose', 'dockerimage', 'static', 'railpack'].map(
                          (bp) => (
                            <option key={bp} value={bp}>
                              {bp}
                            </option>
                          ),
                        )}
                      </select>
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
                    <label className="block text-sm">
                      <span className="mb-1 block text-gray-500 dark:text-gray-400">
                        WWW redirect
                      </span>
                      <select
                        value={cfg.redirect}
                        onChange={(e) => setCfg({ ...cfg, redirect: e.target.value })}
                        className="panel-field w-full max-w-xs rounded-lg px-3 py-2 text-sm"
                      >
                        <option value="both">Both www & non-www</option>
                        <option value="www">Redirect to www</option>
                        <option value="non-www">Redirect to non-www</option>
                      </select>
                    </label>
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
                    <label className="block text-sm">
                      <span className="mb-1 block text-gray-500 dark:text-gray-400">
                        WWW redirect
                      </span>
                      <select
                        value={cfg.redirect}
                        onChange={(e) => setCfg({ ...cfg, redirect: e.target.value })}
                        className="panel-field w-full max-w-xs rounded-lg px-3 py-2 text-sm"
                      >
                        <option value="both">Both www & non-www</option>
                        <option value="www">Redirect to www</option>
                        <option value="non-www">Redirect to non-www</option>
                      </select>
                    </label>
                  </div>
                )}

                <details className="panel-card group overflow-hidden">
                  <summary className="cursor-pointer list-none px-5 py-3 text-sm font-semibold text-gray-900 marker:hidden dark:text-white [&::-webkit-details-marker]:hidden">
                    <span className="flex items-center justify-between gap-2">
                      Build paths &amp; ports
                      <span className="text-xs font-normal text-gray-500 group-open:hidden dark:text-gray-400">
                        Show
                      </span>
                      <span className="hidden text-xs font-normal text-gray-500 group-open:inline dark:text-gray-400">
                        Hide
                      </span>
                    </span>
                  </summary>
                  <div className="grid gap-4 border-t border-gray-200 p-5 sm:grid-cols-2 dark:border-gray-800">
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
                      <Input
                        label="Ports mappings"
                        value={cfg.ports_mappings}
                        onChange={(v) => setCfg({ ...cfg, ports_mappings: v })}
                        required={false}
                      />
                      <Input
                        label="Custom network aliases"
                        value={cfg.custom_network_aliases}
                        onChange={(v) => setCfg({ ...cfg, custom_network_aliases: v })}
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
                        label="Ports mappings"
                        value={cfg.ports_mappings}
                        onChange={(v) => setCfg({ ...cfg, ports_mappings: v })}
                        required={false}
                      />
                      <Input
                        label="Custom network aliases"
                        value={cfg.custom_network_aliases}
                        onChange={(v) => setCfg({ ...cfg, custom_network_aliases: v })}
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
                      {a.build_pack === 'dockerimage' ? (
                        <label className="block text-sm sm:col-span-2">
                          <span className="mb-1 block text-gray-500 dark:text-gray-400">
                            Docker registry
                          </span>
                          <select
                            value={cfg.docker_registry_id}
                            onChange={(e) => setCfg({ ...cfg, docker_registry_id: e.target.value })}
                            className="panel-field w-full rounded-lg px-3 py-2 text-sm"
                          >
                            <option value="">Public / none</option>
                            {(registries.data?.docker_registries || []).map((r) => (
                              <option key={r.id} value={r.id}>
                                {r.name} ({r.url})
                              </option>
                            ))}
                          </select>
                          <span className="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                            Manage registries under Settings → Docker Registries.
                          </span>
                        </label>
                      ) : null}
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
                </details>

                {(cfg.build_pack === 'static' || cfg.build_pack === 'railpack') && (
                  <div className="panel-card grid gap-4 p-5 sm:grid-cols-2">
                    <h3 className="text-sm font-semibold text-gray-900 dark:text-white sm:col-span-2">
                      {cfg.build_pack === 'static' ? 'Static site' : 'Railpack'} commands
                    </h3>
                    <Input
                      label="Install command"
                      value={cfg.install_command}
                      onChange={(v) => setCfg({ ...cfg, install_command: v })}
                      required={false}
                    />
                    <Input
                      label="Build command"
                      value={cfg.build_command}
                      onChange={(v) => setCfg({ ...cfg, build_command: v })}
                      required={false}
                    />
                    <Input
                      label="Start command"
                      value={cfg.start_command}
                      onChange={(v) => setCfg({ ...cfg, start_command: v })}
                      required={false}
                    />
                    <Input
                      label="Publish directory"
                      value={cfg.publish_directory}
                      onChange={(v) => setCfg({ ...cfg, publish_directory: v })}
                      required={false}
                    />
                    <label className="flex items-center gap-3 text-sm sm:col-span-2">
                      <input
                        type="checkbox"
                        checked={cfg.is_spa}
                        onChange={(e) => setCfg({ ...cfg, is_spa: e.target.checked })}
                      />
                      <span>Single-page application (SPA) — fallback to index.html</span>
                    </label>
                    <label className="block text-sm sm:col-span-2">
                      <span className="mb-1 block text-gray-500 dark:text-gray-400">
                        Custom nginx configuration
                      </span>
                      <textarea
                        rows={6}
                        value={cfg.custom_nginx_configuration}
                        onChange={(e) =>
                          setCfg({ ...cfg, custom_nginx_configuration: e.target.value })
                        }
                        className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
                        placeholder="Optional nginx server block overrides"
                      />
                    </label>
                  </div>
                )}

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
                  <label className={`flex items-center gap-3 text-sm ${httpsRedirectApplies ? '' : 'opacity-60'}`}>
                    <input
                      type="checkbox"
                      checked={httpsRedirectApplies && cfg.is_force_https}
                      disabled={!httpsRedirectApplies}
                      onChange={(e) => {
                        if (!httpsRedirectApplies) return
                        setCfg({ ...cfg, is_force_https: e.target.checked })
                      }}
                    />
                    <span>Force HTTPS redirects</span>
                  </label>
                  <p className="ml-7 text-xs text-gray-500 dark:text-gray-400">
                    {httpsRedirectApplies
                      ? 'Redirect HTTP to HTTPS. Redeploy to apply.'
                      : 'Magic domains stay HTTP. Add a custom domain first.'}
                  </p>
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
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_disable_build_cache}
                      onChange={(e) => setCfg({ ...cfg, is_disable_build_cache: e.target.checked })}
                    />
                    <span>Disable build cache</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_git_shallow_clone_enabled}
                      onChange={(e) =>
                        setCfg({ ...cfg, is_git_shallow_clone_enabled: e.target.checked })
                      }
                    />
                    <span>Shallow clone (recommended)</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_git_lfs_enabled}
                      onChange={(e) => setCfg({ ...cfg, is_git_lfs_enabled: e.target.checked })}
                    />
                    <span>Git LFS</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_gpu_enabled}
                      onChange={(e) => setCfg({ ...cfg, is_gpu_enabled: e.target.checked })}
                    />
                    <span>GPU support</span>
                  </label>
                  {cfg.is_gpu_enabled ? (
                    <label className="block text-sm sm:max-w-xs">
                      <span className="mb-1 block text-gray-500 dark:text-gray-400">GPU count</span>
                      <input
                        type="number"
                        min={0}
                        value={cfg.gpu_count}
                        onChange={(e) =>
                          setCfg({ ...cfg, gpu_count: Number(e.target.value) || 0 })
                        }
                        className="panel-field w-full rounded-lg px-3 py-2 text-sm"
                      />
                    </label>
                  ) : null}
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.inject_build_args_to_dockerfile}
                      onChange={(e) =>
                        setCfg({ ...cfg, inject_build_args_to_dockerfile: e.target.checked })
                      }
                    />
                    <span>Inject build args into Dockerfile</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.use_build_secrets}
                      onChange={(e) => setCfg({ ...cfg, use_build_secrets: e.target.checked })}
                    />
                    <span>Use Docker build secrets</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.include_source_commit_in_build}
                      onChange={(e) =>
                        setCfg({ ...cfg, include_source_commit_in_build: e.target.checked })
                      }
                    />
                    <span>Include SOURCE_COMMIT in build</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_consistent_container_name_enabled}
                      onChange={(e) =>
                        setCfg({ ...cfg, is_consistent_container_name_enabled: e.target.checked })
                      }
                    />
                    <span>Consistent container name</span>
                  </label>
                  <Input
                    label="Custom internal name"
                    value={cfg.custom_internal_name}
                    onChange={(v) => setCfg({ ...cfg, custom_internal_name: v })}
                    required={false}
                  />
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_gzip_enabled}
                      onChange={(e) => setCfg({ ...cfg, is_gzip_enabled: e.target.checked })}
                    />
                    <span>Enable gzip compression (Traefik)</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_stripprefix_enabled}
                      onChange={(e) => setCfg({ ...cfg, is_stripprefix_enabled: e.target.checked })}
                    />
                    <span>Enable StripPrefix middleware</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_log_drain_enabled}
                      onChange={(e) => setCfg({ ...cfg, is_log_drain_enabled: e.target.checked })}
                    />
                    <span>Log drain</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_debug_enabled}
                      onChange={(e) => setCfg({ ...cfg, is_debug_enabled: e.target.checked })}
                    />
                    <span>Debug mode</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_env_sorting_enabled}
                      onChange={(e) => setCfg({ ...cfg, is_env_sorting_enabled: e.target.checked })}
                    />
                    <span>Sort environment variables by key</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_pr_deployments_public_enabled}
                      onChange={(e) =>
                        setCfg({ ...cfg, is_pr_deployments_public_enabled: e.target.checked })
                      }
                    />
                    <span>Public PR preview deployments</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.skip_rebuild_if_unchanged}
                      onChange={(e) =>
                        setCfg({ ...cfg, skip_rebuild_if_unchanged: e.target.checked })
                      }
                    />
                    <span>Skip rebuild if unchanged</span>
                  </label>
                  <label className="block text-sm sm:max-w-xs">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Docker images to keep
                    </span>
                    <input
                      type="number"
                      min={0}
                      value={cfg.docker_images_to_keep}
                      onChange={(e) =>
                        setCfg({ ...cfg, docker_images_to_keep: Number(e.target.value) || 0 })
                      }
                      className="panel-field w-full rounded-lg px-3 py-2 text-sm"
                    />
                  </label>
                  <Input
                    label="GPU driver"
                    value={cfg.gpu_driver}
                    onChange={(v) => setCfg({ ...cfg, gpu_driver: v })}
                    required={false}
                  />
                  <Input
                    label="GPU device IDs"
                    value={cfg.gpu_device_ids}
                    onChange={(v) => setCfg({ ...cfg, gpu_device_ids: v })}
                    required={false}
                  />
                  <Input
                    label="GPU options"
                    value={cfg.gpu_options}
                    onChange={(v) => setCfg({ ...cfg, gpu_options: v })}
                    required={false}
                  />
                  <label className="block text-sm sm:max-w-xs">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Max Docker restart count
                    </span>
                    <input
                      type="number"
                      min={0}
                      value={cfg.custom_docker_max_restart_count}
                      onChange={(e) =>
                        setCfg({
                          ...cfg,
                          custom_docker_max_restart_count: Number(e.target.value) || 0,
                        })
                      }
                      className="panel-field w-full rounded-lg px-3 py-2 text-sm"
                    />
                  </label>
                  <Input
                    label="Pre-deployment command container"
                    value={cfg.pre_deployment_command_container}
                    onChange={(v) => setCfg({ ...cfg, pre_deployment_command_container: v })}
                    required={false}
                  />
                  <Input
                    label="Post-deployment command container"
                    value={cfg.post_deployment_command_container}
                    onChange={(v) => setCfg({ ...cfg, post_deployment_command_container: v })}
                    required={false}
                  />
                  <label className="block text-sm sm:max-w-xs">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Docker stop timeout (seconds)
                    </span>
                    <input
                      type="number"
                      min={0}
                      value={cfg.custom_docker_stop_timeout}
                      onChange={(e) =>
                        setCfg({
                          ...cfg,
                          custom_docker_stop_timeout: Number(e.target.value) || 0,
                        })
                      }
                      className="panel-field w-full rounded-lg px-3 py-2 text-sm"
                    />
                    <span className="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                      0 uses Docker default.
                    </span>
                  </label>
                  <label className="block text-sm sm:max-w-xs">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Restart policy
                    </span>
                    <select
                      value={cfg.custom_docker_restart_policy}
                      onChange={(e) =>
                        setCfg({ ...cfg, custom_docker_restart_policy: e.target.value })
                      }
                      className="panel-field w-full rounded-lg px-3 py-2 text-sm"
                    >
                      <option value="no">no</option>
                      <option value="always">always</option>
                      <option value="unless-stopped">unless-stopped</option>
                      <option value="on-failure">on-failure</option>
                    </select>
                  </label>
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    Build arguments belong under Environment Variables (mark as build-time).
                  </p>
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

                <form
                  className="panel-card space-y-4 p-5"
                  onSubmit={(e) => {
                    e.preventDefault()
                    save.mutate({})
                  }}
                >
                  <h3 className="text-sm font-medium text-gray-900 dark:text-white">Swarm</h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    Docker Swarm placement for swarm destinations.
                  </p>
                  <label className="flex items-center gap-3 text-sm">
                    <input
                      type="checkbox"
                      checked={cfg.is_swarm_only_worker_nodes}
                      onChange={(e) =>
                        setCfg({ ...cfg, is_swarm_only_worker_nodes: e.target.checked })
                      }
                    />
                    <span>Deploy only on worker nodes</span>
                  </label>
                  <label className="block text-sm sm:max-w-xs">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">Replicas</span>
                    <input
                      type="number"
                      min={0}
                      value={cfg.swarm_replicas}
                      onChange={(e) =>
                        setCfg({ ...cfg, swarm_replicas: Number(e.target.value) || 0 })
                      }
                      className="panel-field w-full rounded-lg px-3 py-2 text-sm"
                    />
                  </label>
                  <label className="block text-sm">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Placement constraints
                    </span>
                    <textarea
                      rows={3}
                      value={cfg.swarm_placement_constraints}
                      onChange={(e) =>
                        setCfg({ ...cfg, swarm_placement_constraints: e.target.value })
                      }
                      placeholder="node.role==worker"
                      className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
                    />
                  </label>
                  <Btn primary type="submit" disabled={save.isPending}>
                    {save.isPending ? 'Saving…' : 'Save swarm settings'}
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
                  <label className="block text-sm sm:max-w-xs">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">Type</span>
                    <select
                      value={health.health_check_type}
                      onChange={(e) => setHealth({ ...health, health_check_type: e.target.value })}
                      className="panel-field w-full rounded-lg px-3 py-2 text-sm"
                    >
                      <option value="http">HTTP</option>
                      <option value="cmd">Command</option>
                    </select>
                  </label>
                  {health.health_check_type === 'cmd' ? (
                    <label className="block text-sm sm:col-span-2">
                      <span className="mb-1 block text-gray-500 dark:text-gray-400">Command</span>
                      <input
                        value={health.health_check_command}
                        onChange={(e) =>
                          setHealth({ ...health, health_check_command: e.target.value })
                        }
                        className="panel-field w-full rounded-lg px-3 py-2 font-mono text-sm"
                        placeholder="CMD curl -f http://localhost/ || exit 1"
                      />
                    </label>
                  ) : null}
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
                    <Input
                      label="Start period (s)"
                      value={String(health.health_check_start_period)}
                      onChange={(v) =>
                        setHealth({ ...health, health_check_start_period: Number(v) || 5 })
                      }
                    />
                    <label className="block text-sm">
                      <span className="mb-1 block text-gray-500 dark:text-gray-400">Scheme</span>
                      <select
                        value={health.health_check_scheme}
                        onChange={(e) =>
                          setHealth({ ...health, health_check_scheme: e.target.value })
                        }
                        className="panel-field w-full rounded-lg px-3 py-2 text-sm"
                      >
                        <option value="http">http</option>
                        <option value="https">https</option>
                      </select>
                    </label>
                    <Input
                      label="Host"
                      value={health.health_check_host}
                      onChange={(v) => setHealth({ ...health, health_check_host: v })}
                      required={false}
                    />
                    <Input
                      label="Response text (optional)"
                      value={health.health_check_response_text}
                      onChange={(v) => setHealth({ ...health, health_check_response_text: v })}
                      required={false}
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
                  saveExtraDests.mutate(extraDestIds)
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
                  <div className="border-t border-gray-200 pt-4 dark:border-gray-800">
                    <h3 className="text-sm font-medium text-gray-900 dark:text-white">
                      Additional destinations
                    </h3>
                    <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      Deploy to extra destinations besides the primary (multi-server).
                    </p>
                    <div className="mt-3 space-y-2">
                      {(dests.data?.destinations || [])
                        .filter((d) => d.id !== cfg.destination_id)
                        .map((d) => {
                          const checked = extraDestIds.includes(d.id)
                          return (
                            <label key={d.id} className="flex items-center gap-3 text-sm">
                              <input
                                type="checkbox"
                                checked={checked}
                                onChange={(e) => {
                                  setExtraDestIds((ids) =>
                                    e.target.checked
                                      ? [...ids, d.id]
                                      : ids.filter((id) => id !== d.id),
                                  )
                                }}
                              />
                              <span>
                                {d.name}{' '}
                                <span className="text-xs text-gray-500">({d.network})</span>
                              </span>
                            </label>
                          )
                        })}
                      {!(dests.data?.destinations || []).filter((d) => d.id !== cfg.destination_id)
                        .length && (
                        <p className="text-xs text-gray-500 dark:text-gray-400">
                          No other destinations available.
                        </p>
                      )}
                    </div>
                  </div>
                </div>
                {(save.error || saveExtraDests.error) && (
                  <p className="text-sm text-error-500">
                    {save.error?.message || saveExtraDests.error?.message}
                  </p>
                )}
                <Btn primary type="submit">
                  {save.isPending || saveExtraDests.isPending ? 'Saving…' : 'Save'}
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
              <AppMetricsSection appId={appId} serverId={serverId} />
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
                  <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                    {a.build_pack === 'dockercompose'
                      ? 'Variables are auto-created from your Docker Compose file (${VAR}, defaults, and SERVICE_*). Click Load Compose under General if the list is empty.'
                      : 'Environment (secrets) variables for this application.'}
                  </p>
                </div>
                <EnvVarsPanel
                  resourceType="application"
                  resourceId={appId}
                  title=""
                  previewTabs={Boolean(a.is_preview_enabled || cfg.is_preview_enabled)}
                  sortByKey={cfg.is_env_sorting_enabled}
                />
              </div>
            )}

            {side === 'previews' && (
              <div className="space-y-6">
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
                <form
                  className="panel-card grid gap-4 p-5 sm:grid-cols-2"
                  onSubmit={(e) => {
                    e.preventDefault()
                    save.mutate({})
                  }}
                >
                  <Input
                    label="Preview URL template"
                    value={cfg.preview_url_template}
                    onChange={(v) => setCfg({ ...cfg, preview_url_template: v })}
                    required={false}
                  />
                  <p className="text-xs text-gray-500 dark:text-gray-400 sm:col-span-2">
                    Use <code className="font-mono">{'{{pr_id}}'}</code> and{' '}
                    <code className="font-mono">{'{{domain}}'}</code> placeholders.
                  </p>
                  <div className="flex items-end sm:col-span-2">
                    <Btn primary type="submit" disabled={save.isPending}>
                      {save.isPending ? 'Saving…' : 'Save template'}
                    </Btn>
                  </div>
                </form>
                <form
                  className="panel-card grid gap-4 p-5 sm:grid-cols-2"
                  onSubmit={(e) => {
                    e.preventDefault()
                    if (!previewDeploy.pull_request_id) return
                    deployPreview.mutate({
                      pull_request_id: Number(previewDeploy.pull_request_id),
                      pull_request_title: previewDeploy.pull_request_title || undefined,
                      git_branch: previewDeploy.git_branch || undefined,
                    })
                  }}
                >
                  <h3 className="text-sm font-semibold text-gray-900 dark:text-white sm:col-span-2">
                    Deploy preview manually
                  </h3>
                  <Input
                    label="Pull request ID"
                    value={previewDeploy.pull_request_id}
                    onChange={(v) => setPreviewDeploy({ ...previewDeploy, pull_request_id: v })}
                    required={false}
                  />
                  <Input
                    label="PR title (optional)"
                    value={previewDeploy.pull_request_title}
                    onChange={(v) => setPreviewDeploy({ ...previewDeploy, pull_request_title: v })}
                    required={false}
                  />
                  <Input
                    label="Git branch (optional)"
                    value={previewDeploy.git_branch}
                    onChange={(v) => setPreviewDeploy({ ...previewDeploy, git_branch: v })}
                    required={false}
                  />
                  <p className="text-xs text-gray-500 dark:text-gray-400 sm:col-span-2">
                    Branch defaults to {cfg.git_branch || a.git_branch || 'main'} when empty.
                  </p>
                  <div className="flex items-end">
                    <Btn
                      primary
                      type="submit"
                      disabled={deployPreview.isPending || !previewDeploy.pull_request_id}
                    >
                      {deployPreview.isPending ? 'Deploying…' : 'Deploy preview'}
                    </Btn>
                  </div>
                  {deployPreview.error ? (
                    <p className="text-sm text-error-500 sm:col-span-2">
                      {deployPreview.error.message}
                    </p>
                  ) : null}
                </form>
                <div className="panel-card overflow-hidden">
                  <table className="w-full text-left text-sm">
                    <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
                      <tr>
                        <th className="px-3 py-2">PR</th>
                        <th className="px-3 py-2">Title</th>
                        <th className="px-3 py-2">Branch</th>
                        <th className="px-3 py-2">FQDN</th>
                        <th className="px-3 py-2">Status</th>
                        <th className="px-3 py-2">Actions</th>
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
                          <td className="space-x-3 px-3 py-2 text-right">
                            <button
                              type="button"
                              className="text-brand-600 dark:text-brand-400"
                              disabled={deployPreview.isPending}
                              onClick={() => {
                                deployPreview.mutate({
                                  pull_request_id: p.pull_request_id,
                                  pull_request_title: p.pull_request_title || undefined,
                                  git_branch: p.git_branch || undefined,
                                })
                              }}
                            >
                              Deploy
                            </button>
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
                    <Btn primary onClick={() => requestDeploy({})}>
                      Redeploy
                    </Btn>
                    <Btn onClick={() => requestDeploy({ force: true })}>Force rebuild</Btn>
                  </div>
                </div>
                <div className="panel-card space-y-3 p-5">
                  <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Clone</h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    Duplicate this application (settings and env vars) into the same or another
                    environment.
                  </p>
                  <Btn
                    onClick={() => {
                      void (async () => {
                        const name = window.prompt('Clone name (optional)', `${a.name}-clone`)
                        if (name === null) return
                        cloneApp.mutate(name)
                      })()
                    }}
                    disabled={cloneApp.isPending}
                  >
                    {cloneApp.isPending ? 'Cloning…' : 'Clone application'}
                  </Btn>
                  {cloneApp.error ? (
                    <p className="text-sm text-error-500">{cloneApp.error.message}</p>
                  ) : null}
                </div>
                <div className="panel-card space-y-3 p-5">
                  <h3 className="text-sm font-semibold text-error-500">Stop + Docker cleanup</h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    Stop containers and run Docker prune on the destination server (unused images,
                    networks, etc.).
                  </p>
                  <Btn
                    onClick={() => {
                      void (async () => {
                        if (
                          await confirm({
                            title: 'Stop and cleanup',
                            message:
                              'Stop this application and run Docker cleanup on the server?',
                            confirmLabel: 'Stop + cleanup',
                            danger: true,
                          })
                        ) {
                          stopCleanup.mutate()
                        }
                      })()
                    }}
                    disabled={stopCleanup.isPending}
                  >
                    {stopCleanup.isPending ? 'Working…' : 'Stop + Docker cleanup'}
                  </Btn>
                  {stopCleanup.error ? (
                    <p className="text-sm text-error-500">{stopCleanup.error.message}</p>
                  ) : null}
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
                <form
                  className="panel-card flex flex-wrap items-end gap-4 p-5"
                  onSubmit={(e) => {
                    e.preventDefault()
                    save.mutate({})
                  }}
                >
                  <label className="block text-sm sm:max-w-xs">
                    <span className="mb-1 block text-gray-500 dark:text-gray-400">
                      Docker images to keep
                    </span>
                    <input
                      type="number"
                      min={0}
                      value={cfg.docker_images_to_keep}
                      onChange={(e) =>
                        setCfg({ ...cfg, docker_images_to_keep: Number(e.target.value) || 0 })
                      }
                      className="panel-field w-full rounded-lg px-3 py-2 text-sm"
                    />
                  </label>
                  <Btn primary type="submit" disabled={save.isPending}>
                    {save.isPending ? 'Saving…' : 'Save retention'}
                  </Btn>
                </form>
                {(appImages.data?.images || []).length > 0 ? (
                  <div className="panel-card overflow-hidden">
                    <div className="border-b border-gray-200 px-3 py-2 text-sm font-medium dark:border-gray-800">
                      Built images on server
                    </div>
                    <ul className="divide-y divide-gray-200 dark:divide-gray-800">
                      {(appImages.data?.images || []).map((img) => (
                        <li key={img} className="px-3 py-2 font-mono text-xs">
                          {img}
                        </li>
                      ))}
                    </ul>
                  </div>
                ) : null}
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
                  <Btn onClick={() => requestDeploy({ force: true })}>Force rebuild deploy</Btn>
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
        <div className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Deployments</h2>
              <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                Every build for this application, newest first.
              </p>
            </div>
            <Btn primary onClick={() => requestDeploy({})} disabled={deploy.isPending}>
              {deploy.isPending ? 'Queueing…' : 'Redeploy'}
            </Btn>
          </div>
          <div className="panel-card overflow-hidden">
            {deps.isLoading ? (
              <TableSkeleton rows={5} cols={4} />
            ) : (
              <DeploymentRows
                deployments={deployments}
                onOpen={openDeployment}
                onCancel={(id) => cancel.mutate(id)}
              />
            )}
          </div>
        </div>
      )}

      {topTab === 'backups' && <ApplicationBackupsPanel appId={appId} />}

      {topTab === 'logs' && (
        <div className="space-y-3">
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Live container output. Build history lives on{' '}
            <button
              type="button"
              className="font-medium text-brand-600 hover:underline dark:text-brand-400"
              onClick={() => setAppNav({ tab: 'deployments' })}
            >
              Deployments
            </button>
            .
          </p>
          <LiveContainerLogs
            appId={appId}
            isCompose={a.build_pack === 'dockercompose'}
            includeTimestamps={cfg.is_include_timestamps}
            lineLimit={cfg.logs_line_limit}
            onSettingsChange={(patch) => {
              setCfg((c) => ({ ...c, ...patch }))
              save.mutate(patch)
            }}
          />
        </div>
      )}

      {topTab === 'terminal' && (
        <div>
          {serverId ? (
            (appContainers.data?.containers || []).length > 0 ? (
              <ServerTerminal
                serverId={serverId}
                defaultContainer={(appContainers.data?.containers || [])[0]}
                containerOptions={appContainers.data!.containers}
                hideHostShell
              />
            ) : (
              <div className="panel-card space-y-3 p-5 text-sm text-gray-500 dark:text-gray-400">
                <p>No running containers yet. Deploy first, then open a shell here.</p>
                <Btn primary onClick={() => requestDeploy({})} disabled={deploy.isPending}>
                  {deploy.isPending ? 'Queueing…' : 'Redeploy'}
                </Btn>
              </div>
            )
          ) : (
            <div className="panel-card space-y-3 p-5 text-sm text-gray-500 dark:text-gray-400">
              <p>Assign a destination so the container terminal can connect.</p>
              <Btn onClick={() => setAppNav({ tab: 'configuration', side: 'servers' })}>Choose destination</Btn>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function ApplicationBackupsPanel({ appId }: { appId: string }) {
  const qc = useQueryClient()
  const confirm = useConfirm()
  const toast = useToast()
  const backups = useQuery({ queryKey: ['scheduled-backups'], queryFn: api.scheduledBackups })
  const executions = useQuery({
    queryKey: ['app-backups', appId],
    queryFn: () => api.applicationBackups(appId),
    refetchInterval: (q) => {
      const list = q.state.data?.backup_executions || []
      return list.some((b) => b.status === 'running') ? 2000 : false
    },
  })
  const storages = useQuery({ queryKey: ['s3-storages'], queryFn: api.s3Storages })
  const volumes = useQuery({
    queryKey: ['app-volumes', appId],
    queryFn: () => api.listAppVolumes(appId),
  })
  const [s3Id, setS3Id] = useState('')
  const [volumeId, setVolumeId] = useState('')
  const [frequency, setFrequency] = useState('0 0 * * *')
  const [retention, setRetention] = useState('7')
  const mine = (backups.data?.scheduled_backups || []).filter(
    (b) => b.resource_type === 'application' && b.resource_id === appId,
  )
  const create = useMutation({
    mutationFn: () =>
      api.createScheduledBackup({
        resource_type: 'application',
        resource_id: appId,
        s3_storage_id: s3Id || undefined,
        volume_id: volumeId || undefined,
        frequency,
        retention: Number(retention) || 7,
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['scheduled-backups'] }),
  })
  const removeSchedule = useMutation({
    mutationFn: (id: string) => api.deleteScheduledBackup(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['scheduled-backups'] }),
  })
  const toggleSchedule = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      api.updateScheduledBackup(id, { enabled }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['scheduled-backups'] }),
  })
  const runNow = useMutation({
    mutationFn: () => api.runApplicationBackup(appId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['app-backups', appId] }),
  })
  const restoreBackup = useMutation({
    mutationFn: (executionId: string) =>
      api.restoreApplicationBackup(appId, { execution_id: executionId }),
    onSuccess: () => toast.success('Backup restored'),
    onError: (e: Error) => toast.error(e.message || 'Restore failed'),
  })
  useEffect(() => {
    runNow.reset()
  }, [appId]) // eslint-disable-line react-hooks/exhaustive-deps

  const formatBytes = (n: number) => {
    if (!n) return '—'
    if (n < 1024) return `${n} B`
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
    return `${(n / (1024 * 1024)).toFixed(1)} MB`
  }

  if (executions.isLoading || backups.isLoading) {
    return (
      <div className="panel-card p-5">
        <PanelSkeleton rows={4} showHeader={false} />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Archives application volumes to `/data/dockfin/backups` on the server.
        </p>
        <Btn primary onClick={() => runNow.mutate()}>
          {runNow.isPending ? 'Archiving…' : 'Run backup now'}
        </Btn>
      </div>
      {runNow.error && <p className="text-sm text-error-500">{runNow.error.message}</p>}

      <div className="panel-card overflow-hidden">
        <div className="border-b border-gray-200 px-3 py-2 text-sm font-medium dark:border-gray-800">
          Backup history
        </div>
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Started</th>
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2">Size</th>
              <th className="px-3 py-2">File</th>
              <th className="px-3 py-2">S3</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {(executions.data?.backup_executions || []).map((b) => (
              <tr key={b.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-mono text-xs">
                  {new Date(b.started_at).toLocaleString()}
                </td>
                <td className="px-3 py-2">
                  {b.status}
                  {b.error_message ? (
                    <span className="ml-2 text-xs text-error-500">{b.error_message}</span>
                  ) : null}
                </td>
                <td className="px-3 py-2">{formatBytes(b.size_bytes)}</td>
                <td className="px-3 py-2 font-mono text-xs">{b.filename || '—'}</td>
                <td className="px-3 py-2 text-xs">{b.s3_uploaded ? 'yes' : '—'}</td>
                <td className="px-3 py-2 text-right">
                  {b.status === 'finished' ? (
                    <button
                      type="button"
                      className="text-brand-600 dark:text-brand-400"
                      disabled={restoreBackup.isPending}
                      onClick={() => {
                        void (async () => {
                          if (
                            await confirm({
                              title: 'Restore backup',
                              message: `Restore volumes from ${b.filename || 'this backup'}?`,
                              confirmLabel: 'Restore',
                              danger: true,
                            })
                          ) {
                            restoreBackup.mutate(b.id)
                          }
                        })()
                      }}
                    >
                      Restore
                    </button>
                  ) : (
                    '—'
                  )}
                </td>
              </tr>
            ))}
            {!executions.data?.backup_executions?.length && (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  No backup runs yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="panel-card overflow-hidden">
        <div className="border-b border-gray-200 px-3 py-2 text-sm font-medium dark:border-gray-800">
          Schedules
        </div>
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Frequency</th>
              <th className="px-3 py-2">Retention</th>
              <th className="px-3 py-2">Volume</th>
              <th className="px-3 py-2">S3</th>
              <th className="px-3 py-2">Enabled</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {mine.map((b) => (
              <tr key={b.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-mono text-xs">{b.frequency}</td>
                <td className="px-3 py-2">{b.retention}</td>
                <td className="px-3 py-2 font-mono text-xs">{b.volume_id?.slice(0, 8) || 'all'}</td>
                <td className="px-3 py-2 font-mono text-xs">{b.s3_storage_id?.slice(0, 8) || '—'}</td>
                <td className="px-3 py-2">{b.enabled ? 'yes' : 'no'}</td>
                <td className="space-x-3 px-3 py-2">
                  <button
                    type="button"
                    className="text-brand-600 dark:text-brand-400"
                    onClick={() => toggleSchedule.mutate({ id: b.id, enabled: !b.enabled })}
                  >
                    {b.enabled ? 'Disable' : 'Enable'}
                  </button>
                  <button
                    type="button"
                    className="text-error-500"
                    onClick={() => removeSchedule.mutate(b.id)}
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
            {!mine.length && (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  No scheduled backups for this application.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <form
        className="panel-card grid gap-3 p-4 sm:grid-cols-2"
        onSubmit={(e) => {
          e.preventDefault()
          create.mutate()
        }}
      >
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Cron frequency</span>
          <input
            value={frequency}
            onChange={(e) => setFrequency(e.target.value)}
            className="panel-field w-full rounded-lg px-3 py-2 font-mono text-sm"
          />
        </label>
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Retention (days)</span>
          <input
            value={retention}
            onChange={(e) => setRetention(e.target.value)}
            className="panel-field w-full rounded-lg px-3 py-2 text-sm"
          />
        </label>
        <label className="block text-sm sm:col-span-2">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Volume (optional)</span>
          <select
            value={volumeId}
            onChange={(e) => setVolumeId(e.target.value)}
            className="panel-field w-full rounded-lg px-3 py-2 text-sm"
          >
            <option value="">All volumes</option>
            {(volumes.data?.volumes || []).map((v) => (
              <option key={v.id} value={v.id}>
                {v.name} → {v.mount_path}
              </option>
            ))}
          </select>
        </label>
        <label className="block text-sm sm:col-span-2">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">S3 storage (optional)</span>
          <select
            value={s3Id}
            onChange={(e) => setS3Id(e.target.value)}
            className="panel-field w-full rounded-lg px-3 py-2 text-sm"
          >
            <option value="">None</option>
            {(storages.data?.s3_storages || []).map((s) => (
              <option key={s.id} value={s.id}>
                {s.name} ({s.bucket})
              </option>
            ))}
          </select>
        </label>
        {create.error && <p className="text-sm text-error-500 sm:col-span-2">{create.error.message}</p>}
        <div className="sm:col-span-2">
          <Btn primary type="submit">
            {create.isPending ? 'Saving…' : 'Add schedule'}
          </Btn>
        </div>
      </form>
    </div>
  )
}

function AppMetricsSection({ appId, serverId }: { appId: string; serverId: string }) {
  const appMetrics = useQuery({
    queryKey: ['app-metrics', appId],
    queryFn: () => api.applicationMetrics(appId),
    enabled: Boolean(appId),
    staleTime: 25_000,
    refetchInterval: gentleRefetchInterval(45_000),
    refetchIntervalInBackground: false,
  })
  const metrics = useQuery({
    queryKey: ['server-metrics', serverId],
    queryFn: () => api.serverMetrics(serverId, 20, true),
    enabled: Boolean(serverId),
    staleTime: 25_000,
    refetchInterval: gentleRefetchInterval(30_000),
    refetchIntervalInBackground: false,
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
  const containers = appMetrics.data?.containers || []

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Metrics</h2>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
          Live container stats and host metrics for this application.
        </p>
      </div>

      <div className="panel-card overflow-hidden">
        <div className="border-b border-gray-200 px-3 py-2 text-sm font-medium dark:border-gray-800">
          Containers
        </div>
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">CPU %</th>
              <th className="px-3 py-2">Memory</th>
              <th className="px-3 py-2">Mem %</th>
              <th className="px-3 py-2">Net I/O</th>
              <th className="px-3 py-2">Block I/O</th>
            </tr>
          </thead>
          <tbody>
            {containers.map((c) => (
              <tr key={c.name} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-mono text-xs">{c.name}</td>
                <td className="px-3 py-2 tabular-nums">{c.cpu_percent || '—'}</td>
                <td className="px-3 py-2 font-mono text-xs">{c.mem_usage || '—'}</td>
                <td className="px-3 py-2 tabular-nums">{c.mem_percent || '—'}</td>
                <td className="px-3 py-2 font-mono text-xs">{c.net_io || '—'}</td>
                <td className="px-3 py-2 font-mono text-xs">{c.block_io || '—'}</td>
              </tr>
            ))}
            {!containers.length && (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  {appMetrics.isLoading ? (
                    <div className="mx-auto max-w-md">
                      <TableSkeleton rows={3} cols={3} />
                    </div>
                  ) : (
                    'No container stats yet.'
                  )}
                </td>
              </tr>
            )}
          </tbody>
        </table>
        {appMetrics.error && (
          <p className="border-t border-gray-200 px-3 py-2 text-sm text-error-500 dark:border-gray-800">
            {appMetrics.error.message}
          </p>
        )}
      </div>

      <h3 className="text-sm font-medium text-gray-900 dark:text-white">Host metrics</h3>
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

function LiveContainerLogs({
  appId,
  isCompose,
  includeTimestamps,
  lineLimit,
  onSettingsChange,
}: {
  appId: string
  isCompose: boolean
  includeTimestamps: boolean
  lineLimit: number
  onSettingsChange: (patch: {
    is_include_timestamps?: boolean
    logs_line_limit?: number
  }) => void
}) {
  const [container, setContainer] = useState('')

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

  const name = !isCompose ? `dockfin-${appId}` : container
  const tail = Math.min(Math.max(lineLimit || 200, 50), 5000)
  const streamUrl =
    name && (!isCompose || Boolean(container))
      ? `/api/v1/applications/${appId}/logs/stream?${new URLSearchParams({ tail: String(tail), container: name })}`
      : null
  const { lines, status, error, reconnect } = useLogStream(streamUrl)

  const downloadLogs = () => {
    const blob = new Blob([lines.join('\n')], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${container || appId}-logs.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  const composeNames = containers.data?.containers || []

  return (
    <LiveLogViewer
      status={status}
      error={error}
      lines={lines}
      containers={isCompose ? composeNames : undefined}
      container={isCompose ? container : undefined}
      onContainerChange={isCompose ? setContainer : undefined}
      tail={lineLimit}
      onTailChange={(n) => onSettingsChange({ logs_line_limit: n })}
      timestamps={{
        checked: includeTimestamps,
        onChange: (next) => onSettingsChange({ is_include_timestamps: next }),
      }}
      onDownload={downloadLogs}
      onReconnect={reconnect}
    />
  )
}
