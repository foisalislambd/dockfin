import { Link } from '@tanstack/react-router'
import { Menu, Search, X } from 'lucide-react'
import { BrandLogo } from '../BrandLogo'
import { appConfig } from '../../config/app.config'
import {
  SIDEBAR_WIDTH_COLLAPSED,
  SIDEBAR_WIDTH_EXPANDED,
  useSidebar,
} from '../../context/sidebar-context'
import { ThemeToggle } from '../theme/ThemeToggle'
import { PaletteButton } from './CommandPalette'
import { useConsoleUi } from './console-ui'
import { TeamSwitcher } from './TeamSwitcher'
import { UserMenu } from './UserMenu'

export function PanelHeader() {
  const { isExpanded, isMobileOpen, isDesktop, toggleSidebar, toggleMobileSidebar } = useSidebar()
  const { setPaletteOpen } = useConsoleUi()
  const brandWidth = isDesktop ? (isExpanded ? SIDEBAR_WIDTH_EXPANDED : SIDEBAR_WIDTH_COLLAPSED) : undefined

  return (
    <header className="panel-topbar z-30 flex w-full items-center gap-2 bg-white/95 pr-3 pl-2.5 backdrop-blur supports-[backdrop-filter]:bg-white/80 sm:gap-3 sm:pl-3 dark:bg-gray-900/95 dark:supports-[backdrop-filter]:bg-gray-900/80">
      <div
        className="flex shrink-0 items-center gap-2 lg:border-r lg:border-gray-200 lg:pr-3 dark:lg:border-gray-800"
        style={brandWidth ? { width: brandWidth } : undefined}
      >
        <button
          type="button"
          onClick={isDesktop ? toggleSidebar : toggleMobileSidebar}
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-gray-600 hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500/30 dark:text-gray-300 dark:hover:bg-white/5"
          aria-label={
            isDesktop
              ? isExpanded
                ? 'Collapse sidebar'
                : 'Expand sidebar'
              : isMobileOpen
                ? 'Close menu'
                : 'Open menu'
          }
          aria-expanded={!isDesktop ? isMobileOpen : undefined}
        >
          {!isDesktop && isMobileOpen ? <X className="h-4 w-4" /> : <Menu className="h-4 w-4" />}
        </button>
        {!(isDesktop && !isExpanded) && (
          <Link to="/dashboard" className="flex min-w-0 items-center gap-2 text-gray-900 dark:text-white">
            <BrandLogo className="h-7 w-7 shrink-0 rounded-md" />
            <span className="truncate text-sm font-semibold tracking-tight max-sm:sr-only">
              {appConfig.brand.name}
            </span>
          </Link>
        )}
      </div>

      <TeamSwitcher />
      <PaletteButton />
      <div className="min-w-0 flex-1 md:hidden" />

      <div className="ml-auto flex shrink-0 items-center gap-1.5 sm:gap-2">
        <button
          type="button"
          onClick={() => setPaletteOpen(true)}
          className="flex h-8 w-8 items-center justify-center rounded-md border border-gray-200 text-gray-600 md:hidden dark:border-gray-800 dark:text-gray-300"
          aria-label="Search"
        >
          <Search className="h-4 w-4" />
        </button>
        <ThemeToggle />
        <UserMenu />
      </div>
    </header>
  )
}
