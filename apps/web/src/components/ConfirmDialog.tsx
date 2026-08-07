import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { Btn } from './ui/Button'
import { Modal } from './ui/Modal'

type ConfirmState = {
  title: string
  message: string
  confirmLabel?: string
  danger?: boolean
  resolve: (ok: boolean) => void
}

type ConfirmAPI = {
  confirm: (opts: {
    title?: string
    message: string
    confirmLabel?: string
    danger?: boolean
  }) => Promise<boolean>
}

const ConfirmContext = createContext<ConfirmAPI | null>(null)

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<ConfirmState | null>(null)
  const resolveRef = useRef<((ok: boolean) => void) | null>(null)

  const confirm = useCallback<ConfirmAPI['confirm']>((opts) => {
    return new Promise<boolean>((resolve) => {
      resolveRef.current = resolve
      setState({
        title: opts.title || 'Confirm',
        message: opts.message,
        confirmLabel: opts.confirmLabel,
        danger: opts.danger,
        resolve,
      })
    })
  }, [])

  const close = (ok: boolean) => {
    resolveRef.current?.(ok)
    resolveRef.current = null
    setState(null)
  }

  const api = useMemo(() => ({ confirm }), [confirm])

  return (
    <ConfirmContext.Provider value={api}>
      {children}
      {state && (
        <Modal title={state.title} onClose={() => close(false)}>
          <p className="mb-4 text-sm text-gray-600 dark:text-gray-300">{state.message}</p>
          <div className="flex justify-end gap-2">
            <Btn onClick={() => close(false)}>Cancel</Btn>
            <Btn
              primary={!state.danger}
              onClick={() => close(true)}
            >
              <span className={state.danger ? 'text-error-600 dark:text-error-400' : undefined}>
                {state.confirmLabel || 'Confirm'}
              </span>
            </Btn>
          </div>
        </Modal>
      )}
    </ConfirmContext.Provider>
  )
}

export function useConfirm(): ConfirmAPI['confirm'] {
  const ctx = useContext(ConfirmContext)
  return (
    ctx?.confirm ||
    (async (opts) => window.confirm(opts.message))
  )
}
