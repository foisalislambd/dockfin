import { useQuery } from '@tanstack/react-query'
import { Link, useRouterState } from '@tanstack/react-router'
import { ChevronLeft, ChevronRight, X } from 'lucide-react'
import { useEffect } from 'react'
import { BrandLogo } from '../BrandLogo'
import { appConfig, isNavActive, navGroups } from '../../config/app.config'
import {
  SIDEBAR_WIDTH_COLLAPSED,
  SIDEBAR_WIDTH_EXPANDED,
  useSidebar,
} from '../../context/sidebar-context'
import { api } from '../../lib/api'

/**
 * Sectioned sidebar. Active state uses a brand accent bar (Dockfin blue), not Antirion's accent.
 * Interaction (sections, counts, collapsed icons, version footer) follows the Antirion console.
 */
export function PanelSidebar() {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const { isExpanded, isMobileOpen, isDesktop, toggleSidebar, closeMobileSidebar } = useSidebar()
  const version = useQuery({ queryKey: ['version'], queryFn: api.version, staleTime: Infinity })
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers, staleTime: 30_000 })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects, staleTime: 30_000 })

  useEffect(() => {
    closeMobileSidebar()
  }, [pathname, closeMobileSidebar])

  const showLabels = !isDesktop || isExpanded || isMobileOpen
  const desktopWidth = isExpanded ? SIDEBAR_WIDTH_EXPANDED : SIDEBAR_WIDTH_COLLAPSED
  const mobileClosed = !isDesktop && !isMobileOpen
  const counts: Record<string, number | undefined> = {
    '/servers': servers.data?.servers?.length,
    '/projects': projects.data?.projects?.length,
  }

  return (
    <aside
      style={
        isDesktop
          ? { width: desktopWidth }
          : { width: Math.min(320, SIDEBAR_WIDTH_EXPANDED), top: '3.25rem', height: 'calc(100dvh - 3.25rem)' }
      }
      className={`z-40 flex shrink-0 flex-col border-r border-gray-200 bg-white transition-[width,transform] duration-300 ease-in-out dark:border-gray-800 dark:bg-gray-900 ${
        isDesktop
          ? 'relative h-full'
          : `fixed left-0 ${isMobileOpen ? 'translate-x-0' : '-translate-x-full'}`
      }`}
      aria-label="Main navigation"
      aria-hidden={mobileClosed || undefined}
      inert={mobileClosed || undefined}
    >
      {!isDesktop && (
        <div className="flex items-center justify-between px-3 py-2 lg:hidden">
          <span className="text-xs font-semibold text-gray-500">Menu</span>
          <button
            type="button"
            onClick={closeMobileSidebar}
            className="flex h-8 w-8 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 dark:hover:bg-white/10"
            aria-label="Close menu"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      )}

      <nav className="no-scrollbar flex flex-1 flex-col overflow-y-auto px-2 py-3">
        {navGroups.map((group, gi) => (
          <div key={group.id} className={gi > 0 ? 'mt-4' : ''}>
            {showLabels ? (
              <p className="px-2.5 pb-1.5 text-[11px] font-medium tracking-wide text-gray-400 uppercase dark:text-gray-500">
                {group.label}
              </p>
            ) : (
              gi > 0 && <div className="mx-2 mb-2 border-t border-gray-200 dark:border-gray-800" aria-hidden />
            )}
            <div className="flex flex-col gap-px">
              {group.items.map((item) => {
                const active = isNavActive(pathname, item.href)
                const Icon = item.icon
                const count = counts[item.href]
                const tip = item.shortcut ? `${item.name} (g ${item.shortcut})` : item.name
                return (
                  <Link
                    key={item.href}
                    to={item.href}
                    onClick={closeMobileSidebar}
                    title={!showLabels ? tip : undefined}
                    aria-current={active ? 'page' : undefined}
                    className={`relative flex h-8 items-center gap-2.5 rounded-md px-2.5 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500/40 ${
                      active
                        ? 'bg-brand-50 font-medium text-gray-900 dark:bg-brand-500/15 dark:text-white'
                        : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-white/8'
                    } ${!showLabels ? 'justify-center px-0' : ''}`}
                  >
                    {active && (
                      <span
                        aria-hidden
                        className="absolute top-1.5 bottom-1.5 -left-2 w-0.5 rounded-full bg-brand-500"
                      />
                    )}
                    <Icon
                      className={`h-4 w-4 shrink-0 ${active ? 'text-brand-600 dark:text-brand-300' : 'text-gray-500 dark:text-gray-400'}`}
                      strokeWidth={1.75}
                    />
                    {showLabels ? (
                      <>
                        <span className="truncate">{item.name}</span>
                        {typeof count === 'number' && count > 0 ? (
                          <span className="ml-auto font-mono text-[11px] text-gray-400 tabular-nums">{count}</span>
                        ) : null}
                      </>
                    ) : (
                      <span className="sr-only">{item.name}</span>
                    )}
                  </Link>
                )
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="shrink-0 border-t border-gray-200 p-2.5 dark:border-gray-800">
        {showLabels && (
          <div className="mb-1 flex items-center justify-between px-2.5 text-[11px] text-gray-400">
            <span>Version</span>
            <span className="font-mono text-gray-500 dark:text-gray-300">
              {version.data?.version || '—'}
            </span>
          </div>
        )}
        {isDesktop && (
          <button
            type="button"
            onClick={toggleSidebar}
            className={`flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-xs font-medium text-gray-500 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-white/5 ${!showLabels ? 'justify-center' : ''}`}
            aria-label={isExpanded ? 'Collapse sidebar' : 'Expand sidebar'}
          >
            {isExpanded ? (
              <>
                <ChevronLeft className="h-4 w-4 shrink-0" />
                <span>Collapse</span>
              </>
            ) : (
              <ChevronRight className="h-4 w-4" />
            )}
          </button>
        )}
        {showLabels && (
          <p className="px-2.5 pt-1 text-[10px] text-gray-400">{appConfig.brand.license} license</p>
        )}
        {!showLabels && (
          <div className="flex justify-center py-1" title={appConfig.brand.name}>
            <BrandLogo className="h-6 w-6 rounded-md opacity-80" />
          </div>
        )}
      </div>
    </aside>
  )
}
