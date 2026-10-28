import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api, type TrafficReport } from '../lib/api'
import { useToast } from './Toast'
import { Btn } from './ui/Button'
import { Input } from './ui/Input'

type Range = '24h' | '7d' | '30d'

export function TrafficSummary({
  kind,
  id,
}: {
  kind: 'server' | 'application' | 'service'
  id: string
}) {
  const [range, setRange] = useState<Range>('24h')
  const report = useQuery({
    queryKey: ['traffic', kind, id, range],
    queryFn: () => {
      if (kind === 'application') return api.applicationTraffic(id, range)
      if (kind === 'service') return api.serviceTraffic(id, range)
      return api.serverTraffic(id, range)
    },
  })
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white">Requests</h2>
        <div className="flex gap-2">
          {(['24h', '7d', '30d'] as Range[]).map((item) => (
            <Btn key={item} type="button" primary={range === item} onClick={() => setRange(item)}>
              {item}
            </Btn>
          ))}
        </div>
      </div>
      {report.isLoading && <p className="text-sm text-gray-500 dark:text-gray-400">Loading requests…</p>}
      {report.error && (
        <p className="text-sm text-error-500">
          {report.error instanceof Error ? report.error.message : 'Failed to load requests'}
        </p>
      )}
      {report.data && <TrafficBody report={report.data} />}
    </div>
  )
}

function TrafficBody({ report }: { report: TrafficReport }) {
  const mode = report.ip_mode === 'anonymized' ? 'networks' : 'addresses'
  return (
    <div className="space-y-4">
      <p className="text-sm text-gray-500 dark:text-gray-400">
        {report.requests.toLocaleString()} requests
        {report.enabled ? '' : ' · collection is off'}
        {report.ip_mode === 'off' ? ' · client addresses are not stored' : ` · top ${mode}`}
      </p>
      <CountTable title="Status" rows={report.statuses} />
      <CountTable title="Hosts" rows={report.hosts} />
      <div className="panel-card overflow-hidden">
        <div className="px-3 py-2 text-sm font-medium text-gray-900 dark:text-white">Paths</div>
        <table className="w-full text-left text-sm">
          <tbody>
            {report.paths.length === 0 && (
              <tr>
                <td className="px-3 py-2 text-gray-500 dark:text-gray-400">No requests in this range.</td>
              </tr>
            )}
            {report.paths.map((row) => (
              <tr key={`${row.host} ${row.path}`} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-mono text-xs">
                  {row.host}
                  {row.path}
                </td>
                <td className="px-3 py-2 text-right">{row.requests.toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {report.ip_mode !== 'off' && <CountTable title={report.ip_mode === 'anonymized' ? 'Networks' : 'Clients'} rows={report.clients} />}
    </div>
  )
}

function CountTable({ title, rows }: { title: string; rows: { name: string; requests: number }[] }) {
  return (
    <div className="panel-card overflow-hidden">
      <div className="px-3 py-2 text-sm font-medium text-gray-900 dark:text-white">{title}</div>
      <table className="w-full text-left text-sm">
        <tbody>
          {rows.length === 0 && (
            <tr>
              <td className="px-3 py-2 text-gray-500 dark:text-gray-400">None yet.</td>
            </tr>
          )}
          {rows.map((row) => (
            <tr key={row.name} className="border-t border-gray-200 dark:border-gray-800">
              <td className="px-3 py-2 font-mono text-xs">{row.name}</td>
              <td className="px-3 py-2 text-right">{row.requests.toLocaleString()}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function TrafficSettings({ serverId }: { serverId: string }) {
  const qc = useQueryClient()
  const toast = useToast()
  const ops = useQuery({ queryKey: ['server-ops', serverId], queryFn: () => api.getServerOps(serverId) })
  const [enabled, setEnabled] = useState<boolean | null>(null)
  const [threshold, setThreshold] = useState<string | null>(null)
  const [retention, setRetention] = useState<string | null>(null)
  const [mode, setMode] = useState<string | null>(null)
  const on = enabled ?? Boolean(ops.data?.traffic_analytics_enabled)
  const thresholdValue = threshold ?? String(ops.data?.traffic_sample_threshold ?? 0)
  const retentionValue = retention ?? String(ops.data?.traffic_retention_days ?? 7)
  const modeValue = mode ?? ops.data?.traffic_ip_mode ?? 'anonymized'

  const save = useMutation({
    mutationFn: () =>
      api.patchServerOps(serverId, {
        traffic_analytics_enabled: on,
        traffic_sample_threshold: Number(thresholdValue),
        traffic_retention_days: Number(retentionValue),
        traffic_ip_mode: modeValue,
      }),
    onSuccess: (res) => {
      setEnabled(null)
      setThreshold(null)
      setRetention(null)
      setMode(null)
      void qc.invalidateQueries({ queryKey: ['server-ops', serverId] })
      void qc.invalidateQueries({ queryKey: ['traffic', 'server', serverId] })
      if (res.warnings?.length) toast.error(res.warnings.join(' '))
      else toast.success('Traffic settings saved')
    },
    onError: (e: Error) => toast.error(e.message || 'Failed to save traffic settings'),
  })

  return (
    <form
      className="panel-card space-y-3 p-4"
      onSubmit={(e) => {
        e.preventDefault()
        save.mutate()
      }}
    >
      <h2 className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white">Traffic</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Traefik writes a JSON access log and Sentinel sends the new lines here. Counts are kept for
        the retention window. A sample threshold of 0 keeps every request; a higher number keeps
        the first requests for each host in a batch and then one in ten. Turning this on restarts a
        running Traefik proxy. Caddy is not collected.
      </p>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={on} onChange={(e) => setEnabled(e.target.checked)} />
        <span>Collect request totals</span>
      </label>
      <div className="grid gap-3 sm:grid-cols-3">
        <Input label="Sample threshold" value={thresholdValue} onChange={setThreshold} required={false} />
        <Input label="Retention days" value={retentionValue} onChange={setRetention} required={false} />
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Client addresses</span>
          <select
            value={modeValue}
            onChange={(e) => setMode(e.target.value)}
            className="panel-field w-full rounded-lg px-3 py-2"
          >
            <option value="anonymized">Anonymized</option>
            <option value="full">Full</option>
            <option value="off">Off</option>
          </select>
        </label>
      </div>
      <Btn primary type="submit" disabled={save.isPending || ops.isLoading}>
        {save.isPending ? 'Saving…' : 'Save traffic settings'}
      </Btn>
    </form>
  )
}
