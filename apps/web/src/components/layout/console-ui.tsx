import { useNavigate } from '@tanstack/react-router'
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { goKeyHref } from '../../config/app.config'

type ConsoleUi = {
  paletteOpen: boolean
  shortcutsOpen: boolean
  setPaletteOpen: (open: boolean) => void
  setShortcutsOpen: (open: boolean) => void
}

const ConsoleUiContext = createContext<ConsoleUi | null>(null)

function isTypingTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable) return true
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
}

export function ConsoleUiProvider({ children }: { children: ReactNode }) {
  const nav = useNavigate()
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [shortcutsOpen, setShortcutsOpen] = useState(false)

  const value = useMemo(
    () => ({ paletteOpen, shortcutsOpen, setPaletteOpen, setShortcutsOpen }),
    [paletteOpen, shortcutsOpen],
  )

  const onKey = useCallback(
    (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setShortcutsOpen(false)
        setPaletteOpen((open) => !open)
        return
      }
      if (e.metaKey || e.ctrlKey || e.altKey) return
      if (paletteOpen || shortcutsOpen) return
      if (document.querySelector('[aria-modal="true"]')) return
      if (isTypingTarget(e.target)) return

      if (e.key === '?') {
        e.preventDefault()
        setShortcutsOpen(true)
        return
      }
      if (e.key === '/') {
        e.preventDefault()
        setPaletteOpen(true)
        return
      }
      if (e.key.toLowerCase() !== 'g') return

      const started = performance.now()
      const waitNext = (next: KeyboardEvent) => {
        window.removeEventListener('keydown', waitNext, true)
        if (performance.now() - started > 1200) return
        if (next.metaKey || next.ctrlKey || next.altKey || isTypingTarget(next.target)) return
        const href = goKeyHref(next.key)
        if (!href) return
        next.preventDefault()
        void nav({ to: href })
      }
      window.addEventListener('keydown', waitNext, true)
      window.setTimeout(() => window.removeEventListener('keydown', waitNext, true), 1200)
    },
    [nav, paletteOpen, shortcutsOpen],
  )

  useEffect(() => {
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onKey])

  return <ConsoleUiContext.Provider value={value}>{children}</ConsoleUiContext.Provider>
}

export function useConsoleUi() {
  const ctx = useContext(ConsoleUiContext)
  if (!ctx) throw new Error('useConsoleUi must be used within ConsoleUiProvider')
  return ctx
}
