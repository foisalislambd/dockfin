import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Check, ChevronDown } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import { api } from '../../lib/api'
import { useAuth } from '../../lib/auth'

export function TeamSwitcher() {
  const { team, teams, refresh } = useAuth()
  const qc = useQueryClient()
  const nav = useNavigate()
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const menuId = useId()

  useEffect(() => {
    if (!open) return
    function onDoc(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [open])

  if (!team) return null

  async function pick(id: string) {
    if (id === team?.id) {
      setOpen(false)
      return
    }
    setBusy(true)
    try {
      await api.switchTeam(id)
      await refresh()
      await qc.invalidateQueries()
      setOpen(false)
      void nav({ to: '/dashboard' })
    } finally {
      setBusy(false)
    }
  }

  const many = teams.length > 1

  return (
    <div className="relative hidden min-w-0 sm:block" ref={ref}>
      <button
        type="button"
        disabled={!many || busy}
        onClick={() => many && setOpen((v) => !v)}
        aria-haspopup={many ? 'menu' : undefined}
        aria-expanded={many ? open : undefined}
        aria-controls={many ? menuId : undefined}
        className="flex max-w-[10rem] items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-gray-700 hover:bg-gray-100 disabled:hover:bg-transparent lg:max-w-[14rem] dark:text-gray-200 dark:hover:bg-white/5"
      >
        <span className="truncate">{team.name}</span>
        {many ? <ChevronDown className="h-3.5 w-3.5 shrink-0 text-gray-400" /> : null}
      </button>
      {open && many && (
        <div
          id={menuId}
          role="menu"
          className="absolute left-0 z-50 mt-1 w-56 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-800 dark:bg-gray-900"
        >
          {teams.map((item) => (
            <button
              key={item.id}
              type="button"
              role="menuitem"
              onClick={() => void pick(item.id)}
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-white/5"
            >
              <span className="min-w-0 flex-1 truncate">
                {item.name}
                {item.personal ? <span className="text-gray-400"> · personal</span> : null}
              </span>
              {item.id === team.id ? <Check className="h-3.5 w-3.5 text-brand-500" /> : null}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
