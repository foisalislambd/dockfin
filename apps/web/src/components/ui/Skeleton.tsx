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

/** Full-viewport auth bootstrap */
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

/** Detail / form page chrome */
export function PageSkeleton({ cards = 2 }: { cards?: number }) {
  return (
    <div className="space-y-6" role="status" aria-label="Loading">
      <div className="space-y-2">
        <Skeleton className="h-3 w-40" />
        <Skeleton className="h-7 w-56" />
        <Skeleton className="h-4 w-72 max-w-full" />
      </div>
      <div className="flex flex-wrap gap-2">
        <Skeleton className="h-8 w-24 rounded-md" />
        <Skeleton className="h-8 w-28 rounded-md" />
        <Skeleton className="h-8 w-20 rounded-md" />
      </div>
      {Array.from({ length: cards }).map((_, i) => (
        <div key={i} className="panel-card space-y-3 p-5">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-5/6 max-w-md" />
          <Skeleton className="h-24 w-full rounded-lg" />
        </div>
      ))}
    </div>
  )
}

/** Card grid (projects, databases, services, env resources) */
export function CardGridSkeleton({ count = 6 }: { count?: number }) {
  return (
    <div
      className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3"
      role="status"
      aria-label="Loading"
    >
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="panel-card space-y-3 p-5">
          <div className="flex items-start gap-3">
            <Skeleton className="h-10 w-10 shrink-0 rounded-lg" />
            <div className="min-w-0 flex-1 space-y-2">
              <Skeleton className="h-4 w-2/3 max-w-[10rem]" />
              <Skeleton className="h-3 w-1/2 max-w-[7rem]" />
            </div>
          </div>
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-8 w-20 rounded-md" />
        </div>
      ))}
    </div>
  )
}

/** Compact list rows */
export function ListSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div className="space-y-2" role="status" aria-label="Loading">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="panel-card flex items-center gap-4 p-4">
          <Skeleton className="h-9 w-9 shrink-0 rounded-lg" />
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-4 w-40 max-w-full" />
            <Skeleton className="h-3 w-56 max-w-full" />
          </div>
          <Skeleton className="hidden h-7 w-16 rounded-md sm:block" />
        </div>
      ))}
    </div>
  )
}

/** Table body placeholder */
export function TableSkeleton({ rows = 5, cols = 4 }: { rows?: number; cols?: number }) {
  return (
    <div
      className="overflow-hidden rounded-xl border border-gray-200 dark:border-gray-800"
      role="status"
      aria-label="Loading"
    >
      <div className="border-b border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-800 dark:bg-white/5">
        <div className="flex gap-6">
          {Array.from({ length: cols }).map((_, i) => (
            <Skeleton key={i} className="h-3 w-16" />
          ))}
        </div>
      </div>
      <div className="divide-y divide-gray-200 dark:divide-gray-800">
        {Array.from({ length: rows }).map((_, r) => (
          <div key={r} className="flex gap-6 px-4 py-3.5">
            {Array.from({ length: cols }).map((_, c) => (
              <Skeleton key={c} className={cx('h-3', c === 0 ? 'w-28' : 'w-20')} />
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}

/** Dashboard stat cards */
export function StatsSkeleton({ count = 4 }: { count?: number }) {
  return (
    <div
      className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4"
      role="status"
      aria-label="Loading"
    >
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="panel-card flex items-center justify-between gap-3 p-4">
          <div className="space-y-2">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-6 w-10" />
          </div>
          <Skeleton className="h-8 w-8 rounded-lg" />
        </div>
      ))}
    </div>
  )
}

/** Metrics tiles */
export function MetricsSkeleton() {
  return (
    <div className="grid gap-4 sm:grid-cols-3" role="status" aria-label="Loading metrics">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="panel-card space-y-3 p-4">
          <Skeleton className="h-3 w-14" />
          <Skeleton className="h-7 w-16" />
          <Skeleton className="h-16 w-full rounded-md" />
        </div>
      ))}
    </div>
  )
}

/** One-click template catalog */
export function TemplateGridSkeleton({ count = 12 }: { count?: number }) {
  return (
    <div
      className="grid max-h-[min(40rem,70vh)] gap-2 overflow-hidden sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
      role="status"
      aria-label="Loading templates"
    >
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="flex items-start gap-3 rounded-lg border border-gray-200 p-2.5 dark:border-gray-800">
          <Skeleton className="h-9 w-9 shrink-0 rounded-lg" />
          <div className="min-w-0 flex-1 space-y-2 pt-0.5">
            <Skeleton className="h-3.5 w-24 max-w-full" />
            <Skeleton className="h-2.5 w-12" />
          </div>
        </div>
      ))}
    </div>
  )
}
