import type { ReactNode } from 'react'

export function Header({
  title,
  actions,
}: {
  title: string
  subtitle?: string
  actions?: ReactNode
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">{title}</h1>
      </div>
      <div className="flex gap-2">{actions}</div>
    </div>
  )
}
