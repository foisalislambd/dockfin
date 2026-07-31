import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { DangerConfirmModal, DangerZoneCard } from '../components/DangerConfirmModal'
import { EnvVarsPanel } from '../components/EnvVarsPanel'
import { ServerTerminal } from '../components/Terminal'
import { PageSkeleton } from '../components/ui/Skeleton'
import { Meta, ResourceTabs, TabPanel } from '../components/ui/tabs'
import { api } from '../lib/api'
import { Btn, Input } from './Servers'

const DB_TABS = [
  { id: 'configuration', label: 'Configuration' },
  { id: 'environment', label: 'Environment Variables' },
  { id: 'backups', label: 'Backups' },
  { id: 'danger', label: 'Danger Zone' },
]

const SERVER_TABS = [
  { id: 'overview', label: 'Overview' },
  { id: 'metrics', label: 'Metrics' },
  { id: 'proxy', label: 'Proxy' },
  { id: 'destinations', label: 'Destinations' },
  { id: 'settings', label: 'Settings' },
  { id: 'terminal', label: 'Terminal' },
  { id: 'danger', label: 'Danger' },
]

function DatabaseBackupsPanel({ dbId }: { dbId: string }) {
  const qc = useQueryClient()
  const backups = useQuery({ queryKey: ['scheduled-backups'], queryFn: api.scheduledBackups })
  const executions = useQuery({
    queryKey: ['db-backups', dbId],
    queryFn: () => api.databaseBackups(dbId),
    refetchInterval: (q) => {
      const list = q.state.data?.backup_executions || []
      return list.some((b) => b.status === 'running') ? 2000 : false
    },
  })
  const storages = useQuery({ queryKey: ['s3-storages'], queryFn: api.s3Storages })
  const [s3Id, setS3Id] = useState('')
  const [frequency, setFrequency] = useState('0 0 * * *')
  const [retention, setRetention] = useState('7')
  const mine = (backups.data?.scheduled_backups || []).filter(
    (b) => b.resource_type === 'database' && b.resource_id === dbId,
  )
  const create = useMutation({
    mutationFn: () =>
      api.createScheduledBackup({
        resource_type: 'database',
        resource_id: dbId,
        s3_storage_id: s3Id || undefined,
        frequency,
        retention: Number(retention) || 7,
      }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['scheduled-backups'] }),
  })
  const runNow = useMutation({
    mutationFn: () => api.runDatabaseBackup(dbId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['db-backups', dbId] }),
  })
  const restore = useMutation({
    mutationFn: (executionId: string) =>
      api.restoreDatabaseBackup(dbId, { execution_id: executionId }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['db-backups', dbId] }),
  })
  useEffect(() => {
    restore.reset()
    runNow.reset()
  }, [dbId]) // eslint-disable-line react-hooks/exhaustive-deps

  const formatBytes = (n: number) => {
    if (!n) return '—'
    if (n < 1024) return `${n} B`
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
    return `${(n / (1024 * 1024)).toFixed(1)} MB`
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Manual dumps write to `/data/goolify/backups` on the server (PostgreSQL).
        </p>
        <Btn primary onClick={() => runNow.mutate()}>
          {runNow.isPending ? 'Dumping…' : 'Run backup now'}
        </Btn>
      </div>
      {runNow.error && <p className="text-sm text-error-500">{runNow.error.message}</p>}
      {restore.error && <p className="text-sm text-error-500">{restore.error.message}</p>}
      {restore.isSuccess && (
        <p className="text-sm text-emerald-600 dark:text-emerald-400">Restore completed.</p>
      )}

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
                <td className="px-3 py-2">
                  {b.status === 'finished' && (
                    <button
                      type="button"
                      className="text-brand-600 dark:text-brand-400"
                      disabled={restore.isPending}
                      onClick={() => {
                        if (window.confirm('Restore this dump into the running database?')) {
                          restore.mutate(b.id)
                        }
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
              <th className="px-3 py-2">S3</th>
              <th className="px-3 py-2">Enabled</th>
            </tr>
          </thead>
          <tbody>
            {mine.map((b) => (
              <tr key={b.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-mono text-xs">{b.frequency}</td>
                <td className="px-3 py-2">{b.retention}</td>
                <td className="px-3 py-2 font-mono text-xs">{b.s3_storage_id?.slice(0, 8) || '—'}</td>
                <td className="px-3 py-2">{b.enabled ? 'yes' : 'no'}</td>
              </tr>
            ))}
            {!mine.length && (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  No scheduled backups for this database.
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


export function DatabaseDetailPage() {
  const { projectId, envId, dbId } = useParams({ strict: false }) as {
    projectId?: string
    envId?: string
    dbId: string
  }
  const nav = useNavigate()
  const qc = useQueryClient()
  const [tab, setTab] = useState('configuration')
  const [deleteOpen, setDeleteOpen] = useState(false)
  useEffect(() => {
    setTab('configuration')
  }, [dbId])
  const db = useQuery({ queryKey: ['database', dbId], queryFn: () => api.getDatabase(dbId) })

  const start = useMutation({
    mutationFn: () => api.startDatabase(dbId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['database', dbId] }),
  })
  const stop = useMutation({
    mutationFn: () => api.stopDatabase(dbId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['database', dbId] }),
  })
  const remove = useMutation({
    mutationFn: (body: Parameters<typeof api.deleteDatabase>[1]) => api.deleteDatabase(dbId, body),
    onSuccess: () => {
      setDeleteOpen(false)
      void qc.invalidateQueries({ queryKey: ['databases'] })
      if (projectId && envId) {
        void nav({ to: '/projects/$projectId/environments/$envId', params: { projectId, envId } })
      } else {
        void nav({ to: '/projects' })
      }
    },
  })

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

  if (db.isLoading) return <PageSkeleton cards={2} />
  if (db.error || !db.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{db.error?.message || 'Database not found'}</p>
        {back}
      </div>
    )
  }

  const d = db.data

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          {back}
          <h1 className="mt-2 text-xl font-semibold tracking-tight text-gray-900 dark:text-white">{d.name}</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {d.engine} · {d.status}
          </p>
        </div>
        <div className="flex gap-2">
          <Btn onClick={() => start.mutate()}>Start</Btn>
          <Btn onClick={() => stop.mutate()}>Stop</Btn>
        </div>
      </div>

      <ResourceTabs tabs={DB_TABS} active={tab} onChange={setTab} />

      {tab === 'configuration' && (
        <TabPanel>
          <div className="grid gap-4 sm:grid-cols-3">
            <Meta label="Status" value={d.status} />
            <Meta label="Engine" value={d.engine} />
            <Meta label="Image" value={d.image || '—'} />
          </div>
          {(start.error || stop.error) && (
            <p className="mt-4 text-sm text-error-500">{(start.error || stop.error)?.message}</p>
          )}
        </TabPanel>
      )}

      {tab === 'environment' && (
        <TabPanel>
          <EnvVarsPanel resourceType="database" resourceId={dbId} title="" />
        </TabPanel>
      )}

      {tab === 'backups' && (
        <TabPanel>
          <DatabaseBackupsPanel dbId={dbId} />
        </TabPanel>
      )}

      {tab === 'danger' && (
        <TabPanel>
          <div className="space-y-4">
            <div className="panel-card space-y-3 border-error-200 p-5 dark:border-error-500/30">
              <h2 className="text-sm font-semibold text-error-500">Stop database</h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Stops the container on the remote server without removing Goolify configuration.
              </p>
              <p className="text-sm text-gray-600 dark:text-gray-300">
                Current status: <span className="font-medium capitalize">{d.status || 'unknown'}</span>
              </p>
              <Btn onClick={() => stop.mutate()} disabled={stop.isPending}>
                {stop.isPending ? 'Stopping…' : 'Stop now'}
              </Btn>
            </div>
            <DangerZoneCard>
              <div>
                <h3 className="text-sm font-semibold text-error-500">Delete Resource</h3>
                <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                  This will stop your containers, delete related data, and remove the database from
                  Goolify. Beware — there is no coming back.
                </p>
                <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
                  Container status:{' '}
                  <span className="font-medium capitalize">{d.status || 'unknown'}</span>
                  {d.status === 'running'
                    ? ' — will be stopped and removed.'
                    : ' — will be removed if present on the server.'}
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
              expectedName={d.name}
              statusLine={
                d.status === 'running'
                  ? 'Database container is RUNNING. Deleting will stop it and remove data options you select.'
                  : `Current status: ${d.status || 'unknown'}.`
              }
              actions={[
                'Permanently delete all containers of this resource.',
                'Remove schedules, backup history, and the database record from Goolify.',
              ]}
              requirePassword
              showResourceCheckboxes
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

function Sparkline({
  values,
  maxHint,
  color = '#0d9488',
}: {
  values: number[]
  maxHint?: number
  color?: string
}) {
  if (!values.length) {
    return <div className="flex h-16 items-center text-sm text-gray-500 dark:text-gray-400">No data</div>
  }
  const w = 280
  const h = 64
  const max = Math.max(maxHint || 0, ...values, 1)
  const pts = values
    .map((v, i) => {
      const x = values.length === 1 ? 0 : (i / (values.length - 1)) * w
      const y = h - (v / max) * (h - 4) - 2
      return `${x},${y}`
    })
    .join(' ')
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="h-16 w-full" preserveAspectRatio="none">
      <polyline fill="none" stroke={color} strokeWidth="2" points={pts} />
    </svg>
  )
}

function ServerMetricsView({
  metrics,
  loading,
}: {
  metrics: import('../lib/api').ServerMetric[]
  loading: boolean
}) {
  if (loading) return null
  const latest = metrics[metrics.length - 1]
  const cpu = metrics.map((m) => m.cpu_percent)
  const memPct = metrics.map((m) =>
    m.memory_total_bytes > 0 ? (m.memory_used_bytes / m.memory_total_bytes) * 100 : 0,
  )
  const diskPct = metrics.map((m) =>
    m.disk_total_bytes > 0 ? (m.disk_used_bytes / m.disk_total_bytes) * 100 : 0,
  )
  const fmtGiB = (n: number) => `${(n / (1024 ** 3)).toFixed(1)} GiB`

  return (
    <div className="space-y-4">
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Recent samples from Sentinel ingest. Deploy the agent on the server to populate charts.
      </p>
      <div className="grid gap-4 sm:grid-cols-3">
        <div className="panel-card p-4">
          <div className="text-xs text-gray-500 dark:text-gray-400">CPU</div>
          <div className="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">
            {latest ? `${latest.cpu_percent.toFixed(1)}%` : '—'}
          </div>
          <Sparkline values={cpu} maxHint={100} color="#0d9488" />
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
          <Sparkline values={memPct} maxHint={100} color="#2563eb" />
        </div>
        <div className="panel-card p-4">
          <div className="text-xs text-gray-500 dark:text-gray-400">Disk</div>
          <div className="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">
            {latest && latest.disk_total_bytes
              ? `${((latest.disk_used_bytes / latest.disk_total_bytes) * 100).toFixed(0)}%`
              : '—'}
          </div>
          <div className="text-xs text-gray-500 dark:text-gray-400">
            {latest ? `${fmtGiB(latest.disk_used_bytes)} / ${fmtGiB(latest.disk_total_bytes)}` : ''}
          </div>
          <Sparkline values={diskPct} maxHint={100} color="#d97706" />
        </div>
      </div>
      {!metrics.length && (
        <div className="panel-card p-6 text-center text-sm text-gray-500 dark:text-gray-400">
          No metrics yet. Ensure Sentinel is configured and posting to the ingest endpoint.
        </div>
      )}
    </div>
  )
}

export function ServerDetailPage() {
  const { serverId } = useParams({ strict: false }) as { serverId: string }
  const nav = useNavigate()
  const qc = useQueryClient()
  const [tab, setTab] = useState('overview')
  const [destName, setDestName] = useState('')
  const [destKind, setDestKind] = useState('standalone')
  const [destNetwork, setDestNetwork] = useState('goolify')
  const [wildcardDomain, setWildcardDomain] = useState('')
  const [magicDomain, setMagicDomain] = useState('sslip.io')
  const [publicIP, setPublicIP] = useState('')
  useEffect(() => {
    setTab('overview')
    setDestName('')
    setDestKind('standalone')
    setDestNetwork('goolify')
  }, [serverId])

  const server = useQuery({ queryKey: ['server', serverId], queryFn: () => api.getServer(serverId) })
  useEffect(() => {
    if (!server.data) return
    setWildcardDomain(server.data.wildcard_domain || '')
    setMagicDomain(server.data.magic_domain || 'sslip.io')
    setPublicIP(server.data.public_ip || '')
  }, [server.data])
  const destinations = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const metrics = useQuery({
    queryKey: ['server-metrics', serverId],
    queryFn: () => api.serverMetrics(serverId, 60),
    refetchInterval: tab === 'metrics' ? 15000 : false,
  })

  const validate = useMutation({
    mutationFn: () => api.validateServer(serverId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['server', serverId] }),
  })
  const startProxy = useMutation({
    mutationFn: () => api.startProxy(serverId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['server', serverId] }),
  })
  const stopProxy = useMutation({
    mutationFn: () => api.stopProxy(serverId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['server', serverId] }),
  })
  const remove = useMutation({
    mutationFn: () => api.deleteServer(serverId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['servers'] })
      void nav({ to: '/servers' })
    },
  })
  const patchSettings = useMutation({
    mutationFn: (body: {
      is_build_server?: boolean
      is_swarm_manager?: boolean
      wildcard_domain?: string
      magic_domain?: string
      public_ip?: string
    }) => api.patchServerSettings(serverId, body),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['server', serverId] })
      void qc.invalidateQueries({ queryKey: ['servers'] })
    },
  })
  const createDest = useMutation({
    mutationFn: () =>
      api.createDestination(serverId, {
        name: destName,
        kind: destKind,
        network: destNetwork || undefined,
      }),
    onSuccess: () => {
      setDestName('')
      void qc.invalidateQueries({ queryKey: ['destinations'] })
    },
  })

  if (server.isLoading) return <PageSkeleton cards={3} />
  if (server.error || !server.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{server.error?.message || 'Server not found'}</p>
        <Link to="/servers" className="text-brand-600 dark:text-brand-400">
          ← Servers
        </Link>
      </div>
    )
  }

  const s = server.data
  const serverDests = (destinations.data?.destinations || []).filter((d) => d.server_id === serverId)

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <Link
            to="/servers"
            className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
          >
            ← Servers
          </Link>
          <h1 className="mt-2 text-xl font-semibold tracking-tight text-gray-900 dark:text-white">{s.name}</h1>
          <p className="mt-1 font-mono text-sm text-gray-500 dark:text-gray-400">
            {s.user_name}@{s.ip}:{s.port}
          </p>
        </div>
        <Btn onClick={() => validate.mutate()}>{validate.isPending ? 'Validating…' : 'Validate'}</Btn>
      </div>

      <ResourceTabs tabs={SERVER_TABS} active={tab} onChange={setTab} />

      {tab === 'overview' && (
        <TabPanel>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <Meta label="Reachable" value={s.is_reachable ? 'Yes' : 'No'} />
            <Meta label="Docker" value={s.is_usable ? s.docker_version || 'ok' : 'Unavailable'} />
            <Meta label="Proxy type" value={s.proxy_type || '—'} />
            <Meta label="Proxy status" value={s.proxy_status || '—'} />
            <Meta label="Build server" value={s.is_build_server ? 'Yes' : 'No'} />
            <Meta label="Swarm manager" value={s.is_swarm_manager ? 'Yes' : 'No'} />
          </div>
          {validate.error && <p className="mt-4 text-sm text-error-500">{validate.error.message}</p>}
        </TabPanel>
      )}

      {tab === 'metrics' && (
        <TabPanel>
          <ServerMetricsView metrics={metrics.data?.metrics || []} loading={metrics.isLoading} />
          {metrics.error && <p className="mt-3 text-sm text-error-500">{metrics.error.message}</p>}
        </TabPanel>
      )}

      {tab === 'proxy' && (
        <TabPanel>
          <div className="panel-card space-y-4 p-5">
            <div className="grid gap-3 sm:grid-cols-2">
              <Meta label="Type" value={s.proxy_type || 'traefik'} />
              <Meta label="Status" value={s.proxy_status || '—'} />
            </div>
            <div className="flex flex-wrap gap-2">
              <Btn
                primary
                onClick={() => {
                  if (s.proxy_type === 'none' || startProxy.isPending) return
                  startProxy.mutate()
                }}
              >
                Start proxy
              </Btn>
              <Btn onClick={() => stopProxy.mutate()}>Stop proxy</Btn>
            </div>
            {s.proxy_type === 'none' && (
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Proxy is disabled for this server. Recreate with Traefik or Caddy to enable routing.
              </p>
            )}
            {(startProxy.error || stopProxy.error) && (
              <p className="text-sm text-error-500">
                {(startProxy.error || stopProxy.error)?.message}
              </p>
            )}
          </div>
        </TabPanel>
      )}

      {tab === 'destinations' && (
        <TabPanel>
          <div className="grid gap-3 sm:grid-cols-2">
            {serverDests.map((d) => (
              <div key={d.id} className="panel-card p-4">
                <div className="font-medium text-gray-900 dark:text-white">{d.name}</div>
                <div className="mt-1 font-mono text-xs text-gray-500 dark:text-gray-400">
                  {d.kind || 'standalone'} · {d.network}
                </div>
              </div>
            ))}
            {!serverDests.length && (
              <div className="panel-card col-span-full p-8 text-center text-sm text-gray-500 dark:text-gray-400">
                No destinations on this server yet.
              </div>
            )}
          </div>
          <form
            className="panel-card mt-4 space-y-3 p-5"
            onSubmit={(e) => {
              e.preventDefault()
              createDest.mutate()
            }}
          >
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Add destination</h2>
            <div className="grid gap-3 sm:grid-cols-3">
              <Input label="Name" value={destName} onChange={setDestName} />
              <label className="block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">Kind</span>
                <select
                  value={destKind}
                  onChange={(e) => setDestKind(e.target.value)}
                  className="w-full panel-field rounded-lg px-3 py-2"
                >
                  <option value="standalone">Standalone</option>
                  <option value="swarm">Swarm</option>
                </select>
              </label>
              <Input label="Network" value={destNetwork} onChange={setDestNetwork} required={false} />
            </div>
            {createDest.error && <p className="text-sm text-error-500">{createDest.error.message}</p>}
            <Btn primary type="submit">
              {createDest.isPending ? 'Creating…' : 'Create destination'}
            </Btn>
          </form>
        </TabPanel>
      )}

      {tab === 'settings' && (
        <TabPanel>
          <div className="panel-card space-y-4 p-5">
            <label className="flex items-center gap-3 text-sm">
              <input
                type="checkbox"
                checked={Boolean(s.is_build_server)}
                onChange={(e) => patchSettings.mutate({ is_build_server: e.target.checked })}
              />
              <span>
                <span className="font-medium text-gray-900 dark:text-white">Build server</span>
                <span className="mt-0.5 block text-gray-500 dark:text-gray-400">
                  Use this host to build images, then transfer to deploy servers.
                </span>
              </span>
            </label>
            <label className="flex items-center gap-3 text-sm">
              <input
                type="checkbox"
                checked={Boolean(s.is_swarm_manager)}
                onChange={(e) => patchSettings.mutate({ is_swarm_manager: e.target.checked })}
              />
              <span>
                <span className="font-medium text-gray-900 dark:text-white">Swarm manager</span>
                <span className="mt-0.5 block text-gray-500 dark:text-gray-400">
                  Mark this node as a Docker Swarm manager for swarm destinations.
                </span>
              </span>
            </label>
            <div className="border-t border-gray-200 pt-4 dark:border-gray-800">
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Free domains</h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
                Auto-generated hostnames use your wildcard domain, or free{' '}
                <code className="text-xs">
                  {'*.' +
                    (publicIP || (s.ip !== '127.0.0.1' && s.ip !== 'localhost' ? s.ip : 'PUBLIC_IP')) +
                    '.' +
                    (magicDomain || 'sslip.io')}
                </code>
                . Never use 127.0.0.1 — browsers would open localhost on the visitor&apos;s PC.
              </p>
              <div className="mt-3 grid gap-3 sm:grid-cols-2">
                <label className="block text-sm sm:col-span-2">
                  <span className="mb-1 block text-gray-500 dark:text-gray-400">
                    Public IP (for sslip.io / nip.io)
                  </span>
                  <input
                    value={publicIP}
                    onChange={(e) => setPublicIP(e.target.value)}
                    placeholder={s.ip === '127.0.0.1' ? 'e.g. 178.18.243.148' : s.ip}
                    className="w-full panel-field rounded-lg px-3 py-2"
                  />
                  <span className="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                    SSH can stay on 127.0.0.1 for local servers. Run Validate to auto-detect, or set
                    the VPS public IP here.
                  </span>
                </label>
                <label className="block text-sm">
                  <span className="mb-1 block text-gray-500 dark:text-gray-400">Wildcard domain</span>
                  <input
                    value={wildcardDomain}
                    onChange={(e) => setWildcardDomain(e.target.value)}
                    placeholder="apps.example.com"
                    className="w-full panel-field rounded-lg px-3 py-2"
                  />
                </label>
                <label className="block text-sm">
                  <span className="mb-1 block text-gray-500 dark:text-gray-400">Magic DNS provider</span>
                  <select
                    value={magicDomain}
                    onChange={(e) => setMagicDomain(e.target.value)}
                    className="w-full panel-field rounded-lg px-3 py-2"
                  >
                    <option value="sslip.io">sslip.io</option>
                    <option value="nip.io">nip.io</option>
                  </select>
                </label>
              </div>
              <button
                type="button"
                className="mt-3 inline-flex h-8 items-center rounded-md bg-brand-600 px-2.5 text-xs font-medium text-white hover:bg-brand-500"
                onClick={() =>
                  patchSettings.mutate({
                    wildcard_domain: wildcardDomain,
                    magic_domain: magicDomain,
                    public_ip: publicIP,
                  })
                }
              >
                {patchSettings.isPending ? 'Saving…' : 'Save domain settings'}
              </button>
            </div>
            {patchSettings.error && (
              <p className="text-sm text-error-500">{patchSettings.error.message}</p>
            )}
          </div>
        </TabPanel>
      )}

      {tab === 'terminal' && (
        <TabPanel>
          <ServerTerminal serverId={serverId} />
        </TabPanel>
      )}

      {tab === 'danger' && (
        <TabPanel>
          <div className="panel-card space-y-4 border-error-200 p-5 dark:border-error-500/30">
            <h2 className="text-sm font-semibold text-error-500">Danger zone</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Remove this server from Goolify. Containers on the host are not deleted.
            </p>
            <button
              type="button"
              className="inline-flex h-8 items-center rounded-md border border-error-500 px-2.5 text-xs font-medium text-error-500 hover:bg-error-500/10"
              onClick={() => {
                if (confirm('Delete this server from Goolify?')) remove.mutate()
              }}
            >
              Delete server
            </button>
            {remove.error && <p className="text-sm text-error-500">{remove.error.message}</p>}
          </div>
        </TabPanel>
      )}
    </div>
  )
}
