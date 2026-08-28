import type { ReactNode } from 'react'

export function Header({
  title,
  subtitle,
  actions,
}: {
  title: string
  subtitle?: string
  actions?: ReactNode
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div className="min-w-0">
        <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">{title}</h1>
        {subtitle ? (
          <p className="mt-1 max-w-2xl text-sm text-gray-500 dark:text-gray-400">{subtitle}</p>
        ) : null}
      </div>
      {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
    </div>
  )
}
