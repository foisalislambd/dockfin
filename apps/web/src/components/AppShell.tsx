import { Suspense } from 'react'
import { Outlet } from '@tanstack/react-router'
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
        <main className="panel-scrollbar min-h-0 flex-1 overflow-y-auto overflow-x-hidden">
          <div className="w-full px-3 py-4 sm:px-5 sm:py-5 lg:px-6">
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
