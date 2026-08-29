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
  X,
} from 'lucide-react'
import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react'
import { navGroups } from '../../config/app.config'
import { api, fetchAllEnvironments } from '../../lib/api'
import { buildGlobalSearchHits, type SearchTarget } from '../../lib/global-search'

const GROUP_ICON: Record<string, LucideIcon> = {
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

export function GlobalSearch() {
  const nav = useNavigate()
  const inputRef = useRef<HTMLInputElement>(null)
  const rootRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const listId = useId()
  const [q, setQ] = useState('')
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const [edge, setEdge] = useState({ top: false, bottom: false })

  const live = open || q.trim().length > 0
  const needle = q.trim()

  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects, enabled: live })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments, enabled: live })
  const apps = useQuery({ queryKey: ['applications'], queryFn: () => api.applications(), enabled: live })
  const dbs = useQuery({ queryKey: ['databases'], queryFn: () => api.databases(), enabled: live })
  const svcs = useQuery({ queryKey: ['services'], queryFn: () => api.services(), enabled: live })
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers, enabled: live })
  const sources = useQuery({ queryKey: ['git-sources'], queryFn: api.gitSources, enabled: live })
  const destinations = useQuery({ queryKey: ['destinations'], queryFn: api.destinations, enabled: live })
  const storages = useQuery({ queryKey: ['s3-storages'], queryFn: api.s3Storages, enabled: live })
  const tags = useQuery({ queryKey: ['tags'], queryFn: api.tags, enabled: live })

  const resourceQueries = [
    projects,
    envs,
    apps,
    dbs,
    svcs,
    servers,
    sources,
    destinations,
    storages,
    tags,
  ]
  const loading = needle.length > 0 && resourceQueries.some((q) => q.isPending || q.isFetching)

  const hits = useMemo(
    () =>
      buildGlobalSearchHits({
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
      }),
    [
      q,
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
    ],
  )

  const go = (target: SearchTarget) => {
    switch (target.kind) {
      case 'href':
        return void nav({ to: target.href })
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

  const showList = open && (hits.length > 0 || needle.length > 0)

  const syncEdges = () => {
    const el = listRef.current
    if (!el) return
    setEdge({
      top: el.scrollTop > 6,
      bottom: el.scrollTop + el.clientHeight < el.scrollHeight - 6,
    })
  }

  useEffect(() => {
    setActive(0)
  }, [needle, hits.length])

  useLayoutEffect(() => {
    if (!showList) return
    syncEdges()
  }, [showList, hits.length])

  useEffect(() => {
    if (!open) return
    const el = listRef.current?.querySelector('[aria-selected="true"]')
    el?.scrollIntoView({ block: 'nearest' })
    syncEdges()
  }, [active, open])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        inputRef.current?.focus()
        inputRef.current?.select()
        setOpen(true)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const run = (target: SearchTarget) => {
    go(target)
    setQ('')
    setOpen(false)
    inputRef.current?.blur()
  }

  const onKeyDown = (e: ReactKeyboardEvent<HTMLInputElement>) => {
    if (e.nativeEvent.isComposing) return
    if (e.key === 'Escape') {
      if (q) {
        setQ('')
        return
      }
      setOpen(false)
      inputRef.current?.blur()
      return
    }
    if (!hits.length) return
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setOpen(true)
      setActive((i) => (i + 1) % hits.length)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setOpen(true)
      setActive((i) => (i - 1 + hits.length) % hits.length)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const hit = hits[active]
      if (hit) run(hit.target)
    }
  }

  let lastGroup = ''
  const pageIcon = (href: string) => navGroups.flatMap((g) => g.items).find((i) => i.href === href)?.icon

  return (
    <div ref={rootRef} className="relative min-w-0 flex-1 lg:max-w-md">
      <Search className="pointer-events-none absolute top-1/2 left-2.5 z-10 h-3.5 w-3.5 -translate-y-1/2 text-gray-400" />
      <input
        ref={inputRef}
        type="text"
        role="combobox"
        aria-label="Search"
        aria-autocomplete="list"
        aria-expanded={showList}
        aria-controls={listId}
        aria-activedescendant={showList && hits[active] ? `${listId}-${hits[active].id}` : undefined}
        value={q}
        onChange={(e) => {
          setQ(e.target.value)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        onKeyDown={onKeyDown}
        placeholder="Search…"
        autoComplete="off"
        spellCheck={false}
        className="h-8 w-full rounded-md border border-gray-200 bg-gray-50 py-1.5 pr-16 pl-9 text-xs text-gray-900 outline-none transition placeholder:text-gray-500 focus:border-brand-400 focus:bg-white focus:ring-2 focus:ring-brand-500/20 dark:border-gray-800 dark:bg-white/5 dark:text-white dark:placeholder:text-gray-400 dark:focus:bg-gray-900"
      />
      {q ? (
        <button
          type="button"
          onClick={() => {
            setQ('')
            inputRef.current?.focus()
          }}
          className="absolute top-1/2 right-1.5 flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded text-gray-400 hover:bg-gray-200 hover:text-gray-600 dark:hover:bg-white/10 dark:hover:text-gray-200"
          aria-label="Clear search"
        >
          <X className="h-3 w-3" />
        </button>
      ) : (
        <span className="pointer-events-none absolute top-1/2 right-2.5 hidden -translate-y-1/2 text-[10px] text-gray-400 sm:inline">
          ⌘K
        </span>
      )}

      {showList && (
        <div className="absolute top-full z-50 mt-1.5 w-full overflow-hidden rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-800 dark:bg-gray-900">
          <div
            ref={listRef}
            id={listId}
            role="listbox"
            onScroll={syncEdges}
            className="search-scrollbar max-h-[min(22rem,60vh)] overflow-y-auto py-1"
          >
            {hits.length === 0 && (
              <p className="px-3 py-2.5 text-xs text-gray-500 dark:text-gray-400">
                {loading ? 'Searching…' : `No results for “${needle}”.`}
              </p>
            )}
            {hits.map((hit, i) => {
              const showGroup = hit.group !== lastGroup
              lastGroup = hit.group
              const Icon =
                hit.target.kind === 'href'
                  ? pageIcon(hit.target.href) || GROUP_ICON.Pages
                  : GROUP_ICON[hit.group] || Search
              return (
                <div key={hit.id}>
                  {showGroup && (
                    <p className="sticky top-0 z-10 bg-white/95 px-3 pt-2 pb-1 text-[10px] font-semibold tracking-wider text-gray-400 uppercase backdrop-blur dark:bg-gray-900/95">
                      {hit.group}
                    </p>
                  )}
                  <button
                    type="button"
                    id={`${listId}-${hit.id}`}
                    role="option"
                    aria-selected={i === active}
                    onMouseEnter={() => setActive(i)}
                    onClick={() => run(hit.target)}
                    className={`flex w-full items-center gap-2.5 px-3 py-2 pr-4 text-left text-sm ${
                      i === active
                        ? 'bg-gray-50 text-gray-900 dark:bg-white/10 dark:text-white'
                        : 'text-gray-700 dark:text-gray-200'
                    }`}
                  >
                    <Icon className="h-4 w-4 shrink-0 text-gray-400" strokeWidth={1.75} />
                    <span className="min-w-0 flex-1 truncate">{hit.name}</span>
                    {hit.hint && (
                      <span className="max-w-[40%] truncate text-[11px] text-gray-400">{hit.hint}</span>
                    )}
                  </button>
                </div>
              )
            })}
          </div>
          {edge.top && (
            <div
              className="pointer-events-none absolute inset-x-0 top-0 h-6 bg-gradient-to-b from-white to-transparent dark:from-gray-900"
              aria-hidden
            />
          )}
          {edge.bottom && (
            <div
              className="pointer-events-none absolute inset-x-0 bottom-0 h-6 bg-gradient-to-t from-white to-transparent dark:from-gray-900"
              aria-hidden
            />
          )}
        </div>
      )}
    </div>
  )
}
