import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

export type SideNavItem<T extends string> = {
  id: T
  label: string
  icon: LucideIcon
  badge?: number
}

export type SideNavGroup<T extends string> = {
  label: string
  ids: readonly T[]
}

export function ConfigSideNav<T extends string>({
  items,
  groups,
  active,
  onSelect,
  header,
}: {
  items: readonly SideNavItem<T>[]
  groups?: readonly SideNavGroup<T>[]
  active: T
  onSelect: (id: T) => void
  header?: ReactNode
}) {
  const byId = new Map(items.map((i) => [i.id, i]))

  const renderItem = (item: SideNavItem<T>) => {
    const Icon = item.icon
    const isActive = active === item.id
    return (
      <button
        key={item.id}
        type="button"
        onClick={() => onSelect(item.id)}
        className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm ${
          isActive
            ? 'bg-brand-50 font-medium text-brand-700 dark:bg-brand-500/15 dark:text-brand-300'
            : 'text-gray-600 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-white/5'
        }`}
      >
        <Icon
          className={`h-3.5 w-3.5 shrink-0 ${
            isActive ? 'text-brand-600 dark:text-brand-400' : 'text-gray-400 dark:text-gray-500'
          }`}
          aria-hidden
        />
        <span className="min-w-0 flex-1 truncate">{item.label}</span>
        {item.badge ? (
          <span className="inline-flex min-w-4 justify-center rounded-full bg-amber-500/20 px-1.5 text-[10px] font-semibold text-amber-800 dark:text-amber-300">
            {item.badge}
          </span>
        ) : null}
      </button>
    )
  }

  const body = groups?.length ? (
    <div className="space-y-4">
      {groups.map((g) => {
        const list = g.ids.map((id) => byId.get(id)).filter((x): x is SideNavItem<T> => Boolean(x))
        if (!list.length) return null
        return (
          <div key={g.label}>
            <p className="mb-1 px-2 text-[10px] font-semibold tracking-wider text-gray-400 uppercase dark:text-gray-500">
              {g.label}
            </p>
            <nav className="space-y-0.5">{list.map(renderItem)}</nav>
          </div>
        )
      })}
    </div>
  ) : (
    <nav className="space-y-0.5">{items.map(renderItem)}</nav>
  )

  return (
    <aside className="w-full shrink-0 md:w-56">
      {header}
      {body}
    </aside>
  )
}
