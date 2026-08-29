import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'

type ToastKind = 'success' | 'error' | 'info' | 'warning'

type ToastItem = {
  id: number
  message: string
  kind: ToastKind
}

type ToastAPI = {
  push: (message: string, kind?: ToastKind) => void
  success: (message: string) => void
  error: (message: string) => void
  warning: (message: string) => void
}

const ToastContext = createContext<ToastAPI | null>(null)

let nextId = 1

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])

  const push = useCallback((message: string, kind: ToastKind = 'info') => {
    const id = nextId++
    setItems((prev) => [...prev, { id, message, kind }])
    const ms = kind === 'warning' || kind === 'error' ? 6000 : 3500
    window.setTimeout(() => {
      setItems((prev) => prev.filter((t) => t.id !== id))
    }, ms)
  }, [])

  const api = useMemo<ToastAPI>(
    () => ({
      push,
      success: (message) => push(message, 'success'),
      error: (message) => push(message, 'error'),
      warning: (message) => push(message, 'warning'),
    }),
    [push],
  )

  return (
    <ToastContext.Provider value={api}>
      {children}
      {createPortal(
        <div className="pointer-events-none fixed right-4 bottom-4 z-[100] flex w-[min(22rem,calc(100vw-2rem))] flex-col gap-2">
          {items.map((t) => (
            <div
              key={t.id}
              role="status"
              className={`pointer-events-auto rounded-lg border px-3 py-2 text-sm shadow-lg backdrop-blur ${
                t.kind === 'success'
                  ? 'border-success-500/30 bg-white/95 text-success-700 dark:bg-gray-900/95 dark:text-success-400'
                  : t.kind === 'error'
                    ? 'border-error-500/30 bg-white/95 text-error-600 dark:bg-gray-900/95 dark:text-error-400'
                    : t.kind === 'warning'
                      ? 'border-amber-400/40 bg-white/95 text-amber-800 dark:bg-gray-900/95 dark:text-amber-200'
                      : 'border-gray-200 bg-white/95 text-gray-800 dark:border-gray-700 dark:bg-gray-900/95 dark:text-gray-100'
              }`}
            >
              {t.message}
            </div>
          ))}
        </div>,
        document.body,
      )}
    </ToastContext.Provider>
  )
}

export function useToast(): ToastAPI {
  const ctx = useContext(ToastContext)
  if (!ctx) {
    return {
      push: () => undefined,
      success: () => undefined,
      error: () => undefined,
      warning: () => undefined,
    }
  }
  return ctx
}

// Silence unused import warning if tree-shaken oddly in some builds.
void useEffect
