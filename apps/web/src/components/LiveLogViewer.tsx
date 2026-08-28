import { Box, Clock, Download, RefreshCw } from 'lucide-react'
import { useEffect, useRef, type ReactNode } from 'react'

export type LogStreamStatus = 'connecting' | 'live' | 'ended' | 'error'

const TAIL_OPTIONS = [100, 200, 500, 1000, 2000]

const toolBtn =
  'inline-flex h-8 shrink-0 items-center justify-center gap-1.5 rounded-md border border-gray-200 text-gray-600 transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-800 dark:text-gray-300 dark:hover:bg-white/5'

const field =
  'panel-field h-8 min-w-0 rounded-md px-2 text-xs sm:px-2.5'

/** Compose names look like dockfin-svc-<id8>-n8n-1 */
export function shortContainerLabel(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) return trimmed
  const svc = trimmed.match(/^dockfin-svc-[0-9a-f]{8}-(.+)$/i)
  let label = svc ? svc[1] : trimmed
  label = label.replace(/-\d+$/, '')
  return label || trimmed
}

function statusCopy(status: LogStreamStatus): { label: string; dot: string } {
  if (status === 'live') {
    return { label: 'Live', dot: 'bg-emerald-500' }
  }
  if (status === 'connecting') {
    return { label: 'Connecting', dot: 'animate-pulse bg-brand-500' }
  }
  if (status === 'error') {
    return { label: 'Disconnected', dot: 'bg-error-500' }
  }
  return { label: 'Ended', dot: 'bg-gray-400 dark:bg-gray-500' }
}

type Props = {
  status: LogStreamStatus
  error?: string
  lines: string[]
  containers?: string[]
  container?: string
  onContainerChange?: (value: string) => void
  tail?: number
  onTailChange?: (value: number) => void
  extra?: ReactNode
  timestamps?: { checked: boolean; onChange: (next: boolean) => void }
  onDownload: () => void
  onReconnect: () => void
}

export function LiveLogViewer({
  status,
  error,
  lines,
  containers,
  container,
  onContainerChange,
  tail,
  onTailChange,
  extra,
  timestamps,
  onDownload,
  onReconnect,
}: Props) {
  const preRef = useRef<HTMLPreElement>(null)
  const stickRef = useRef(true)
  const { label, dot } = statusCopy(status)
  const showFilters = Boolean((containers && onContainerChange) || onTailChange || extra || timestamps)

  useEffect(() => {
    const el = preRef.current
    if (!el || !stickRef.current) return
    el.scrollTop = el.scrollHeight
  }, [lines])

  const tailChoices = TAIL_OPTIONS.includes(tail ?? -1)
    ? TAIL_OPTIONS
    : [...TAIL_OPTIONS, tail ?? 200].sort((a, b) => a - b)

  return (
    <div className="-mx-3 overflow-hidden border-y border-gray-200 dark:border-gray-800 sm:mx-0 sm:rounded-xl sm:border">
      <div className="flex flex-col gap-2 bg-white px-3 py-2.5 dark:bg-white/[0.03] sm:flex-row sm:items-center sm:gap-3 sm:px-4">
        <div className="flex items-center justify-between gap-2 sm:contents">
          <div className="flex min-w-0 items-center gap-2">
            <span className={`h-2 w-2 shrink-0 rounded-full ${dot}`} aria-hidden />
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Logs</h2>
            <span className="text-xs text-gray-500 dark:text-gray-400">{label}</span>
          </div>
          <div className="flex shrink-0 items-center gap-1 sm:order-last">
            <button
              type="button"
              onClick={onDownload}
              disabled={!lines.length}
              className={`${toolBtn} w-8 md:w-auto md:px-2.5`}
              title="Download"
              aria-label="Download logs"
            >
              <Download className="h-3.5 w-3.5" aria-hidden />
              <span className="hidden md:inline text-xs font-medium">Download</span>
            </button>
            <button
              type="button"
              onClick={onReconnect}
              className={`${toolBtn} w-8 md:w-auto md:px-2.5`}
              title="Reconnect"
              aria-label="Reconnect"
            >
              <RefreshCw className="h-3.5 w-3.5" aria-hidden />
              <span className="hidden md:inline text-xs font-medium">Reconnect</span>
            </button>
          </div>
        </div>

        {showFilters ? (
          <div className="flex min-w-0 flex-1 items-center gap-2">
            {containers && onContainerChange ? (
              <label className="flex min-w-0 flex-1 items-center gap-1.5 sm:max-w-[16rem]">
                <Box className="hidden h-3.5 w-3.5 shrink-0 text-gray-400 sm:block" aria-hidden />
                <span className="sr-only">Container</span>
                <select
                  value={container}
                  onChange={(e) => onContainerChange(e.target.value)}
                  className={`${field} w-full`}
                  title={container}
                >
                  {containers.map((c) => (
                    <option key={c} value={c}>
                      {shortContainerLabel(c)}
                    </option>
                  ))}
                  {!containers.length && <option value="">No containers</option>}
                </select>
              </label>
            ) : null}
            {onTailChange ? (
              <label className="flex shrink-0 items-center">
                <span className="sr-only">Tail</span>
                <select
                  value={tail}
                  onChange={(e) => onTailChange(Number(e.target.value) || 200)}
                  className={`${field} w-[4.75rem] sm:w-[6.5rem]`}
                  title="Lines to load"
                >
                  {tailChoices.map((n) => (
                    <option key={n} value={n}>
                      {n}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            {timestamps ? (
              <label
                className={`${toolBtn} w-8 cursor-pointer md:w-auto md:px-2.5 ${
                  timestamps.checked ? 'border-brand-500/40 bg-brand-50 text-brand-700 dark:bg-brand-500/10 dark:text-brand-300' : ''
                }`}
                title="Timestamps"
              >
                <input
                  type="checkbox"
                  className="sr-only"
                  checked={timestamps.checked}
                  onChange={(e) => timestamps.onChange(e.target.checked)}
                />
                <Clock className="h-3.5 w-3.5" aria-hidden />
                <span className="hidden md:inline text-xs font-medium">Time</span>
              </label>
            ) : null}
            {extra}
          </div>
        ) : null}
      </div>

      {error && status === 'error' ? (
        <p className="border-t border-gray-200 bg-error-50 px-3 py-2 text-sm text-error-600 dark:border-gray-800 dark:bg-error-500/10 dark:text-error-400 sm:px-4">
          {error}
        </p>
      ) : null}

      <pre
        ref={preRef}
        onScroll={(e) => {
          const el = e.currentTarget
          stickRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48
        }}
        className="log-scrollbar min-w-0 max-h-[min(70dvh,36rem)] min-h-[14rem] overflow-x-hidden overflow-y-auto whitespace-pre-wrap break-all bg-[#0b1220] p-3 font-mono text-[11px] leading-relaxed text-gray-300 sm:min-h-[16rem] sm:p-4 sm:text-xs"
      >
        {lines.length
          ? lines.join('\n')
          : status === 'connecting'
            ? 'Connecting to log stream…'
            : status === 'error'
              ? 'Could not stream logs'
              : 'No log output yet'}
      </pre>
      <div className="flex items-center justify-between border-t border-white/10 bg-[#0b1220] px-3 py-1.5 sm:px-4">
        <span className="text-[10px] tabular-nums tracking-wide text-gray-500 uppercase">
          {lines.length ? `${lines.length.toLocaleString()} lines` : 'No output yet'}
        </span>
        {status === 'live' ? (
          <span className="flex items-center gap-1.5 text-[10px] text-emerald-400/80">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
            Streaming
          </span>
        ) : null}
      </div>
    </div>
  )
}
