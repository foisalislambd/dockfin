import type { CSSProperties, HTMLAttributes } from 'react'
import { SIDEBAR_WIDTH_EXPANDED } from '../../context/sidebar-context'

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
      className={cx('dockfin-skeleton rounded-md', className)}
      style={style}
      {...rest}
    />
  )
}

/** Compact inline skeleton for a panel / section that is still fetching */
export function PanelSkeleton({
  rows = 4,
  showHeader = true,
}: {
  rows?: number
  showHeader?: boolean
}) {
  return (
    <div className="space-y-4" role="status" aria-label="Loading">
      {showHeader && (
        <div className="space-y-2">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-3 w-48 max-w-full" />
        </div>
      )}
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="space-y-1.5">
          <Skeleton className="h-3 w-20" />
          <Skeleton className="h-9 w-full rounded-md" />
        </div>
      ))}
    </div>
  )
}

/** Table / list row skeleton — use inside panel-card or table body */
export function TableSkeleton({
  rows = 5,
  cols = 3,
}: {
  rows?: number
  cols?: number
}) {
  return (
    <div className="divide-y divide-gray-100 dark:divide-white/5" role="status" aria-label="Loading">
      {Array.from({ length: rows }).map((_, r) => (
        <div key={r} className="flex items-center gap-3 px-4 py-3">
          <Skeleton className="h-8 w-8 shrink-0 rounded-lg" />
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-3.5 w-40 max-w-[70%]" />
            {cols > 1 && <Skeleton className="h-3 w-56 max-w-full" />}
          </div>
          {cols > 2 && <Skeleton className="hidden h-6 w-16 rounded-md sm:block" />}
          {cols > 3 && <Skeleton className="hidden h-6 w-20 rounded-md md:block" />}
        </div>
      ))}
    </div>
  )
}

/** Auth form bones — use inside AuthShell form column (not centered on viewport) */
export function AuthFormSkeleton() {
  return (
    <div className="space-y-4" role="status" aria-label="Loading">
      <Skeleton className="mb-1 h-9 w-9 rounded-lg lg:hidden" />
      <Skeleton className="h-7 w-36 sm:h-8" />
      <div className="mt-2 space-y-4">
        <div className="space-y-1.5">
          <Skeleton className="h-3 w-10" />
          <Skeleton className="h-9 w-full rounded-md" />
        </div>
        <div className="space-y-1.5">
          <Skeleton className="h-3 w-14" />
          <Skeleton className="h-9 w-full rounded-md" />
        </div>
        <Skeleton className="h-9 w-full rounded-md" />
      </div>
      <Skeleton className="mx-auto mt-2 h-4 w-44" />
    </div>
  )
}

/** Full-page auth bootstrap matching login/register two-column layout */
export function AuthSkeleton() {
  return (
    <div
      className="relative grid min-h-dvh w-full bg-white dark:bg-gray-900 lg:grid-cols-2"
      role="status"
      aria-label="Loading"
    >
      <div className="flex min-h-dvh flex-col justify-center px-5 py-10 sm:px-10 lg:px-14 xl:px-16">
        <div className="mx-auto w-full max-w-[400px]">
          <AuthFormSkeleton />
        </div>
      </div>
      <div className="relative hidden min-h-dvh overflow-hidden bg-brand-950 lg:flex lg:flex-col lg:items-center lg:justify-center lg:px-8 lg:py-12">
        <div className="pointer-events-none absolute inset-0">
          <div className="absolute -top-24 -left-24 h-72 w-72 rounded-full bg-brand-500/35 blur-3xl" />
          <div className="absolute -right-16 bottom-0 h-80 w-80 rounded-full bg-brand-600/25 blur-3xl" />
        </div>
        <div className="relative z-10 flex w-full max-w-md flex-col items-center space-y-4">
          <Skeleton className="h-14 w-14 rounded-2xl bg-white/15 sm:h-16 sm:w-16" />
          <Skeleton className="h-7 w-40 bg-white/15" />
          <Skeleton className="h-4 w-64 max-w-full bg-white/10" />
          <div className="mt-4 w-full space-y-2.5">
            <Skeleton className="h-4 w-full bg-white/10" />
            <Skeleton className="h-4 w-5/6 bg-white/10" />
            <Skeleton className="h-4 w-4/5 bg-white/10" />
          </div>
        </div>
      </div>
    </div>
  )
}

/** App chrome while auth resolves — mirrors AppShell (sidebar + header + content) */
export function AppShellSkeleton() {
  return (
    <div className="panel-shell panel-main flex h-dvh w-full overflow-hidden" role="status" aria-label="Loading">
      <div
        className="hidden shrink-0 border-r border-gray-200 dark:border-gray-800 lg:flex lg:flex-col"
        style={{ width: SIDEBAR_WIDTH_EXPANDED }}
      >
        <div className="panel-topbar flex items-center gap-3 px-4">
          <Skeleton className="h-8 w-8 shrink-0 rounded-lg" />
          <div className="min-w-0 flex-1 space-y-1.5">
            <Skeleton className="h-3.5 w-20" />
            <Skeleton className="h-2.5 w-24" />
          </div>
        </div>
        <div className="flex flex-1 flex-col gap-0.5 overflow-hidden px-2.5 py-3">
          {Array.from({ length: 3 }).map((_, g) => (
            <div key={g} className={g > 0 ? 'mt-3' : ''}>
              <Skeleton className="mb-1.5 h-2.5 w-16" />
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="flex items-center gap-2.5 rounded-lg px-2.5 py-2">
                  <Skeleton className="h-[18px] w-[18px] shrink-0 rounded" />
                  <Skeleton className="h-3.5 flex-1" />
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <div className="panel-topbar flex items-center gap-3 px-4 sm:gap-4 sm:px-6">
          <Skeleton className="h-8 w-8 rounded-md lg:hidden" />
          <Skeleton className="hidden h-8 max-w-md flex-1 rounded-md lg:block" />
          <div className="ml-auto flex items-center gap-2 sm:gap-3">
            <Skeleton className="h-8 w-8 rounded-md" />
            <Skeleton className="h-8 w-8 rounded-full" />
          </div>
        </div>
        <div className="min-h-0 flex-1 overflow-hidden px-3 py-4 sm:px-5 sm:py-5 lg:px-6">
          <PageSkeleton cards={2} />
        </div>
      </div>
    </div>
  )
}

/**
 * List / dashboard page skeleton (header + cards).
 * Renders inside AppShell main — do not include shell chrome.
 */
export function PageSkeleton({ cards = 2 }: { cards?: number }) {
  const n = Math.max(1, Math.min(cards, 6))

  return (
    <div className="space-y-6" role="status" aria-label="Loading page">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-2">
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-8 w-52 max-w-full" />
          <Skeleton className="h-4 w-72 max-w-full" />
        </div>
        <Skeleton className="h-8 w-28 rounded-md" />
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: n }).map((_, i) => (
          <div key={i} className="panel-card space-y-3 p-5">
            <div className="flex items-center gap-3">
              <Skeleton className="h-10 w-10 shrink-0 rounded-lg" />
              <div className="min-w-0 flex-1 space-y-2">
                <Skeleton className="h-4 w-36 max-w-full" />
                <Skeleton className="h-3 w-48 max-w-full" />
              </div>
            </div>
            <Skeleton className="h-3 w-full" />
            <Skeleton className="h-3 w-4/5 max-w-lg" />
          </div>
        ))}
      </div>
    </div>
  )
}

/**
 * Detail page with top tabs + optional configuration side nav
 * (Application / Service detail layout).
 */
export function DetailPageSkeleton({ withSideNav = true }: { withSideNav?: boolean }) {
  return (
    <div className="space-y-6" role="status" aria-label="Loading page">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-2">
          <Skeleton className="h-3.5 w-40" />
          <Skeleton className="h-8 w-56 max-w-full" />
          <Skeleton className="h-4 w-44" />
        </div>
        <div className="flex flex-wrap gap-2">
          <Skeleton className="h-8 w-20 rounded-md" />
          <Skeleton className="h-8 w-20 rounded-md" />
          <Skeleton className="h-8 w-24 rounded-md" />
        </div>
      </div>

      <div className="flex flex-wrap gap-1 border-b border-gray-200 pb-px dark:border-gray-800">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="mb-2 h-8 w-24 rounded-md" />
        ))}
      </div>

      {withSideNav ? (
        <div className="flex flex-col gap-6 md:flex-row">
          <aside className="w-full shrink-0 md:w-56">
            <nav className="space-y-0.5">
              {Array.from({ length: 8 }).map((_, i) => (
                <div key={i} className="flex items-center gap-2 rounded-md px-2 py-1.5">
                  <Skeleton className="h-3.5 w-3.5 shrink-0 rounded" />
                  <Skeleton className="h-3.5 flex-1" />
                </div>
              ))}
            </nav>
          </aside>
          <div className="min-w-0 flex-1">
            <div className="panel-card space-y-5 p-5">
              <PanelSkeleton rows={5} />
            </div>
          </div>
        </div>
      ) : (
        <div className="panel-card space-y-5 p-5">
          <div className="grid gap-4 sm:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="space-y-2">
                <Skeleton className="h-3 w-16" />
                <Skeleton className="h-5 w-28" />
              </div>
            ))}
          </div>
          <PanelSkeleton rows={3} showHeader={false} />
        </div>
      )}
    </div>
  )
}

/** Edit / settings form page skeleton */
export function FormPageSkeleton() {
  return (
    <div className="space-y-6" role="status" aria-label="Loading page">
      <div className="space-y-2">
        <Skeleton className="h-3.5 w-28" />
        <Skeleton className="h-8 w-48 max-w-full" />
      </div>
      <div className="panel-card space-y-5 p-5">
        <PanelSkeleton rows={4} />
        <div className="flex gap-2 pt-2">
          <Skeleton className="h-8 w-20 rounded-md" />
          <Skeleton className="h-8 w-20 rounded-md" />
        </div>
      </div>
    </div>
  )
}
