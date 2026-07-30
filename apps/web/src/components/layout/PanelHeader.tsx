import { appConfig, navItems } from '../../config/app.config'
import { useSidebar } from '../../context/sidebar-context'
import { useAuth } from '../../lib/auth'
import { ThemeToggle } from '../theme/ThemeToggle'
import { Link, useNavigate } from '@tanstack/react-router'
import { LogOut, Menu, Search, X } from 'lucide-react'
import { useEffect, useState } from 'react'

export function PanelHeader() {
  const { isMobileOpen, isDesktop, toggleMobileSidebar } = useSidebar()
  const { user, team, logout } = useAuth()
  const nav = useNavigate()
  const [paletteOpen, setPaletteOpen] = useState(false)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setPaletteOpen((v) => !v)
      }
      if (e.key === 'Escape') setPaletteOpen(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <>
      <header className="panel-topbar sticky top-0 z-30 w-full bg-white/95 backdrop-blur supports-[backdrop-filter]:bg-white/80 dark:bg-gray-900/95 dark:supports-[backdrop-filter]:bg-gray-900/80">
        <div className="flex h-full items-center gap-3 px-4 sm:gap-4 sm:px-6">
          <button
            type="button"
            onClick={toggleMobileSidebar}
            className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-gray-200 text-gray-600 transition hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500/30 dark:border-gray-800 dark:text-gray-400 dark:hover:bg-white/5 ${isDesktop ? 'lg:hidden' : ''}`}
            aria-label={!isDesktop && isMobileOpen ? 'Close menu' : 'Open menu'}
            aria-expanded={!isDesktop ? isMobileOpen : undefined}
          >
            {!isDesktop && isMobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>

          <Link to="/dashboard" className="flex items-center gap-2 lg:hidden">
            <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-brand-500 text-sm font-bold text-white">
              {appConfig.brand.letter}
            </span>
            <span className="text-base font-semibold text-gray-900 dark:text-white">
              {appConfig.brand.name}
            </span>
          </Link>

          <div className="hidden flex-1 lg:block lg:max-w-lg">
            <button
              type="button"
              onClick={() => setPaletteOpen(true)}
              className="relative flex h-10 w-full items-center rounded-lg border border-gray-200 bg-gray-50 py-2 pr-4 pl-10 text-left text-sm text-gray-500 transition hover:border-gray-300 dark:border-gray-800 dark:bg-white/5"
            >
              <Search className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400" />
              Jump to… <span className="ml-auto text-xs text-gray-400">⌘K</span>
            </button>
          </div>

          <div className="ml-auto flex items-center gap-2 sm:gap-3">
            <ThemeToggle />
            <div className="hidden text-right text-sm sm:block">
              <p className="font-medium text-gray-900 dark:text-white">{user?.name}</p>
              <p className="text-xs text-gray-500">{team?.name}</p>
            </div>
            <button
              type="button"
              onClick={() => void logout().then(() => nav({ to: '/login' }))}
              className="flex h-10 items-center gap-2 rounded-lg border border-gray-200 px-3 text-sm font-medium text-gray-600 transition hover:bg-gray-50 dark:border-gray-800 dark:text-gray-400 dark:hover:bg-white/5"
              aria-label="Sign out"
            >
              <LogOut className="h-4 w-4" />
              <span className="hidden sm:inline">Logout</span>
            </button>
          </div>
        </div>
      </header>

      {paletteOpen && (
        <div
          className="fixed inset-0 z-50 grid place-items-start bg-gray-900/50 pt-[15vh]"
          onClick={() => setPaletteOpen(false)}
        >
          <div
            className="panel-card w-full max-w-lg overflow-hidden"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="border-b border-gray-200 px-4 py-3 text-sm text-gray-500 dark:border-gray-800">
              Jump to…
            </div>
            <ul>
              {navItems.map((item) => (
                <li key={item.href}>
                  <Link
                    to={item.href}
                    onClick={() => setPaletteOpen(false)}
                    className="flex items-center gap-3 px-4 py-3 text-sm text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-white/5"
                  >
                    <item.icon className="h-4 w-4 text-gray-400" />
                    {item.name}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </>
  )
}
