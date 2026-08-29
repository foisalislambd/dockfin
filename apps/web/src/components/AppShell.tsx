import { Suspense } from 'react'
import { Outlet, useRouterState } from '@tanstack/react-router'
import {
  SIDEBAR_WIDTH_COLLAPSED,
  SIDEBAR_WIDTH_EXPANDED,
  SidebarProvider,
  useSidebar,
} from '../context/sidebar-context'
import { PanelBackdrop } from './layout/PanelBackdrop'
import { PanelHeader } from './layout/PanelHeader'
import { PanelSidebar } from './layout/PanelSidebar'
import { PageSkeleton } from './ui/Skeleton'

function PanelShellInner() {
  const { isExpanded, isDesktop } = useSidebar()
  const fillViewport = useRouterState({ select: (s) => s.location.pathname === '/terminal' })
  const sidebarWidth = isDesktop
    ? isExpanded
      ? SIDEBAR_WIDTH_EXPANDED
      : SIDEBAR_WIDTH_COLLAPSED
    : 0

  return (
    <div className="panel-shell panel-main flex h-dvh w-full overflow-hidden">
      <div
        className="hidden shrink-0 transition-[width] duration-300 ease-in-out lg:block"
        style={{ width: sidebarWidth }}
        aria-hidden
      />
      <PanelSidebar />
      <PanelBackdrop />
      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        <PanelHeader />
        <main
          className={
            fillViewport
              ? 'flex min-h-0 flex-1 flex-col overflow-hidden'
              : 'panel-scrollbar min-h-0 flex-1 overflow-y-auto overflow-x-hidden'
          }
        >
          <div
            className={
              fillViewport
                ? 'flex h-full min-h-0 w-full min-w-0 flex-1 flex-col px-3 py-3 sm:px-5 sm:py-4 lg:px-6'
                : 'w-full px-3 py-4 sm:px-5 sm:py-5 lg:px-6'
            }
          >
            {/* Keep shell chrome mounted; only the page body suspends */}
            <Suspense fallback={<PageSkeleton cards={2} />}>
              <Outlet />
            </Suspense>
          </div>
        </main>
      </div>
    </div>
  )
}

export function AppShell() {
  return (
    <SidebarProvider>
      <PanelShellInner />
    </SidebarProvider>
  )
}
