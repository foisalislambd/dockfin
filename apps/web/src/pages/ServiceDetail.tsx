import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams, useSearch } from '@tanstack/react-router'
import {
  AlertTriangle,
  CalendarClock,
  Globe,
  HardDrive,
  Link2,
  ScrollText,
  Settings2,
  Terminal,
  Variable,
  Webhook,
  Wrench,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { DangerConfirmModal, DangerZoneCard } from '../components/DangerConfirmModal'
import { DeployLogPanel } from '../components/DeployLogPanel'
import { DomainsPanel } from '../components/DomainsPanel'
import { EnvVarsPanel } from '../components/EnvVarsPanel'
import { LinksMenu, LinksPanel } from '../components/LinksMenu'
import { MoveResourcePanel } from '../components/MoveResourcePanel'
import { PersistentStoragesPanel } from '../components/PersistentStoragesPanel'
import { ScheduledTasksPanel } from '../components/ScheduledTasksPanel'
import { ServiceLogo } from '../components/ServiceLogo'
import { ServiceWebhooksPanel } from '../components/ServiceWebhooksPanel'
import { ServerTerminal } from '../components/Terminal'
import { CodeEditor } from '../components/CodeEditor'
import { PageSkeleton } from '../components/ui/Skeleton'
import { useConfirm } from '../components/ConfirmDialog'
import { useToast } from '../components/Toast'
import { api, type Service, type ServiceUnit } from '../lib/api'
import { Btn, Input } from './Servers'

const TOP_TABS = [
  { id: 'configuration', label: 'Configuration', icon: Settings2 },
  { id: 'logs', label: 'Logs', icon: ScrollText },
  { id: 'terminal', label: 'Terminal', icon: Terminal },
  { id: 'links', label: 'Links', icon: Link2 },
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

function titleCase(s: string) {
  return s
    .replace(/[-_]+/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

function statusTone(status: string) {
  const s = (status || '').toLowerCase()
  if (s.includes('run') || s.includes('healthy')) return 'ok'
  if (s.includes('deploy') || s.includes('start')) return 'warn'
  if (s.includes('exit') || s.includes('stop') || s.includes('fail') || s.includes('error')) return 'bad'
  return 'muted'
}

function StatusText({ status }: { status: string }) {
  const tone = statusTone(status)
  const label =
    tone === 'ok'
      ? `Running${status.toLowerCase().includes('healthy') ? ' (healthy)' : ''}`
      : status || 'Unknown'
  const color =
    tone === 'ok'
      ? 'text-emerald-600 dark:text-emerald-400'
      : tone === 'warn'
        ? 'text-amber-600 dark:text-amber-400'
        : tone === 'bad'
          ? 'text-error-500'
          : 'text-gray-500 dark:text-gray-400'
  return <span className={`text-sm ${color}`}>{label}</span>
}

export function ServiceDetailPage() {
  const { projectId, envId, svcId } = useParams({ strict: false }) as {
    projectId?: string
    envId?: string
    svcId: string
  }
  const search = useSearch({ strict: false }) as { deploy?: string }
  const nav = useNavigate()
  const qc = useQueryClient()
  const [topTab, setTopTab] = useState<(typeof TOP_TABS)[number]['id']>(
    search.deploy === '1' ? 'logs' : 'configuration',
  )
  const [side, setSide] = useState<(typeof SIDE_ITEMS)[number]['id']>('general')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [fqdn, setFqdn] = useState('')
  const [showCompose, setShowCompose] = useState(false)
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
    setTopTab(search.deploy === '1' ? 'logs' : 'configuration')
    setSide('general')
    setShowCompose(false)
    setShowDetails(false)
    setFqdn('')
    setDeployLines([])
    setDeployError('')
    setDeployBusy(false)
    autoDeployed.current = false
    abortRef.current?.abort()
  }, [svcId, search.deploy])

  const svc = useQuery({ queryKey: ['service', svcId], queryFn: () => api.getService(svcId) })
  const templates = useQuery({ queryKey: ['templates'], queryFn: api.templates })
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })

  useEffect(() => {
    if (!svc.data) return
    setName(svc.data.name || '')
    setDescription(svc.data.description || '')
    setFqdn(svc.data.fqdn || '')
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
    abortRef.current?.abort()
    const ac = new AbortController()
    abortRef.current = ac
    setTopTab('logs')
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
    autoDeployed.current = true
    void runDeploy()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search.deploy, svc.data])

  const back =
    projectId && envId ? (
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

  if (svc.isLoading) return <PageSkeleton cards={2} />
  if (svc.error || !svc.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{svc.error?.message || 'Service not found'}</p>
        {back}
      </div>
    )
  }

  const s = svc.data
  const logo = (templates.data?.templates || []).find((t) => t.type === s.service_type)?.logo
  const units = s.units || []
  const docsURL = `https://www.google.com/search?q=${encodeURIComponent(s.service_type + ' documentation')}`

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          {back}
          <div className="mt-2 flex items-center gap-3">
            <ServiceLogo src={logo} name={s.name} className="h-11 w-11" />
            <div>
              <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">
                {s.name}
              </h1>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {titleCase(s.service_type)} · <StatusText status={s.status} />
              </p>
            </div>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
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
            {deployBusy ? 'Deploying…' : 'Deploy'}
          </Btn>
        </div>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 dark:border-gray-800">
        <nav className="flex flex-wrap gap-1">
          {TOP_TABS.map((t) => {
            const Icon = t.icon
            const active = topTab === t.id
            return (
              <button
                key={t.id}
                type="button"
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

      {(stop.error || restart.error || save.error || saveDomains.error) && (
        <p className="text-sm text-error-500">
          {(stop.error || restart.error || save.error || saveDomains.error)?.message}
        </p>
      )}

      {topTab === 'links' && <LinksPanel links={s.links || []} />}

      {topTab === 'logs' && (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Deploy / compose output from the target server.
            </p>
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
          <aside className="w-full shrink-0 md:w-52">
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
            <nav className="space-y-0.5">
              {SIDE_ITEMS.map((item) => {
                const Icon = item.icon
                const active = side === item.id
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setSide(item.id)}
                    className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm ${
                      active
                        ? 'bg-gray-100 font-medium text-gray-900 dark:bg-white/10 dark:text-white'
                        : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-white/5'
                    }`}
                  >
                    <Icon
                      className={`h-3.5 w-3.5 shrink-0 ${
                        active
                          ? 'text-gray-700 dark:text-gray-200'
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
              <GeneralPanel
                s={s}
                name={name}
                description={description}
                setName={setName}
                setDescription={setDescription}
                units={units}
                showCompose={showCompose}
                setShowCompose={setShowCompose}
                showDetails={showDetails}
                setShowDetails={setShowDetails}
                saveBusy={save.isPending}
                onSave={(e) => {
                  e.preventDefault()
                  save.mutate()
                }}
                onRestartUnit={() => restart.mutate()}
                restartBusy={restart.isPending}
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
  showDetails,
  setShowDetails,
  saveBusy,
  onSave,
  onRestartUnit,
  restartBusy,
}: {
  s: Service
  name: string
  description: string
  setName: (v: string) => void
  setDescription: (v: string) => void
  units: ServiceUnit[]
  showCompose: boolean
  setShowCompose: (v: boolean) => void
  showDetails: boolean
  setShowDetails: (v: boolean) => void
  saveBusy: boolean
  onSave: (e: FormEvent) => void
  onRestartUnit: () => void
  restartBusy: boolean
}) {
  const composeText = s.docker_compose || s.docker_compose_raw || ''

  return (
    <form className="space-y-8" onSubmit={onSave}>
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="mr-2 text-lg font-semibold text-gray-900 dark:text-white">Service Stack</h2>
        <Btn primary type="submit">
          {saveBusy ? 'Saving…' : 'Save'}
        </Btn>
        <Btn type="button" onClick={() => setShowCompose(!showCompose)}>
          {showCompose ? 'Hide Compose' : 'Edit Compose File'}
        </Btn>
        <Btn type="button" onClick={() => setShowDetails(!showDetails)}>
          Details
        </Btn>
      </div>

      <section className="space-y-4">
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
      </section>

      <section className="space-y-3">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Network</h3>
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
        <section className="space-y-2">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Compose file</h3>
          <CodeEditor
            language="yaml"
            readOnly
            height="28rem"
            ariaLabel="Docker Compose YAML"
            value={composeText || ''}
          />
          <p className="text-xs text-gray-500 dark:text-gray-400">
            Compose editing from the UI is view-only for now. Redeploy uses the prepared compose
            stored with this service.
          </p>
        </section>
      )}

      {showDetails && (
        <section className="grid gap-3 sm:grid-cols-2">
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
}: {
  unit: ServiceUnit
  stackStatus: string
  onRestart: () => void
  restartBusy: boolean
}) {
  const status = unit.status || stackStatus
  const tone = statusTone(status)
  const bar =
    tone === 'ok' ? 'bg-emerald-500' : tone === 'warn' ? 'bg-amber-500' : tone === 'bad' ? 'bg-error-500' : 'bg-gray-400'
  const link = unit.links?.[0]

  return (
    <div className="flex overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-950">
      <div className={`w-1 shrink-0 ${bar}`} />
      <div className="flex min-w-0 flex-1 flex-wrap items-center justify-between gap-3 px-4 py-3">
        <div className="min-w-0">
          <div className="font-medium text-gray-900 dark:text-white">
            {titleCase(unit.name)}
            {unit.image ? (
              <span className="font-normal text-gray-500 dark:text-gray-400"> ({unit.image})</span>
            ) : null}
          </div>
          {link ? (
            <a
              href={link.url}
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
          <div className="mt-1">
            <StatusText status={status} />
          </div>
        </div>
        <div className="flex shrink-0 gap-2">
          <Btn type="button" onClick={() => undefined}>
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
