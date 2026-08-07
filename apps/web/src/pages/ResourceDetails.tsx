import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams, useSearch } from '@tanstack/react-router'
import {
  Activity,
  AlertTriangle,
  Archive,
  Cloud,
  LayoutDashboard,
  Network,
  Route,
  ScrollText,
  Settings2,
  Shield,
  Tags,
  Terminal,
  Trash2,
  Variable,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { DangerConfirmModal, DangerZoneCard } from '../components/DangerConfirmModal'
import { useConfirm } from '../components/ConfirmDialog'
import { EnvVarsPanel } from '../components/EnvVarsPanel'
import { ResourceTagsPanel } from '../components/ResourceTagsPanel'
import { ServerTerminal } from '../components/Terminal'
import { PageSkeleton } from '../components/ui/Skeleton'
import { Meta, ResourceTabs, TabPanel } from '../components/ui/tabs'
import { api } from '../lib/api'
import { Btn, Input } from './Servers'

const DB_TABS = [
  { id: 'configuration', label: 'Configuration', icon: Settings2 },
  { id: 'environment', label: 'Environment Variables', icon: Variable },
  { id: 'backups', label: 'Backups', icon: Archive },
  { id: 'logs', label: 'Logs', icon: ScrollText },
  { id: 'terminal', label: 'Terminal', icon: Terminal },
  { id: 'metrics', label: 'Metrics', icon: Activity },
  { id: 'tags', label: 'Tags', icon: Tags },
  { id: 'danger', label: 'Danger Zone', icon: AlertTriangle },
]

const SERVER_TABS = [
  { id: 'overview', label: 'Overview', icon: LayoutDashboard },
  { id: 'metrics', label: 'Metrics', icon: Activity },
  { id: 'sentinel', label: 'Sentinel', icon: Shield },
  { id: 'cleanup', label: 'Docker Cleanup', icon: Trash2 },
  { id: 'proxy', label: 'Proxy', icon: Route },
  { id: 'destinations', label: 'Destinations', icon: Network },
  { id: 'edge', label: 'Edge', icon: Cloud },
  { id: 'settings', label: 'Settings', icon: Settings2 },
  { id: 'terminal', label: 'Terminal', icon: Terminal },
  { id: 'danger', label: 'Danger', icon: AlertTriangle },
]

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = String(reader.result || '')
      const comma = result.indexOf(',')
      resolve(comma >= 0 ? result.slice(comma + 1) : result)
    }
    reader.onerror = () => reject(reader.error || new Error('Failed to read file'))
    reader.readAsDataURL(file)
  })
}

function DatabaseBackupsPanel({ dbId }: { dbId: string }) {
  const qc = useQueryClient()
  const confirm = useConfirm()
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
    mutationFn: () => api.runDatabaseBackup(dbId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['db-backups', dbId] }),
  })
  const restore = useMutation({
    mutationFn: (executionId: string) =>
      api.restoreDatabaseBackup(dbId, { execution_id: executionId }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['db-backups', dbId] }),
  })
  const [importFile, setImportFile] = useState<File | null>(null)
  const [importRestore, setImportRestore] = useState(true)
  const importBackup = useMutation({
    mutationFn: async () => {
      if (!importFile) throw new Error('Choose a file to import')
      const contentBase64 = await fileToBase64(importFile)
      return api.importDatabaseBackup(dbId, {
        filename: importFile.name,
        content_base64: contentBase64,
        restore: importRestore,
      })
    },
    onSuccess: () => {
      setImportFile(null)
      void qc.invalidateQueries({ queryKey: ['db-backups', dbId] })
    },
  })
  useEffect(() => {
    restore.reset()
    runNow.reset()
    importBackup.reset()
    setImportFile(null)
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
          Manual dumps write to `/data/dockfin/backups` on the server (PostgreSQL, MySQL/MariaDB, Redis).
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
                        void (async () => {
                          if (
                            await confirm({
                              title: 'Restore backup',
                              message: 'Restore this dump into the running database?',
                              confirmLabel: 'Restore',
                              danger: true,
                            })
                          ) {
                            restore.mutate(b.id)
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
                <td colSpan={6} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  No backup runs yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="panel-card space-y-3 p-4">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Import backup</h3>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Upload a `.sql` dump (PostgreSQL / MySQL / MariaDB) to store it under `/data/dockfin/backups`
          and optionally restore it immediately.
        </p>
        <div className="flex flex-wrap items-end gap-3">
          <label className="block min-w-[220px] flex-1 text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">File</span>
            <input
              type="file"
              onChange={(e) => setImportFile(e.target.files?.[0] || null)}
              className="panel-field w-full rounded-lg px-3 py-2 text-sm"
            />
          </label>
          <label className="flex items-center gap-2 pb-2 text-sm">
            <input
              type="checkbox"
              checked={importRestore}
              onChange={(e) => setImportRestore(e.target.checked)}
            />
            <span className="text-gray-700 dark:text-gray-300">Restore immediately</span>
          </label>
          <Btn primary onClick={() => importBackup.mutate()} disabled={!importFile || importBackup.isPending}>
            {importBackup.isPending ? 'Importing…' : 'Import'}
          </Btn>
        </div>
        {importBackup.error && <p className="text-sm text-error-500">{importBackup.error.message}</p>}
        {importBackup.isSuccess && (
          <p className="text-sm text-emerald-600 dark:text-emerald-400">
            Imported {importBackup.data?.filename}
            {importRestore ? ' and restored.' : '.'}
          </p>
        )}
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
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {mine.map((b) => (
              <tr key={b.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-mono text-xs">{b.frequency}</td>
                <td className="px-3 py-2">{b.retention}</td>
                <td className="px-3 py-2 font-mono text-xs">{b.s3_storage_id?.slice(0, 8) || '—'}</td>
                <td className="px-3 py-2">{b.enabled ? 'yes' : 'no'}</td>
                <td className="px-3 py-2 space-x-3">
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
                <td colSpan={5} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
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


function DatabaseLiveLogs({ dbId }: { dbId: string }) {
  const [lines, setLines] = useState<string[]>([])
  const [status, setStatus] = useState<'connecting' | 'live' | 'ended' | 'error'>('connecting')
  const [error, setError] = useState('')
  const [nonce, setNonce] = useState(0)

  useEffect(() => {
    setLines([])
    setStatus('connecting')
    setError('')
    const qs = new URLSearchParams({ tail: '200' })
    const es = new EventSource(`/api/v1/databases/${dbId}/logs/stream?${qs}`, {
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
  }, [dbId, nonce])

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs text-gray-500 dark:text-gray-400">
          {status === 'live'
            ? 'Streaming live'
            : status === 'connecting'
              ? 'Connecting…'
              : status === 'ended'
                ? 'Stream ended'
                : 'Disconnected'}
        </span>
        <Btn onClick={() => setNonce((n) => n + 1)}>Reconnect</Btn>
      </div>
      {error && <p className="text-sm text-error-500">{error}</p>}
      <pre className="max-h-[32rem] overflow-auto rounded-lg bg-gray-950 p-3 font-mono text-xs text-gray-200">
        {lines.length ? lines.join('\n') : 'Waiting for log output…'}
      </pre>
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
  const destinations = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const serverId =
    (destinations.data?.destinations || []).find((dd) => dd.id === db.data?.destination_id)?.server_id || ''
  const metrics = useQuery({
    queryKey: ['server-metrics', serverId],
    queryFn: () => api.serverMetrics(serverId, 60),
    enabled: Boolean(serverId) && tab === 'metrics',
    refetchInterval: tab === 'metrics' ? 15000 : false,
  })

  const [isPublic, setIsPublic] = useState(false)
  const [publicPort, setPublicPort] = useState('')
  useEffect(() => {
    if (!db.data) return
    setIsPublic(Boolean(db.data.is_public))
    setPublicPort(db.data.public_port != null ? String(db.data.public_port) : '')
  }, [db.data])

  const start = useMutation({
    mutationFn: () => api.startDatabase(dbId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['database', dbId] }),
  })
  const stop = useMutation({
    mutationFn: () => api.stopDatabase(dbId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['database', dbId] }),
  })
  const patch = useMutation({
    mutationFn: (body: Parameters<typeof api.updateDatabase>[1]) => api.updateDatabase(dbId, body),
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

          <div className="mt-6 panel-card space-y-4 p-5">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Public networking</h3>
            <label className="flex items-center gap-3 text-sm">
              <input
                type="checkbox"
                checked={isPublic}
                onChange={(e) => setIsPublic(e.target.checked)}
              />
              <span>
                <span className="font-medium text-gray-900 dark:text-white">Publish port</span>
                <span className="mt-0.5 block text-gray-500 dark:text-gray-400">
                  Expose the database on a host port so it is reachable from outside Docker.
                </span>
              </span>
            </label>
            {isPublic && (
              <Input label="Public port" value={publicPort} onChange={setPublicPort} />
            )}
            <div className="flex flex-wrap items-center gap-3">
              <Btn
                primary
                onClick={() =>
                  patch.mutate({
                    is_public: isPublic,
                    public_port: isPublic ? Number(publicPort) || null : null,
                  })
                }
                disabled={patch.isPending}
              >
                {patch.isPending ? 'Saving…' : 'Save'}
              </Btn>
              <p className="text-xs text-gray-500 dark:text-gray-400">
                Restart or redeploy the database for port publishing changes to take effect on the
                running container.
              </p>
            </div>
            {patch.error && <p className="text-sm text-error-500">{patch.error.message}</p>}
          </div>
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

      {tab === 'logs' && (
        <TabPanel>
          <DatabaseLiveLogs dbId={dbId} />
        </TabPanel>
      )}

      {tab === 'terminal' && (
        <TabPanel>
          {serverId ? (
            <ServerTerminal
              serverId={serverId}
              defaultContainer={`dockfin-db-${dbId}`}
              containerOptions={[`dockfin-db-${dbId}`]}
              hideHostShell
            />
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Assign a destination so the database container terminal can connect.
            </p>
          )}
        </TabPanel>
      )}

      {tab === 'metrics' && (
        <TabPanel>
          {serverId ? (
            <div className="space-y-3">
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Host-level metrics for the server running this database.{' '}
                <Link
                  to="/servers/$serverId"
                  params={{ serverId }}
                  className="text-brand-600 dark:text-brand-400"
                >
                  Open server →
                </Link>
              </p>
              <ServerMetricsView metrics={metrics.data?.metrics || []} loading={metrics.isLoading} />
              {metrics.error && <p className="text-sm text-error-500">{metrics.error.message}</p>}
            </div>
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Assign a destination to view server metrics for this database.
            </p>
          )}
        </TabPanel>
      )}

      {tab === 'tags' && (
        <TabPanel>
          <ResourceTagsPanel resourceType="database" resourceId={dbId} />
        </TabPanel>
      )}

      {tab === 'danger' && (
        <TabPanel>
          <div className="space-y-4">
            <div className="panel-card space-y-3 border-error-200 p-5 dark:border-error-500/30">
              <h2 className="text-sm font-semibold text-error-500">Stop database</h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Stops the container on the remote server without removing Dockfin configuration.
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
                  Dockfin. Beware — there is no coming back.
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
                'Remove schedules, backup history, and the database record from Dockfin.',
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

function ServerEdgePanel({ serverId }: { serverId: string }) {
  const qc = useQueryClient()
  const ops = useQuery({
    queryKey: ['server-ops', serverId],
    queryFn: () => api.getServerOps(serverId),
  })
  const members = useQuery({ queryKey: ['team-members'], queryFn: api.teamMembers })
  const [tunnelToken, setTunnelToken] = useState('')
  const [drainEnabled, setDrainEnabled] = useState(false)
  const [drainType, setDrainType] = useState('newrelic')
  const [drainConfig, setDrainConfig] = useState('')
  const [caCert, setCaCert] = useState('')
  const [aclIds, setAclIds] = useState<string[]>([])
  const [aclText, setAclText] = useState('')

  useEffect(() => {
    if (!ops.data) return
    setDrainEnabled(Boolean(ops.data.log_drain_enabled))
    setDrainType(ops.data.log_drain_type || 'newrelic')
    setDrainConfig(ops.data.log_drain_config || '')
    setCaCert(ops.data.ca_certificate || '')
    setAclIds(ops.data.terminal_acl_user_ids || [])
    setAclText((ops.data.terminal_acl_user_ids || []).join(', '))
  }, [ops.data])

  const patchOps = useMutation({
    mutationFn: (body: Parameters<typeof api.patchServerOps>[1]) =>
      api.patchServerOps(serverId, body),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['server-ops', serverId] }),
  })
  const tunnel = useMutation({
    mutationFn: (action: 'install' | 'stop' | 'status') =>
      api.cloudflareTunnelAction(serverId, action, tunnelToken || undefined),
    onSuccess: () => {
      setTunnelToken('')
      void qc.invalidateQueries({ queryKey: ['server-ops', serverId] })
    },
  })
  const patches = useMutation({ mutationFn: () => api.checkServerPatches(serverId) })

  const memberList = members.data?.members || []
  const warnings = patchOps.data?.warnings || []

  return (
    <div className="space-y-4">
      <div className="panel-card space-y-4 p-5">
        <div>
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Cloudflare Tunnel</h3>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Runs <code className="text-xs">cloudflare/cloudflared</code> on the host so services can
            be reached without opening inbound ports.
          </p>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <Meta label="Enabled" value={ops.data?.cloudflare_tunnel_enabled ? 'Yes' : 'No'} />
          <Meta label="Saved token" value={ops.data?.cloudflare_tunnel_token || '—'} />
        </div>
        <Input
          label="Tunnel token"
          value={tunnelToken}
          onChange={setTunnelToken}
          required={false}
        />
        <div className="flex flex-wrap gap-2">
          <Btn primary onClick={() => tunnel.mutate('install')}>
            {tunnel.isPending ? 'Working…' : 'Install / Start'}
          </Btn>
          <Btn onClick={() => tunnel.mutate('stop')}>Stop</Btn>
          <Btn onClick={() => tunnel.mutate('status')}>Status</Btn>
        </div>
        {tunnel.data?.status && (
          <p className="text-sm text-gray-600 dark:text-gray-300">
            Container status: <span className="font-mono">{tunnel.data.status}</span>
          </p>
        )}
        {tunnel.error && <p className="text-sm text-error-500">{tunnel.error.message}</p>}
      </div>

      <div className="panel-card space-y-4 p-5">
        <div>
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Log drain</h3>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Saved settings are written to{' '}
            <code className="text-xs">/data/dockfin/log-drain.env</code> on the server for the log
            shipper to consume.
          </p>
        </div>
        <label className="flex items-center gap-3 text-sm">
          <input
            type="checkbox"
            checked={drainEnabled}
            onChange={(e) => setDrainEnabled(e.target.checked)}
          />
          <span className="font-medium text-gray-900 dark:text-white">Enable log drain</span>
        </label>
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Type</span>
          <select
            value={drainType}
            onChange={(e) => setDrainType(e.target.value)}
            className="w-full panel-field rounded-lg px-3 py-2"
          >
            <option value="newrelic">New Relic</option>
            <option value="axiom">Axiom</option>
            <option value="custom">Custom</option>
          </select>
        </label>
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">
            Config (JSON, e.g. endpoint + API key)
          </span>
          <textarea
            value={drainConfig}
            onChange={(e) => setDrainConfig(e.target.value)}
            rows={4}
            placeholder='{"endpoint":"https://log-api.newrelic.com/log/v1","api_key":"…"}'
            className="w-full panel-field rounded-lg px-3 py-2 font-mono text-xs"
          />
        </label>
        <Btn
          primary
          onClick={() =>
            patchOps.mutate({
              log_drain_enabled: drainEnabled,
              log_drain_type: drainType,
              log_drain_config: drainConfig,
            })
          }
        >
          {patchOps.isPending ? 'Saving…' : 'Save log drain'}
        </Btn>
      </div>

      <div className="panel-card space-y-4 p-5">
        <div>
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">CA certificate</h3>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Written to <code className="text-xs">/data/dockfin/ca/custom-ca.crt</code> on save.
          </p>
        </div>
        <textarea
          value={caCert}
          onChange={(e) => setCaCert(e.target.value)}
          rows={6}
          placeholder="-----BEGIN CERTIFICATE-----"
          className="w-full panel-field rounded-lg px-3 py-2 font-mono text-xs"
        />
        <Btn primary onClick={() => patchOps.mutate({ ca_certificate: caCert })}>
          {patchOps.isPending ? 'Saving…' : 'Save certificate'}
        </Btn>
      </div>

      <div className="panel-card space-y-4 p-5">
        <div>
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Terminal access</h3>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Restrict who may open terminals on this server. Owners and admins always have access;
            an empty list allows every team member.
          </p>
        </div>
        {memberList.length ? (
          <div className="space-y-2">
            {memberList.map((m) => (
              <label key={m.user_id} className="flex items-center gap-3 text-sm">
                <input
                  type="checkbox"
                  checked={aclIds.includes(m.user_id)}
                  onChange={(e) =>
                    setAclIds((prev) =>
                      e.target.checked
                        ? [...prev, m.user_id]
                        : prev.filter((id) => id !== m.user_id),
                    )
                  }
                />
                <span className="text-gray-900 dark:text-white">
                  {m.name || m.email}{' '}
                  <span className="text-xs text-gray-500 dark:text-gray-400">({m.role})</span>
                </span>
              </label>
            ))}
          </div>
        ) : (
          <label className="block text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">
              Allowed user IDs (comma separated)
            </span>
            <textarea
              value={aclText}
              onChange={(e) => {
                setAclText(e.target.value)
                setAclIds(
                  e.target.value
                    .split(',')
                    .map((v) => v.trim())
                    .filter(Boolean),
                )
              }}
              rows={3}
              className="w-full panel-field rounded-lg px-3 py-2 font-mono text-xs"
            />
          </label>
        )}
        <Btn primary onClick={() => patchOps.mutate({ terminal_acl_user_ids: aclIds })}>
          {patchOps.isPending ? 'Saving…' : 'Save terminal access'}
        </Btn>
      </div>

      <div className="panel-card space-y-4 p-5">
        <div>
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Security patches</h3>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Lists pending OS package updates via the host package manager.
          </p>
        </div>
        <Btn primary onClick={() => patches.mutate()}>
          {patches.isPending ? 'Checking…' : 'Check for updates'}
        </Btn>
        {patches.data && (
          <>
            <p className="text-sm text-gray-600 dark:text-gray-300">
              {patches.data.count} update(s) pending.
            </p>
            <pre className="max-h-64 overflow-auto rounded-lg bg-gray-950 p-3 font-mono text-xs text-gray-200">
              {patches.data.output}
            </pre>
          </>
        )}
        {patches.error && <p className="text-sm text-error-500">{patches.error.message}</p>}
      </div>

      {warnings.length > 0 && (
        <p className="text-sm text-warning-500">{warnings.join(' · ')}</p>
      )}
      {patchOps.error && <p className="text-sm text-error-500">{patchOps.error.message}</p>}
    </div>
  )
}

export function ServerDetailPage() {
  const { serverId } = useParams({ strict: false }) as { serverId: string }
  const search = useSearch({ strict: false }) as { tab?: string }
  const nav = useNavigate()
  const qc = useQueryClient()
  const [tab, setTab] = useState(search.tab || 'overview')
  const [destName, setDestName] = useState('')
  const [destKind, setDestKind] = useState('standalone')
  const [destNetwork, setDestNetwork] = useState('dockfin')
  const [wildcardDomain, setWildcardDomain] = useState('')
  const [magicDomain, setMagicDomain] = useState('sslip.io')
  const [publicIP, setPublicIP] = useState('')
  const [cleanupFreq, setCleanupFreq] = useState('0 0 * * *')
  const [cleanupThreshold, setCleanupThreshold] = useState('80')
  const [forceCleanup, setForceCleanup] = useState(false)
  const [proxyCfgName, setProxyCfgName] = useState('custom.yml')
  const [proxyCfgValue, setProxyCfgValue] = useState('')
  const [freshToken, setFreshToken] = useState('')
  useEffect(() => {
    setTab(search.tab || 'overview')
    setDestName('')
    setDestKind('standalone')
    setDestNetwork('dockfin')
    setFreshToken('')
  }, [serverId, search.tab])

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
  const ops = useQuery({
    queryKey: ['server-ops', serverId],
    queryFn: () => api.getServerOps(serverId),
    enabled: tab === 'sentinel' || tab === 'cleanup' || tab === 'settings',
  })
  useEffect(() => {
    if (!ops.data) return
    setCleanupFreq(ops.data.docker_cleanup_frequency || '0 0 * * *')
    setCleanupThreshold(String(ops.data.docker_cleanup_threshold || 80))
    setForceCleanup(Boolean(ops.data.force_docker_cleanup))
  }, [ops.data])
  const cleanupExecs = useQuery({
    queryKey: ['docker-cleanup', serverId],
    queryFn: () => api.dockerCleanupExecutions(serverId),
    enabled: tab === 'cleanup',
    refetchInterval: tab === 'cleanup' ? 5000 : false,
  })
  const proxyDynamic = useQuery({
    queryKey: ['proxy-dynamic', serverId],
    queryFn: () => api.listProxyDynamic(serverId),
    enabled: tab === 'proxy',
  })
  const proxyLogs = useQuery({
    queryKey: ['proxy-logs', serverId],
    queryFn: () => api.proxyLogs(serverId),
    enabled: tab === 'proxy',
  })
  const sentinelLogs = useQuery({
    queryKey: ['sentinel-logs', serverId],
    queryFn: () => api.sentinelLogs(serverId),
    enabled: tab === 'sentinel',
    refetchInterval: tab === 'sentinel' ? 10000 : false,
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
  const patchOps = useMutation({
    mutationFn: (body: Parameters<typeof api.patchServerOps>[1]) => api.patchServerOps(serverId, body),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['server-ops', serverId] }),
  })
  const sentinelMut = useMutation({
    mutationFn: (action: 'install' | 'restart' | 'stop') => api.sentinelAction(serverId, action),
    onSuccess: (data) => {
      if (data.sentinel_token) setFreshToken(data.sentinel_token)
      void qc.invalidateQueries({ queryKey: ['server-ops', serverId] })
      void qc.invalidateQueries({ queryKey: ['sentinel-logs', serverId] })
    },
  })
  const rotateToken = useMutation({
    mutationFn: () => api.rotateSentinelToken(serverId),
    onSuccess: (data) => {
      setFreshToken(data.sentinel_token)
      void qc.invalidateQueries({ queryKey: ['server-ops', serverId] })
    },
  })
  const runCleanup = useMutation({
    mutationFn: () => api.runDockerCleanup(serverId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['docker-cleanup', serverId] }),
  })
  const saveProxyCfg = useMutation({
    mutationFn: () => api.upsertProxyDynamic(serverId, { name: proxyCfgName, value: proxyCfgValue }),
    onSuccess: () => {
      setProxyCfgValue('')
      void qc.invalidateQueries({ queryKey: ['proxy-dynamic', serverId] })
    },
  })
  const deleteProxyCfg = useMutation({
    mutationFn: (configId: string) => api.deleteProxyDynamic(serverId, configId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['proxy-dynamic', serverId] }),
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

      {tab === 'sentinel' && (
        <TabPanel>
          <div className="panel-card space-y-4 p-5">
            <div className="grid gap-3 sm:grid-cols-2">
              <Meta label="Enabled" value={ops.data?.sentinel_enabled ? 'Yes' : 'No'} />
              <Meta label="Token" value={freshToken || ops.data?.sentinel_token || '—'} />
            </div>
            <label className="flex items-center gap-3 text-sm">
              <input
                type="checkbox"
                checked={Boolean(ops.data?.sentinel_enabled)}
                onChange={(e) => patchOps.mutate({ sentinel_enabled: e.target.checked })}
              />
              <span className="font-medium text-gray-900 dark:text-white">Sentinel metrics agent</span>
            </label>
            <div className="flex flex-wrap gap-2">
              <Btn primary onClick={() => sentinelMut.mutate('install')}>
                {sentinelMut.isPending ? 'Working…' : 'Install / Start'}
              </Btn>
              <Btn onClick={() => sentinelMut.mutate('restart')}>Restart</Btn>
              <Btn onClick={() => sentinelMut.mutate('stop')}>Stop</Btn>
              <Btn onClick={() => rotateToken.mutate()}>Rotate token</Btn>
            </div>
            {(sentinelMut.error || rotateToken.error || patchOps.error) && (
              <p className="text-sm text-error-500">
                {(sentinelMut.error || rotateToken.error || patchOps.error)?.message}
              </p>
            )}
            <div>
              <div className="mb-2 flex items-center justify-between">
                <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Logs</h3>
                <button
                  type="button"
                  className="text-xs text-brand-600 dark:text-brand-400"
                  onClick={() => setTab('metrics')}
                >
                  View metrics →
                </button>
              </div>
              <pre className="max-h-64 overflow-auto rounded-lg bg-gray-950 p-3 font-mono text-xs text-gray-200">
                {sentinelLogs.data?.logs || sentinelLogs.data?.error || 'No logs yet.'}
              </pre>
            </div>
          </div>
        </TabPanel>
      )}

      {tab === 'cleanup' && (
        <TabPanel>
          <div className="panel-card space-y-4 p-5">
            <div className="grid gap-3 sm:grid-cols-2">
              <Input label="Cron frequency" value={cleanupFreq} onChange={setCleanupFreq} />
              <Input label="Disk threshold %" value={cleanupThreshold} onChange={setCleanupThreshold} />
            </div>
            <label className="flex items-center gap-3 text-sm">
              <input
                type="checkbox"
                checked={forceCleanup}
                onChange={(e) => setForceCleanup(e.target.checked)}
              />
              <span>
                <span className="font-medium text-gray-900 dark:text-white">Force cleanup</span>
                <span className="mt-0.5 block text-gray-500 dark:text-gray-400">
                  Ignore disk threshold and prune every run.
                </span>
              </span>
            </label>
            <div className="flex flex-wrap gap-2">
              <Btn
                primary
                onClick={() =>
                  patchOps.mutate({
                    docker_cleanup_frequency: cleanupFreq,
                    docker_cleanup_threshold: Number(cleanupThreshold) || 80,
                    force_docker_cleanup: forceCleanup,
                  })
                }
              >
                {patchOps.isPending ? 'Saving…' : 'Save schedule'}
              </Btn>
              <Btn onClick={() => runCleanup.mutate()}>
                {runCleanup.isPending ? 'Running…' : 'Run now'}
              </Btn>
            </div>
            {(patchOps.error || runCleanup.error) && (
              <p className="text-sm text-error-500">
                {(patchOps.error || runCleanup.error)?.message}
              </p>
            )}
            <div className="space-y-2">
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white">History</h3>
              {(cleanupExecs.data?.executions || []).map((e) => (
                <div key={e.id} className="rounded-lg border border-gray-200 p-3 text-sm dark:border-gray-800">
                  <div className="flex justify-between gap-2">
                    <span className="font-medium capitalize text-gray-900 dark:text-white">{e.status}</span>
                    <span className="text-xs text-gray-500">{new Date(e.started_at).toLocaleString()}</span>
                  </div>
                  {e.message && (
                    <p className="mt-1 font-mono text-xs text-gray-500 dark:text-gray-400">{e.message}</p>
                  )}
                </div>
              ))}
              {!cleanupExecs.data?.executions?.length && (
                <p className="text-sm text-gray-500 dark:text-gray-400">No cleanup runs yet.</p>
              )}
            </div>
          </div>
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
              <Btn onClick={() => void proxyLogs.refetch()}>Refresh logs</Btn>
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
            <div>
              <h3 className="mb-2 text-sm font-semibold text-gray-900 dark:text-white">
                Dynamic Traefik configs
              </h3>
              <div className="space-y-2">
                {(proxyDynamic.data?.configurations || []).map((c) => (
                  <div
                    key={c.id}
                    className="flex items-start justify-between gap-2 rounded-lg border border-gray-200 p-3 dark:border-gray-800"
                  >
                    <div className="min-w-0 flex-1">
                      <div className="font-mono text-xs text-gray-900 dark:text-white">{c.name}</div>
                      <pre className="mt-1 max-h-24 overflow-auto text-xs text-gray-500">{c.value || '(empty)'}</pre>
                    </div>
                    <button
                      type="button"
                      className="text-xs text-error-500"
                      onClick={() => deleteProxyCfg.mutate(c.id)}
                    >
                      Delete
                    </button>
                  </div>
                ))}
              </div>
              <div className="mt-3 space-y-2">
                <Input label="Filename" value={proxyCfgName} onChange={setProxyCfgName} />
                <label className="block text-sm">
                  <span className="mb-1 block text-gray-500 dark:text-gray-400">YAML value</span>
                  <textarea
                    value={proxyCfgValue}
                    onChange={(e) => setProxyCfgValue(e.target.value)}
                    rows={5}
                    className="w-full panel-field rounded-lg px-3 py-2 font-mono text-xs"
                  />
                </label>
                <Btn primary onClick={() => saveProxyCfg.mutate()}>
                  {saveProxyCfg.isPending ? 'Saving…' : 'Save config'}
                </Btn>
                {saveProxyCfg.error && (
                  <p className="text-sm text-error-500">{saveProxyCfg.error.message}</p>
                )}
              </div>
            </div>
            <div>
              <h3 className="mb-2 text-sm font-semibold text-gray-900 dark:text-white">Proxy logs</h3>
              <pre className="max-h-64 overflow-auto rounded-lg bg-gray-950 p-3 font-mono text-xs text-gray-200">
                {proxyLogs.data?.logs || proxyLogs.data?.error || 'No logs.'}
              </pre>
            </div>
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

      {tab === 'edge' && (
        <TabPanel>
          <ServerEdgePanel serverId={serverId} />
        </TabPanel>
      )}

      {tab === 'settings' && (
        <TabPanel>
          <div className="panel-card space-y-4 p-5">
            <div className="flex items-center justify-between gap-3 border-b border-gray-200 pb-4 dark:border-gray-800">
              <div>
                <h3 className="text-sm font-medium text-gray-900 dark:text-white">Shared variables</h3>
                <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                  Variables shared across resources deployed on this server.
                </p>
              </div>
              <Link
                to="/shared-variables"
                search={{ scope: 'server', server_id: serverId }}
                className="inline-flex h-8 items-center rounded-md border border-gray-200 px-2.5 text-xs font-medium hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-white/5"
              >
                Manage →
              </Link>
            </div>
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
              Remove this server from Dockfin. Containers on the host are not deleted.
            </p>
            <button
              type="button"
              className="inline-flex h-8 items-center rounded-md border border-error-500 px-2.5 text-xs font-medium text-error-500 hover:bg-error-500/10"
              onClick={() => {
                if (confirm('Delete this server from Dockfin?')) remove.mutate()
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
