import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { api } from '../lib/api'
import { Btn, Header } from './Servers'

export function DatabaseDetailPage() {
  const { projectId, envId, dbId } = useParams({ strict: false }) as {
    projectId?: string
    envId?: string
    dbId: string
  }
  const qc = useQueryClient()
  const db = useQuery({ queryKey: ['database', dbId], queryFn: () => api.getDatabase(dbId) })

  const start = useMutation({
    mutationFn: () => api.startDatabase(dbId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['database', dbId] }),
  })
  const stop = useMutation({
    mutationFn: () => api.stopDatabase(dbId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['database', dbId] }),
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
      <Link to="/databases" className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400">
        ← Databases
      </Link>
    )

  if (db.isLoading) return <p className="text-gray-500 dark:text-gray-400">Loading…</p>
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

      <div className="grid gap-4 sm:grid-cols-3">
        <Meta label="Status" value={d.status} />
        <Meta label="Engine" value={d.engine} />
        <Meta label="Image" value={d.image || '—'} />
      </div>

      {(start.error || stop.error) && (
        <p className="text-sm text-error-500">{(start.error || stop.error)?.message}</p>
      )}
    </div>
  )
}

export function ServiceDetailPage() {
  const { projectId, envId, svcId } = useParams({ strict: false }) as {
    projectId?: string
    envId?: string
    svcId: string
  }
  const qc = useQueryClient()
  const svc = useQuery({ queryKey: ['service', svcId], queryFn: () => api.getService(svcId) })

  const deploy = useMutation({
    mutationFn: () => api.deployService(svcId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['service', svcId] }),
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
      <Link to="/services" className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400">
        ← Services
      </Link>
    )

  if (svc.isLoading) return <p className="text-gray-500 dark:text-gray-400">Loading…</p>
  if (svc.error || !svc.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{svc.error?.message || 'Service not found'}</p>
        {back}
      </div>
    )
  }

  const s = svc.data

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          {back}
          <h1 className="mt-2 text-xl font-semibold tracking-tight text-gray-900 dark:text-white">{s.name}</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {s.service_type} · {s.status}
          </p>
        </div>
        <Btn primary onClick={() => deploy.mutate()}>
          Deploy
        </Btn>
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        <Meta label="Status" value={s.status} />
        <Meta label="Type" value={s.service_type} />
        <Meta label="Description" value={s.description || '—'} />
      </div>

      {deploy.error && <p className="text-sm text-error-500">{deploy.error.message}</p>}
    </div>
  )
}

export function ServerDetailPage() {
  const { serverId } = useParams({ strict: false }) as { serverId: string }
  const nav = useNavigate()
  const qc = useQueryClient()
  const server = useQuery({ queryKey: ['server', serverId], queryFn: () => api.getServer(serverId) })

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

  if (server.isLoading) return <p className="text-gray-500 dark:text-gray-400">Loading…</p>
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

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <Link to="/servers" className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400">
            ← Servers
          </Link>
          <Header title={s.name} />
          <p className="mt-1 font-mono text-sm text-gray-500 dark:text-gray-400">
            {s.user_name}@{s.ip}:{s.port}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Btn onClick={() => validate.mutate()}>Validate</Btn>
          <Btn primary onClick={() => startProxy.mutate()}>
            Start proxy
          </Btn>
          <Btn onClick={() => stopProxy.mutate()}>Stop proxy</Btn>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Meta label="Reachable" value={s.is_reachable ? 'Yes' : 'No'} />
        <Meta label="Docker" value={s.is_usable ? s.docker_version || 'ok' : 'Unavailable'} />
        <Meta label="Proxy type" value={s.proxy_type || '—'} />
        <Meta label="Proxy status" value={s.proxy_status || '—'} />
      </div>

      {(validate.error || startProxy.error || stopProxy.error) && (
        <p className="text-sm text-error-500">
          {(validate.error || startProxy.error || stopProxy.error)?.message}
        </p>
      )}

      <div className="panel-card border-error-200 p-5 dark:border-error-500/30">
        <h2 className="text-sm font-semibold text-error-500">Danger zone</h2>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Remove this server from Goolify. Containers on the host are not deleted.
        </p>
        <div className="mt-4">
          <button
            type="button"
            className="inline-flex h-8 items-center rounded-md border border-error-500 px-2.5 text-xs font-medium text-error-500 hover:bg-error-500/10"
            onClick={() => {
              if (confirm('Delete this server from Goolify?')) {
                remove.mutate()
              }
            }}
          >
            Delete server
          </button>
          {remove.error && <p className="mt-2 text-sm text-error-500">{remove.error.message}</p>}
        </div>
      </div>
    </div>
  )
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div className="panel-card p-4">
      <div className="text-xs text-gray-500 dark:text-gray-400">{label}</div>
      <div className="mt-1 font-medium text-gray-900 dark:text-white">{value}</div>
    </div>
  )
}
