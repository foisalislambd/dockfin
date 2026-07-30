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
          <div className="w-full px-4 py-5 sm:px-6 sm:py-6 lg:px-8">
            <Outlet />
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
