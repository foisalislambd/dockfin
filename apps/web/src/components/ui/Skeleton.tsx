import type { CSSProperties, HTMLAttributes } from 'react'

function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(' ')
}

/** Single shimmer bone */
export function Skeleton({
  className,
  style,
  ...rest
}: HTMLAttributes<HTMLDivElement> & { style?: CSSProperties }) {
  return (
    <div
      aria-hidden
      className={cx('goolify-skeleton rounded-md', className)}
      style={style}
      {...rest}
    />
  )
}

/** Full-viewport auth bootstrap (before AppShell) */
export function AuthSkeleton() {
  return (
    <div className="grid min-h-dvh place-items-center p-6" role="status" aria-label="Loading">
      <div className="w-full max-w-sm space-y-4">
        <div className="flex justify-center">
          <Skeleton className="h-10 w-10 rounded-lg" />
        </div>
        <Skeleton className="mx-auto h-5 w-40" />
        <Skeleton className="h-10 w-full rounded-lg" />
        <Skeleton className="h-10 w-full rounded-lg" />
        <Skeleton className="h-9 w-full rounded-md" />
      </div>
    </div>
  )
}

/**
 * Full page skeleton for the main content area (inside AppShell).
 * Use as early return while the page’s primary data is loading.
 */
export function PageSkeleton({ cards = 3 }: { cards?: number }) {
  return (
    <div className="space-y-6" role="status" aria-label="Loading page">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-3">
          <Skeleton className="h-3 w-28" />
          <Skeleton className="h-8 w-52" />
          <Skeleton className="h-4 w-72 max-w-full" />
        </div>
        <Skeleton className="h-8 w-24 rounded-md" />
      </div>

      <div className="flex flex-wrap gap-2">
        <Skeleton className="h-8 w-28 rounded-md" />
        <Skeleton className="h-8 w-24 rounded-md" />
        <Skeleton className="h-8 w-32 rounded-md" />
        <Skeleton className="h-8 w-20 rounded-md" />
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: Math.min(cards, 3) }).map((_, i) => (
          <div key={`stat-${i}`} className="panel-card space-y-3 p-4">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-7 w-12" />
          </div>
        ))}
      </div>

      {Array.from({ length: cards }).map((_, i) => (
        <div key={`card-${i}`} className="panel-card space-y-4 p-5">
          <div className="flex items-center gap-3">
            <Skeleton className="h-10 w-10 shrink-0 rounded-lg" />
            <div className="min-w-0 flex-1 space-y-2">
              <Skeleton className="h-4 w-40 max-w-full" />
              <Skeleton className="h-3 w-56 max-w-full" />
            </div>
          </div>
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-4/5 max-w-lg" />
          <Skeleton className="h-28 w-full rounded-lg" />
        </div>
      ))}
    </div>
  )
}
