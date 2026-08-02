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
      className={cx('dockfin-skeleton rounded-md', className)}
      style={style}
      {...rest}
    />
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
          <Skeleton className="h-4 w-72 max-w-full bg-white/10" />
          <Skeleton className="h-4 w-64 max-w-full bg-white/10" />
          <div className="mt-4 w-full space-y-3">
            <Skeleton className="h-5 w-full bg-white/10" />
            <Skeleton className="h-5 w-5/6 bg-white/10" />
            <Skeleton className="h-5 w-4/5 bg-white/10" />
          </div>
        </div>
      </div>
    </div>
  )
}

/** App chrome bootstrap while auth resolves on protected routes (not login-shaped) */
export function AppShellSkeleton() {
  return (
    <div className="panel-shell panel-main flex h-dvh w-full overflow-hidden" role="status" aria-label="Loading">
      <div className="hidden w-[232px] shrink-0 border-r border-gray-200 dark:border-white/10 lg:block">
        <div className="space-y-3 p-4">
          <Skeleton className="h-8 w-28" />
          <Skeleton className="mt-6 h-4 w-20" />
          <Skeleton className="h-8 w-full rounded-md" />
          <Skeleton className="h-8 w-full rounded-md" />
          <Skeleton className="h-8 w-full rounded-md" />
          <Skeleton className="h-8 w-full rounded-md" />
          <Skeleton className="h-8 w-full rounded-md" />
        </div>
      </div>
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <div className="flex h-14 shrink-0 items-center justify-between border-b border-gray-200 px-4 dark:border-white/10">
          <Skeleton className="h-6 w-32" />
          <Skeleton className="h-8 w-8 rounded-full" />
        </div>
        <div className="min-h-0 flex-1 overflow-hidden px-3 py-4 sm:px-5 sm:py-5 lg:px-6">
          <PageSkeleton cards={2} />
        </div>
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
