import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import {
  AlertTriangle,
  Archive,
  CalendarClock,
  ExternalLink,
  Globe,
  HardDrive,
  LayoutDashboard,
  Rocket,
  ScrollText,
  Settings2,
  Terminal,
  Variable,
  Webhook,
  Wrench,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { DangerConfirmModal, DangerZoneCard } from '../components/DangerConfirmModal'
import { ConfigSideNav } from '../components/ConfigSideNav'
import { DeployLogPanel } from '../components/DeployLogPanel'
import { LiveLogViewer } from '../components/LiveLogViewer'
import { DomainsPanel, domainsWantAutoHttps } from '../components/DomainsPanel'
import { EnvVarsPanel } from '../components/EnvVarsPanel'
import { LinksMenu } from '../components/LinksMenu'
import { MoveResourcePanel } from '../components/MoveResourcePanel'
import { PersistentStoragesPanel } from '../components/PersistentStoragesPanel'
import { ResourceSetupBanner, type SetupCheck } from '../components/ResourceSetupBanner'
import { ScheduledTasksPanel } from '../components/ScheduledTasksPanel'
import { BackLink } from '../components/BackLink'
import { ResourceSwitcher } from '../components/ResourceSwitcher'
import { ServiceLogo, logoForServiceType } from '../components/ServiceLogo'
import { StatusBadge, statusTone } from '../components/StatusBadge'
import { primaryVisitUrl, ServiceOverview } from '../components/AppOverview'
import { ServiceWebhooksPanel } from '../components/ServiceWebhooksPanel'
import { ServerTerminal } from '../components/Terminal'
import { CodeEditor } from '../components/CodeEditor'
import { DetailPageSkeleton } from '../components/ui/Skeleton'
import { useConfirm } from '../components/ConfirmDialog'
import { useToast } from '../components/Toast'
import { api, type Service, type ServiceUnit } from '../lib/api'
import { deployBlockFromEnv, emptyUserEnvVars } from '../lib/env-readiness'
import { useLogStream } from '../lib/useLogStream'
import { safeExternalHref } from '../lib/url'
import { Btn, Input } from './Servers'

const TOP_TABS = [
  { id: 'overview', label: 'Overview', icon: LayoutDashboard },
  { id: 'logs', label: 'Logs', icon: ScrollText },
  { id: 'terminal', label: 'Terminal', icon: Terminal },
  { id: 'backups', label: 'Backups', icon: Archive },
  { id: 'configuration', label: 'Settings', icon: Settings2 },
] as const

const SIDE_ITEMS = [
  { id: 'general', label: 'General', icon: Settings2 },
  { id: 'domains', label: 'Domains', icon: Globe },
  { id: 'environment', label: 'Environment Variables', icon: Variable },
  { id: 'storages', label: 'Persistent Storages', icon: HardDrive },
  { id: 'tasks', label: 'Scheduled Tasks', icon: CalendarClock },
  { id: 'webhooks', label: 'Webhooks', icon: Webhook },
  { id: 'operations', label: 'Resource Operations', icon: Wrench },
  { id: 'danger', label: 'Danger Zone', icon: AlertTriangle },
] as const

const SVC_SIDE_GROUPS = [
  { label: 'Setup', ids: ['general', 'domains', 'environment'] },
  { label: 'Runtime', ids: ['storages', 'tasks', 'webhooks'] },
  { label: 'Manage', ids: ['operations', 'danger'] },
] as const

type TopTabId = (typeof TOP_TABS)[number]['id']
type SideId = (typeof SIDE_ITEMS)[number]['id']

function isTopTabId(v: string | undefined): v is TopTabId {
  return !!v && TOP_TABS.some((t) => t.id === v)
}

function isSideId(v: string | undefined): v is SideId {
  return !!v && SIDE_ITEMS.some((t) => t.id === v)
}

export type ServiceDetailSearch = {
  deploy?: string
  tab?: TopTabId
  side?: SideId
}

export function parseServiceDetailSearch(s: Record<string, unknown>): ServiceDetailSearch {
  return {
    deploy: typeof s.deploy === 'string' ? s.deploy : undefined,
    tab: typeof s.tab === 'string' && isTopTabId(s.tab) ? s.tab : undefined,
    side: typeof s.side === 'string' && isSideId(s.side) ? s.side : undefined,
  }
}

function titleCase(s: string) {
  return s
    .replace(/[-_]+/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

export function ServiceDetailPage() {
  const { projectId, envId, svcId } = useParams({ strict: false }) as {
    projectId?: string
    envId?: string
    svcId: string
  }
  const search = useSearch({ strict: false }) as ServiceDetailSearch
  const nav = useNavigate()
  const qc = useQueryClient()
  const topTab: TopTabId = isTopTabId(search.tab)
    ? search.tab
    : search.deploy === '1'
      ? 'logs'
      : 'overview'
  const side: SideId = isSideId(search.side) ? search.side : 'general'

  const setSvcNav = (next: { tab?: TopTabId; side?: SideId }) => {
    const tab = next.tab ?? topTab
    const nextSide = next.side ?? side
    void nav({
      search: ((prev: Record<string, unknown>) => {
        const { tab: _t, side: _s, deploy: _d, ...rest } = prev
        const out: Record<string, unknown> = { ...rest }
        if (tab !== 'overview') out.tab = tab
        if (nextSide !== 'general') out.side = nextSide
        return out
      }) as never,
      replace: true,
    })
  }
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [fqdn, setFqdn] = useState('')
  const httpsRedirectApplies = useMemo(() => domainsWantAutoHttps(fqdn), [fqdn])
  const [showCompose, setShowCompose] = useState(false)
  const [composeDraft, setComposeDraft] = useState('')
  const [showDetails, setShowDetails] = useState(false)
  const [deployLines, setDeployLines] = useState<string[]>([])
  const [deployBusy, setDeployBusy] = useState(false)
  const [deployError, setDeployError] = useState('')
  const [deleteOpen, setDeleteOpen] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  const autoDeployed = useRef(false)
  const confirm = useConfirm()
  const toast = useToast()

  useEffect(() => {
    setShowCompose(false)
    setShowDetails(false)
    setFqdn('')
    setDeployLines([])
    setDeployError('')
    setDeployBusy(false)
    autoDeployed.current = false
    abortRef.current?.abort()
  }, [svcId])

  const svc = useQuery({ queryKey: ['service', svcId], queryFn: () => api.getService(svcId) })
  const templates = useQuery({ queryKey: ['templates'], queryFn: api.templates })
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const envVarsQ = useQuery({
    queryKey: ['env-vars', 'service', svcId, 'prod'],
    queryFn: () => api.envVars('service', svcId, true),
    enabled: Boolean(svcId),
  })
  const emptyEnv = useMemo(
    () => emptyUserEnvVars(envVarsQ.data?.environment_variables),
    [envVarsQ.data?.environment_variables],
  )
  const sideItems = useMemo(
    () =>
      SIDE_ITEMS.map((item) =>
        item.id === 'environment' && emptyEnv.length ? { ...item, badge: emptyEnv.length } : item,
      ),
    [emptyEnv.length],
  )

  useEffect(() => {
    if (!svc.data) return
    setName(svc.data.name || '')
    setDescription(svc.data.description || '')
    setFqdn(svc.data.fqdn || '')
    setComposeDraft(svc.data.docker_compose_raw || svc.data.docker_compose || '')
  }, [svc.data])

  const serverId = useMemo(() => {
    const s = svc.data
    if (!s) return ''
    if (s.server_id) return s.server_id
    if (s.destination_id) {
      return (dests.data?.destinations || []).find((d) => d.id === s.destination_id)?.server_id || ''
    }
    return ''
  }, [svc.data, dests.data])

  const containerOptions = useMemo(() => {
    const s = svc.data
    if (!s) return [] as string[]
    const units = s.units || []
    return units.map((u) => `dockfin-svc-${svcId.slice(0, 8)}-${u.name}-1`)
  }, [svc.data, svcId])

  const save = useMutation({
    mutationFn: () => api.updateService(svcId, { name, description }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['service', svcId] }),
  })
  const saveCompose = useMutation({
    mutationFn: (raw: string) => api.updateService(svcId, { docker_compose_raw: raw }),
    onSuccess: () => {
      toast.success('Compose saved — redeploy to apply')
      void qc.invalidateQueries({ queryKey: ['service', svcId] })
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : 'Failed to save compose'),
  })
  const saveDomains = useMutation({
    mutationFn: (nextFqdn: string) => api.updateService(svcId, { fqdn: nextFqdn }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['service', svcId] }),
  })
  const stop = useMutation({
    mutationFn: () => api.stopService(svcId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['service', svcId] })
      void qc.invalidateQueries({ queryKey: ['services'] })
    },
  })
  const restart = useMutation({
    mutationFn: () => api.restartService(svcId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['service', svcId] })
      void qc.invalidateQueries({ queryKey: ['services'] })
    },
  })
  const remove = useMutation({
    mutationFn: (body: Parameters<typeof api.deleteService>[1]) => api.deleteService(svcId, body),
    onSuccess: () => {
      setDeleteOpen(false)
      void qc.invalidateQueries({ queryKey: ['services'] })
      void qc.invalidateQueries({ queryKey: ['project'] })
      if (projectId && envId) {
        void nav({
          to: '/projects/$projectId/environments/$envId',
          params: { projectId, envId },
        })
      } else {
        void nav({ to: '/projects' })
      }
    },
  })

  const runDeploy = async () => {
    const gate = deployBlockFromEnv(envVarsQ)
    if (gate.block) {
      toast.warning(gate.message || 'Finish setup before deploying.')
      if (gate.empty.length) setSvcNav({ tab: 'configuration', side: 'environment' })
      return
    }
    abortRef.current?.abort()
    const ac = new AbortController()
    abortRef.current = ac
    setSvcNav({ tab: 'logs' })
    setDeployBusy(true)
    setDeployError('')
    setDeployLines([])
    try {
      await api.deployServiceStream(
        svcId,
        (ev) => {
          const prefix = ev.stage ? `[${ev.stage}] ` : ''
          setDeployLines((prev) => [...prev, `${prefix}${ev.line}`])
        },
        ac.signal,
      )
      void qc.invalidateQueries({ queryKey: ['service', svcId] })
      void qc.invalidateQueries({ queryKey: ['services'] })
      toast.success('Deploy finished')
    } catch (e) {
      if ((e as Error).name === 'AbortError') return
      const msg = e instanceof Error ? e.message : 'Deploy failed'
      setDeployError(msg)
      toast.error(msg)
      setDeployLines((prev) => (prev.some((l) => l.includes(msg)) ? prev : [...prev, `[error] ${msg}`]))
      void qc.invalidateQueries({ queryKey: ['service', svcId] })
    } finally {
      setDeployBusy(false)
    }
  }

  useEffect(() => {
    if (search.deploy !== '1' || !svc.data || autoDeployed.current || deployBusy) return
    // Wait until env vars are fetched so we do not skip the empty-env gate.
    if (envVarsQ.isPending && !envVarsQ.data) return
    autoDeployed.current = true
    void runDeploy()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search.deploy, svc.data, envVarsQ.isPending, envVarsQ.data])

  const back =
    projectId && envId ? (
      <BackLink
        label="Resources"
        to="/projects/$projectId/environments/$envId"
        params={{ projectId, envId }}
      />
    ) : (
      <BackLink label="Projects" to="/projects" />
    )

  if (svc.isLoading) return <DetailPageSkeleton withSideNav />
  if (svc.error || !svc.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{svc.error?.message || 'Service not found'}</p>
        {back}
      </div>
    )
  }

  const s = svc.data
  const logo = logoForServiceType(s.service_type, templates.data?.templates || [])
  const units = s.units || []
  const docsURL = `https://www.google.com/search?q=${encodeURIComponent(s.service_type + ' documentation')}`
  const visitHref = primaryVisitUrl(s)
  const destMeta = (dests.data?.destinations || []).find((d) => d.id === s.destination_id)
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
      onAction: () => setSvcNav({ tab: 'configuration', side: 'environment' }),
    },
    {
      id: 'dest',
      ok: Boolean(s.destination_id || s.server_id),
      title: 'Destination server',
      hint: 'This stack has no destination assigned yet.',
    },
    {
      id: 'domain',
      ok:
        Boolean((fqdn || s.fqdn || '').trim()) || Boolean((s.links || []).length),
      title: 'Public domain',
      hint: 'Add a magic or custom domain so Traefik can route traffic.',
      actionLabel: 'Set domain',
      onAction: () => setSvcNav({ tab: 'configuration', side: 'domains' }),
    },
  ]

  return (
    <div className="space-y-5">
      {back}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 items-center gap-3">
          <ServiceLogo src={logo} name={s.name} className="h-10 w-10" />
          <div className="min-w-0">
            <h1 className="flex flex-wrap items-center gap-2 text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
              {s.name}
              <ResourceSwitcher
                kind="service"
                currentId={svcId}
                environmentId={s.environment_id || envId}
                projectId={projectId}
              />
            </h1>
            <div className="mt-1.5 flex flex-wrap items-center gap-2">
              <span className="text-xs capitalize text-gray-500 dark:text-gray-400">
                {titleCase(s.service_type)}
              </span>
              <StatusBadge status={s.status} />
              {emptyEnv.length ? (
                <button
                  type="button"
                  onClick={() => setSvcNav({ tab: 'configuration', side: 'environment' })}
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
          <LinksMenu links={s.links || []} />
          <button
            type="button"
            title="Restart"
            className="inline-flex h-8 items-center gap-1.5 rounded-md bg-amber-500/15 px-2.5 text-xs font-medium text-amber-700 hover:bg-amber-500/25 dark:text-amber-300"
            disabled={restart.isPending || deployBusy}
            onClick={() => restart.mutate()}
          >
            {restart.isPending ? 'Restarting…' : 'Restart'}
          </button>
          <button
            type="button"
            title="Stop"
            className="inline-flex h-8 items-center gap-1.5 rounded-md bg-error-500/15 px-2.5 text-xs font-medium text-error-500 hover:bg-error-500/25"
            disabled={stop.isPending || deployBusy}
            onClick={() => {
              void (async () => {
                if (
                  await confirm({
                    title: 'Stop service',
                    message: 'Stop this service stack?',
                    confirmLabel: 'Stop',
                    danger: true,
                  })
                ) {
                  stop.mutate()
                }
              })()
            }}
          >
            {stop.isPending ? 'Stopping…' : 'Stop'}
          </button>
          <Btn primary onClick={() => void runDeploy()} disabled={deployBusy}>
            <span className="inline-flex items-center gap-1.5">
              <Rocket className="h-3.5 w-3.5" />
              {deployBusy ? 'Deploying…' : 'Redeploy'}
            </span>
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
                onClick={() => setSvcNav({ tab: t.id })}
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

      {(stop.error || restart.error || save.error || saveDomains.error) && (
        <p className="text-sm text-error-500">
          {(stop.error || restart.error || save.error || saveDomains.error)?.message}
        </p>
      )}

      {topTab === 'overview' && (
        <div className="space-y-6">
          <ResourceSetupBanner checks={setupChecks} />
          <ServiceOverview
            service={s}
            logoSrc={logo || ''}
            destinationName={destMeta?.name}
            emptyEnvCount={emptyEnv.length}
            envTotal={(envVarsQ.data?.environment_variables || []).length}
            onRedeploy={() => void runDeploy()}
            onOpenLogs={() => setSvcNav({ tab: 'logs' })}
            onOpenSettings={(sideId) => setSvcNav({ tab: 'configuration', side: sideId })}
            deployBusy={deployBusy}
          />
        </div>
      )}

      {topTab === 'backups' && <ServiceBackupsPanel svcId={svcId} />}

      {topTab === 'logs' && (
        <div className="space-y-6">
          {(deployBusy || deployLines.length > 0 || deployError) && (
            <div className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                  Deploy output
                </h2>
                <Btn primary onClick={() => void runDeploy()} disabled={deployBusy}>
                  {deployBusy ? 'Deploying…' : 'Redeploy'}
                </Btn>
              </div>
              <DeployLogPanel
                lines={deployLines}
                busy={deployBusy}
                emptyHint="Click Redeploy to stream compose output here…"
              />
              {deployError && <p className="text-sm text-error-500">{deployError}</p>}
            </div>
          )}
          <ServiceLiveLogs svcId={svcId} fallbackContainers={containerOptions} />
        </div>
      )}

      {topTab === 'terminal' && (
        <div className="space-y-3">
          {serverId ? (
            <ServerTerminal
              serverId={serverId}
              defaultContainer={containerOptions[0] || ''}
              containerOptions={containerOptions}
              hideHostShell
            />
          ) : (
            <div className="panel-card p-5 text-sm text-gray-500 dark:text-gray-400">
              This service has no server/destination assigned, so a container terminal cannot open yet.
            </div>
          )}
        </div>
      )}

      {topTab === 'configuration' && (
        <div className="flex flex-col gap-6 md:flex-row">
          <ConfigSideNav
            items={sideItems}
            groups={SVC_SIDE_GROUPS}
            active={side}
            onSelect={(id) => setSvcNav({ tab: 'configuration', side: id })}
            header={
              <a
                href={docsURL}
                target="_blank"
                rel="noreferrer"
                className="mb-2 flex items-center gap-1.5 px-2 py-1.5 text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
              >
                Documentation
                <span aria-hidden className="text-xs">
                  ↗
                </span>
              </a>
            }
          />

          <div className="min-w-0 flex-1 space-y-6">
            <ResourceSetupBanner checks={setupChecks} />
            {side === 'general' && (
              <GeneralPanel
                s={s}
                name={name}
                description={description}
                setName={setName}
                setDescription={setDescription}
                units={units}
                showCompose={showCompose}
                setShowCompose={setShowCompose}
                composeDraft={composeDraft}
                setComposeDraft={setComposeDraft}
                onSaveCompose={() => saveCompose.mutate(composeDraft)}
                composeBusy={saveCompose.isPending}
                showDetails={showDetails}
                setShowDetails={setShowDetails}
                saveBusy={save.isPending}
                onSave={(e) => {
                  e.preventDefault()
                  save.mutate()
                }}
                onRestartUnit={() => restart.mutate()}
                restartBusy={restart.isPending}
                onOpenUnitSettings={() => setSvcNav({ tab: 'configuration', side: 'environment' })}
              />
            )}
            {side === 'domains' && (
              <div className="panel-card space-y-4 p-5">
                <DomainsPanel
                  value={fqdn}
                  onChange={setFqdn}
                  onSave={(next) => {
                    setFqdn(next)
                    saveDomains.mutate(next)
                  }}
                  saveBusy={saveDomains.isPending}
                  serverId={serverId || undefined}
                  destinationId={s.destination_id || undefined}
                  resourceId={svcId}
                  resourceName={s.name}
                />
                <label className={`flex items-start gap-3 text-sm ${httpsRedirectApplies ? '' : 'opacity-60'}`}>
                  <input
                    type="checkbox"
                    className="mt-0.5 h-4 w-4 accent-[var(--color-accent)]"
                    checked={httpsRedirectApplies && s.is_force_https !== false}
                    disabled={!httpsRedirectApplies}
                    onChange={(e) => {
                      if (!httpsRedirectApplies) return
                      void api
                        .updateService(svcId, { is_force_https: e.target.checked })
                        .then(() => qc.invalidateQueries({ queryKey: ['service', svcId] }))
                    }}
                  />
                  <span>
                    <span className="font-medium text-gray-900 dark:text-white">Force HTTPS redirects</span>
                    <span className="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">
                      {httpsRedirectApplies
                        ? 'Redirect HTTP to HTTPS. Redeploy to apply.'
                        : 'Magic domains stay HTTP. Add a custom domain first.'}
                    </span>
                  </span>
                </label>
              </div>
            )}
            {side === 'environment' && (
              <EnvVarsPanel resourceType="service" resourceId={svcId} />
            )}
            {side === 'storages' && (
              <PersistentStoragesPanel
                compose={s.docker_compose || s.docker_compose_raw || ''}
                volumes={s.volumes}
              />
            )}
            {side === 'tasks' && (
              <ScheduledTasksPanel
                resourceType="service"
                resourceId={svcId}
                containerOptions={(units || []).map((u) => u.name)}
              />
            )}
            {side === 'webhooks' && <ServiceWebhooksPanel serviceId={svcId} />}
            {side === 'operations' && (
              <div className="space-y-4">
                <div className="panel-card space-y-3 p-5">
                  <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                    Resource Operations
                  </h2>
                  <div className="flex flex-wrap gap-2">
                    <Btn primary onClick={() => void runDeploy()} disabled={deployBusy}>
                      {deployBusy ? 'Deploying…' : 'Deploy / Restart stack'}
                    </Btn>
                    <Btn onClick={() => restart.mutate()} disabled={restart.isPending}>
                      Restart
                    </Btn>
                    <Btn onClick={() => stop.mutate()} disabled={stop.isPending}>
                      Stop
                    </Btn>
                  </div>
                </div>
                <MoveResourcePanel
                  resourceType="service"
                  resourceId={svcId}
                  currentEnvironmentId={s.environment_id}
                  projectId={projectId}
                />
              </div>
            )}
            {side === 'danger' && (
              <div className="space-y-4">
                <DangerZoneCard>
                  <div>
                    <h3 className="text-sm font-semibold text-error-500">Force deploy</h3>
                    <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                      Recreates containers from the stored compose file.
                    </p>
                  </div>
                  <Btn primary onClick={() => void runDeploy()} disabled={deployBusy}>
                    {deployBusy ? 'Deploying…' : 'Force deploy'}
                  </Btn>
                </DangerZoneCard>
                <DangerZoneCard>
                  <div>
                    <h3 className="text-sm font-semibold text-error-500">Delete Resource</h3>
                    <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                      This will stop your compose stack, optionally remove volumes/config files, and
                      delete the service from Dockfin. Beware — there is no coming back.
                    </p>
                    <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
                      Stack status:{' '}
                      <span className="font-medium capitalize">{s.status || 'unknown'}</span>
                      {s.status === 'running'
                        ? ' — containers are running and will be stopped.'
                        : ' — containers will be removed if still present.'}
                    </p>
                  </div>
                  <Btn type="button" onClick={() => setDeleteOpen(true)}>
                    Delete
                  </Btn>
                </DangerZoneCard>
                <DangerConfirmModal
                  open={deleteOpen}
                  onClose={() => setDeleteOpen(false)}
                  title="Confirm Resource Deletion?"
                  resourceLabel="Resource Name"
                  expectedName={s.name}
                  statusLine={
                    s.status === 'running'
                      ? 'Service stack is RUNNING. Deleting runs docker compose down and removes selected data.'
                      : `Current status: ${s.status || 'unknown'}.`
                  }
                  actions={[
                    'Permanently delete all containers of this resource.',
                    'Optionally remove volumes, config directory, and run docker image prune.',
                    'Remove the service record from Dockfin.',
                  ]}
                  requirePassword
                  showResourceCheckboxes
                  busy={remove.isPending}
                  error={remove.error?.message}
                  onConfirm={(payload) => remove.mutate(payload)}
                />
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function GeneralPanel({
  s,
  name,
  description,
  setName,
  setDescription,
  units,
  showCompose,
  setShowCompose,
  composeDraft,
  setComposeDraft,
  onSaveCompose,
  composeBusy,
  showDetails,
  setShowDetails,
  saveBusy,
  onSave,
  onRestartUnit,
  restartBusy,
  onOpenUnitSettings,
}: {
  s: Service
  name: string
  description: string
  setName: (v: string) => void
  setDescription: (v: string) => void
  units: ServiceUnit[]
  showCompose: boolean
  setShowCompose: (v: boolean) => void
  composeDraft: string
  setComposeDraft: (v: string) => void
  onSaveCompose: () => void
  composeBusy: boolean
  showDetails: boolean
  setShowDetails: (v: boolean) => void
  saveBusy: boolean
  onSave: (e: FormEvent) => void
  onRestartUnit: () => void
  restartBusy: boolean
  onOpenUnitSettings: () => void
}) {
  const storedCompose = s.docker_compose_raw || s.docker_compose || ''
  const composeDirty = composeDraft !== storedCompose

  return (
    <form className="space-y-6" onSubmit={onSave}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Service stack</h2>
          <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
            Name, compose services, and network membership.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Btn primary type="submit">
            {saveBusy ? 'Saving…' : 'Save'}
          </Btn>
          <Btn type="button" onClick={() => setShowCompose(!showCompose)}>
            {showCompose ? 'Hide Compose' : 'Edit Compose File'}
          </Btn>
          <Btn type="button" onClick={() => setShowDetails(!showDetails)}>
            {showDetails ? 'Hide details' : 'Details'}
          </Btn>
        </div>
      </div>

      <section className="panel-card space-y-4 p-5">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Configuration</h3>
        <div className="grid gap-4 sm:grid-cols-2">
          <Input label="Service Name" value={name} onChange={setName} />
          <Input
            label="Description"
            value={description}
            onChange={setDescription}
            required={false}
          />
        </div>
        <label className="flex items-center gap-3 text-sm text-gray-700 dark:text-gray-300">
          <input type="checkbox" checked readOnly className="rounded border-gray-300" />
          Connect To Predefined Network
        </label>
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Stack joins the destination Docker network (usually <code>dockfin</code>) so Traefik can
          route public domains.
        </p>
      </section>

      <section className="space-y-3">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Services</h3>
        <div className="space-y-3">
          {units.map((u) => (
            <UnitCard
              key={u.name}
              unit={u}
              stackStatus={s.status}
              onRestart={onRestartUnit}
              restartBusy={restartBusy}
              onSettings={onOpenUnitSettings}
            />
          ))}
          {!units.length && (
            <div className="rounded-lg border border-dashed border-gray-300 p-6 text-sm text-gray-500 dark:border-gray-700 dark:text-gray-400">
              No compose services found.
            </div>
          )}
        </div>
      </section>

      {showCompose && (
        <section className="panel-card space-y-2 p-5">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="mr-2 text-sm font-semibold text-gray-900 dark:text-white">
              Compose file
            </h3>
            <Btn primary type="button" onClick={onSaveCompose} disabled={composeBusy || !composeDirty}>
              {composeBusy ? 'Saving…' : 'Save compose'}
            </Btn>
            <Btn
              type="button"
              onClick={() => setComposeDraft(storedCompose)}
              disabled={composeBusy || !composeDirty}
            >
              Reset
            </Btn>
          </div>
          <CodeEditor
            language="yaml"
            readOnly={false}
            height="28rem"
            ariaLabel="Docker Compose YAML"
            value={composeDraft}
            onChange={setComposeDraft}
          />
          <p className="text-xs text-gray-500 dark:text-gray-400">
            Saving stores your YAML and discards the prepared copy — the next deploy re-applies
            networks, magic environment variables, and Traefik labels.
          </p>
        </section>
      )}

      {showDetails && (
        <section className="panel-card grid gap-3 p-5 sm:grid-cols-2">
          <DetailRow label="ID" value={s.id} mono />
          <DetailRow label="Type" value={s.service_type} />
          <DetailRow label="Status" value={s.status} />
          <DetailRow label="FQDN" value={s.fqdn || '—'} mono />
          <DetailRow label="Destination" value={s.destination_id || '—'} mono />
          <DetailRow label="Server" value={s.server_id || '—'} mono />
        </section>
      )}
    </form>
  )
}

function UnitCard({
  unit,
  stackStatus,
  onRestart,
  restartBusy,
  onSettings,
}: {
  unit: ServiceUnit
  stackStatus: string
  onRestart: () => void
  restartBusy: boolean
  onSettings: () => void
}) {
  const status = unit.status || stackStatus
  const tone = statusTone(status)
  const bar =
    tone === 'ok' ? 'bg-emerald-500' : tone === 'warn' ? 'bg-amber-500' : tone === 'bad' ? 'bg-error-500' : 'bg-gray-400'
  const link = unit.links?.[0]

  return (
    <div className="flex overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-950">
      <div className={`w-1 shrink-0 ${bar}`} />
      <div className="flex min-w-0 flex-1 flex-wrap items-center justify-between gap-3 px-4 py-3">
        <div className="min-w-0">
          <div className="font-medium text-gray-900 dark:text-white">
            {titleCase(unit.name)}
            {unit.image ? (
              <span className="font-normal text-gray-500 dark:text-gray-400"> ({unit.image})</span>
            ) : null}
          </div>
          {link && safeExternalHref(link.url) ? (
            <a
              href={safeExternalHref(link.url)}
              target="_blank"
              rel="noreferrer"
              className="mt-1 inline-flex max-w-full items-center gap-1 truncate text-sm text-brand-600 hover:underline dark:text-brand-400"
            >
              <span className="truncate">{link.url}</span>
              <span aria-hidden className="shrink-0 text-xs">
                ↗
              </span>
            </a>
          ) : (
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">No public URL</p>
          )}
          <div className="mt-1.5">
            <StatusBadge status={status} />
          </div>
        </div>
        <div className="flex shrink-0 gap-2">
          <Btn type="button" onClick={onSettings}>
            Settings
          </Btn>
          <Btn type="button" onClick={onRestart} disabled={restartBusy}>
            Restart
          </Btn>
        </div>
      </div>
    </div>
  )
}

function ServiceLiveLogs({
  svcId,
  fallbackContainers,
}: {
  svcId: string
  fallbackContainers: string[]
}) {
  const [container, setContainer] = useState('')
  const [tail, setTail] = useState(200)

  const containers = useQuery({
    queryKey: ['service-containers', svcId],
    queryFn: () => api.serviceContainers(svcId),
  })

  const options = useMemo(() => {
    const list = containers.data?.containers || []
    return list.length ? list : fallbackContainers
  }, [containers.data, fallbackContainers])

  useEffect(() => {
    if (options.length && (!container || !options.includes(container))) {
      setContainer(options[0])
    }
  }, [options, container])

  const streamUrl = container
    ? `/api/v1/services/${svcId}/logs/stream?${new URLSearchParams({ tail: String(tail), container })}`
    : null
  const { lines, status, error, reconnect } = useLogStream(streamUrl)

  const downloadLogs = () => {
    const blob = new Blob([lines.join('\n')], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${container || svcId}-logs.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <LiveLogViewer
      status={status}
      error={error}
      lines={lines}
      containers={options}
      container={container}
      onContainerChange={setContainer}
      tail={tail}
      onTailChange={setTail}
      onDownload={downloadLogs}
      onReconnect={reconnect}
    />
  )
}

function DetailRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-lg border border-gray-200 px-3 py-2 dark:border-gray-800">
      <div className="text-xs text-gray-500 dark:text-gray-400">{label}</div>
      <div className={`mt-0.5 text-sm text-gray-900 dark:text-white ${mono ? 'font-mono text-xs break-all' : ''}`}>
        {value}
      </div>
    </div>
  )
}

function ServiceBackupsPanel({ svcId }: { svcId: string }) {
  const qc = useQueryClient()
  const confirm = useConfirm()
  const toast = useToast()
  const backups = useQuery({ queryKey: ['scheduled-backups'], queryFn: api.scheduledBackups })
  const executions = useQuery({
    queryKey: ['svc-backups', svcId],
    queryFn: () => api.serviceBackups(svcId),
    refetchInterval: (q) => {
      const list = q.state.data?.backup_executions || []
      return list.some((b) => b.status === 'running') ? 2000 : false
    },
  })
  const [frequency, setFrequency] = useState('0 0 * * *')
  const [retention, setRetention] = useState('7')
  const mine = (backups.data?.scheduled_backups || []).filter(
    (b) => b.resource_type === 'service' && b.resource_id === svcId,
  )
  const create = useMutation({
    mutationFn: () =>
      api.createScheduledBackup({
        resource_type: 'service',
        resource_id: svcId,
        frequency,
        retention: Number(retention) || 7,
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['scheduled-backups'] }),
  })
  const runNow = useMutation({
    mutationFn: () => api.runServiceBackup(svcId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['svc-backups', svcId] }),
  })
  const restoreBackup = useMutation({
    mutationFn: (executionId: string) => api.restoreServiceBackup(svcId, { execution_id: executionId }),
    onSuccess: () => toast.success('Backup restored'),
    onError: (e: Error) => toast.error(e.message || 'Restore failed'),
  })
  const removeSchedule = useMutation({
    mutationFn: (id: string) => api.deleteScheduledBackup(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['scheduled-backups'] }),
  })

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Archives <code className="text-xs">/data/dockfin/services/{svcId}</code> on the server.
        </p>
        <Btn primary disabled={runNow.isPending} onClick={() => runNow.mutate()}>
          {runNow.isPending ? 'Archiving…' : 'Run backup now'}
        </Btn>
      </div>
      {runNow.error && <p className="text-sm text-error-500">{runNow.error.message}</p>}
      <div className="panel-card overflow-hidden">
        <table className="panel-table">
          <thead>
            <tr>
              <th>Status</th>
              <th>File</th>
              <th>Started</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {(executions.data?.backup_executions || []).map((b) => (
              <tr key={b.id}>
                <td className="capitalize">{b.status}</td>
                <td className="font-mono text-xs">{b.filename}</td>
                <td className="text-xs text-gray-500">
                  {b.started_at ? new Date(b.started_at).toLocaleString() : '—'}
                </td>
                <td>
                  {b.status === 'finished' && (
                    <button
                      type="button"
                      className="text-xs text-brand-600 hover:underline"
                      onClick={() => {
                        void (async () => {
                          if (
                            await confirm({
                              title: 'Restore backup',
                              message: `Restore files from ${b.filename}?`,
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
                  )}
                </td>
              </tr>
            ))}
            {!executions.data?.backup_executions?.length && (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-gray-500">
                  No backup runs yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <div className="panel-card space-y-3 p-5">
        <h3 className="text-sm font-medium">Schedule</h3>
        <div className="grid gap-3 sm:grid-cols-2">
          <Input label="Cron" value={frequency} onChange={setFrequency} />
          <Input label="Retention" value={retention} onChange={setRetention} />
        </div>
        <Btn onClick={() => create.mutate()} disabled={create.isPending}>
          {create.isPending ? 'Saving…' : 'Add schedule'}
        </Btn>
        {mine.map((b) => (
          <div key={b.id} className="flex items-center justify-between text-sm">
            <span className="font-mono text-xs">{b.frequency}</span>
            <button type="button" className="text-error-500" onClick={() => removeSchedule.mutate(b.id)}>
              Remove
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
