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
  return (
    <div>
      <Header title="Fleet" subtitle="Latest CPU, memory, and disk sample from each server." />
      {fleet.isLoading ? <PageSkeleton cards={2} /> : null}
      {fleet.error ? <p className="text-sm text-error-500">{fleet.error.message}</p> : null}
      <div className="panel-card overflow-hidden">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-gray-200 text-xs text-gray-500 dark:border-gray-800">
            <tr>
              <th className="px-4 py-3 font-medium">Server</th>
              <th className="px-4 py-3 font-medium">Proxy</th>
              <th className="px-4 py-3 font-medium">CPU</th>
              <th className="px-4 py-3 font-medium">Memory</th>
              <th className="px-4 py-3 font-medium">Disk</th>
              <th className="px-4 py-3 font-medium">Sampled</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200 dark:divide-gray-800">
            {(fleet.data?.servers || []).map((s) => (
              <tr key={s.server_id}>
                <td className="px-4 py-3">
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
                <td className="px-4 py-3 text-gray-600 dark:text-gray-300">
                  {s.proxy_type || 'traefik'}
                  <div className="text-xs text-gray-500">{s.proxy_status || 'unknown'}</div>
                </td>
                <td className="px-4 py-3">{s.cpu_percent == null ? '—' : `${Math.round(s.cpu_percent)}%`}</td>
                <td className="px-4 py-3">{pct(s.memory_used_bytes, s.memory_total_bytes)}</td>
                <td className="px-4 py-3">{pct(s.disk_used_bytes, s.disk_total_bytes)}</td>
                <td className="px-4 py-3 text-xs text-gray-500">
                  {s.recorded_at ? new Date(s.recorded_at).toLocaleString() : 'No sample yet'}
                </td>
              </tr>
            ))}
            {!fleet.isLoading && !(fleet.data?.servers || []).length ? (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-500">
                  No servers on this team.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  )
}
