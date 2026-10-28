import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api, type GithubRunnerConfig } from '../lib/api'
import { useToast } from './Toast'
import { Btn } from './ui/Button'
import { Input } from './ui/Input'

export function GithubRunners({ serverId }: { serverId: string }) {
  const qc = useQueryClient()
  const toast = useToast()
  const runners = useQuery({
    queryKey: ['github-runners', serverId],
    queryFn: () => api.githubRunners(serverId),
    refetchInterval: 10000,
  })
  const sources = useQuery({ queryKey: ['git-sources'], queryFn: () => api.gitSources() })
  const orgApps = (sources.data?.git_sources ?? []).filter(
    (source) => source.provider === 'github' && source.installed && source.organization,
  )
  const saved = runners.data?.config
  const [draft, setDraft] = useState<Partial<GithubRunnerConfig> & { labelsText?: string }>({})
  const labelsText = draft.labelsText ?? saved?.labels.join(', ') ?? ''
  const sourceId = draft.git_source_id ?? saved?.git_source_id ?? orgApps[0]?.id ?? ''
  const enabled = draft.enabled ?? saved?.enabled ?? false
  const allowPR = draft.allow_pull_requests ?? saved?.allow_pull_requests ?? false
  const maxRunners = draft.max_runners ?? saved?.max_runners ?? 2
  const mode = draft.docker_mode ?? saved?.docker_mode ?? 'none'
  const image = draft.runner_image ?? saved?.runner_image ?? ''
  const cpu = draft.cpu_limit ?? saved?.cpu_limit ?? ''
  const memory = draft.memory_limit ?? saved?.memory_limit ?? ''
  const queueWait = draft.queue_wait_minutes ?? saved?.queue_wait_minutes ?? 60
  const idle = draft.idle_timeout_minutes ?? saved?.idle_timeout_minutes ?? 10
  const job = draft.job_timeout_minutes ?? saved?.job_timeout_minutes ?? 360

  const save = useMutation({
    mutationFn: () =>
      api.saveGithubRunners(serverId, {
        git_source_id: sourceId,
        labels: labelsText,
        enabled,
        allow_pull_requests: allowPR,
        max_runners: Number(maxRunners),
        docker_mode: mode,
        runner_image: image,
        cpu_limit: cpu,
        memory_limit: memory,
        queue_wait_minutes: Number(queueWait),
        idle_timeout_minutes: Number(idle),
        job_timeout_minutes: Number(job),
      }),
    onSuccess: () => {
      setDraft({})
      void qc.invalidateQueries({ queryKey: ['github-runners', serverId] })
      toast.success('Runner settings saved')
    },
    onError: (e: Error) => toast.error(e.message || 'Failed to save runner settings'),
  })

  return (
    <div className="space-y-4">
      <form
        className="panel-card space-y-3 p-4"
        onSubmit={(e) => {
          e.preventDefault()
          save.mutate()
        }}
      >
        <h2 className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white">GitHub Actions runners</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          A workflow job that asks for <span className="font-mono">self-hosted</span>, <span className="font-mono">linux</span>,
          and one of your labels runs in a new container. The container is removed when the job ends. The host Docker
          socket is not mounted. The GitHub App needs the workflow job event and permission to manage organization
          runners.
        </p>
        {orgApps.length === 0 && (
          <p className="text-sm text-error-500">
            Add an installed GitHub App that belongs to an organization under Sources.
          </p>
        )}
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={enabled} onChange={(e) => setDraft({ ...draft, enabled: e.target.checked })} />
          <span>Take workflow jobs on this server</span>
        </label>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Jobs use this server&apos;s CPU, memory, and disk alongside your other containers.
        </p>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">GitHub App</span>
            <select
              value={sourceId}
              onChange={(e) => setDraft({ ...draft, git_source_id: e.target.value })}
              className="panel-field w-full rounded-lg px-3 py-2"
            >
              {orgApps.map((source) => (
                <option key={source.id} value={source.id}>
                  {source.name} ({source.organization})
                </option>
              ))}
            </select>
          </label>
          <Input label="Labels" value={labelsText} onChange={(value) => setDraft({ ...draft, labelsText: value })} required={false} />
          <Input label="Parallel runners" value={String(maxRunners)} onChange={(value) => setDraft({ ...draft, max_runners: Number(value) })} required={false} />
          <label className="block text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">Docker in jobs</span>
            <select
              value={mode}
              onChange={(e) => setDraft({ ...draft, docker_mode: e.target.value })}
              className="panel-field w-full rounded-lg px-3 py-2"
            >
              <option value="none">No Docker</option>
              <option value="dind">Privileged Docker</option>
            </select>
          </label>
          <Input label="CPU limit" value={cpu} onChange={(value) => setDraft({ ...draft, cpu_limit: value })} required={false} />
          <Input label="Memory limit" value={memory} onChange={(value) => setDraft({ ...draft, memory_limit: value })} required={false} />
          <Input label="Runner image" value={image} onChange={(value) => setDraft({ ...draft, runner_image: value })} required={false} />
          <Input label="Queue wait minutes" value={String(queueWait)} onChange={(value) => setDraft({ ...draft, queue_wait_minutes: Number(value) })} required={false} />
          <Input label="Idle minutes" value={String(idle)} onChange={(value) => setDraft({ ...draft, idle_timeout_minutes: Number(value) })} required={false} />
          <Input label="Job minutes" value={String(job)} onChange={(value) => setDraft({ ...draft, job_timeout_minutes: Number(value) })} required={false} />
        </div>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={allowPR} onChange={(e) => setDraft({ ...draft, allow_pull_requests: e.target.checked })} />
          <span>Run pull request jobs</span>
        </label>
        {mode === 'dind' && (
          <p className="text-sm text-amber-600 dark:text-amber-400">
            Privileged Docker gives a workflow root on this server. Use it only for repositories you trust.
          </p>
        )}
        {allowPR && (
          <p className="text-sm text-amber-600 dark:text-amber-400">
            Anyone who can change a pull request workflow can run code here. Require approval for fork pull requests
            in the organization&apos;s Actions settings.
          </p>
        )}
        <Btn primary type="submit" disabled={save.isPending || !sourceId}>
          {save.isPending ? 'Saving…' : 'Save runners'}
        </Btn>
      </form>
      <div className="panel-card overflow-hidden">
        <div className="px-3 py-2 text-sm font-medium text-gray-900 dark:text-white">Recent runners</div>
        <table className="w-full text-left text-sm">
          <tbody>
            {(runners.data?.executions.length ?? 0) === 0 && (
              <tr>
                <td className="px-3 py-2 text-gray-500 dark:text-gray-400">No runners yet.</td>
              </tr>
            )}
            {runners.data?.executions.map((row) => (
              <tr key={row.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2">
                  <div className="font-medium">{row.job_name || row.workflow_name || row.runner_name || 'Queued job'}</div>
                  <div className="text-xs text-gray-500">{row.repository}</div>
                </td>
                <td className="px-3 py-2 text-right">
                  {row.status}
                  {row.conclusion ? ` · ${row.conclusion}` : ''}
                  {row.error_message ? ` · ${row.error_message}` : ''}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
