import { useEffect, useRef } from 'react'

type Props = {
  lines: string[]
  busy?: boolean
  emptyHint?: string
  className?: string
}

/** Deploy output panel (read-only live logs). */
export function DeployLogPanel({
  lines,
  busy,
  emptyHint = 'Deploy output will appear here…',
  className = '',
}: Props) {
  const ref = useRef<HTMLPreElement>(null)

  useEffect(() => {
    if (ref.current) ref.current.scrollTop = ref.current.scrollHeight
  }, [lines])

  return (
    <div className={`overflow-hidden rounded-xl border border-gray-800 bg-[#0b1220] shadow-inner ${className}`}>
      <div className="flex items-center gap-2 border-b border-white/10 px-3 py-2">
        <span className="h-2.5 w-2.5 rounded-full bg-error-500/80" />
        <span className="h-2.5 w-2.5 rounded-full bg-warning-500/80" />
        <span className="h-2.5 w-2.5 rounded-full bg-success-500/80" />
        <span className="ml-2 text-[11px] font-medium tracking-wide text-gray-400 uppercase">
          Deploy output
        </span>
        {busy && (
          <span className="ml-auto flex items-center gap-1.5 text-[11px] text-brand-300">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-brand-400" />
            Running
          </span>
        )}
      </div>
      <pre
        ref={ref}
        className="log-scrollbar max-h-[min(28rem,55vh)] min-h-[12rem] overflow-auto p-4 font-mono text-[12px] leading-relaxed text-gray-200"
      >
        {lines.length ? lines.join('\n') : emptyHint}
      </pre>
    </div>
  )
}
