import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { Meta, ResourceTabs, TabPanel } from '../components/ui/tabs'
import { api } from '../lib/api'
import { Btn, Input } from './Servers'

const DB_TABS = [
  { id: 'configuration', label: 'Configuration' },
  { id: 'environment', label: 'Environment Variables' },
  { id: 'danger', label: 'Danger' },
]

const SVC_TABS = [
  { id: 'configuration', label: 'Configuration' },
  { id: 'environment', label: 'Environment Variables' },
  { id: 'danger', label: 'Danger' },
]

const SERVER_TABS = [
  { id: 'overview', label: 'Overview' },
  { id: 'proxy', label: 'Proxy' },
  { id: 'destinations', label: 'Destinations' },
  { id: 'terminal', label: 'Terminal' },
  { id: 'danger', label: 'Danger' },
]

function EnvVarsEditor({ resourceType, resourceId }: { resourceType: string; resourceId: string }) {
  const qc = useQueryClient()
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const vars = useQuery({
    queryKey: ['env-vars', resourceType, resourceId],
    queryFn: () => api.envVars(resourceType, resourceId, true),
  })
  const add = useMutation({
    mutationFn: () =>
      api.upsertEnvVar({
        resource_type: resourceType,
        resource_id: resourceId,
        key,
        value,
      }),
    onSuccess: () => {
      setKey('')
      setValue('')
      void qc.invalidateQueries({ queryKey: ['env-vars', resourceType, resourceId] })
    },
  })
  const del = useMutation({
    mutationFn: (id: string) => api.deleteEnvVar(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['env-vars', resourceType, resourceId] }),
  })

  return (
    <div className="space-y-4">
      <div className="panel-card overflow-hidden">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Key</th>
              <th className="px-3 py-2">Value</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {(vars.data?.environment_variables || []).map((v) => (
              <tr key={v.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-mono text-xs">{v.key}</td>
                <td className="px-3 py-2 font-mono text-xs text-gray-500">{v.value ?? '••••'}</td>
                <td className="px-3 py-2">
                  <button type="button" className="text-error-500" onClick={() => del.mutate(v.id)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
            {!vars.data?.environment_variables?.length && (
              <tr>
                <td colSpan={3} className="px-4 py-8 text-center text-gray-500">
                  No env vars yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <form
        className="flex flex-wrap items-end gap-3"
        onSubmit={(e) => {
          e.preventDefault()
          add.mutate()
        }}
      >
        <div className="min-w-[140px] flex-1">
          <Input label="Key" value={key} onChange={setKey} />
        </div>
        <div className="min-w-[180px] flex-1">
          <Input label="Value" value={value} onChange={setValue} />
        </div>
        <Btn primary type="submit">
          Add
        </Btn>
        {add.error && <p className="w-full text-sm text-error-500">{add.error.message}</p>}
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
  const qc = useQueryClient()
  const [tab, setTab] = useState('configuration')
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
        to="/databases"
        className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
      >
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
          <EnvVarsEditor resourceType="database" resourceId={dbId} />
        </TabPanel>
      )}

      {tab === 'danger' && (
        <TabPanel>
          <div className="panel-card space-y-3 border-error-200 p-5 dark:border-error-500/30">
            <h2 className="text-sm font-semibold text-error-500">Stop database</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Stops the container on the remote server. Permanent delete is not exposed in the API yet.
            </p>
            <Btn onClick={() => stop.mutate()}>Stop now</Btn>
          </div>
        </TabPanel>
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
  const [tab, setTab] = useState('configuration')
  useEffect(() => {
    setTab('configuration')
  }, [svcId])
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
      <Link
        to="/services"
        className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
      >
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

      <ResourceTabs tabs={SVC_TABS} active={tab} onChange={setTab} />

      {tab === 'configuration' && (
        <TabPanel>
          <div className="grid gap-4 sm:grid-cols-3">
            <Meta label="Status" value={s.status} />
            <Meta label="Type" value={s.service_type} />
            <Meta label="Description" value={s.description || '—'} />
          </div>
          {deploy.error && <p className="mt-4 text-sm text-error-500">{deploy.error.message}</p>}
        </TabPanel>
      )}

      {tab === 'environment' && (
        <TabPanel>
          <EnvVarsEditor resourceType="service" resourceId={svcId} />
        </TabPanel>
      )}

      {tab === 'danger' && (
        <TabPanel>
          <div className="panel-card space-y-3 border-error-200 p-5 dark:border-error-500/30">
            <h2 className="text-sm font-semibold text-error-500">Redeploy</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Runs `docker compose up` again on the target server. Permanent delete is not exposed yet.
            </p>
            <Btn primary onClick={() => deploy.mutate()}>
              Force deploy
            </Btn>
          </div>
        </TabPanel>
      )}
    </div>
  )
}

export function ServerDetailPage() {
  const { serverId } = useParams({ strict: false }) as { serverId: string }
  const nav = useNavigate()
  const qc = useQueryClient()
  const [tab, setTab] = useState('overview')
  const [command, setCommand] = useState('docker ps --format "{{.Names}}\t{{.Status}}"')
  const [execOut, setExecOut] = useState('')
  useEffect(() => {
    setTab('overview')
    setExecOut('')
    setCommand('docker ps --format "{{.Names}}\t{{.Status}}"')
  }, [serverId])

  const server = useQuery({ queryKey: ['server', serverId], queryFn: () => api.getServer(serverId) })
  const destinations = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })

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
  const exec = useMutation({
    mutationFn: () => api.serverExec(serverId, command),
    onSuccess: (data) => {
      const parts = [data.stdout, data.stderr, data.output, data.error ? `error: ${data.error}` : '']
        .filter(Boolean)
        .join('\n')
      setExecOut(parts || '(no output)')
    },
    onError: (e: Error) => setExecOut(e.message),
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
          </div>
          {validate.error && <p className="mt-4 text-sm text-error-500">{validate.error.message}</p>}
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
              <Btn primary onClick={() => startProxy.mutate()}>
                Start proxy
              </Btn>
              <Btn onClick={() => stopProxy.mutate()}>Stop proxy</Btn>
            </div>
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
                <div className="mt-1 font-mono text-xs text-gray-500 dark:text-gray-400">{d.network}</div>
              </div>
            ))}
            {!serverDests.length && (
              <div className="panel-card col-span-full p-8 text-center text-sm text-gray-500">
                No destinations on this server yet. They are created when the server is added.
              </div>
            )}
          </div>
        </TabPanel>
      )}

      {tab === 'terminal' && (
        <TabPanel>
          <div className="panel-card space-y-3 p-5">
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Non-interactive remote exec over SSH (not a full xterm session).
            </p>
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Command</span>
              <input
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 font-mono text-sm dark:border-gray-800 dark:bg-gray-900"
              />
            </label>
            <Btn primary onClick={() => exec.mutate()}>
              {exec.isPending ? 'Running…' : 'Run'}
            </Btn>
            <pre className="max-h-80 overflow-auto rounded-lg border border-gray-200 bg-white p-3 font-mono text-xs dark:border-gray-800 dark:bg-gray-900">
              {execOut || 'Output will appear here.'}
            </pre>
          </div>
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
