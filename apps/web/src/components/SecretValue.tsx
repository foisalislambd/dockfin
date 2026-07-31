import { Eye, EyeOff } from 'lucide-react'
import { useState } from 'react'
import { isSecretEnvKey } from '../lib/secrets'

const MASK = '••••••••••••'

export function SecretValue({
  value,
  secret = true,
  className = '',
}: {
  value?: string | null
  secret?: boolean
  className?: string
}) {
  const [open, setOpen] = useState(false)
  const text = value ?? ''

  if (!secret) {
    return (
      <span className={`font-mono text-xs break-all text-gray-600 dark:text-gray-300 ${className}`}>
        {text || '—'}
      </span>
    )
  }

  const show = open && text !== ''
  return (
    <span className={`inline-flex max-w-full items-center gap-1.5 ${className}`}>
      <span className="min-w-0 flex-1 font-mono text-xs break-all text-gray-600 dark:text-gray-300">
        {show ? text : MASK}
      </span>
      {text !== '' ? (
        <button
          type="button"
          className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-gray-400 transition hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-white/10 dark:hover:text-gray-200"
          aria-label={show ? 'Hide value' : 'Show value'}
          title={show ? 'Hide' : 'Show'}
          onClick={() => setOpen((v) => !v)}
        >
          {show ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
        </button>
      ) : null}
    </span>
  )
}

export function EnvSecretCell({ envKey, value }: { envKey: string; value?: string | null }) {
  return <SecretValue value={value} secret={isSecretEnvKey(envKey)} />
}

/** Password-style input with eye toggle. */
export function SecretInput({
  label,
  value,
  onChange,
  secret = true,
  required,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  secret?: boolean
  required?: boolean
}) {
  const [open, setOpen] = useState(false)
  return (
    <label className="block text-sm">
      <span className="mb-1 block text-gray-500 dark:text-gray-400">{label}</span>
      <span className="relative block">
        <input
          required={required}
          type={secret && !open ? 'password' : 'text'}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          autoComplete="off"
          spellCheck={false}
          className="panel-field w-full rounded-lg py-2 pr-10 pl-3 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
        />
        {secret ? (
          <button
            type="button"
            className="absolute top-1/2 right-1.5 inline-flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-white/10 dark:hover:text-gray-200"
            aria-label={open ? 'Hide value' : 'Show value'}
            onClick={() => setOpen((v) => !v)}
          >
            {open ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
          </button>
        ) : null}
      </span>
    </label>
  )
}
