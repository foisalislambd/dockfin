import { BrandLogo } from '../BrandLogo'
import { appConfig, isNavActive, navGroups } from '../../config/app.config'
import {
  SIDEBAR_WIDTH_COLLAPSED,
  SIDEBAR_WIDTH_EXPANDED,
  useSidebar,
} from '../../context/sidebar-context'
import { ChevronLeft, ChevronRight, X } from 'lucide-react'
import { Link, useRouterState } from '@tanstack/react-router'
import { useEffect } from 'react'

export function PanelSidebar() {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const { isExpanded, isMobileOpen, isDesktop, toggleSidebar, closeMobileSidebar } = useSidebar()

  useEffect(() => {
    closeMobileSidebar()
  }, [pathname, closeMobileSidebar])

  const showLabels = !isDesktop || isExpanded || isMobileOpen
  const desktopWidth = isExpanded ? SIDEBAR_WIDTH_EXPANDED : SIDEBAR_WIDTH_COLLAPSED
  const { brand } = appConfig
  const mobileClosed = !isDesktop && !isMobileOpen

  return (
    <aside
      style={
        isDesktop
          ? { width: desktopWidth }
          : { width: Math.min(320, SIDEBAR_WIDTH_EXPANDED) }
      }
      className={`fixed top-0 left-0 z-50 flex h-dvh flex-col border-r border-gray-200 bg-white transition-[width,transform] duration-300 ease-in-out dark:border-gray-800 dark:bg-gray-900 ${
        isMobileOpen ? 'translate-x-0' : '-translate-x-full'
      } lg:translate-x-0`}
      aria-label="Main navigation"
      aria-hidden={mobileClosed || undefined}
      inert={mobileClosed || undefined}
    >
      <div className="panel-topbar flex items-center gap-3 px-4">
        <Link
          to="/dashboard"
          onClick={closeMobileSidebar}
          className={`flex min-w-0 flex-1 items-center gap-3 ${!showLabels ? 'justify-center' : ''}`}
        >
          <BrandLogo className="h-8 w-8 shrink-0 rounded-lg" />
          {showLabels && (
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold text-gray-900 dark:text-white">
                {brand.name}
              </p>
              <p className="truncate text-[11px] text-gray-500 dark:text-gray-400">{brand.tagline}</p>
            </div>
          )}
        </Link>
        {!isDesktop && (
          <button
            type="button"
            onClick={closeMobileSidebar}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-white/10"
            aria-label="Close menu"
          >
            <X className="h-4 w-4" />
          </button>
        )}
      </div>

      <nav className="no-scrollbar flex flex-1 flex-col overflow-y-auto px-2.5 py-3">
        {navGroups.map((group, gi) => (
          <div key={group.id} className={gi > 0 ? 'mt-4' : ''}>
            {showLabels ? (
              <p className="px-2.5 pb-1.5 text-[11px] font-semibold tracking-wider text-gray-400 uppercase dark:text-gray-500">
                {group.label}
              </p>
            ) : (
              gi > 0 && (
                <div
                  className="mx-2 mb-2 border-t border-gray-200 dark:border-gray-800"
                  aria-hidden
                />
              )
            )}
            <div className="flex flex-col gap-0.5">
              {group.items.map((item) => {
                const active = isNavActive(pathname, item.href)
                const Icon = item.icon
                return (
                  <Link
                    key={item.href}
                    to={item.href}
                    onClick={closeMobileSidebar}
                    title={!showLabels ? item.name : undefined}
                    aria-current={active ? 'page' : undefined}
                    className={`group relative flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500/40 ${
                      active
                        ? 'bg-brand-50 text-brand-800 dark:bg-brand-500/15 dark:text-brand-200'
                        : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-white/8'
                    } ${!showLabels ? 'justify-center px-0' : ''}`}
                  >
                    <Icon
                      className={`h-[18px] w-[18px] shrink-0 ${
                        active
                          ? 'text-brand-600 dark:text-brand-400'
                          : 'text-gray-400 group-hover:text-gray-600 dark:text-gray-500 dark:group-hover:text-gray-300'
                      }`}
                      strokeWidth={1.75}
                    />
                    {showLabels && <span className="truncate">{item.name}</span>}
                  </Link>
                )
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="shrink-0 space-y-1 border-t border-gray-200 p-2.5 dark:border-gray-800">
        {isDesktop && (
          <button
            type="button"
            onClick={toggleSidebar}
            className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-xs font-medium text-gray-500 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-white/5 ${!showLabels ? 'justify-center' : ''}`}
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
      </div>
    </aside>
  )
}
