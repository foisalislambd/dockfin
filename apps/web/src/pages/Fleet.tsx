import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { api } from '../lib/api'
import { Header } from '../components/ui/Header'
import { PageSkeleton } from '../components/ui/Skeleton'

function pct(used?: number | null, total?: number | null) {
  if (!used || !total) return '—'
  return `${Math.round((used / total) * 100)}%`
}

export function FleetPage() {
  const fleet = useQuery({ queryKey: ['fleet-metrics'], queryFn: api.fleetMetrics })
  const [q, setQ] = useState('')
  const servers = fleet.data?.servers || []
  const needle = q.trim().toLowerCase()
  const shown = needle
    ? servers.filter((s) => `${s.name} ${s.ip} ${s.proxy_type || ''}`.toLowerCase().includes(needle))
    : servers
  return (
    <div className="space-y-4">
      <Header title="Fleet" subtitle="Latest CPU, memory, and disk sample from each server." />
      {fleet.isLoading ? <PageSkeleton cards={2} /> : null}
      {fleet.error ? (
        <p className="text-sm text-error-500" role="alert">
          Could not load fleet metrics. {fleet.error.message}
        </p>
      ) : null}
      {!fleet.isLoading && !fleet.error && servers.length > 0 ? (
        <label className="block max-w-xl text-xs font-medium text-gray-600 dark:text-gray-300">
          Search
          <input
            type="search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Name, IP, or proxy"
            className="panel-field mt-1 h-9 w-full rounded-md px-2 text-sm"
          />
        </label>
      ) : null}
      {!fleet.isLoading && !fleet.error ? (
        <div className="panel-card overflow-x-auto" tabIndex={0} aria-label="Server fleet">
          <table className="panel-table min-w-[40rem]">
            <thead>
              <tr>
                <th>Server</th>
                <th>Proxy</th>
                <th>CPU</th>
                <th>Memory</th>
                <th>Disk</th>
                <th>Sampled</th>
              </tr>
            </thead>
            <tbody>
              {shown.map((s) => (
                <tr key={s.server_id}>
                  <td>
                    <Link
                      to="/servers/$serverId"
                      params={{ serverId: s.server_id }}
                      className="font-medium text-gray-900 hover:underline dark:text-white"
                    >
                      {s.name}
                    </Link>
                    <div className="text-xs text-gray-500">{s.ip}</div>
                    <div className={s.reachable ? 'text-xs text-success-600' : 'text-xs text-warning-500'}>
                      {s.reachable ? 'Reachable' : 'Unreachable'}
                    </div>
                  </td>
                  <td className="text-gray-600 dark:text-gray-300">
                    {s.proxy_type || 'traefik'}
                    <div className="text-xs text-gray-500">{s.proxy_status || 'unknown'}</div>
                  </td>
                  <td>{s.cpu_percent == null ? '—' : `${Math.round(s.cpu_percent)}%`}</td>
                  <td>{pct(s.memory_used_bytes, s.memory_total_bytes)}</td>
                  <td>{pct(s.disk_used_bytes, s.disk_total_bytes)}</td>
                  <td className="text-xs text-gray-500">
                    {s.recorded_at ? new Date(s.recorded_at).toLocaleString() : 'No sample yet'}
                  </td>
                </tr>
              ))}
              {!shown.length ? (
                <tr>
                  <td colSpan={6} className="panel-table-empty">
                    {servers.length ? 'No servers match that search.' : 'No servers on this team.'}
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      ) : null}
    </div>
  )
}
