import { ChevronLeft, ChevronRight, type LucideIcon } from 'lucide-react'
import { useState, type ReactNode } from 'react'

const RESOURCE_NAV_KEY = 'dockfin.resourceNavOpen'

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
  const [open, setOpen] = useState(() => {
    try {
      return localStorage.getItem(RESOURCE_NAV_KEY) !== '0'
    } catch {
      return true
    }
  })
  const byId = new Map(items.map((i) => [i.id, i]))

  const togglePanel = () => {
    setOpen((current) => {
      const next = !current
      try {
        localStorage.setItem(RESOURCE_NAV_KEY, next ? '1' : '0')
      } catch {
        /* ignore private-mode storage failures */
      }
      return next
    })
  }

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
            : 'font-medium text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-white/8'
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
          <p className="mb-1 px-2 text-[11px] font-semibold tracking-wider text-gray-400 uppercase dark:text-gray-500">
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
    <aside
      className={`panel-subnav flex w-full shrink-0 flex-col border-r border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900 ${open ? '' : 'is-collapsed'}`}
    >
      {open ? (
        <div className="panel-scrollbar min-h-0 flex-1 overflow-y-auto p-2">
          {header}
          {body}
        </div>
      ) : (
        <div className="panel-scrollbar flex min-h-0 flex-1 flex-col items-center gap-1 overflow-y-auto px-1.5 py-2">
          {(grouped
            ? [{ label: 'pinned', list: pinnedItems }, ...grouped]
            : [{ label: 'all', list: [...items] }]
          )
            .filter((g) => g.list.length > 0)
            .map((g, index) => (
              <div
                key={g.label}
                className={`flex flex-col items-center gap-1 ${index > 0 ? 'mt-1 border-t border-gray-200 pt-1 dark:border-gray-800' : ''}`}
              >
                {g.list.map((item) => {
                  const Icon = item.icon
                  const isActive = active === item.id
                  return (
                    <button
                      key={item.id}
                      type="button"
                      title={item.label}
                      aria-label={item.label}
                      aria-current={isActive ? 'page' : undefined}
                      onClick={() => onSelect(item.id)}
                      className={`flex h-8 w-8 items-center justify-center rounded-lg ${
                        isActive
                          ? 'bg-brand-500 text-white'
                          : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-white/8'
                      }`}
                    >
                      <Icon className="h-4 w-4" aria-hidden />
                    </button>
                  )
                })}
              </div>
            ))}
        </div>
      )}
      <div className="shrink-0 border-t border-gray-200 p-2.5 dark:border-gray-800">
        <button
          type="button"
          onClick={togglePanel}
          className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-xs font-medium text-gray-500 hover:bg-gray-50 dark:text-gray-400 dark:hover:bg-white/5 ${open ? '' : 'justify-center px-0'}`}
          aria-label={open ? 'Hide resource menu' : 'Show resource menu'}
          title={open ? 'Collapse' : 'Expand'}
        >
          {open ? (
            <>
              <ChevronLeft className="h-4 w-4 shrink-0" />
              <span>Collapse</span>
            </>
          ) : (
            <ChevronRight className="h-4 w-4" />
          )}
        </button>
      </div>
    </aside>
  )
}
