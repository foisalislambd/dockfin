import { Link } from '@tanstack/react-router'
import { ChevronLeft } from 'lucide-react'

/** Chevron back control. Parent routes must not look like the current page. */
export function BackLink({
  label,
  to,
  params,
}: {
  label: string
  to: string
  params?: Record<string, string>
}) {
  return (
    <Link
      to={to as never}
      params={params as never}
      activeOptions={{ exact: true, includeSearch: false }}
      activeProps={{ className: '' }}
      className="group inline-flex h-8 max-w-full items-center gap-1.5 rounded-lg px-1.5 -ml-1.5 text-sm font-medium text-gray-500 transition hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-white/10 dark:hover:text-white"
    >
      <span className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md border border-gray-200 bg-white text-gray-600 shadow-sm transition group-hover:border-gray-300 group-hover:text-gray-900 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300 dark:group-hover:border-gray-600 dark:group-hover:text-white">
        <ChevronLeft className="h-3.5 w-3.5" strokeWidth={2.25} />
      </span>
      <span className="truncate">{label}</span>
    </Link>
  )
}
