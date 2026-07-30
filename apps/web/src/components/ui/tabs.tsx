import type { ReactNode } from 'react'

export function ResourceTabs({
  tabs,
  active,
  onChange,
}: {
  tabs: { id: string; label: string }[]
  active: string
  onChange: (id: string) => void
}) {
  return (
    <div className="flex flex-wrap gap-1 border-b border-gray-200 dark:border-gray-800">
      {tabs.map((t) => {
        const isActive = t.id === active
        return (
          <button
            key={t.id}
            type="button"
            onClick={() => onChange(t.id)}
            className={`-mb-px border-b-2 px-3 py-2 text-sm font-medium transition ${
              isActive
                ? 'border-brand-500 text-brand-600 dark:text-brand-400'
                : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
            }`}
          >
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
