import { useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import type { ReactNode } from 'react'

export function CreatePageShell({
  title,
  subtitle,
  backTo,
  backLabel,
  children,
}: {
  title: string
  subtitle: string
  backTo: string
  backLabel: string
  children: ReactNode
}) {
  const nav = useNavigate()
  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div>
        <button
          type="button"
          onClick={() => void nav({ to: backTo })}
          className="mb-4 inline-flex items-center gap-1.5 text-sm font-medium text-gray-500 transition hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
        >
          <ArrowLeft className="h-4 w-4" />
          {backLabel}
        </button>
        <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">{title}</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{subtitle}</p>
      </div>
      <div className="panel-card p-4 sm:p-5">{children}</div>
    </div>
  )
}

export function FieldLabel({ children, hint }: { children: ReactNode; hint?: string }) {
  return (
    <div className="mb-1.5 flex items-end justify-between gap-2">
      <span className="block text-xs font-medium text-gray-700 dark:text-gray-300">{children}</span>
      {hint && <span className="text-xs text-gray-400">{hint}</span>}
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
        className="h-9 w-full rounded-md border border-gray-200 bg-white px-2.5 text-sm text-gray-900 shadow-sm transition placeholder:text-gray-400 focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20 focus:outline-none dark:border-gray-700 dark:bg-gray-900 dark:text-white"
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
        className="h-9 w-full rounded-md border border-gray-200 bg-white px-2.5 text-sm text-gray-900 shadow-sm transition focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20 focus:outline-none dark:border-gray-700 dark:bg-gray-900 dark:text-white"
      >
        {children}
      </select>
    </label>
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
        onClick={() => void nav({ to: cancelTo })}
        className="inline-flex h-9 items-center justify-center rounded-md border border-gray-200 px-3.5 text-sm font-medium text-gray-600 transition hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-white/5"
      >
        Cancel
      </button>
    </div>
  )
}

export function ChoiceCard({
  active,
  title,
  description,
  onClick,
}: {
  active: boolean
  title: string
  description: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-lg border p-3 text-left transition ${
        active
          ? 'border-brand-500 bg-brand-50 shadow-sm ring-1 ring-brand-500/30 dark:bg-brand-500/10'
          : 'border-gray-200 hover:border-gray-300 dark:border-gray-700 dark:hover:border-gray-600'
      }`}
    >
      <div
        className={`text-sm font-medium ${active ? 'text-brand-700 dark:text-brand-300' : 'text-gray-900 dark:text-white'}`}
      >
        {title}
      </div>
      <div className="mt-0.5 text-xs leading-relaxed text-gray-500 dark:text-gray-400">{description}</div>
    </button>
  )
}
