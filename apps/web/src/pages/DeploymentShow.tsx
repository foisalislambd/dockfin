import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { BackLink } from '../components/BackLink'
import { useConfirm } from '../components/ConfirmDialog'
import { DeployLogPanel } from '../components/DeployLogPanel'
import { useToast } from '../components/Toast'
import { SimpleDetailSkeleton } from '../components/ui/Skeleton'
import { Meta } from '../components/ui/tabs'
import { api } from '../lib/api'
import { Btn } from '../components/ui/Button'

const PROGRESS = ['prepare', 'fetch', 'build', 'transfer', 'run', 'healthcheck', 'finalize']

export function DeploymentShowPage() {
  const { appId, deploymentId, projectId, envId } = useParams({ strict: false }) as {
    appId: string
    deploymentId: string
    projectId?: string
    envId?: string
  }
  const qc = useQueryClient()
  const nav = useNavigate()
  const confirm = useConfirm()
  const toast = useToast()
  const nested = Boolean(projectId && envId)
  const [logs, setLogs] = useState<string[]>([])
  const [liveStage, setLiveStage] = useState('')
  const [liveStatus, setLiveStatus] = useState('')
  const [liveError, setLiveError] = useState('')

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
    onSuccess: () => {
      toast.success('Cancellation requested')
      void qc.invalidateQueries({ queryKey: ['deployment', deploymentId] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const rollback = useMutation({
    mutationFn: () => api.rollbackApplication(appId),
    onSuccess: (dep) => {
      toast.success('Rollback queued')
      void nav({
        to: nested
          ? '/projects/$projectId/environments/$envId/applications/$appId/deployments/$deploymentId'
          : '/applications/$appId/deployments/$deploymentId',
        params: { projectId: projectId || '', envId: envId || '', appId, deploymentId: dep.id },
      })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  useEffect(() => {
    setLogs([])
    setLiveStage('')
    setLiveStatus('')
    setLiveError('')
    const es = new EventSource(`/api/v1/deployments/${deploymentId}/logs/stream`, {
      withCredentials: true,
    })
    es.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data) as {
          type?: string
          stage?: string
          line?: string
          status?: string
          error?: string
        }
        if (data.stage) setLiveStage(data.stage)
        if (data.type === 'status') {
          if (data.status) setLiveStatus(data.status)
          if (data.error) setLiveError(data.error)
          void qc.invalidateQueries({ queryKey: ['deployment', deploymentId] })
        }
        const line = data.line ?? ev.data
        const prefix = data.stage && data.type !== 'status' ? `[${data.stage}] ` : ''
        setLogs((prev) => [...prev, `${prefix}${line}`])
      } catch {
        setLogs((prev) => [...prev, ev.data])
      }
    }
    es.onerror = () => {
      /* EventSource reconnects while the deploy is still running. */
    }
    return () => es.close()
  }, [deploymentId, qc])

  const back =
    nested && projectId && envId ? (
      <BackLink
        label={app.data?.name || 'Application'}
        to="/projects/$projectId/environments/$envId/applications/$appId"
        params={{ projectId, envId, appId }}
      />
    ) : (
      <BackLink label={app.data?.name || 'Application'} to="/applications/$appId" params={{ appId }} />
    )

  if (dep.isLoading) return <SimpleDetailSkeleton />
  if (dep.error || !dep.data) {
    return (
      <div className="space-y-4">
        <p className="text-error-500">{dep.error?.message || 'Deployment not found'}</p>
        {back}
      </div>
    )
  }

  const d = dep.data
  const status = liveStatus || d.status
  const stage = liveStage || d.current_stage
  const failure = liveError || d.error_message
  const busy = status === 'queued' || status === 'in_progress'
  const stageIndex = PROGRESS.indexOf(stage)
  const canRollback = Boolean(appId) && (status === 'failed' || status === 'finished')

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
        <div className="flex flex-wrap gap-2">
          {busy && (
            <Btn onClick={() => cancel.mutate()} disabled={cancel.isPending}>
              {cancel.isPending ? 'Cancelling…' : 'Cancel'}
            </Btn>
          )}
          {canRollback && (
            <Btn
              onClick={() => {
                void (async () => {
                  const ok = await confirm({
                    title: 'Roll back',
                    message: 'Queue a rollback to the previous finished deployment?',
                    confirmLabel: 'Roll back',
                    danger: status === 'failed',
                  })
                  if (ok) rollback.mutate()
                })()
              }}
              disabled={rollback.isPending}
            >
              {rollback.isPending ? 'Queuing…' : 'Roll back'}
            </Btn>
          )}
        </div>
      </div>

      <ol className="flex gap-2 overflow-x-auto pb-1" aria-label="Deploy progress">
        {PROGRESS.map((name, i) => {
          const done = stageIndex > i || status === 'finished'
          const current = name === stage || (stageIndex < 0 && i === 0 && busy && name === 'prepare')
          return (
            <li
              key={name}
              className={`shrink-0 rounded-full px-2.5 py-1 text-xs capitalize ${
                done
                  ? 'bg-success-500/15 text-success-700 dark:text-success-400'
                  : current
                    ? 'bg-brand-500 text-white'
                    : 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400'
              }`}
              aria-current={current ? 'step' : undefined}
            >
              {name}
            </li>
          )
        })}
      </ol>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Meta label="Status" value={status} />
        <Meta label="Stage" value={stage || '—'} />
        <Meta label="Commit" value={d.commit_sha ? d.commit_sha.slice(0, 12) : '—'} />
        <Meta label="Created" value={new Date(d.created_at).toLocaleString()} />
      </div>

      {failure && status === 'failed' && (
        <div className="rounded-lg border border-error-200 bg-error-500/5 p-3 text-sm text-error-500 dark:border-error-500/30" role="alert">
          {failure}
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
