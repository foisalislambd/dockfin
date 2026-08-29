import { useEffect, useState, type ReactNode } from 'react'
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

const CONTAINER_COLORS = [
  '#0d9488',
  '#2563eb',
  '#d97706',
  '#7c3aed',
  '#db2777',
  '#0891b2',
  '#65a30d',
  '#ea580c',
  '#4f46e5',
  '#c026d3',
  '#0f766e',
  '#b45309',
]

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
  id: string
  name: string
  color: string
  cpu: number
  memUsed: number
  memLabel: string
}

function slicesFromRows(rows: AppContainerMetric[]): ContainerSlice[] {
  const seen = new Map<string, number>()
  return rows.map((c, i) => {
    let name = shortName(c.name)
    const n = (seen.get(name) || 0) + 1
    seen.set(name, n)
    if (n > 1) name = `${name} (${n})`
    const mem = parseMemUsage(c.mem_usage)
    return {
      id: c.name || `${name}-${i}`,
      name,
      color: CONTAINER_COLORS[i % CONTAINER_COLORS.length],
      cpu: parseCPU(c.cpu_percent),
      memUsed: mem.used,
      memLabel: c.mem_usage?.split('/')[0]?.trim() || fmtBytes(mem.used),
    }
  })
}

function memoryShareSlices(slices: ContainerSlice[]): ContainerSlice[] {
  if (slices.length <= 8) return slices
  const ranked = [...slices].sort((a, b) => b.memUsed - a.memUsed)
  const top = ranked.slice(0, 7)
  const rest = ranked.slice(7)
  const memUsed = rest.reduce((a, s) => a + s.memUsed, 0)
  return [
    ...top,
    {
      id: '__others__',
      name: `Others (${rest.length})`,
      color: '#64748b',
      cpu: rest.reduce((a, s) => a + s.cpu, 0),
      memUsed,
      memLabel: fmtBytes(memUsed),
    },
  ]
}

function cpuScale(n: number) {
  if (n <= 10) return 10
  if (n <= 20) return 20
  if (n <= 50) return 50
  if (n <= 100) return 100
  return Math.ceil(n / 50) * 50
}

function cpuTicks(max: number) {
  const step = max <= 10 ? 2 : max <= 20 ? 5 : max <= 50 ? 10 : 25
  const out: number[] = []
  for (let v = 0; v <= max + 1e-6; v += step) out.push(v)
  return out
}

function memScale(n: number) {
  const mib = Math.max(n / 1024 ** 2, 64)
  const steps = [64, 128, 256, 512, 1024, 1536, 2048, 3072, 4096, 8192]
  const top = steps.find((s) => s >= mib) || Math.ceil(mib / 1024) * 1024
  return top * 1024 ** 2
}

function memTicks(max: number) {
  const mib = max / 1024 ** 2
  const step = mib <= 128 ? 32 : mib <= 256 ? 64 : mib <= 512 ? 128 : mib <= 1024 ? 256 : mib <= 2048 ? 512 : 1024
  const out: number[] = []
  for (let v = 0; v <= mib + 1e-6; v += step) out.push(v * 1024 ** 2)
  return out
}

type HistoryPoint = { t: number; values: Record<string, { cpu: number; mem: number }> }

function fmtCompact(n: number) {
  if (!Number.isFinite(n) || n <= 0) return '0'
  if (n < 1024) return String(Math.round(n))
  if (n < 1024 ** 2) return `${Math.round(n / 1024)}K`
  if (n < 1024 ** 3) {
    const m = n / 1024 ** 2
    return `${m >= 10 ? m.toFixed(0) : m.toFixed(1)}M`
  }
  const g = n / 1024 ** 3
  return `${g >= 10 ? g.toFixed(0) : g.toFixed(1)}G`
}

function ChartCard({
  title,
  unit,
  children,
}: {
  title: string
  unit?: string
  children: ReactNode
}) {
  return (
    <div className="rounded-xl border border-gray-100 bg-gray-50/70 p-3.5 dark:border-white/[0.06] dark:bg-white/[0.03]">
      <div className="mb-2 flex items-baseline justify-between gap-2">
        <p className="text-xs font-semibold tracking-tight text-gray-900 dark:text-white">{title}</p>
        {unit ? <p className="text-[11px] text-gray-400 dark:text-gray-500">{unit}</p> : null}
      </div>
      {children}
    </div>
  )
}

function slug(id: string) {
  return id.replace(/[^a-zA-Z0-9_-]/g, '_')
}

function ColumnChart({ slices, metric }: { slices: ContainerSlice[]; metric: 'cpu' | 'mem' }) {
  const w = 380
  const h = 210
  const l = 48
  const r = 16
  const t = 28
  const b = 32
  const innerW = w - l - r
  const innerH = h - t - b
  const values = slices.map((s) => (metric === 'cpu' ? s.cpu : s.memUsed))
  const max = metric === 'cpu' ? cpuScale(Math.max(...values, 0)) : memScale(Math.max(...values, 0))
  const ticks = metric === 'cpu' ? cpuTicks(max) : memTicks(max)
  const n = Math.max(slices.length, 1)
  const slot = innerW / n
  const barW = Math.min(52, Math.max(10, slot * 0.55))
  const fmt = (v: number) => (metric === 'cpu' ? `${v.toFixed(v >= 10 ? 0 : 1)}%` : fmtBytes(v))
  const gid = (id: string) => `col-${metric}-${slug(id)}`
  const showValue = n <= 8

  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="h-[13.5rem] w-full" role="img">
      <defs>
        {slices.map((s) => (
          <linearGradient key={s.id} id={gid(s.id)} x1="0" y1="1" x2="0" y2="0">
            <stop offset="0%" stopColor={s.color} stopOpacity="0.72" />
            <stop offset="100%" stopColor={s.color} stopOpacity="1" />
          </linearGradient>
        ))}
      </defs>
      {ticks.map((val) => {
        const y = t + innerH - (max > 0 ? (val / max) * innerH : 0)
        return (
          <g key={val}>
            <line
              x1={l}
              x2={w - r}
              y1={y}
              y2={y}
              className="stroke-gray-200/90 dark:stroke-white/[0.08]"
              strokeWidth="1"
            />
            <text
              x={l - 8}
              y={y + 4}
              textAnchor="end"
              className="fill-gray-500 text-[11px] tabular-nums dark:fill-gray-400"
            >
              {metric === 'cpu' ? `${val.toFixed(0)}` : fmtCompact(val)}
            </text>
          </g>
        )
      })}
      <line
        x1={l}
        x2={w - r}
        y1={t + innerH}
        y2={t + innerH}
        className="stroke-gray-300 dark:stroke-white/20"
        strokeWidth="1.25"
      />
      {slices.map((s, i) => {
        const v = values[i]
        const bh = max > 0 ? (v / max) * innerH : 0
        const x = l + slot * i + (slot - barW) / 2
        const y = t + innerH - bh
        const tall = bh > 28
        const label = n > 6 ? s.name.slice(0, 6) : s.name.length > 14 ? `${s.name.slice(0, 13)}…` : s.name
        return (
          <g key={s.id}>
            <rect x={x} y={y} width={barW} height={Math.max(bh, v > 0 ? 3 : 0)} rx="6" fill={`url(#${gid(s.id)})`}>
              <title>{`${s.name}: ${fmt(v)}`}</title>
            </rect>
            {showValue ? (
              <text
                x={x + barW / 2}
                y={tall ? y + 16 : Math.max(14, y - 8)}
                textAnchor="middle"
                className={`text-[11px] font-medium tabular-nums ${
                  tall ? 'fill-white' : 'fill-gray-800 dark:fill-gray-100'
                }`}
              >
                {fmt(v)}
              </text>
            ) : null}
            <text
              x={x + barW / 2}
              y={h - 10}
              textAnchor="middle"
              className="fill-gray-600 text-[11px] font-medium dark:fill-gray-300"
            >
              {label}
            </text>
          </g>
        )
      })}
    </svg>
  )
}

function polar(cx: number, cy: number, r: number, deg: number) {
  const rad = ((deg - 90) * Math.PI) / 180
  return [cx + r * Math.cos(rad), cy + r * Math.sin(rad)] as const
}

function donutSlice(cx: number, cy: number, outer: number, inner: number, start: number, end: number) {
  const [ox1, oy1] = polar(cx, cy, outer, start)
  const [ox2, oy2] = polar(cx, cy, outer, end)
  const [ix2, iy2] = polar(cx, cy, inner, end)
  const [ix1, iy1] = polar(cx, cy, inner, start)
  const large = end - start > 180 ? 1 : 0
  return `M ${ox1} ${oy1} A ${outer} ${outer} 0 ${large} 1 ${ox2} ${oy2} L ${ix2} ${iy2} A ${inner} ${inner} 0 ${large} 0 ${ix1} ${iy1} Z`
}

function DonutChart({ slices }: { slices: ContainerSlice[] }) {
  const [hover, setHover] = useState<number | null>(null)
  const [tip, setTip] = useState<{ x: number; y: number } | null>(null)
  const cx = 90
  const cy = 90
  const outer = 72
  const inner = 48
  const values = slices.map((s) => s.memUsed)
  const total = values.reduce((a, v) => a + v, 0)
  const live = values.filter((v) => v > 0).length
  const gap = live > 6 ? 1.6 : live > 1 ? 4 : 0
  const active = hover != null ? slices[hover] : null
  const activeVal = hover != null ? values[hover] : 0
  const activeShare = total > 0 && hover != null ? (activeVal / total) * 100 : 0
  let angle = 0
  return (
    <div
      className="relative flex h-[13.5rem] items-center justify-center overflow-visible"
      onMouseLeave={() => {
        setHover(null)
        setTip(null)
      }}
    >
      <svg viewBox="0 0 180 180" className="h-full max-h-[13.5rem] w-auto" role="img">
        <circle
          cx={cx}
          cy={cy}
          r={(outer + inner) / 2}
          fill="none"
          strokeWidth={outer - inner}
          className="stroke-gray-200 dark:stroke-white/10"
        />
        {slices.map((s, i) => {
          const v = values[i]
          const sweep = total > 0 ? (v / total) * 360 : 0
          const start = angle + gap / 2
          const end = angle + sweep - gap / 2
          angle += sweep
          if (end - start < 0.4) return null
          const on = hover === i
          return (
            <path
              key={s.id}
              d={donutSlice(cx, cy, outer, inner, start, end)}
              fill={s.color}
              opacity={hover == null || on ? 1 : 0.38}
              className="cursor-pointer"
              onMouseEnter={() => setHover(i)}
              onMouseMove={(e) => {
                const box = e.currentTarget.ownerSVGElement?.parentElement
                if (!box) return
                const rect = box.getBoundingClientRect()
                setTip({ x: e.clientX - rect.left, y: e.clientY - rect.top })
              }}
            />
          )
        })}
        <text
          x={cx}
          y={active ? cy - 8 : cy - 2}
          textAnchor="middle"
          className="pointer-events-none fill-gray-900 text-[13px] font-semibold dark:fill-white"
        >
          {active ? (active.name.length > 16 ? `${active.name.slice(0, 15)}…` : active.name) : fmtBytes(total)}
        </text>
        <text
          x={cx}
          y={active ? cy + 10 : cy + 16}
          textAnchor="middle"
          className="pointer-events-none fill-gray-500 text-[11px] tabular-nums dark:fill-gray-400"
        >
          {active ? `${fmtBytes(activeVal)} · ${activeShare.toFixed(0)}%` : 'used'}
        </text>
      </svg>
      {active && tip ? (
        <div
          className="pointer-events-none absolute z-10 max-w-[14rem] rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 shadow-lg dark:border-gray-700 dark:bg-gray-900"
          style={{ left: tip.x + 12, top: tip.y - 8, transform: 'translateY(-100%)' }}
        >
          <p className="text-[12px] font-semibold text-gray-900 dark:text-white">{active.name}</p>
          <p className="text-[11px] tabular-nums text-gray-500 dark:text-gray-400">
            {fmtBytes(activeVal)} · {activeShare.toFixed(0)}%
          </p>
        </div>
      ) : null}
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
  const h = 188
  const l = 48
  const r = 14
  const t = 14
  const b = 18
  const innerW = w - l - r
  const innerH = h - t - b
  const colorOf = Object.fromEntries(slices.map((s) => [s.id, s.color]))
  const series = slices.map((s) =>
    history.map((p) => (metric === 'cpu' ? p.values[s.id]?.cpu ?? 0 : p.values[s.id]?.mem ?? 0)),
  )
  const max = metric === 'cpu' ? cpuScale(Math.max(0, ...series.flat())) : memScale(Math.max(0, ...series.flat()))
  const ticks = metric === 'cpu' ? cpuTicks(max) : memTicks(max)
  const xy = (vals: number[], i: number) => {
    const x = l + (vals.length === 1 ? innerW / 2 : (i / (vals.length - 1)) * innerW)
    const y = t + innerH - (max > 0 ? (vals[i] / max) * innerH : 0)
    return { x, y }
  }
  const linePts = (vals: number[]) =>
    vals.length ? vals.map((_, i) => `${xy(vals, i).x},${xy(vals, i).y}`).join(' ') : ''
  const areaPts = (vals: number[]) => {
    if (!vals.length) return ''
    const first = xy(vals, 0)
    const last = xy(vals, vals.length - 1)
    return `${first.x},${t + innerH} ${linePts(vals)} ${last.x},${t + innerH}`
  }
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="h-44 w-full" role="img">
      {ticks.map((val) => {
        const y = t + innerH - (max > 0 ? (val / max) * innerH : 0)
        return (
          <g key={val}>
            <line
              x1={l}
              x2={w - r}
              y1={y}
              y2={y}
              className="stroke-gray-200/90 dark:stroke-white/[0.08]"
              strokeWidth="1"
            />
            <text
              x={l - 8}
              y={y + 4}
              textAnchor="end"
              className="fill-gray-500 text-[11px] tabular-nums dark:fill-gray-400"
            >
              {metric === 'cpu' ? `${val.toFixed(0)}` : fmtCompact(val)}
            </text>
          </g>
        )
      })}
      <line
        x1={l}
        x2={w - r}
        y1={t + innerH}
        y2={t + innerH}
        className="stroke-gray-300 dark:stroke-white/20"
        strokeWidth="1.25"
      />
      {slices.map((s, i) => (
        <g key={s.id}>
          <polygon points={areaPts(series[i])} fill={colorOf[s.id]} opacity="0.12" />
          <polyline
            fill="none"
            stroke={colorOf[s.id]}
            strokeWidth="2.5"
            strokeLinejoin="round"
            strokeLinecap="round"
            points={linePts(series[i])}
          />
          {series[i].map((_, j) => {
            const { x, y } = xy(series[i], j)
            return (
              <circle
                key={j}
                cx={x}
                cy={y}
                r="3"
                fill={colorOf[s.id]}
                className="stroke-white dark:stroke-gray-950"
                strokeWidth="1.5"
              />
            )
          })}
        </g>
      ))}
    </svg>
  )
}

function ContainerCharts({ rows, history }: { rows: AppContainerMetric[]; history: HistoryPoint[] }) {
  const slices = slicesFromRows(rows)
  const share = memoryShareSlices(slices)
  return (
    <div className="space-y-4 border-t border-gray-200 px-5 py-4 dark:border-gray-800">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <p className="text-xs font-semibold text-gray-900 dark:text-white">By container</p>
        {slices.map((s) => (
          <span key={s.id} className="inline-flex items-center gap-1.5 text-[12px] text-gray-600 dark:text-gray-300">
            <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: s.color }} />
            <span className="font-medium">{s.name}</span>
            <span className="tabular-nums text-gray-400 dark:text-gray-500">
              {s.cpu.toFixed(1)}% · {s.memLabel}
            </span>
          </span>
        ))}
      </div>
      <div className="grid gap-3 lg:grid-cols-3">
        <ChartCard title="CPU" unit="%">
          <ColumnChart slices={slices} metric="cpu" />
        </ChartCard>
        <ChartCard title="Memory" unit="used">
          <ColumnChart slices={slices} metric="mem" />
        </ChartCard>
        <ChartCard title="Memory share">
          <DonutChart slices={share} />
        </ChartCard>
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <ChartCard title="CPU over time" unit="%">
          <LineChart history={history} slices={slices} metric="cpu" />
        </ChartCard>
        <ChartCard title="Memory over time" unit="used">
          <LineChart history={history} slices={slices} metric="mem" />
        </ChartCard>
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
      values[s.id] = { cpu: s.cpu, mem: s.memUsed }
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
