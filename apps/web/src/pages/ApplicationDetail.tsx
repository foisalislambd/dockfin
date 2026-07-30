import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { api } from '../lib/api'
import { Btn, Header, Input } from './Servers'

export function ApplicationDetailPage() {
  const { appId } = useParams({ strict: false }) as { appId: string }
  const qc = useQueryClient()
  const app = useQuery({ queryKey: ['application', appId], queryFn: () => api.application(appId) })
  const deps = useQuery({
    queryKey: ['deployments', appId],
    queryFn: () => api.deployments(appId),
    refetchInterval: (q) => {
      const list = q.state.data?.deployments || []
      const busy = list.some((d) => d.status === 'queued' || d.status === 'in_progress')
      return busy ? 3000 : false
    },
  })
  const envVars = useQuery({
    queryKey: ['env-vars', appId],
    queryFn: () => api.envVars('application', appId),
  })

  const [streamId, setStreamId] = useState<string | null>(null)
  const [logs, setLogs] = useState<string[]>([])
  const [envKey, setEnvKey] = useState('')
  const [envValue, setEnvValue] = useState('')
  const logRef = useRef<HTMLPreElement>(null)

  const activeDep = (deps.data?.deployments || []).find(
    (d) => d.status === 'queued' || d.status === 'in_progress',
  )

  useEffect(() => {
    if (activeDep && !streamId) {
      setStreamId(activeDep.id)
      setLogs([])
    }
  }, [activeDep, streamId])

  useEffect(() => {
    if (!streamId) return
    const es = new EventSource(`/api/v1/deployments/${streamId}/logs/stream`, {
      withCredentials: true,
    })
    es.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data) as { stage?: string; line?: string; ts?: string }
        const line = data.line ?? ev.data
        const prefix = data.stage ? `[${data.stage}] ` : ''
        setLogs((prev) => [...prev, `${prefix}${line}`])
      } catch {
        setLogs((prev) => [...prev, ev.data])
      }
    }
    es.onerror = () => {
      // keep connection; browser may reconnect; stop on terminal statuses via polling
    }
    return () => es.close()
  }, [streamId])

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight
    }
  }, [logs])

  useEffect(() => {
    if (!streamId) return
    const dep = (deps.data?.deployments || []).find((d) => d.id === streamId)
    if (dep && (dep.status === 'finished' || dep.status === 'failed' || dep.status === 'cancelled')) {
      // leave streamId so logs stay visible; user can clear by deploying again
    }
  }, [deps.data, streamId])

  const deploy = useMutation({
    mutationFn: () => api.deployApplication(appId),
    onSuccess: (dep) => {
      setStreamId(dep.id)
      setLogs([])
      void qc.invalidateQueries({ queryKey: ['deployments', appId] })
      void qc.invalidateQueries({ queryKey: ['application', appId] })
    },
  })

  const cancel = useMutation({
    mutationFn: (id: string) => api.cancelDeployment(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['deployments', appId] }),
  })

  const addEnv = useMutation({
    mutationFn: () =>
      api.upsertEnvVar({
        resource_type: 'application',
        resource_id: appId,
        key: envKey,
        value: envValue,
      }),
    onSuccess: () => {
      setEnvKey('')
      setEnvValue('')
      void qc.invalidateQueries({ queryKey: ['env-vars', appId] })
    },
  })

  if (app.isLoading) {
    return <p className="text-gray-500 dark:text-gray-400">Loading…</p>
  }
  if (app.error || !app.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{app.error?.message || 'Application not found'}</p>
        <Link to="/applications" className="text-brand-600 dark:text-brand-400">
          ← Applications
        </Link>
      </div>
    )
  }

  const a = app.data

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <Link to="/applications" className="text-sm text-gray-500 dark:text-gray-400 hover:text-brand-600 dark:text-brand-400">
            ← Applications
          </Link>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight">{a.name}</h1>
          <p className="mt-2 text-gray-500 dark:text-gray-400">
            {a.build_pack} · {a.status}
            {a.fqdn ? ` · ${a.fqdn}` : ''}
          </p>
        </div>
        <div className="flex gap-2">
          {activeDep && (
            <Btn onClick={() => cancel.mutate(activeDep.id)}>Cancel deploy</Btn>
          )}
          <Btn primary onClick={() => deploy.mutate()}>
            Deploy
          </Btn>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        <Meta label="Status" value={a.status} />
        <Meta label="Build pack" value={a.build_pack} />
        <Meta label="FQDN" value={a.fqdn || '—'} />
      </div>

      {(streamId || activeDep) && (
        <section className="space-y-2">
          <Header title="Live logs" subtitle={`Deployment ${streamId || activeDep?.id}`} />
          <pre
            ref={logRef}
            className="max-h-80 overflow-auto rounded-xl border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 p-4 font-mono text-xs leading-relaxed text-gray-500 dark:text-gray-400"
          >
            {logs.length ? logs.join('\n') : 'Waiting for log events…'}
          </pre>
        </section>
      )}

      <section className="space-y-3">
        <h2 className="text-lg font-medium">Deployments</h2>
        <div className="overflow-hidden rounded-xl border border-gray-200 dark:border-gray-800">
          <table className="w-full text-left text-sm">
            <thead className="panel-card bg-white dark:bg-white/3 text-gray-500 dark:text-gray-400">
              <tr>
                <th className="px-4 py-3">ID</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">Stage</th>
                <th className="px-4 py-3">Created</th>
                <th className="px-4 py-3">Actions</th>
              </tr>
            </thead>
            <tbody>
              {(deps.data?.deployments || []).map((d) => (
                <tr key={d.id} className="border-t border-gray-200 dark:border-gray-800">
                  <td className="px-4 py-3 font-mono text-xs">{d.id.slice(0, 8)}…</td>
                  <td className="px-4 py-3">{d.status}</td>
                  <td className="px-4 py-3">{d.current_stage || '—'}</td>
                  <td className="px-4 py-3 text-gray-500 dark:text-gray-400">
                    {new Date(d.created_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-3 space-x-2">
                    <button
                      type="button"
                      className="text-brand-600 dark:text-brand-400"
                      onClick={() => {
                        setStreamId(d.id)
                        setLogs([])
                      }}
                    >
                      Stream
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
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-medium">Environment variables</h2>
        <div className="overflow-hidden rounded-xl border border-gray-200 dark:border-gray-800">
          <table className="w-full text-left text-sm">
            <thead className="panel-card bg-white dark:bg-white/3 text-gray-500 dark:text-gray-400">
              <tr>
                <th className="px-4 py-3">Key</th>
                <th className="px-4 py-3">Value</th>
                <th className="px-4 py-3">Flags</th>
              </tr>
            </thead>
            <tbody>
              {(envVars.data?.environment_variables || []).map((v) => (
                <tr key={v.id} className="border-t border-gray-200 dark:border-gray-800">
                  <td className="px-4 py-3 font-mono text-xs">{v.key}</td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-500 dark:text-gray-400">
                    {v.value ?? '••••'}
                  </td>
                  <td className="px-4 py-3 text-xs text-gray-500 dark:text-gray-400">
                    {[v.is_runtime && 'runtime', v.is_buildtime && 'build', v.is_literal && 'literal']
                      .filter(Boolean)
                      .join(', ') || '—'}
                  </td>
                </tr>
              ))}
              {!envVars.data?.environment_variables?.length && (
                <tr>
                  <td colSpan={3} className="px-4 py-6 text-center text-gray-500 dark:text-gray-400">
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
            addEnv.mutate()
          }}
        >
          <div className="min-w-[140px] flex-1">
            <Input label="Key" value={envKey} onChange={setEnvKey} />
          </div>
          <div className="min-w-[180px] flex-1">
            <Input label="Value" value={envValue} onChange={setEnvValue} />
          </div>
          <Btn primary type="submit">
            Add
          </Btn>
          {addEnv.error && (
            <p className="w-full text-sm text-error-500">{addEnv.error.message}</p>
          )}
        </form>
      </section>

      {deploy.error && <p className="text-sm text-error-500">{deploy.error.message}</p>}
    </div>
  )
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-gray-200 dark:border-gray-800 panel-card bg-white dark:bg-white/3/60 p-4">
      <div className="text-xs text-gray-500 dark:text-gray-400">{label}</div>
      <div className="mt-1 font-medium">{value}</div>
    </div>
  )
}
