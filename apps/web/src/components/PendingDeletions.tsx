import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../lib/api'
import { useToast } from './Toast'

export function PendingDeletions() {
  const qc = useQueryClient()
  const toast = useToast()
  const list = useQuery({
    queryKey: ['deletions'],
    queryFn: api.deletions,
    refetchInterval: 4000,
  })
  const retry = useMutation({
    mutationFn: (row: { resource_type: string; resource_id: string }) =>
      api.retryDeletion(row.resource_type, row.resource_id),
    onSuccess: () => {
      toast.success('Cleanup queued again')
      void qc.invalidateQueries({ queryKey: ['deletions'] })
    },
    onError: (e: Error) => toast.error(e.message),
  })
  const forget = useMutation({
    mutationFn: (row: { resource_type: string; resource_id: string }) =>
      api.forgetDeletion(row.resource_type, row.resource_id),
    onSuccess: () => {
      toast.success('Removed from Dockfin. Containers were left as they are.')
      void qc.invalidateQueries({ queryKey: ['deletions'] })
    },
    onError: (e: Error) => toast.error(e.message),
  })
  const rows = list.data?.deletions || []
  if (!rows.length) return null
  return (
    <section className="panel-card border-error-200 p-4 dark:border-error-500/40" role="status">
      <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Cleanup still running or failed</h2>
      <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
        These resources are hidden from their projects. Dockfin removes the record after the server cleanup succeeds.
      </p>
      <ul className="mt-3 space-y-2">
        {rows.map((row) => (
          <li key={`${row.resource_type}-${row.resource_id}`} className="flex flex-wrap items-center justify-between gap-2 text-sm">
            <div>
              <span className="font-medium text-gray-900 dark:text-white">{row.name}</span>
              <span className="ml-2 text-gray-500">{row.resource_type}</span>
              <span className="ml-2 text-gray-500">{row.error ? 'failed' : row.status}</span>
              {row.error ? <p className="text-error-500">{row.error}</p> : null}
            </div>
            <div className="flex gap-3">
              {row.error ? (
                <button type="button" className="text-brand-600 hover:underline" onClick={() => retry.mutate(row)}>
                  Retry cleanup
                </button>
              ) : null}
              <button
                type="button"
                className="text-error-500 hover:underline"
                onClick={() => {
                  if (confirm(`Forget ${row.name}? Dockfin drops the record and leaves containers on the server.`)) {
                    forget.mutate(row)
                  }
                }}
              >
                Forget anyway
              </button>
            </div>
          </li>
        ))}
      </ul>
    </section>
  )
}
