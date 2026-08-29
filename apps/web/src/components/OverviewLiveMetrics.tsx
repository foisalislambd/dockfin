import { useQuery } from '@tanstack/react-query'
import { Cpu, HardDrive, MemoryStick } from 'lucide-react'
import { api, type AppContainerMetric } from '../lib/api'
import { gentleRefetchInterval } from '../lib/poll'

const POLL_MS = 30_000
const STALE_MS = 25_000

function parseCPU(s: string) {
  const n = Number.parseFloat(String(s).replace(/%/g, '').trim())
  return Number.isFinite(n) ? n : 0
}

function parseMemPart(part: string) {
  const t = part.trim().replace(/,/g, '')
  const m = t.match(/^([\d.]+)\s*([KMGTPE]i?B)?$/i)
  if (!m) return 0
  const n = Number.parseFloat(m[1])
  const u = (m[2] || 'B').toUpperCase()
  const mul =
    u.startsWith('KI') ? 1024
    : u.startsWith('K') ? 1000
    : u.startsWith('MI') ? 1024 ** 2
    : u.startsWith('M') ? 1000 ** 2
    : u.startsWith('GI') ? 1024 ** 3
    : u.startsWith('G') ? 1000 ** 3
    : u.startsWith('TI') ? 1024 ** 4
    : 1
  return n * mul
}

function parseMemUsage(s: string) {
  const [used = '', limit = ''] = String(s || '').split('/')
  return { used: parseMemPart(used), limit: parseMemPart(limit) }
}

/** Same host RAM shown on every unlimited container — do not add those limits together. */
function sharedHostMemLimit(limits: number[]) {
  if (limits.length < 2) return false
  const maxL = Math.max(...limits)
  const minL = Math.min(...limits)
  return maxL > 1024 ** 3 && (maxL - minL) / maxL < 0.02
}

function fmtBytes(n: number) {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i += 1
  }
  return `${v >= 10 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`
}

function shortName(name: string) {
  return name.replace(/^\/?dockfin-(svc|db)-[a-f0-9]+-/i, '').replace(/-1$/, '') || name
}

function UsageBar({ value, warn = 90 }: { value: number; warn?: number }) {
  const high = value >= warn
  return (
    <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-white/10">
      <div
        className={`h-full rounded-full ${high ? 'bg-amber-500' : 'bg-brand-500'}`}
        style={{ width: `${Math.min(100, Math.max(0, value))}%` }}
      />
    </div>
  )
}

function summarize(rows: AppContainerMetric[]) {
  let cpu = 0
  let memUsed = 0
  let diskR = 0
  let diskW = 0
  const limits: number[] = []
  for (const c of rows) {
    cpu += parseCPU(c.cpu_percent)
    const mem = parseMemUsage(c.mem_usage)
    memUsed += mem.used
    if (mem.limit > 0) limits.push(mem.limit)
    const io = parseMemUsage(c.block_io)
    diskR += io.used
    diskW += io.limit
  }
  const hostish = sharedHostMemLimit(limits)
  const memLimit = hostish ? Math.max(...limits) : limits.reduce((a, b) => a + b, 0)
  const memPct = memLimit > 0 ? (memUsed / memLimit) * 100 : 0
  const memCapped = Boolean(memLimit) && !hostish && memPct >= 5
  const diskIO = rows.length ? `${fmtBytes(diskR)} / ${fmtBytes(diskW)}` : ''
  return { cpu, memUsed, memLimit, memPct, memCapped, diskIO }
}

/** This stack’s containers via docker stats — not the whole VPS. Cached ~25s on the API. */
export function OverviewLiveMetrics({
  kind,
  resourceId,
}: {
  kind: 'application' | 'service'
  resourceId?: string
}) {
  const q = useQuery({
    queryKey: [kind === 'service' ? 'service-metrics' : 'app-metrics', resourceId],
    queryFn: () =>
      kind === 'service' ? api.serviceMetrics(resourceId!) : api.applicationMetrics(resourceId!),
    enabled: Boolean(resourceId),
    staleTime: STALE_MS,
    refetchInterval: gentleRefetchInterval(POLL_MS),
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
  })
  const rows = q.data?.containers || []
  const { cpu, memUsed, memLimit, memPct, memCapped, diskIO } = summarize(rows)

  if (!resourceId) return null

  return (
    <section className="panel-card overflow-hidden">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 px-5 py-3 dark:border-gray-800">
        <div>
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Live usage</h3>
          <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            This stack’s containers only · not the VPS · 30s while this tab is open
          </p>
        </div>
      </div>
      {q.isLoading && !rows.length ? (
        <div className="grid gap-px bg-gray-200 sm:grid-cols-3 dark:bg-gray-800">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-28 animate-pulse bg-white p-5 dark:bg-gray-950" />
          ))}
        </div>
      ) : !rows.length ? (
        <div className="px-5 py-6 text-sm text-gray-500 dark:text-gray-400">
          {q.error
            ? q.error.message
            : 'No container stats yet. Deploy this resource, then wait a few seconds.'}
        </div>
      ) : (
        <>
          <div className="grid sm:grid-cols-3">
            <div className="border-b border-gray-200 p-5 sm:border-b-0 sm:border-r dark:border-gray-800">
              <div className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                <Cpu className="h-3.5 w-3.5" />
                CPU
              </div>
              <p className="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">
                {cpu.toFixed(1)}%
              </p>
              <p className="text-xs text-gray-500 dark:text-gray-400">
                {cpu > 100 ? 'Over 100% means more than one CPU core' : 'of one host CPU'}
              </p>
              <UsageBar value={Math.min(cpu, 100)} />
            </div>
            <div className="border-b border-gray-200 p-5 sm:border-b-0 sm:border-r dark:border-gray-800">
              <div className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                <MemoryStick className="h-3.5 w-3.5" />
                Memory
              </div>
              <p className="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">
                {fmtBytes(memUsed)}
              </p>
              <p className="text-xs text-gray-500 dark:text-gray-400">
                {memCapped ? `${fmtBytes(memLimit)} container limit` : 'No memory cap on this stack'}
              </p>
              <UsageBar value={memCapped ? Math.min(memPct, 100) : 0} />
            </div>
            <div className="p-5">
              <div className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                <HardDrive className="h-3.5 w-3.5" />
                Disk I/O
              </div>
              <p className="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">
                {diskIO || '—'}
              </p>
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Read / write across this stack. Volume size is not in this sample.
              </p>
            </div>
          </div>
          {rows.length > 1 ? (
            <ul className="divide-y divide-gray-200 border-t border-gray-200 dark:divide-gray-800 dark:border-gray-800">
              {rows.map((c) => (
                <li
                  key={c.name}
                  className="flex flex-wrap items-center justify-between gap-2 px-5 py-2 text-xs text-gray-600 dark:text-gray-300"
                >
                  <span className="truncate font-mono">{shortName(c.name)}</span>
                  <span className="tabular-nums">
                    {parseCPU(c.cpu_percent).toFixed(1)}% · {c.mem_usage || '—'}
                  </span>
                </li>
              ))}
            </ul>
          ) : null}
        </>
      )}
    </section>
  )
}
