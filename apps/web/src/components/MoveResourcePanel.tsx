import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '../lib/api'
import { Btn } from '../pages/Servers'
import { PanelSkeleton } from './ui/Skeleton'

type Props = {
  resourceType: 'application' | 'database' | 'service'
  resourceId: string
  currentEnvironmentId: string
  projectId?: string
}

export function MoveResourcePanel({
  resourceType,
  resourceId,
  currentEnvironmentId,
  projectId,
}: Props) {
  const qc = useQueryClient()
  const envMeta = useQuery({
    queryKey: ['environment-by-id', currentEnvironmentId],
    queryFn: () => api.getEnvironmentById(currentEnvironmentId),
    enabled: !projectId && Boolean(currentEnvironmentId),
  })
  const resolvedProjectId = projectId || envMeta.data?.project_id || ''
  const envs = useQuery({
    queryKey: ['environments', resolvedProjectId],
    queryFn: () => api.environments(resolvedProjectId),
    enabled: Boolean(resolvedProjectId),
  })
  const [target, setTarget] = useState('')
  const siblings = (envs.data?.environments || []).filter((e) => e.id !== currentEnvironmentId)

  const move = useMutation({
    mutationFn: () =>
      api.moveResource({
        resource_type: resourceType,
        resource_id: resourceId,
        environment_id: target,
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['application', resourceId] })
      void qc.invalidateQueries({ queryKey: ['database', resourceId] })
      void qc.invalidateQueries({ queryKey: ['service', resourceId] })
      void qc.invalidateQueries({ queryKey: ['applications'] })
      void qc.invalidateQueries({ queryKey: ['databases'] })
      void qc.invalidateQueries({ queryKey: ['services'] })
      void qc.invalidateQueries({ queryKey: ['project'] })
      if (resolvedProjectId && target) {
        const segment =
          resourceType === 'application'
            ? 'applications'
            : resourceType === 'database'
              ? 'databases'
              : 'services'
        window.location.assign(
          `/projects/${resolvedProjectId}/environments/${target}/${segment}/${resourceId}`,
        )
      }
    },
  })

  const loading = (!projectId && envMeta.isLoading) || (Boolean(resolvedProjectId) && envs.isLoading)

  return (
    <div className="panel-card space-y-3 p-5">
      <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Move resource</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Move this resource to another environment in the same project. Containers and destination stay
        the same — only the environment association changes.
      </p>
      {loading ? (
        <PanelSkeleton rows={2} showHeader={false} />
      ) : !siblings.length ? (
        <p className="text-sm text-gray-500 dark:text-gray-400">
          No other environments in this project. Clone or create one first.
        </p>
      ) : (
        <div className="flex flex-wrap items-end gap-3">
          <label className="block min-w-[12rem] flex-1 text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">Target environment</span>
            <select
              value={target}
              onChange={(e) => setTarget(e.target.value)}
              className="panel-field w-full rounded-lg px-3 py-2"
            >
              <option value="">Select…</option>
              {siblings.map((e) => (
                <option key={e.id} value={e.id}>
                  {e.name}
                </option>
              ))}
            </select>
          </label>
          <Btn
            primary
            disabled={!target || move.isPending}
            onClick={() => {
              if (confirm('Move this resource to the selected environment?')) move.mutate()
            }}
          >
            {move.isPending ? 'Moving…' : 'Move'}
          </Btn>
        </div>
      )}
      {move.error && <p className="text-sm text-error-500">{move.error.message}</p>}
    </div>
  )
}
