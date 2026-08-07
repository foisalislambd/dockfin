import { useNavigate } from '@tanstack/react-router'
import { ArrowLeft, Info } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'

export const fieldClass =
  'panel-field h-9 w-full rounded-md px-2.5 text-sm shadow-sm transition placeholder:text-gray-400 focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20 focus:outline-none'

export function CreatePageShell({
  title,
  backTo,
  backLabel,
  children,
}: {
  title: string
  subtitle?: string
  backTo: string
  backLabel: string
  children: ReactNode
}) {
  const nav = useNavigate()
  return (
    <div className="w-full space-y-6">
      <div>
        <button
          type="button"
          onClick={() => void nav({ to: backTo as '/' })}
          className="mb-4 inline-flex items-center gap-1.5 text-sm font-medium text-gray-500 transition hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
        >
          <ArrowLeft className="h-4 w-4" />
          {backLabel}
        </button>
        <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">{title}</h1>
      </div>
      <div className="panel-card w-full p-4 sm:p-5 lg:p-6">{children}</div>
    </div>
  )
}

/** Compact info icon; click shows helper text (keeps field labels clean). */
export function InfoHint({ text }: { text: string }) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLSpanElement>(null)

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <span className="relative inline-flex" ref={rootRef}>
      <button
        type="button"
        className={`rounded p-0.5 transition ${
          open
            ? 'text-brand-600 dark:text-brand-400'
            : 'text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'
        }`}
        aria-label="More info"
        aria-expanded={open}
        onClick={(e) => {
          e.preventDefault()
          e.stopPropagation()
          setOpen((v) => !v)
        }}
      >
        <Info className="h-3.5 w-3.5" aria-hidden />
      </button>
      {open ? (
        <span
          role="tooltip"
          className="absolute top-full left-0 z-40 mt-1.5 w-64 rounded-lg border border-gray-200 bg-white p-2.5 text-xs leading-relaxed text-gray-600 shadow-lg dark:border-gray-700 dark:bg-gray-950 dark:text-gray-300"
        >
          {text}
        </span>
      ) : null}
    </span>
  )
}

export function FieldLabel({
  children,
  hint,
  status,
}: {
  children: ReactNode
  /** Help text shown via info icon tooltip (not inline). */
  hint?: string
  /** Short status on the right (e.g. loading count) — not help copy. */
  status?: string
}) {
  return (
    <div className="mb-1.5 flex items-center gap-1.5">
      <span className="block text-xs font-medium text-gray-700 dark:text-gray-300">{children}</span>
      {hint ? <InfoHint text={hint} /> : null}
      {status ? (
        <span className="ml-auto text-xs text-gray-400 dark:text-gray-500">{status}</span>
      ) : null}
    </div>
  )
}

export function FormInput(props: {
  label: string
  value: string
  onChange: (v: string) => void
  type?: string
  required?: boolean
  placeholder?: string
  hint?: string
}) {
  const { label, value, onChange, type = 'text', required = true, placeholder, hint } = props
  return (
    <label className="block">
      <FieldLabel hint={hint}>{label}</FieldLabel>
      <input
        type={type}
        required={required}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className={fieldClass}
      />
    </label>
  )
}

export function FormSelect(props: {
  label: string
  value: string
  onChange: (v: string) => void
  required?: boolean
  hint?: string
  children: ReactNode
}) {
  const { label, value, onChange, required = true, hint, children } = props
  return (
    <label className="block">
      <FieldLabel hint={hint}>{label}</FieldLabel>
      <select
        required={required}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={fieldClass}
      >
        {children}
      </select>
    </label>
  )
}

export type SearchSelectOption = { value: string; label: string }

/** Combobox-style select with filter — better for long lists (repos, etc.). */
export function FormSearchSelect(props: {
  label: string
  value: string
  onChange: (v: string) => void
  options: SearchSelectOption[]
  required?: boolean
  hint?: string
  placeholder?: string
  loading?: boolean
  emptyMessage?: string
}) {
  const {
    label,
    value,
    onChange,
    options,
    required = true,
    hint,
    placeholder = 'Search…',
    loading,
    emptyMessage = 'No matches',
  } = props
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const rootRef = useRef<HTMLDivElement>(null)

  const selected = options.find((o) => o.value === value)
  const q = query.trim().toLowerCase()
  const filtered = useMemo(() => {
    if (!q) return options
    return options.filter(
      (o) => o.label.toLowerCase().includes(q) || o.value.toLowerCase().includes(q),
    )
  }, [options, q])

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [open])

  useEffect(() => {
    if (!open) setQuery('')
  }, [open])

  return (
    <div className="block" ref={rootRef}>
      <FieldLabel
        hint={hint}
        status={loading ? 'Loading…' : options.length ? `${options.length}` : undefined}
      >
        {label}
      </FieldLabel>
      <div className="relative">
        <button
          type="button"
          className={`${fieldClass} flex items-center justify-between gap-2 text-left`}
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          aria-haspopup="listbox"
        >
          <span className={selected ? 'truncate text-gray-900 dark:text-white' : 'truncate text-gray-400'}>
            {selected?.label || (loading ? 'Loading…' : 'Select…')}
          </span>
          <span className="shrink-0 text-gray-400">▾</span>
        </button>
        {/* Keep a hidden required input for native form validation */}
        <input tabIndex={-1} className="sr-only" required={required} value={value} onChange={() => {}} />
        {open && (
          <div className="absolute z-30 mt-1 w-full overflow-hidden rounded-md border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-950">
            <div className="border-b border-gray-100 p-2 dark:border-gray-800">
              <input
                autoFocus
                type="search"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={placeholder}
                className={`${fieldClass} h-8`}
                onKeyDown={(e) => {
                  if (e.key === 'Escape') setOpen(false)
                  if (e.key === 'Enter' && filtered[0]) {
                    e.preventDefault()
                    onChange(filtered[0].value)
                    setOpen(false)
                  }
                }}
              />
            </div>
            <ul className="max-h-64 overflow-y-auto py-1" role="listbox">
              {filtered.map((o) => (
                <li key={o.value}>
                  <button
                    type="button"
                    role="option"
                    aria-selected={o.value === value}
                    className={`block w-full truncate px-3 py-2 text-left text-sm hover:bg-brand-50 dark:hover:bg-brand-500/10 ${
                      o.value === value
                        ? 'bg-brand-50 font-medium text-brand-700 dark:bg-brand-500/15 dark:text-brand-300'
                        : 'text-gray-800 dark:text-gray-200'
                    }`}
                    onClick={() => {
                      onChange(o.value)
                      setOpen(false)
                    }}
                  >
                    {o.label}
                  </button>
                </li>
              ))}
              {!filtered.length && (
                <li className="px-3 py-4 text-center text-sm text-gray-500 dark:text-gray-400">
                  {emptyMessage}
                </li>
              )}
            </ul>
          </div>
        )}
      </div>
    </div>
  )
}

export function FormActions({
  busy,
  submitLabel,
  cancelTo,
}: {
  busy?: boolean
  submitLabel: string
  cancelTo: string
}) {
  const nav = useNavigate()
  return (
    <div className="flex flex-wrap items-center gap-2 border-t border-gray-100 pt-4 dark:border-gray-800">
      <button
        type="submit"
        disabled={busy}
        className="inline-flex h-9 items-center justify-center rounded-md bg-brand-500 px-3.5 text-sm font-medium text-white shadow-sm transition hover:bg-brand-600 disabled:cursor-not-allowed disabled:opacity-60"
      >
        {busy ? 'Saving…' : submitLabel}
      </button>
      <button
        type="button"
        onClick={() => void nav({ to: cancelTo as '/' })}
        className="inline-flex h-9 items-center justify-center rounded-md border border-gray-200 px-3.5 text-sm font-medium text-gray-600 transition hover:bg-gray-50 dark:border-gray-800 dark:text-gray-300 dark:hover:bg-white/5"
      >
        Cancel
      </button>
    </div>
  )
}

export function ChoiceCard({
  active,
  title,
  onClick,
}: {
  active: boolean
  title: string
  description?: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-lg border p-3 text-left transition ${
        active
          ? 'border-brand-500 bg-brand-50 shadow-sm ring-1 ring-brand-500/30 dark:bg-brand-500/10'
          : 'border-gray-200 hover:border-gray-300 dark:border-gray-800 dark:hover:border-gray-600'
      }`}
    >
      <div
        className={`text-sm font-medium ${active ? 'text-brand-700 dark:text-brand-300' : 'text-gray-900 dark:text-white'}`}
      >
        {title}
      </div>
    </button>
  )
}
