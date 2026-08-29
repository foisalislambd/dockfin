export function statusTone(status: string) {
  const s = (status || '').toLowerCase().replace(/[_-]+/g, ' ')
  // More specific phrases first: "not running" contains "running", "unhealthy" contains "healthy".
  if (
    s.includes('unhealthy') ||
    s.includes('not running') ||
    s.includes('exit') ||
    s.includes('stop') ||
    s.includes('fail') ||
    s.includes('error')
  ) {
    return 'bad' as const
  }
  if (s.includes('running') || s.includes('healthy')) return 'ok' as const
  if (s.includes('deploy') || s.includes('queue') || s.includes('progress') || s.includes('start')) {
    return 'warn' as const
  }
  return 'muted' as const
}

const TONE_PILL: Record<ReturnType<typeof statusTone>, string> = {
  ok: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
  warn: 'bg-amber-500/15 text-amber-800 dark:text-amber-300',
  bad: 'bg-error-500/15 text-error-600 dark:text-error-400',
  muted: 'bg-gray-100 text-gray-600 dark:bg-white/10 dark:text-gray-400',
}

const TONE_DOT: Record<ReturnType<typeof statusTone>, string> = {
  ok: 'bg-emerald-500',
  warn: 'bg-amber-500',
  bad: 'bg-error-500',
  muted: 'bg-gray-400',
}

export function StatusBadge({ status, className = '' }: { status: string; className?: string }) {
  const tone = statusTone(status)
  const label = status?.trim() ? status.replace(/_/g, ' ') : 'Unknown'
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium capitalize ${TONE_PILL[tone]} ${className}`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${TONE_DOT[tone]} ${tone === 'warn' ? 'animate-pulse' : ''}`} />
      {label}
    </span>
  )
}
