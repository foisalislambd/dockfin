import { useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import type { LucideIcon } from 'lucide-react'
import {
  Box,
  Database,
  FolderKanban,
  GitBranch,
  Layers,
  Rocket,
  Search,
  Server,
  Tags,
  Waypoints,
  X,
} from 'lucide-react'
import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react'
import { navGroups } from '../../config/app.config'
import { api, fetchAllEnvironments } from '../../lib/api'

type Hit = {
  id: string
  group: string
  name: string
  hint?: string
  icon: LucideIcon
  go: () => void
}

function matches(needle: string, ...parts: Array<string | undefined>) {
  if (!needle) return true
  return parts.some((p) => (p || '').toLowerCase().includes(needle))
}

export function GlobalSearch() {
  const nav = useNavigate()
  const inputRef = useRef<HTMLInputElement>(null)
  const rootRef = useRef<HTMLDivElement>(null)
  const listId = useId()
  const [q, setQ] = useState('')
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)

  const live = open || q.trim().length > 0
  const needle = q.trim().toLowerCase()

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

  const envById = useMemo(() => {
    const m = new Map<string, { project_id: string; name: string; project_name: string }>()
    for (const e of envs.data || []) {
      m.set(e.id, { project_id: e.project_id, name: e.name, project_name: e.project_name })
    }
    return m
  }, [envs.data])

  const hits = useMemo(() => {
    const out: Hit[] = []

    for (const group of navGroups) {
      for (const item of group.items) {
        if (!matches(needle, item.name, group.label, item.href)) continue
        out.push({
          id: `page:${item.href}`,
          group: 'Pages',
          name: item.name,
          hint: group.label,
          icon: item.icon,
          go: () => void nav({ to: item.href }),
        })
      }
    }

    if (!needle) return out

    for (const p of projects.data?.projects || []) {
      if (!matches(needle, p.name, p.description)) continue
      out.push({
        id: `project:${p.id}`,
        group: 'Projects',
        name: p.name,
        hint: p.description || undefined,
        icon: FolderKanban,
        go: () => void nav({ to: '/projects/$projectId', params: { projectId: p.id } }),
      })
    }

    for (const e of envs.data || []) {
      if (!matches(needle, e.name, e.project_name)) continue
      out.push({
        id: `env:${e.id}`,
        group: 'Environments',
        name: e.name,
        hint: e.project_name,
        icon: Layers,
        go: () =>
          void nav({
            to: '/projects/$projectId/environments/$envId',
            params: { projectId: e.project_id, envId: e.id },
          }),
      })
    }

    for (const a of apps.data?.applications || []) {
      const env = envById.get(a.environment_id)
      if (!env || !matches(needle, a.name, a.description, a.fqdn, a.git_repository)) continue
      out.push({
        id: `app:${a.id}`,
        group: 'Applications',
        name: a.name,
        hint: env.project_name,
        icon: Rocket,
        go: () =>
          void nav({
            to: '/projects/$projectId/environments/$envId/applications/$appId',
            params: { projectId: env.project_id, envId: a.environment_id, appId: a.id },
          }),
      })
    }

    for (const d of dbs.data?.databases || []) {
      const env = envById.get(d.environment_id)
      if (!env || !matches(needle, d.name, d.description, d.engine)) continue
      out.push({
        id: `db:${d.id}`,
        group: 'Databases',
        name: d.name,
        hint: d.engine || env.project_name,
        icon: Database,
        go: () =>
          void nav({
            to: '/projects/$projectId/environments/$envId/databases/$dbId',
            params: { projectId: env.project_id, envId: d.environment_id, dbId: d.id },
          }),
      })
    }

    for (const s of svcs.data?.services || []) {
      const env = envById.get(s.environment_id)
      if (!env || !matches(needle, s.name, s.description, s.service_type, s.fqdn)) continue
      out.push({
        id: `svc:${s.id}`,
        group: 'Services',
        name: s.name,
        hint: s.service_type || env.project_name,
        icon: Box,
        go: () =>
          void nav({
            to: '/projects/$projectId/environments/$envId/services/$svcId',
            params: { projectId: env.project_id, envId: s.environment_id, svcId: s.id },
          }),
      })
    }

    for (const s of servers.data?.servers || []) {
      if (!matches(needle, s.name, s.ip, s.public_ip, s.description)) continue
      out.push({
        id: `server:${s.id}`,
        group: 'Servers',
        name: s.name,
        hint: s.ip || s.public_ip,
        icon: Server,
        go: () => void nav({ to: '/servers/$serverId', params: { serverId: s.id } }),
      })
    }

    for (const g of sources.data?.git_sources || []) {
      if (!matches(needle, g.name, g.provider, g.organization)) continue
      out.push({
        id: `git:${g.id}`,
        group: 'Sources',
        name: g.name,
        hint: g.provider,
        icon: GitBranch,
        go: () => void nav({ to: '/git-sources/$sourceId', params: { sourceId: g.id } }),
      })
    }

    for (const d of destinations.data?.destinations || []) {
      if (!matches(needle, d.name, d.network, d.kind)) continue
      out.push({
        id: `dest:${d.id}`,
        group: 'Destinations',
        name: d.name,
        hint: d.kind || d.network,
        icon: Waypoints,
        go: () =>
          void nav({
            to: '/servers/$serverId',
            params: { serverId: d.server_id },
            search: { tab: 'destinations' },
          }),
      })
    }

    for (const s of storages.data?.s3_storages || []) {
      if (!matches(needle, s.name, s.bucket, s.endpoint)) continue
      out.push({
        id: `s3:${s.id}`,
        group: 'Storages',
        name: s.name,
        hint: s.bucket,
        icon: Box,
        go: () => void nav({ to: '/storages' }),
      })
    }

    for (const t of tags.data?.tags || []) {
      if (!matches(needle, t.name)) continue
      out.push({
        id: `tag:${t.id}`,
        group: 'Tags',
        name: t.name,
        icon: Tags,
        go: () => void nav({ to: '/tags' }),
      })
    }

    return out.slice(0, 40)
  }, [
    needle,
    nav,
    projects.data,
    envs.data,
    envById,
    apps.data,
    dbs.data,
    svcs.data,
    servers.data,
    sources.data,
    destinations.data,
    storages.data,
    tags.data,
  ])

  const loading =
    live &&
    (projects.isLoading ||
      envs.isLoading ||
      apps.isLoading ||
      dbs.isLoading ||
      svcs.isLoading ||
      servers.isLoading)

  useEffect(() => {
    setActive(0)
  }, [needle, hits.length])

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

  const run = (hit: Hit) => {
    hit.go()
    setQ('')
    setOpen(false)
    inputRef.current?.blur()
  }

  const onKeyDown = (e: ReactKeyboardEvent<HTMLInputElement>) => {
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
      if (hit) run(hit)
    }
  }

  const showList = open && (hits.length > 0 || needle.length > 0)
  let lastGroup = ''

  return (
    <div ref={rootRef} className="relative min-w-0 flex-1 lg:max-w-md">
      <Search className="pointer-events-none absolute top-1/2 left-2.5 z-10 h-3.5 w-3.5 -translate-y-1/2 text-gray-400" />
      <input
        ref={inputRef}
        type="text"
        role="combobox"
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
        <div
          id={listId}
          role="listbox"
          className="absolute top-full z-50 mt-1.5 max-h-[min(24rem,70vh)] w-full overflow-y-auto rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-gray-800 dark:bg-gray-900"
        >
          {hits.length === 0 && (
            <p className="px-3 py-2.5 text-xs text-gray-500 dark:text-gray-400">
              {loading ? 'Searching…' : `No results for “${q.trim()}”.`}
            </p>
          )}
          {hits.map((hit, i) => {
            const showGroup = hit.group !== lastGroup
            lastGroup = hit.group
            const Icon = hit.icon
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
                  onClick={() => run(hit)}
                  className={`flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm ${
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
      )}
    </div>
  )
}
