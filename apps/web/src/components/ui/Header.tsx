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
    <header className="flex min-h-12 flex-wrap items-center gap-x-4 gap-y-2 border-b border-gray-200 pb-3 dark:border-gray-800">
      <div className="min-w-0 flex-[1_1_18rem]">
        <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
          <h1 className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white">{title}</h1>
          {subtitle ? (
            <p className="min-w-0 text-sm text-gray-500 max-md:basis-full dark:text-gray-400">
              <span className="mr-2 hidden text-gray-300 md:inline" aria-hidden>
                ·
              </span>
              {subtitle}
            </p>
          ) : null}
        </div>
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </header>
  )
}
