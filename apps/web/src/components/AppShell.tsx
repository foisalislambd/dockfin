import { Suspense } from 'react'
import { Outlet, useRouterState } from '@tanstack/react-router'
import { SidebarProvider } from '../context/sidebar-context'
import { CommandPalette, ShortcutsDialog } from './layout/CommandPalette'
import { ConsoleUiProvider } from './layout/console-ui'
import { KeyboardHints } from './layout/KeyboardHints'
import { PanelBackdrop } from './layout/PanelBackdrop'
import { PanelHeader } from './layout/PanelHeader'
import { PanelSidebar } from './layout/PanelSidebar'
import { RoutePageSkeleton } from './ui/Skeleton'

/**
 * Console frame: a full-width top bar, then sidebar and page.
 * Structure is a clean-room take on the Antirion console shell (Apache-2.0);
 * see apps/web/NOTICE. Colors, logo, and copy stay Dockfin's.
 */
function PanelShellInner() {
  const fillViewport = useRouterState({ select: (s) => s.location.pathname === '/terminal' })
  const fallback = useRouterState({
    select: (s) => ({ pathname: s.location.pathname, search: s.location.search }),
  })

  return (
    <div className="panel-shell panel-main flex h-dvh w-full flex-col overflow-hidden">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-[90] focus:rounded-md focus:bg-white focus:px-3 focus:py-2 dark:focus:bg-gray-900"
      >
        Skip to main content
      </a>
      <PanelHeader />
      <div className="flex min-h-0 min-w-0 flex-1">
        <PanelSidebar />
        <PanelBackdrop />
        <main
          id="main"
          tabIndex={-1}
          className={
            fillViewport
              ? 'flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden outline-none'
              : 'panel-scrollbar flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto overflow-x-hidden outline-none'
          }
        >
          <div
            className={
              fillViewport
                ? 'flex h-full min-h-0 w-full min-w-0 flex-1 flex-col px-2 py-3 sm:px-3'
                : 'flex w-full grow flex-col px-2 py-3 sm:px-3'
            }
          >
            <Suspense fallback={<RoutePageSkeleton pathname={fallback.pathname} search={fallback.search} />}>
              <Outlet />
            </Suspense>
          </div>
          {!fillViewport && <KeyboardHints />}
        </main>
      </div>
      <CommandPalette />
      <ShortcutsDialog />
    </div>
  )
}

export function AppShell() {
  return (
    <SidebarProvider>
      <ConsoleUiProvider>
        <PanelShellInner />
      </ConsoleUiProvider>
    </SidebarProvider>
  )
}
