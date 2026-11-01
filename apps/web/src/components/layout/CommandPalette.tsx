import { useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import type { LucideIcon } from 'lucide-react'
import {
  Box,
  Database,
  FolderKanban,
  GitBranch,
  Layers,
  LayoutDashboard,
  Rocket,
  Search,
  Server,
  Tags,
  Waypoints,
} from 'lucide-react'
import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { navGroups, navItems } from '../../config/app.config'
import { api, fetchAllEnvironments } from '../../lib/api'
import { buildGlobalSearchHits, type SearchHit, type SearchTarget } from '../../lib/global-search'
import { useConsoleUi } from './console-ui'

const GROUP_ICON: Record<string, LucideIcon> = {
  Actions: Rocket,
  Pages: LayoutDashboard,
  Projects: FolderKanban,
  Environments: Layers,
  Applications: Rocket,
  Databases: Database,
  Services: Box,
  Servers: Server,
  Sources: GitBranch,
  Destinations: Waypoints,
  Storages: Box,
  Tags: Tags,
}

const ACTIONS: SearchHit[] = [
  {
    id: 'action:add-server',
    group: 'Actions',
    name: 'Add server',
    hint: 'SSH host',
    target: { kind: 'href', href: '/servers/new' },
  },
  {
    id: 'action:new-project',
    group: 'Actions',
    name: 'New project',
    hint: 'Then deploy an app or service',
    target: { kind: 'href', href: '/projects?new=1' },
  },
  {
    id: 'action:catalog',
    group: 'Actions',
    name: 'One-click services',
    hint: 'Open a project, then add a service',
    target: { kind: 'href', href: '/projects' },
  },
]

function shortcutFor(href: string) {
  return navItems.find((item) => item.href === href)?.shortcut
}

/**
 * Command palette. Layout (centered dialog, grouped results, go-to hints) follows the
 * Antirion console interaction model; the search index and Dockfin routes are original.
 */
export function CommandPalette() {
  const { paletteOpen, setPaletteOpen } = useConsoleUi()
  const nav = useNavigate()
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const listId = useId()
  const [q, setQ] = useState('')
  const [active, setActive] = useState(0)

  const live = paletteOpen
  const needle = q.trim().toLowerCase()

  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects, enabled: live })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments, enabled: live && needle.length > 0 })
  const apps = useQuery({ queryKey: ['applications'], queryFn: () => api.applications(), enabled: live && needle.length > 0 })
  const dbs = useQuery({ queryKey: ['databases'], queryFn: () => api.databases(), enabled: live && needle.length > 0 })
  const svcs = useQuery({ queryKey: ['services'], queryFn: () => api.services(), enabled: live && needle.length > 0 })
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers, enabled: live && needle.length > 0 })
  const sources = useQuery({ queryKey: ['git-sources'], queryFn: api.gitSources, enabled: live && needle.length > 0 })
  const destinations = useQuery({ queryKey: ['destinations'], queryFn: api.destinations, enabled: live && needle.length > 0 })
  const storages = useQuery({ queryKey: ['s3-storages'], queryFn: api.s3Storages, enabled: live && needle.length > 0 })
  const tags = useQuery({ queryKey: ['tags'], queryFn: api.tags, enabled: live && needle.length > 0 })

  const loading =
    needle.length > 0 &&
    [projects, envs, apps, dbs, svcs, servers, sources, destinations, storages, tags].some(
      (query) => query.isPending || query.isFetching,
    )

  const hits = useMemo(() => {
    const pages = buildGlobalSearchHits({
      query: q,
      navGroups,
      projects: projects.data?.projects || [],
      environments: envs.data || [],
      applications: apps.data?.applications || [],
      databases: dbs.data?.databases || [],
      services: svcs.data?.services || [],
      servers: servers.data?.servers || [],
      gitSources: sources.data?.git_sources || [],
      destinations: destinations.data?.destinations || [],
      storages: storages.data?.s3_storages || [],
      tags: tags.data?.tags || [],
    })
    const actions = ACTIONS.filter((action) => {
      if (!needle) return true
      return `${action.name} ${action.hint || ''}`.toLowerCase().includes(needle)
    })
    return [...actions, ...pages]
  }, [
    q,
    needle,
    projects.data,
    envs.data,
    apps.data,
    dbs.data,
    svcs.data,
    servers.data,
    sources.data,
    destinations.data,
    storages.data,
    tags.data,
  ])

  useEffect(() => {
    setActive(0)
  }, [needle, hits.length])

  useEffect(() => {
    if (!paletteOpen) return
    const id = window.setTimeout(() => inputRef.current?.focus(), 0)
    return () => window.clearTimeout(id)
  }, [paletteOpen])

  useEffect(() => {
    if (!paletteOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        setPaletteOpen(false)
        setQ('')
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [paletteOpen, setPaletteOpen])

  useEffect(() => {
    if (!paletteOpen) return
    listRef.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' })
  }, [active, paletteOpen])

  if (!paletteOpen) return null

  const go = (target: SearchTarget) => {
    setPaletteOpen(false)
    setQ('')
    if (target.kind === 'href') {
      const [path, qs] = target.href.split('?')
      if (path === '/projects' && qs) {
        void nav({ to: '/projects', search: { new: '1' } })
        return
      }
      void nav({ to: path })
      return
    }
    switch (target.kind) {
      case 'project':
        return void nav({ to: '/projects/$projectId', params: { projectId: target.projectId } })
      case 'environment':
        return void nav({
          to: '/projects/$projectId/environments/$envId',
          params: { projectId: target.projectId, envId: target.envId },
        })
      case 'application':
        return target.projectId && target.envId
          ? void nav({
              to: '/projects/$projectId/environments/$envId/applications/$appId',
              params: { projectId: target.projectId, envId: target.envId, appId: target.appId },
            })
          : void nav({ to: '/applications/$appId', params: { appId: target.appId } })
      case 'database':
        return target.projectId && target.envId
          ? void nav({
              to: '/projects/$projectId/environments/$envId/databases/$dbId',
              params: { projectId: target.projectId, envId: target.envId, dbId: target.dbId },
            })
          : void nav({ to: '/databases/$dbId', params: { dbId: target.dbId } })
      case 'service':
        return target.projectId && target.envId
          ? void nav({
              to: '/projects/$projectId/environments/$envId/services/$svcId',
              params: { projectId: target.projectId, envId: target.envId, svcId: target.svcId },
            })
          : void nav({ to: '/services/$svcId', params: { svcId: target.svcId } })
      case 'server':
        return void nav({ to: '/servers/$serverId', params: { serverId: target.serverId } })
      case 'git-source':
        return void nav({ to: '/git-sources/$sourceId', params: { sourceId: target.sourceId } })
      case 'destination':
        return void nav({
          to: '/servers/$serverId',
          params: { serverId: target.serverId },
          search: { tab: 'destinations' },
        })
      case 'storages':
        return void nav({ to: '/storages' })
      case 'tags':
        return void nav({ to: '/tags' })
    }
  }

  const onInputKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.nativeEvent.isComposing) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActive((i) => (hits.length ? (i + 1) % hits.length : 0))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActive((i) => (hits.length ? (i - 1 + hits.length) % hits.length : 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const hit = hits[active]
      if (hit) go(hit.target)
    }
  }

  let lastGroup = ''
  const pageIcon = (href: string) => navItems.find((item) => item.href === href.split('?')[0])?.icon

  return (
    <div className="fixed inset-0 z-[80] flex items-start justify-center px-3 pt-[12vh]" role="presentation">
      <button
        type="button"
        className="absolute inset-0 bg-gray-950/40 backdrop-blur-[1px]"
        aria-label="Close search"
        onClick={() => {
          setPaletteOpen(false)
          setQ('')
        }}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Search"
        className="relative z-10 flex w-full max-w-xl flex-col overflow-hidden rounded-xl border border-gray-200 bg-white shadow-2xl dark:border-gray-800 dark:bg-gray-900"
      >
        <div className="flex items-center gap-2 border-b border-gray-200 px-3 dark:border-gray-800">
          <Search className="h-4 w-4 shrink-0 text-gray-400" />
          <input
            ref={inputRef}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={onInputKey}
            role="combobox"
            aria-expanded
            aria-controls={listId}
            aria-activedescendant={hits[active] ? `${listId}-${hits[active].id}` : undefined}
            placeholder="Search or jump to…"
            className="h-12 w-full bg-transparent text-sm text-gray-900 outline-none placeholder:text-gray-400 dark:text-white"
            autoComplete="off"
            spellCheck={false}
          />
        </div>
        <div ref={listRef} id={listId} role="listbox" className="max-h-[min(24rem,55vh)] overflow-y-auto py-1">
          {hits.length === 0 && (
            <p className="px-3 py-6 text-center text-sm text-gray-500">
              {loading ? 'Searching…' : `No results for “${q.trim()}”.`}
            </p>
          )}
          {hits.map((hit, i) => {
            const showGroup = hit.group !== lastGroup
            lastGroup = hit.group
            const Icon =
              hit.target.kind === 'href'
                ? pageIcon(hit.target.href) || GROUP_ICON.Pages
                : GROUP_ICON[hit.group] || Search
            const keys =
              hit.target.kind === 'href' ? shortcutFor(hit.target.href.split('?')[0]) : undefined
            return (
              <div key={hit.id}>
                {showGroup && (
                  <p className="px-3 pt-2 pb-1 text-[10px] font-semibold tracking-wider text-gray-400 uppercase">
                    {hit.group}
                  </p>
                )}
                <button
                  type="button"
                  id={`${listId}-${hit.id}`}
                  role="option"
                  aria-selected={i === active}
                  onMouseEnter={() => setActive(i)}
                  onClick={() => go(hit.target)}
                  className={`flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm ${
                    i === active
                      ? 'bg-brand-50 text-gray-900 dark:bg-brand-500/15 dark:text-white'
                      : 'text-gray-700 dark:text-gray-200'
                  }`}
                >
                  <Icon className="h-4 w-4 shrink-0 text-gray-400" strokeWidth={1.75} />
                  <span className="min-w-0 flex-1 truncate">{hit.name}</span>
                  {hit.hint ? (
                    <span className="hidden max-w-[40%] truncate text-xs text-gray-400 sm:inline">{hit.hint}</span>
                  ) : null}
                  {keys ? (
                    <span className="ml-2 shrink-0 font-mono text-[10px] text-gray-400">g {keys}</span>
                  ) : null}
                </button>
              </div>
            )
          })}
        </div>
        <div className="flex items-center gap-3 border-t border-gray-200 px-3 py-2 text-[11px] text-gray-400 dark:border-gray-800">
          <span>↑↓ move</span>
          <span>↵ open</span>
          <span>esc close</span>
        </div>
      </div>
    </div>
  )
}

export function ShortcutsDialog() {
  const { shortcutsOpen, setShortcutsOpen } = useConsoleUi()
  useEffect(() => {
    if (!shortcutsOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        setShortcutsOpen(false)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [shortcutsOpen, setShortcutsOpen])

  if (!shortcutsOpen) return null

  return (
    <div className="fixed inset-0 z-[80] flex items-start justify-center px-3 pt-[14vh]">
      <button
        type="button"
        className="absolute inset-0 bg-gray-950/40"
        aria-label="Close shortcuts"
        onClick={() => setShortcutsOpen(false)}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="shortcuts-title"
        className="relative z-10 w-full max-w-md overflow-hidden rounded-xl border border-gray-200 bg-white shadow-2xl dark:border-gray-800 dark:bg-gray-900"
      >
        <div className="border-b border-gray-200 px-4 py-3 dark:border-gray-800">
          <h2 id="shortcuts-title" className="text-sm font-semibold text-gray-900 dark:text-white">
            Keyboard shortcuts
          </h2>
        </div>
        <ul className="max-h-[50vh] overflow-y-auto px-4 py-2 text-sm">
          <ShortcutRow keys="⌘K" label="Search or jump" />
          <ShortcutRow keys="/" label="Open search" />
          <ShortcutRow keys="?" label="Show this list" />
          {navItems
            .filter((item) => item.shortcut)
            .map((item) => (
              <ShortcutRow key={item.href} keys={`g ${item.shortcut}`} label={item.name} />
            ))}
        </ul>
      </div>
    </div>
  )
}

function ShortcutRow({ keys, label }: { keys: string; label: string }) {
  return (
    <li className="flex items-center justify-between gap-3 border-b border-gray-100 py-2 last:border-0 dark:border-gray-800">
      <span className="text-gray-700 dark:text-gray-200">{label}</span>
      <kbd className="rounded border border-gray-200 bg-gray-50 px-1.5 py-0.5 font-mono text-[11px] text-gray-500 dark:border-gray-700 dark:bg-white/5">
        {keys}
      </kbd>
    </li>
  )
}

export function PaletteButton() {
  const { setPaletteOpen } = useConsoleUi()
  return (
    <button
      type="button"
      onClick={() => setPaletteOpen(true)}
      className="hidden h-8 min-w-0 flex-1 items-center gap-2 rounded-md border border-gray-200 bg-gray-50 px-2.5 text-left text-xs text-gray-400 hover:border-gray-300 hover:bg-white md:flex dark:border-gray-800 dark:bg-white/5 dark:hover:border-gray-700 dark:hover:bg-gray-900"
      aria-label="Search"
    >
      <Search className="h-3.5 w-3.5 shrink-0" />
      <span className="flex-1 truncate">Search or jump to…</span>
      <kbd className="rounded border border-gray-200 bg-white px-1 py-0.5 font-mono text-[10px] text-gray-400 dark:border-gray-700 dark:bg-gray-900">
        ⌘K
      </kbd>
    </button>
  )
}
