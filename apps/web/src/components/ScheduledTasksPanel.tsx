import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { api, type ScheduledTask } from '../lib/api'
import { Btn } from '../pages/Servers'
import { PanelSkeleton, TableSkeleton } from './ui/Skeleton'

const CRON_PRESETS = [
  { label: 'Every minute', value: '* * * * *' },
  { label: 'Every 5 minutes', value: '*/5 * * * *' },
  { label: 'Hourly', value: '0 * * * *' },
  { label: 'Daily at midnight', value: '0 0 * * *' },
  { label: 'Weekly (Sunday)', value: '0 0 * * 0' },
]

type Props = {
  resourceType: 'service' | 'application' | 'database'
  resourceId: string
  /** Compose service names for the container dropdown (services only). */
  containerOptions?: string[]
}

export function ScheduledTasksPanel({ resourceType, resourceId, containerOptions = [] }: Props) {
  const qc = useQueryClient()
  const qKey = ['scheduled-tasks', resourceType, resourceId]
  const tasks = useQuery({
    queryKey: qKey,
    queryFn: () => api.scheduledTasks({ resource_type: resourceType, resource_id: resourceId }),
  })

  const [name, setName] = useState('')
  const [command, setCommand] = useState('')
  const [frequency, setFrequency] = useState('0 * * * *')
  const [container, setContainer] = useState(containerOptions[0] || '')
  const [showForm, setShowForm] = useState(false)
  const [expanded, setExpanded] = useState<string | null>(null)

  const create = useMutation({
    mutationFn: () =>
      api.createScheduledTask({
        resource_type: resourceType,
        resource_id: resourceId,
        name,
        command,
        frequency,
        container_name: container || undefined,
      }),
    onSuccess: () => {
      setName('')
      setCommand('')
      setFrequency('0 * * * *')
      setShowForm(false)
      void qc.invalidateQueries({ queryKey: qKey })
    },
  })

  const patch = useMutation({
    mutationFn: ({ id, body }: { id: string; body: Parameters<typeof api.patchScheduledTask>[1] }) =>
      api.patchScheduledTask(id, body),
    onSuccess: () => void qc.invalidateQueries({ queryKey: qKey }),
  })

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteScheduledTask(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: qKey }),
  })

  const run = useMutation({
    mutationFn: (id: string) => api.runScheduledTask(id),
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    create.mutate()
  }

  const list = tasks.data?.scheduled_tasks || []

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Scheduled Tasks</h2>
          <p className="mt-1 max-w-2xl text-sm text-gray-500 dark:text-gray-400">
            Cron jobs run every minute from the Dockfin control plane. Commands execute via{' '}
            <code className="font-mono text-xs">docker exec</code>
            {resourceType === 'service'
              ? ' inside the selected compose container.'
              : ' inside the resource container.'}
          </p>
        </div>
        <Btn primary type="button" onClick={() => setShowForm((v) => !v)}>
          {showForm ? 'Cancel' : '+ Add'}
        </Btn>
      </div>

      {showForm && (
        <form className="panel-card grid gap-3 p-4 sm:grid-cols-2" onSubmit={onSubmit}>
          <label className="block text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">Name</span>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              placeholder="Nightly cleanup"
              className="panel-field w-full rounded-lg px-3 py-2 text-sm"
            />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">Frequency (cron)</span>
            <input
              value={frequency}
              onChange={(e) => setFrequency(e.target.value)}
              required
              list="cron-presets"
              className="panel-field w-full rounded-lg px-3 py-2 font-mono text-sm"
            />
            <datalist id="cron-presets">
              {CRON_PRESETS.map((p) => (
                <option key={p.value} value={p.value}>
                  {p.label}
                </option>
              ))}
            </datalist>
          </label>
          {containerOptions.length > 0 && (
            <label className="block text-sm sm:col-span-2">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Container</span>
              <select
                value={container}
                onChange={(e) => setContainer(e.target.value)}
                className="panel-field w-full rounded-lg px-3 py-2 text-sm"
              >
                {containerOptions.map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
              </select>
            </label>
          )}
          <label className="block text-sm sm:col-span-2">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">Command</span>
            <input
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              required
              placeholder="php artisan schedule:run"
              className="panel-field w-full rounded-lg px-3 py-2 font-mono text-sm"
            />
          </label>
          {create.error && (
            <p className="text-sm text-error-500 sm:col-span-2">{create.error.message}</p>
          )}
          <div className="sm:col-span-2">
            <Btn primary type="submit" disabled={create.isPending}>
              {create.isPending ? 'Saving…' : 'Save task'}
            </Btn>
          </div>
        </form>
      )}

      <div className="panel-card overflow-hidden">
        {tasks.isLoading ? (
          <TableSkeleton rows={4} cols={3} />
        ) : (
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Command</th>
              <th className="px-3 py-2">Frequency</th>
              {containerOptions.length > 0 && <th className="px-3 py-2">Container</th>}
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {list.map((t) => (
              <TaskRow
                key={t.id}
                task={t}
                showContainer={containerOptions.length > 0}
                expanded={expanded === t.id}
                onToggleExpand={() => setExpanded(expanded === t.id ? null : t.id)}
                onToggleEnabled={() =>
                  patch.mutate({ id: t.id, body: { enabled: !t.enabled } })
                }
                onRun={() => run.mutate(t.id)}
                onDelete={() => {
                  if (confirm(`Delete scheduled task “${t.name}”?`)) remove.mutate(t.id)
                }}
                busy={patch.isPending || remove.isPending || run.isPending}
              />
            ))}
            {!list.length && (
              <tr>
                <td
                  colSpan={containerOptions.length > 0 ? 6 : 5}
                  className="px-4 py-10 text-center text-gray-500 dark:text-gray-400"
                >
                  No scheduled tasks yet. Add a cron command to run inside this resource.
                </td>
              </tr>
            )}
          </tbody>
        </table>
        )}
      </div>
      {run.isSuccess && (
        <p className="text-sm text-emerald-600 dark:text-emerald-400">
          Task started (execution {run.data.execution_id.slice(0, 8)}…).
        </p>
      )}
      {run.error && <p className="text-sm text-error-500">{run.error.message}</p>}
    </div>
  )
}

function TaskRow({
  task,
  showContainer,
  expanded,
  onToggleExpand,
  onToggleEnabled,
  onRun,
  onDelete,
  busy,
}: {
  task: ScheduledTask
  showContainer: boolean
  expanded: boolean
  onToggleExpand: () => void
  onToggleEnabled: () => void
  onRun: () => void
  onDelete: () => void
  busy: boolean
}) {
  const execs = useQuery({
    queryKey: ['scheduled-task-executions', task.id],
    queryFn: () => api.scheduledTaskExecutions(task.id),
    enabled: expanded,
  })

  return (
    <>
      <tr className="border-t border-gray-200 dark:border-gray-800">
        <td className="px-3 py-2 font-medium text-gray-900 dark:text-white">{task.name}</td>
        <td className="max-w-[220px] truncate px-3 py-2 font-mono text-xs" title={task.command}>
          {task.command}
        </td>
        <td className="px-3 py-2 font-mono text-xs">{task.frequency}</td>
        {showContainer && (
          <td className="px-3 py-2 font-mono text-xs text-gray-500">
            {task.container_name || '—'}
          </td>
        )}
        <td className="px-3 py-2">
          <button
            type="button"
            disabled={busy}
            onClick={onToggleEnabled}
            className={`rounded-full px-2 py-0.5 text-xs font-medium ${
              task.enabled
                ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400'
                : 'bg-gray-500/15 text-gray-600 dark:text-gray-400'
            }`}
          >
            {task.enabled ? 'Enabled' : 'Disabled'}
          </button>
        </td>
        <td className="px-3 py-2">
          <div className="flex flex-wrap justify-end gap-1">
            <button
              type="button"
              className="rounded px-2 py-1 text-xs text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-white/10"
              onClick={onToggleExpand}
            >
              {expanded ? 'Hide' : 'Logs'}
            </button>
            <button
              type="button"
              disabled={busy || !task.enabled}
              className="rounded px-2 py-1 text-xs text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-white/10"
              onClick={onRun}
            >
              Run
            </button>
            <button
              type="button"
              disabled={busy}
              className="rounded px-2 py-1 text-xs text-error-500 hover:bg-error-500/10"
              onClick={onDelete}
            >
              Delete
            </button>
          </div>
        </td>
      </tr>
      {expanded && (
        <tr className="border-t border-gray-100 bg-gray-50/80 dark:border-gray-800 dark:bg-white/[0.03]">
          <td colSpan={showContainer ? 6 : 5} className="px-3 py-3">
            {execs.isLoading ? (
              <PanelSkeleton rows={2} showHeader={false} />
            ) : (execs.data?.executions || []).length === 0 ? (
              <p className="text-xs text-gray-500">No executions yet.</p>
            ) : (
            <ul className="space-y-2">
              {(execs.data?.executions || []).slice(0, 5).map((e) => (
                <li key={e.id} className="rounded-lg border border-gray-200 p-2 dark:border-gray-800">
                  <div className="flex flex-wrap items-center gap-2 text-xs">
                    <span
                      className={
                        e.status === 'finished'
                          ? 'text-emerald-600 dark:text-emerald-400'
                          : e.status === 'failed'
                            ? 'text-error-500'
                            : 'text-amber-600'
                      }
                    >
                      {e.status}
                    </span>
                    <span className="text-gray-500">{new Date(e.started_at).toLocaleString()}</span>
                  </div>
                  {e.output && (
                    <pre className="mt-1 max-h-40 overflow-auto whitespace-pre-wrap font-mono text-[11px] text-gray-600 dark:text-gray-400">
                      {e.output}
                    </pre>
                  )}
                </li>
              ))}
            </ul>
            )}
          </td>
        </tr>
      )}
    </>
  )
}
