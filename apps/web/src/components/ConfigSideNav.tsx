import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

export type SideNavItem<T extends string> = {
  id: T
  label: string
  icon: LucideIcon
  badge?: number
  /** Small status pip (for example a deploy in progress). */
  dot?: boolean
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
  pinned,
}: {
  items: readonly SideNavItem<T>[]
  groups?: readonly SideNavGroup<T>[]
  active: T
  onSelect: (id: T) => void
  header?: ReactNode
  /** Items shown above the first group (Overview). */
  pinned?: readonly T[]
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
        className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] leading-5 ${
          isActive
            ? 'bg-brand-500 font-semibold text-white shadow-sm'
            : 'font-medium text-gray-800 hover:bg-gray-100 dark:text-gray-100 dark:hover:bg-white/10'
        }`}
      >
        <Icon
          className={`h-4 w-4 shrink-0 ${
            isActive ? 'text-white' : 'text-gray-600 dark:text-gray-300'
          }`}
          aria-hidden
        />
        <span className="min-w-0 flex-1 truncate">{item.label}</span>
        {item.badge ? (
          <span className="inline-flex min-w-4 justify-center rounded-full bg-amber-500/20 px-1.5 text-[10px] font-semibold text-amber-800 dark:text-amber-300">
            {item.badge}
          </span>
        ) : null}
        {item.dot ? <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-amber-500" aria-hidden /> : null}
      </button>
    )
  }

  const pinnedItems = (pinned || []).map((id) => byId.get(id)).filter((x): x is SideNavItem<T> => Boolean(x))

  const grouped = groups?.length
    ? groups
        .map((g) => {
          const list = g.ids.map((id) => byId.get(id)).filter((x): x is SideNavItem<T> => Boolean(x))
          return { label: g.label, list }
        })
        .filter((g) => g.list.length > 0)
    : null

  const body = grouped ? (
    <div className="space-y-2.5">
      {pinnedItems.length ? <nav className="space-y-0.5">{pinnedItems.map(renderItem)}</nav> : null}
      {grouped.map((g) => (
        <div key={g.label} className="border-t border-gray-200 pt-2 first:border-t-0 first:pt-0 dark:border-gray-700">
          <p className="mb-1 px-2 text-[11px] font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400">
            {g.label}
          </p>
          <nav className="space-y-0.5">{g.list.map(renderItem)}</nav>
        </div>
      ))}
    </div>
  ) : (
    <nav className="space-y-0.5">{items.map(renderItem)}</nav>
  )

  return (
    <aside className="panel-card panel-scrollbar panel-subnav w-full shrink-0 overflow-y-auto p-2">
      {header}
      {body}
    </aside>
  )
}
