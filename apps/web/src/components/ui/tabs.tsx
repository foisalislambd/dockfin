import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

export type ResourceTab = {
  id: string
  label: string
  icon?: LucideIcon
}

export function ResourceTabs({
  tabs,
  active,
  onChange,
}: {
  tabs: ResourceTab[]
  active: string
  onChange: (id: string) => void
}) {
  return (
    <div className="flex flex-wrap gap-1 border-b border-gray-200 dark:border-gray-800">
      {tabs.map((t) => {
        const isActive = t.id === active
        const Icon = t.icon
        return (
          <button
            key={t.id}
            type="button"
            onClick={() => onChange(t.id)}
            className={`-mb-px inline-flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium transition ${
              isActive
                ? 'border-brand-500 text-brand-600 dark:text-brand-400'
                : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
            }`}
          >
            {Icon ? (
              <Icon
                className={`h-3.5 w-3.5 shrink-0 ${
                  isActive ? 'opacity-100' : 'opacity-70'
                }`}
                aria-hidden
              />
            ) : null}
            {t.label}
          </button>
        )
      })}
    </div>
  )
}

export function TabPanel({ children }: { children: ReactNode }) {
  return <div className="pt-5">{children}</div>
}

export function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div className="panel-card p-4">
      <div className="text-xs text-gray-500 dark:text-gray-400">{label}</div>
      <div className="mt-1 break-all font-medium text-gray-900 dark:text-white">{value}</div>
    </div>
  )
}
