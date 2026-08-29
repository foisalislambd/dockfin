import { BrandLogo } from '../BrandLogo'
import { appConfig } from '../../config/app.config'
import { useSidebar } from '../../context/sidebar-context'
import { ThemeToggle } from '../theme/ThemeToggle'
import { UserMenu } from './UserMenu'
import { GlobalSearch } from './GlobalSearch'
import { Link } from '@tanstack/react-router'
import { Menu, X } from 'lucide-react'

export function PanelHeader() {
  const { isMobileOpen, isDesktop, toggleMobileSidebar } = useSidebar()

  return (
    <header className="panel-topbar sticky top-0 z-30 w-full bg-white/95 backdrop-blur supports-[backdrop-filter]:bg-white/80 dark:bg-gray-900/95 dark:supports-[backdrop-filter]:bg-gray-900/80">
      <div className="flex h-full items-center gap-3 px-4 sm:gap-4 sm:px-6">
        <button
          type="button"
          onClick={toggleMobileSidebar}
          className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-gray-200 text-gray-600 transition hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500/30 dark:border-gray-800 dark:text-gray-400 dark:hover:bg-white/5 ${isDesktop ? 'lg:hidden' : ''}`}
          aria-label={!isDesktop && isMobileOpen ? 'Close menu' : 'Open menu'}
          aria-expanded={!isDesktop ? isMobileOpen : undefined}
        >
          {!isDesktop && isMobileOpen ? <X className="h-4 w-4" /> : <Menu className="h-4 w-4" />}
        </button>

        <Link to="/dashboard" className="flex shrink-0 items-center gap-2 lg:hidden">
          <BrandLogo className="h-8 w-8 rounded-md" />
          <span className="hidden text-sm font-semibold text-gray-900 sm:inline dark:text-white">
            {appConfig.brand.name}
          </span>
        </Link>

        <GlobalSearch />

        <div className="ml-auto flex shrink-0 items-center gap-2 sm:gap-3">
          <ThemeToggle />
          <UserMenu />
        </div>
      </div>
    </header>
  )
}
