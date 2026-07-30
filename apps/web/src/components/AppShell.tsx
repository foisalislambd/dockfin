import { Link, Outlet, useRouterState } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { useAuth } from '../lib/auth'

const nav = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/onboarding', label: 'Onboarding' },
  { to: '/servers', label: 'Servers' },
  { to: '/projects', label: 'Projects' },
  { to: '/applications', label: 'Applications' },
  { to: '/databases', label: 'Databases' },
  { to: '/services', label: 'Services' },
  { to: '/notifications', label: 'Notifications' },
  { to: '/settings', label: 'Settings' },
]

export function AppShell() {
  const { user, team, logout } = useAuth()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
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

  const filtered = useMemo(() => nav, [])

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-20 border-b border-[var(--color-line)]/80 bg-[rgba(11,20,18,0.85)] backdrop-blur-md">
        <div className="mx-auto flex max-w-7xl items-center gap-6 px-4 py-3">
          <Link to="/dashboard" className="flex items-center gap-2">
            <span className="grid h-8 w-8 place-items-center rounded-lg bg-[var(--color-panel-2)] text-[var(--color-accent)] ring-1 ring-[var(--color-line)]">
              ◆
            </span>
            <span className="text-lg font-semibold tracking-tight">Goolify</span>
          </Link>
          <nav className="hidden items-center gap-1 md:flex">
            {nav.map((item) => (
              <Link
                key={item.to}
                to={item.to}
                className={`rounded-md px-3 py-1.5 text-sm transition ${
                  pathname.startsWith(item.to)
                    ? 'bg-[var(--color-panel-2)] text-[var(--color-accent)]'
                    : 'text-[var(--color-muted)] hover:text-[var(--color-text)]'
                }`}
              >
                {item.label}
              </Link>
            ))}
          </nav>
          <div className="ml-auto flex items-center gap-3 text-sm text-[var(--color-muted)]">
            <button
              type="button"
              onClick={() => setPaletteOpen(true)}
              className="hidden rounded-md border border-[var(--color-line)] px-2 py-1 text-xs md:inline"
            >
              ⌘K
            </button>
            <span>{team?.name}</span>
            <span className="text-[var(--color-text)]">{user?.name}</span>
            <button
              type="button"
              onClick={() => void logout()}
              className="rounded-md border border-[var(--color-line)] px-2 py-1 hover:border-[var(--color-accent)]"
            >
              Logout
            </button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-7xl px-4 py-8">
        <Outlet />
      </main>

      {paletteOpen && (
        <div
          className="fixed inset-0 z-50 grid place-items-start bg-black/50 pt-[15vh]"
          onClick={() => setPaletteOpen(false)}
        >
          <div
            className="w-full max-w-lg overflow-hidden rounded-xl border border-[var(--color-line)] bg-[var(--color-panel)] shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="border-b border-[var(--color-line)] px-4 py-3 text-sm text-[var(--color-muted)]">
              Jump to…
            </div>
            <ul>
              {filtered.map((item) => (
                <li key={item.to}>
                  <Link
                    to={item.to}
                    onClick={() => setPaletteOpen(false)}
                    className="block px-4 py-3 text-sm hover:bg-[var(--color-panel-2)]"
                  >
                    {item.label}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </div>
  )
}
