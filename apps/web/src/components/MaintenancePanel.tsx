import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { Btn } from './ui/Button'

export function MaintenancePanel({
  kind,
  id,
}: {
  kind: 'applications' | 'services'
  id: string
}) {
  const qc = useQueryClient()
  const q = useQuery({
    queryKey: ['maintenance', kind, id],
    queryFn: () => api.maintenance(kind, id),
  })
  const [enabled, setEnabled] = useState(false)
  const [message, setMessage] = useState('')
  const [warning, setWarning] = useState('')

  useEffect(() => {
    if (!q.data) return
    setEnabled(q.data.maintenance_enabled)
    setMessage(q.data.maintenance_message || '')
  }, [q.data])

  const save = useMutation({
    mutationFn: () => api.setMaintenance(kind, id, enabled, message),
    onSuccess: (res) => {
      setWarning((res.warnings || []).join(' '))
      void qc.invalidateQueries({ queryKey: ['maintenance', kind, id] })
      void qc.invalidateQueries({ queryKey: [kind === 'applications' ? 'application' : 'service', id] })
    },
  })

  return (
    <div className="panel-card space-y-3 p-4">
      <div>
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Maintenance mode</h3>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Visitors to this resource&apos;s domains see a maintenance page. The containers keep running, and deploys still work.
          Traefik applies the page immediately. Caddy servers keep the setting until the proxy is Traefik.
        </p>
      </div>
      {q.isLoading && <p className="text-sm text-gray-500">Loading…</p>}
      {q.error && <p className="text-sm text-error-500">{(q.error as Error).message}</p>}
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
        <span className="font-medium text-gray-900 dark:text-white">Show maintenance page</span>
      </label>
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">Message</span>
        <input
          className="panel-field w-full rounded-lg px-3 py-2"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="Back at 18:00 UTC"
        />
      </label>
      <Btn primary disabled={save.isPending} onClick={() => save.mutate()}>
        {save.isPending ? 'Saving…' : 'Save maintenance'}
      </Btn>
      {save.error && <p className="text-sm text-error-500">{(save.error as Error).message}</p>}
      {warning && <p className="text-sm text-amber-600 dark:text-amber-400">{warning}</p>}
    </div>
  )
}
