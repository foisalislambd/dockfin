import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { DeployLogPanel } from '../components/DeployLogPanel'
import { DetailPageSkeleton } from '../components/ui/Skeleton'
import { Meta } from '../components/ui/tabs'
import { api } from '../lib/api'
import { Btn } from './Servers'

export function DeploymentShowPage() {
  const { appId, deploymentId, projectId, envId } = useParams({ strict: false }) as {
    appId: string
    deploymentId: string
    projectId?: string
    envId?: string
  }
  const qc = useQueryClient()
  const nested = Boolean(projectId && envId)
  const [logs, setLogs] = useState<string[]>([])

  const dep = useQuery({
    queryKey: ['deployment', deploymentId],
    queryFn: () => api.getDeployment(deploymentId),
    refetchInterval: (q) => {
      const s = q.state.data?.status
      return s === 'queued' || s === 'in_progress' ? 2500 : false
    },
  })

  const app = useQuery({
    queryKey: ['application', appId],
    queryFn: () => api.application(appId),
  })

  const cancel = useMutation({
    mutationFn: () => api.cancelDeployment(deploymentId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['deployment', deploymentId] }),
  })

  useEffect(() => {
    setLogs([])
    const es = new EventSource(`/api/v1/deployments/${deploymentId}/logs/stream`, {
      withCredentials: true,
    })
    es.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data) as { stage?: string; line?: string }
        const line = data.line ?? ev.data
        const prefix = data.stage ? `[${data.stage}] ` : ''
        setLogs((prev) => [...prev, `${prefix}${line}`])
      } catch {
        setLogs((prev) => [...prev, ev.data])
      }
    }
    return () => es.close()
  }, [deploymentId])

  useEffect(() => {
    const raw = dep.data?.logs
    if (!raw || logs.length) return
    try {
      const entries = (typeof raw === 'string' ? JSON.parse(raw) : raw) as Array<{
        stage?: string
        line?: string
      }>
      if (Array.isArray(entries) && entries.length) {
        setLogs(
          entries.map((e) => {
            const prefix = e.stage ? `[${e.stage}] ` : ''
            return `${prefix}${e.line || ''}`
          }),
        )
      }
    } catch {
      /* ignore */
    }
  }, [dep.data?.logs, logs.length])

  const back =
    nested && projectId && envId ? (
      <Link
        to="/projects/$projectId/environments/$envId/applications/$appId"
        params={{ projectId, envId, appId }}
        className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
      >
        ← {app.data?.name || 'Application'}
      </Link>
    ) : (
      <Link
        to="/applications/$appId"
        params={{ appId }}
        className="text-sm text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
      >
        ← {app.data?.name || 'Application'}
      </Link>
    )

  if (dep.isLoading) return <DetailPageSkeleton withSideNav={false} />
  if (dep.error || !dep.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{dep.error?.message || 'Deployment not found'}</p>
        {back}
      </div>
    )
  }

  const d = dep.data
  const busy = d.status === 'queued' || d.status === 'in_progress'

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          {back}
          <h1 className="mt-2 text-xl font-semibold tracking-tight text-gray-900 dark:text-white">
            Deployment
          </h1>
          <p className="mt-1 font-mono text-xs text-gray-500 dark:text-gray-400">{d.id}</p>
        </div>
        {busy && (
          <Btn onClick={() => cancel.mutate()}>
            {cancel.isPending ? 'Cancelling…' : 'Cancel'}
          </Btn>
        )}
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Meta label="Status" value={d.status} />
        <Meta label="Stage" value={d.current_stage || '—'} />
        <Meta label="Commit" value={d.commit_sha ? d.commit_sha.slice(0, 12) : '—'} />
        <Meta label="Created" value={new Date(d.created_at).toLocaleString()} />
      </div>

      {d.error_message && (
        <div className="rounded-lg border border-error-200 bg-error-500/5 p-3 text-sm text-error-500 dark:border-error-500/30">
          {d.error_message}
        </div>
      )}

      {d.commit_message && (
        <p className="text-sm text-gray-600 dark:text-gray-300">{d.commit_message}</p>
      )}

      <DeployLogPanel
        lines={logs}
        busy={busy}
        emptyHint={busy ? 'Waiting for log events…' : 'No log lines.'}
      />

      {cancel.error && <p className="text-sm text-error-500">{cancel.error.message}</p>}
    </div>
  )
}
