import { useEffect, useState } from 'react'
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

const CONTAINER_COLORS = ['#0d9488', '#2563eb', '#d97706', '#7c3aed', '#db2777', '#0891b2']

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

type ContainerSlice = {
  name: string
  color: string
  cpu: number
  memUsed: number
  memLabel: string
}

function slicesFromRows(rows: AppContainerMetric[]): ContainerSlice[] {
  return rows.map((c, i) => {
    const mem = parseMemUsage(c.mem_usage)
    return {
      name: shortName(c.name),
      color: CONTAINER_COLORS[i % CONTAINER_COLORS.length],
      cpu: parseCPU(c.cpu_percent),
      memUsed: mem.used,
      memLabel: c.mem_usage?.split('/')[0]?.trim() || fmtBytes(mem.used),
    }
  })
}

function niceMax(n: number, floor: number) {
  const v = Math.max(n, floor)
  const exp = 10 ** Math.floor(Math.log10(v))
  const f = v / exp
  const nice = f <= 1 ? 1 : f <= 2 ? 2 : f <= 5 ? 5 : 10
  return nice * exp
}

type HistoryPoint = { t: number; values: Record<string, { cpu: number; mem: number }> }

function ColumnChart({
  slices,
  metric,
}: {
  slices: ContainerSlice[]
  metric: 'cpu' | 'mem'
}) {
  const w = 360
  const h = 200
  const l = 42
  const r = 12
  const t = 22
  const b = 36
  const innerW = w - l - r
  const innerH = h - t - b
  const values = slices.map((s) => (metric === 'cpu' ? s.cpu : s.memUsed))
  const max = niceMax(Math.max(...values, 0), metric === 'cpu' ? 10 : 1024 ** 2)
  const ticks = 4
  const n = Math.max(slices.length, 1)
  const slot = innerW / n
  const barW = Math.min(46, slot * 0.55)
  const fmt = (v: number) => (metric === 'cpu' ? `${v.toFixed(v >= 10 ? 0 : 1)}%` : fmtBytes(v))

  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="h-48 w-full" role="img">
      {Array.from({ length: ticks + 1 }, (_, i) => {
        const frac = i / ticks
        const y = t + innerH - frac * innerH
        const val = max * frac
        return (
          <g key={i}>
            <line x1={l} x2={w - r} y1={y} y2={y} className="stroke-gray-100 dark:stroke-white/10" strokeWidth="1" />
            <text
              x={l - 6}
              y={y + 3}
              textAnchor="end"
              className="fill-gray-400 text-[10px] tabular-nums dark:fill-gray-500"
            >
              {metric === 'cpu' ? `${val.toFixed(0)}` : fmtBytes(val)}
            </text>
          </g>
        )
      })}
      {slices.map((s, i) => {
        const v = values[i]
        const bh = max > 0 ? (v / max) * innerH : 0
        const x = l + slot * i + (slot - barW) / 2
        const y = t + innerH - bh
        return (
          <g key={s.name}>
            <rect
              x={x}
              y={y}
              width={barW}
              height={Math.max(bh, v > 0 ? 2 : 0)}
              rx="4"
              fill={s.color}
            >
              <title>{`${s.name}: ${fmt(v)}`}</title>
            </rect>
            <text
              x={x + barW / 2}
              y={h - 14}
              textAnchor="middle"
              className="fill-gray-500 font-mono text-[10px] dark:fill-gray-400"
            >
              {s.name.length > 12 ? `${s.name.slice(0, 11)}…` : s.name}
            </text>
            <text
              x={x + barW / 2}
              y={Math.max(12, y - 6)}
              textAnchor="middle"
              className="fill-gray-700 text-[10px] tabular-nums dark:fill-gray-200"
            >
              {fmt(v)}
            </text>
          </g>
        )
      })}
    </svg>
  )
}

function DonutChart({ slices, metric }: { slices: ContainerSlice[]; metric: 'cpu' | 'mem' }) {
  const cx = 88
  const cy = 88
  const r = 54
  const sw = 22
  const c = 2 * Math.PI * r
  const values = slices.map((s) => (metric === 'cpu' ? s.cpu : s.memUsed))
  const total = values.reduce((a, v) => a + v, 0)
  let offset = 0
  return (
    <div className="flex items-center gap-4">
      <svg viewBox="0 0 176 176" className="h-40 w-40 shrink-0" role="img">
        <circle
          cx={cx}
          cy={cy}
          r={r}
          fill="none"
          strokeWidth={sw}
          className="stroke-gray-100 dark:stroke-white/10"
        />
        {slices.map((s, i) => {
          const v = values[i]
          const len = total > 0 ? (v / total) * c : 0
          const el = (
            <circle
              key={s.name}
              cx={cx}
              cy={cy}
              r={r}
              fill="none"
              stroke={s.color}
              strokeWidth={sw}
              strokeDasharray={`${len} ${c - len}`}
              strokeDashoffset={-offset}
              transform={`rotate(-90 ${cx} ${cy})`}
            >
              <title>{`${s.name}: ${metric === 'cpu' ? `${v.toFixed(1)}%` : fmtBytes(v)}`}</title>
            </circle>
          )
          offset += len
          return el
        })}
        <text
          x={cx}
          y={cy - 4}
          textAnchor="middle"
          className="fill-gray-900 text-sm font-semibold tabular-nums dark:fill-white"
        >
          {metric === 'cpu' ? `${total.toFixed(1)}%` : fmtBytes(total)}
        </text>
        <text x={cx} y={cy + 14} textAnchor="middle" className="fill-gray-400 text-[10px] dark:fill-gray-500">
          {metric === 'cpu' ? 'CPU total' : 'RAM used'}
        </text>
      </svg>
      <ul className="min-w-0 space-y-2">
        {slices.map((s, i) => {
          const v = values[i]
          const share = total > 0 ? (v / total) * 100 : 0
          return (
            <li key={s.name} className="flex items-center gap-2 text-xs">
              <span className="h-2.5 w-2.5 shrink-0 rounded-sm" style={{ backgroundColor: s.color }} />
              <span className="truncate font-mono text-gray-700 dark:text-gray-200">{s.name}</span>
              <span className="ml-auto tabular-nums text-gray-500 dark:text-gray-400">
                {share.toFixed(0)}%
              </span>
            </li>
          )
        })}
      </ul>
    </div>
  )
}

function LineChart({
  history,
  slices,
  metric,
}: {
  history: HistoryPoint[]
  slices: ContainerSlice[]
  metric: 'cpu' | 'mem'
}) {
  const w = 520
  const h = 180
  const l = 42
  const r = 12
  const t = 12
  const b = 24
  const innerW = w - l - r
  const innerH = h - t - b
  const names = slices.map((s) => s.name)
  const colorOf = Object.fromEntries(slices.map((s) => [s.name, s.color]))
  const series = names.map((name) =>
    history.map((p) => (metric === 'cpu' ? p.values[name]?.cpu ?? 0 : p.values[name]?.mem ?? 0)),
  )
  const max = niceMax(Math.max(0, ...series.flat()), metric === 'cpu' ? 10 : 1024 ** 2)
  const pts = (vals: number[]) => {
    if (!vals.length) return ''
    return vals
      .map((v, i) => {
        const x = l + (vals.length === 1 ? innerW / 2 : (i / (vals.length - 1)) * innerW)
        const y = t + innerH - (max > 0 ? (v / max) * innerH : 0)
        return `${x},${y}`
      })
      .join(' ')
  }
  const ticks = 4
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="h-44 w-full" role="img">
      {Array.from({ length: ticks + 1 }, (_, i) => {
        const frac = i / ticks
        const y = t + innerH - frac * innerH
        const val = max * frac
        return (
          <g key={i}>
            <line x1={l} x2={w - r} y1={y} y2={y} className="stroke-gray-100 dark:stroke-white/10" strokeWidth="1" />
            <text
              x={l - 6}
              y={y + 3}
              textAnchor="end"
              className="fill-gray-400 text-[10px] tabular-nums dark:fill-gray-500"
            >
              {metric === 'cpu' ? `${val.toFixed(0)}` : fmtBytes(val)}
            </text>
          </g>
        )
      })}
      {names.map((name, i) => (
        <g key={name}>
          <polyline
            fill="none"
            stroke={colorOf[name]}
            strokeWidth="2.25"
            strokeLinejoin="round"
            strokeLinecap="round"
            points={pts(series[i])}
          />
          {series[i].map((v, j) => {
            const x =
              l + (series[i].length === 1 ? innerW / 2 : (j / (series[i].length - 1)) * innerW)
            const y = t + innerH - (max > 0 ? (v / max) * innerH : 0)
            return <circle key={j} cx={x} cy={y} r="2.5" fill={colorOf[name]} />
          })}
        </g>
      ))}
      <text x={l} y={h - 6} className="fill-gray-400 text-[10px] dark:fill-gray-500">
        {history.length < 2 ? 'More points appear every 30s while this tab stays open' : 'Last samples on this tab'}
      </text>
    </svg>
  )
}

function ContainerCharts({ rows, history }: { rows: AppContainerMetric[]; history: HistoryPoint[] }) {
  const slices = slicesFromRows(rows)
  return (
    <div className="space-y-5 border-t border-gray-200 px-5 py-4 dark:border-gray-800">
      <div>
        <p className="text-xs font-medium text-gray-700 dark:text-gray-200">Containers on one chart</p>
        <p className="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400">
          Column + donut for this sample · line chart fills in as stats refresh
        </p>
      </div>
      <div className="flex flex-wrap gap-x-4 gap-y-1">
        {slices.map((s) => (
          <span key={s.name} className="inline-flex items-center gap-1.5 text-[11px] text-gray-600 dark:text-gray-300">
            <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: s.color }} />
            <span className="font-mono">{s.name}</span>
            <span className="tabular-nums text-gray-400 dark:text-gray-500">
              {s.cpu.toFixed(1)}% · {s.memLabel}
            </span>
          </span>
        ))}
      </div>
      <div className="grid gap-6 lg:grid-cols-3">
        <div>
          <p className="mb-1 text-[11px] font-medium text-gray-500 dark:text-gray-400">CPU % · column</p>
          <ColumnChart slices={slices} metric="cpu" />
        </div>
        <div>
          <p className="mb-1 text-[11px] font-medium text-gray-500 dark:text-gray-400">Memory · column</p>
          <ColumnChart slices={slices} metric="mem" />
        </div>
        <div>
          <p className="mb-1 text-[11px] font-medium text-gray-500 dark:text-gray-400">Memory share · donut</p>
          <DonutChart slices={slices} metric="mem" />
        </div>
      </div>
      <div className="grid gap-6 sm:grid-cols-2">
        <div>
          <p className="mb-1 text-[11px] font-medium text-gray-500 dark:text-gray-400">CPU over time · line</p>
          <LineChart history={history} slices={slices} metric="cpu" />
        </div>
        <div>
          <p className="mb-1 text-[11px] font-medium text-gray-500 dark:text-gray-400">Memory over time · line</p>
          <LineChart history={history} slices={slices} metric="mem" />
        </div>
      </div>
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
  const [history, setHistory] = useState<HistoryPoint[]>([])

  useEffect(() => {
    setHistory([])
  }, [resourceId])

  useEffect(() => {
    if (!rows.length) return
    const values: HistoryPoint['values'] = {}
    for (const s of slicesFromRows(rows)) {
      values[s.name] = { cpu: s.cpu, mem: s.memUsed }
    }
    setHistory((prev) => [...prev, { t: Date.now(), values }].slice(-40))
  }, [q.dataUpdatedAt])

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
          {rows.length > 1 ? <ContainerCharts rows={rows} history={history} /> : null}
        </>
      )}
    </section>
  )
}
