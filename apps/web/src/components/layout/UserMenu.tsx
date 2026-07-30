import { Link, useNavigate } from '@tanstack/react-router'
import { ChevronDown, LogOut, Settings } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import { useAuth } from '../../lib/auth'

export function UserMenu() {
  const { user, team, logout } = useAuth()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const menuId = useId()

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  useEffect(() => {
    if (!open) return
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        setOpen(false)
        triggerRef.current?.focus()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [open])

  async function handleLogout() {
    setOpen(false)
    await logout()
    void navigate({ to: '/login' })
  }

  const initial = user?.name?.charAt(0)?.toUpperCase() || user?.email?.charAt(0)?.toUpperCase() || 'U'

  return (
    <div className="relative" ref={ref}>
      <button
        ref={triggerRef}
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-2 rounded-lg border border-gray-200 bg-white py-1 pr-1.5 pl-1 transition hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500/30 dark:border-gray-800 dark:bg-gray-900 dark:hover:bg-white/5"
        aria-expanded={open}
        aria-haspopup="menu"
        aria-controls={menuId}
      >
        <span className="flex h-7 w-7 items-center justify-center rounded-md bg-brand-500 text-xs font-semibold text-white">
          {initial}
        </span>
        <span className="hidden min-w-0 text-left md:block">
          <span className="block max-w-[120px] truncate text-xs font-medium text-gray-800 dark:text-white/90">
            {user?.name || 'User'}
          </span>
          <span className="block max-w-[120px] truncate text-[11px] text-gray-500 dark:text-gray-400">
            {user?.email || team?.name}
          </span>
        </span>
        <ChevronDown
          className={`hidden h-3.5 w-3.5 shrink-0 text-gray-500 dark:text-gray-400 transition md:block ${open ? 'rotate-180' : ''}`}
          aria-hidden
        />
      </button>

      {open && (
        <div
          id={menuId}
          role="menu"
          className="absolute right-0 z-50 mt-1.5 w-56 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-800 dark:bg-gray-900"
        >
          <div className="border-b border-gray-100 px-3 py-2.5 dark:border-gray-800">
            <p className="truncate text-xs font-medium text-gray-800 dark:text-white/90">{user?.name}</p>
            <p className="truncate text-[11px] text-gray-500 dark:text-gray-400">{user?.email}</p>
            {team && (
              <p className="mt-1 truncate text-[11px] text-gray-400">
                Team: <span className="text-gray-600 dark:text-gray-300">{team.name}</span>
                {team.personal ? ' · personal' : ''}
              </p>
            )}
          </div>
          <Link
            to="/settings"
            role="menuitem"
            onClick={() => setOpen(false)}
            className="flex items-center gap-2 px-3 py-2 text-xs text-gray-700 hover:bg-gray-50 focus-visible:bg-gray-50 focus-visible:outline-none dark:text-gray-300 dark:hover:bg-white/5"
          >
            <Settings className="h-3.5 w-3.5" aria-hidden />
            Settings
          </Link>
          <button
            type="button"
            role="menuitem"
            onClick={() => void handleLogout()}
            className="flex w-full items-center gap-2 px-3 py-2 text-xs text-error-500 hover:bg-error-50 focus-visible:outline-none dark:hover:bg-error-500/10"
          >
            <LogOut className="h-3.5 w-3.5" aria-hidden />
            Sign out
          </button>
        </div>
      )}
    </div>
  )
}
