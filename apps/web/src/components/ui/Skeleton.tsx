import type { CSSProperties, HTMLAttributes } from 'react'
import { SIDEBAR_WIDTH_EXPANDED } from '../../context/sidebar-context'
import { routeSkeletonKind } from '../../lib/route-skeleton'

function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(' ')
}

function loadingRegion(labelled: boolean, label: string) {
  if (!labelled) return {}
  return { role: 'status' as const, 'aria-label': label }
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
  labelled = true,
}: {
  rows?: number
  showHeader?: boolean
  labelled?: boolean
}) {
  return (
    <div className="space-y-4" {...loadingRegion(labelled, 'Loading')}>
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
  labelled = true,
}: {
  rows?: number
  cols?: number
  labelled?: boolean
}) {
  return (
    <div className="divide-y divide-gray-100 dark:divide-white/5" {...loadingRegion(labelled, 'Loading')}>
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
export function AuthFormSkeleton({ labelled = true }: { labelled?: boolean }) {
  return (
    <div className="space-y-4" {...loadingRegion(labelled, 'Loading')}>
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
          <AuthFormSkeleton labelled={false} />
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
          {[3, 5, 5].map((count, g) => (
            <div key={g} className={g > 0 ? 'mt-3' : ''}>
              <Skeleton className="mb-1.5 h-2.5 w-16" />
              {Array.from({ length: count }).map((_, i) => (
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
          <RoutePageSkeleton
            pathname={typeof window !== 'undefined' ? window.location.pathname : '/'}
            search={typeof window !== 'undefined' ? window.location.search : ''}
            labelled={false}
          />
        </div>
      </div>
    </div>
  )
}

/**
 * List / dashboard page skeleton (header + cards).
 * Renders inside AppShell main — do not include shell chrome.
 */
export function PageSkeleton({
  cards = 2,
  labelled = true,
}: {
  cards?: number
  labelled?: boolean
}) {
  const n = Math.max(1, Math.min(cards, 6))

  return (
    <div className="space-y-6" {...loadingRegion(labelled, 'Loading page')}>
      <div className="flex flex-wrap items-end justify-between gap-3">
        <Skeleton className="h-7 w-40 max-w-full" />
        <Skeleton className="h-8 w-28 rounded-md" />
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {Array.from({ length: n }).map((_, i) => (
          <div
            key={i}
            className="panel-card flex min-h-[5.5rem] items-center gap-4 px-5 py-4"
          >
            <Skeleton className="h-10 w-10 shrink-0 rounded-xl" />
            <div className="min-w-0 flex-1 space-y-2">
              <Skeleton className="h-4 w-36 max-w-full" />
              <Skeleton className="h-3 w-48 max-w-full" />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

/**
 * Application / service detail: back link, title row, top tabs, overview (or settings nav).
 */
export function DetailPageSkeleton({
  withSideNav = false,
  tabs = 6,
  labelled = true,
  metrics = false,
}: {
  withSideNav?: boolean
  tabs?: number
  labelled?: boolean
  /** Application/service Overview live-usage tiles — not DB/server/git. */
  metrics?: boolean
}) {
  return (
    <div className="space-y-5" {...loadingRegion(labelled, 'Loading page')}>
      <div className="inline-flex h-8 items-center gap-1.5">
        <Skeleton className="h-6 w-6 rounded-md" />
        <Skeleton className="h-3.5 w-24" />
      </div>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 items-center gap-3">
          <Skeleton className="h-10 w-10 shrink-0 rounded-xl" />
          <div className="min-w-0 space-y-2">
            <Skeleton className="h-7 w-48 max-w-full" />
            <Skeleton className="h-5 w-24 rounded-full" />
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Skeleton className="h-8 w-16 rounded-md" />
          <Skeleton className="h-8 w-24 rounded-md" />
          <Skeleton className="h-8 w-8 rounded-md" />
        </div>
      </div>

      <div className="flex flex-nowrap gap-1 overflow-x-auto border-b border-gray-200 pb-px dark:border-gray-800">
        {Array.from({ length: tabs }).map((_, i) => (
          <Skeleton key={i} className="mb-2 h-8 w-[4.75rem] shrink-0 rounded-md" />
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
              <PanelSkeleton rows={5} showHeader={false} labelled={false} />
            </div>
          </div>
        </div>
      ) : metrics ? (
        <div className="space-y-5">
          <div className="panel-card space-y-4 p-5">
            <div className="flex items-center justify-between gap-3">
              <Skeleton className="h-4 w-40" />
              <Skeleton className="h-5 w-16 rounded-full" />
            </div>
            <Skeleton className="h-4 w-64 max-w-full" />
          </div>
          <div className="grid sm:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <div
                key={i}
                className="border-b border-gray-200 p-5 last:border-b-0 sm:border-b-0 sm:border-r sm:last:border-r-0 dark:border-gray-800"
              >
                <Skeleton className="h-3 w-16" />
                <Skeleton className="mt-2 h-8 w-24" />
                <Skeleton className="mt-3 h-1.5 w-full rounded-full" />
              </div>
            ))}
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
          <PanelSkeleton rows={3} showHeader={false} labelled={false} />
        </div>
      )}
    </div>
  )
}

/** Environment resources grid (project details). */
export function EnvResourcesSkeleton({ labelled = true }: { labelled?: boolean }) {
  return (
    <div className="space-y-8" {...loadingRegion(labelled, 'Loading page')}>
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <Skeleton className="h-3.5 w-16" />
          <Skeleton className="h-3 w-2" />
          <Skeleton className="h-3.5 w-28" />
          <Skeleton className="h-3 w-2" />
          <Skeleton className="h-3.5 w-20" />
        </div>
        <div className="flex flex-wrap items-end justify-between gap-3">
          <Skeleton className="h-7 w-44 max-w-full" />
          <div className="flex gap-2">
            <Skeleton className="h-8 w-20 rounded-md" />
            <Skeleton className="h-8 w-16 rounded-md" />
            <Skeleton className="h-8 w-16 rounded-md" />
          </div>
        </div>
      </div>
      <Skeleton className="h-9 max-w-xl rounded-md" />
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="panel-card flex min-h-[8rem] flex-col justify-between gap-3 p-4">
            <div className="flex items-start gap-3">
              <Skeleton className="h-9 w-9 shrink-0 rounded-lg" />
              <div className="min-w-0 flex-1 space-y-2">
                <Skeleton className="h-4 w-32" />
                <Skeleton className="h-4 w-16 rounded-md" />
              </div>
            </div>
            <Skeleton className="h-3 w-40" />
            <Skeleton className="h-4 w-14" />
          </div>
        ))}
      </div>
    </div>
  )
}

/** Dashboard welcome + stat tiles. */
export function DashboardSkeleton({ labelled = true }: { labelled?: boolean }) {
  return (
    <div className="space-y-6" {...loadingRegion(labelled, 'Loading page')}>
      <Skeleton className="h-7 w-56 max-w-full" />
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="panel-card p-4">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="mt-2 h-6 w-12" />
          </div>
        ))}
      </div>
      <div className="panel-card space-y-3 p-5">
        <Skeleton className="h-4 w-32" />
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-10 w-full rounded-md" />
        ))}
      </div>
    </div>
  )
}

/** New-resource picker: search + choice cards. */
export function ChoiceGridSkeleton({ labelled = true }: { labelled?: boolean }) {
  return (
    <div className="space-y-6" {...loadingRegion(labelled, 'Loading page')}>
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <Skeleton className="h-3.5 w-16" />
          <Skeleton className="h-3.5 w-24" />
        </div>
        <Skeleton className="h-7 w-40" />
      </div>
      <Skeleton className="h-9 w-full max-w-xl rounded-md" />
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="panel-card flex min-h-[5.5rem] items-center gap-4 px-5 py-4">
            <Skeleton className="h-10 w-10 shrink-0 rounded-xl" />
            <div className="min-w-0 flex-1 space-y-2">
              <Skeleton className="h-4 w-36 max-w-full" />
              <Skeleton className="h-3 w-48 max-w-full" />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

/** Deployment log / single-panel detail (no app Overview). */
export function SimpleDetailSkeleton({ labelled = true }: { labelled?: boolean }) {
  return (
    <div className="space-y-5" {...loadingRegion(labelled, 'Loading page')}>
      <div className="inline-flex h-8 items-center gap-1.5">
        <Skeleton className="h-6 w-6 rounded-md" />
        <Skeleton className="h-3.5 w-28" />
      </div>
      <Skeleton className="h-7 w-56 max-w-full" />
      <div className="panel-card space-y-3 p-5">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-40 w-full rounded-md" />
      </div>
    </div>
  )
}

/** Edit / settings form page skeleton */
export function FormPageSkeleton() {
  return (
    <div className="space-y-6" role="status" aria-label="Loading page">
      <Skeleton className="h-7 w-48 max-w-full" />
      <div className="panel-card space-y-5 p-5">
        <PanelSkeleton rows={4} showHeader={false} labelled={false} />
        <div className="flex gap-2 pt-2">
          <Skeleton className="h-8 w-20 rounded-md" />
          <Skeleton className="h-8 w-20 rounded-md" />
        </div>
      </div>
    </div>
  )
}

/** Notifications / settings-style page: header, tabs, then cards */
export function TabbedPageSkeleton({ tabs = 6 }: { tabs?: number }) {
  return (
    <div className="space-y-6" role="status" aria-label="Loading page">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <Skeleton className="h-7 w-40" />
      </div>
      <div className="flex flex-wrap gap-1 border-b border-gray-200 pb-px dark:border-gray-800">
        {Array.from({ length: tabs }).map((_, i) => (
          <Skeleton key={i} className="mb-2 h-8 w-[4.5rem] rounded-md" />
        ))}
      </div>
      <div className="panel-card flex flex-wrap items-center gap-3 px-5 py-4">
        <Skeleton className="h-10 w-10 shrink-0 rounded-xl" />
        <div className="min-w-0 flex-1 space-y-2">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-3 w-36" />
        </div>
        <Skeleton className="h-6 w-11 rounded-full" />
        <Skeleton className="h-8 w-24 rounded-md" />
        <Skeleton className="h-8 w-16 rounded-md" />
      </div>
      <div className="panel-card space-y-4 p-5">
        <Skeleton className="h-4 w-24" />
        <PanelSkeleton rows={3} showHeader={false} labelled={false} />
      </div>
      <div className="panel-card p-5">
        <div className="mb-3 flex justify-between">
          <Skeleton className="h-4 w-16" />
          <Skeleton className="h-3 w-20" />
        </div>
        <div className="grid gap-6 xl:grid-cols-2">
          <PanelSkeleton rows={4} showHeader={false} labelled={false} />
          <PanelSkeleton rows={4} showHeader={false} labelled={false} />
        </div>
      </div>
    </div>
  )
}

export function RoutePageSkeleton({
  pathname,
  search,
  labelled = true,
}: {
  pathname: string
  search?: unknown
  labelled?: boolean
}) {
  const kind = routeSkeletonKind(pathname, search)
  switch (kind) {
    case 'form':
      return <FormPageSkeleton />
    case 'choice':
      return <ChoiceGridSkeleton labelled={labelled} />
    case 'app':
      return <DetailPageSkeleton withSideNav={false} tabs={6} metrics labelled={labelled} />
    case 'app-settings':
      return <DetailPageSkeleton withSideNav tabs={6} labelled={labelled} />
    case 'service':
      return <DetailPageSkeleton withSideNav={false} tabs={5} metrics labelled={labelled} />
    case 'service-settings':
      return <DetailPageSkeleton withSideNav tabs={5} labelled={labelled} />
    case 'resource':
      return <DetailPageSkeleton withSideNav={false} tabs={6} labelled={labelled} />
    case 'simple':
      return <SimpleDetailSkeleton labelled={labelled} />
    case 'env':
      return <EnvResourcesSkeleton labelled={labelled} />
    case 'project':
      return <PageSkeleton cards={1} labelled={labelled} />
    case 'dashboard':
      return <DashboardSkeleton labelled={labelled} />
    case 'tabbed':
      return <TabbedPageSkeleton />
    case 'list-3':
      return <PageSkeleton cards={3} labelled={labelled} />
    case 'list-1':
      return <PageSkeleton cards={1} labelled={labelled} />
    default:
      return <PageSkeleton cards={2} labelled={labelled} />
  }
}
