import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { api, ApiError } from '../lib/api'

/** Registers the machine running Dockfin as a server (`POST /servers/bootstrap-self`). */
export function UseThisHostButton({
  className = '',
  label = 'Use this machine',
}: {
  className?: string
  label?: string
}) {
  const qc = useQueryClient()
  const nav = useNavigate()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function run() {
    setBusy(true)
    setError('')
    try {
      const res = await api.bootstrapSelf(true)
      await qc.invalidateQueries({ queryKey: ['servers'] })
      void nav({ to: '/servers/$serverId', params: { serverId: res.server.id } })
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : 'Could not add this machine'
      setError(msg)
    } finally {
      setBusy(false)
    }
  }

  return (
    <span className="inline-flex flex-col items-start gap-1">
      <button
        type="button"
        onClick={() => void run()}
        disabled={busy}
        className={`inline-flex h-8 items-center rounded-md border border-brand-200 bg-white px-2.5 text-xs font-medium text-brand-700 hover:bg-brand-50 disabled:opacity-60 dark:border-brand-500/30 dark:bg-transparent dark:text-brand-300 dark:hover:bg-brand-500/10 ${className}`}
      >
        {busy ? 'Adding…' : label}
      </button>
      {error ? <span className="max-w-xs text-xs text-error-500">{error}</span> : null}
    </span>
  )
}
