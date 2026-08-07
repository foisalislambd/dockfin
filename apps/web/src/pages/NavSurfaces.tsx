import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { ServerTerminal } from '../components/Terminal'
import { PageSkeleton, TableSkeleton } from '../components/ui/Skeleton'
import { api, type Server, type Tag } from '../lib/api'
import { Header } from './Servers'

/** All destinations across every server, deep-linking to the server's Destinations tab. */
export function DestinationsPage() {
  const destinations = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers })
  const [search, setSearch] = useState('')

  const serverById = useMemo(() => {
    const m = new Map<string, Server>()
    for (const s of servers.data?.servers || []) m.set(s.id, s)
    return m
  }, [servers.data])

  const rows = useMemo(() => {
    const list = destinations.data?.destinations || []
    const q = search.trim().toLowerCase()
    return list
      .map((d) => ({ ...d, serverName: serverById.get(d.server_id)?.name || d.server_id }))
      .filter(
        (d) =>
          !q ||
          d.name.toLowerCase().includes(q) ||
          d.serverName.toLowerCase().includes(q) ||
          (d.kind || '').toLowerCase().includes(q),
      )
  }, [destinations.data, serverById, search])

  if (destinations.isLoading || servers.isLoading) return <PageSkeleton cards={2} />

  return (
    <div className="space-y-6">
      <Header title="Destinations" />
      <div className="relative max-w-md">
        <Search className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400" />
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search by name, server, kind…"
          className="panel-field h-9 w-full rounded-md py-1.5 pr-3 pl-9 text-sm"
        />
      </div>
      <div className="panel-card overflow-hidden">
        <table className="panel-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Kind</th>
              <th>Network</th>
              <th>Server</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((d) => (
              <tr key={d.id}>
                <td className="font-medium text-gray-900 dark:text-white">{d.name}</td>
                <td className="text-gray-600 dark:text-gray-300">{d.kind || 'standalone'}</td>
                <td className="font-mono text-xs text-gray-500 dark:text-gray-400">{d.network}</td>
                <td>
                  <Link
                    to="/servers/$serverId"
                    params={{ serverId: d.server_id }}
                    search={{ tab: 'destinations' }}
                    className="text-brand-600 hover:underline dark:text-brand-400"
                  >
                    {d.serverName}
                  </Link>
                </td>
                <td>
                  <Link
                    to="/servers/$serverId"
                    params={{ serverId: d.server_id }}
                    search={{ tab: 'destinations' }}
                    className="text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
                  >
                    Open
                  </Link>
                </td>
              </tr>
            ))}
            {!rows.length && (
              <tr>
                <td colSpan={5} className="panel-table-empty">
                  {search ? 'No destinations match your search.' : 'No destinations yet.'}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function TagChipStatic({ tag, active, onClick }: { tag: Tag; active: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs transition ${
        active
          ? 'border-brand-500 bg-brand-50 text-brand-700 dark:border-brand-400 dark:bg-brand-500/10 dark:text-brand-300'
          : 'border-gray-200 hover:bg-gray-50 dark:border-gray-700 dark:hover:bg-white/5'
      }`}
    >
      <span className="h-2 w-2 rounded-full" style={{ backgroundColor: tag.color || '#14b8a6' }} />
      {tag.name}
    </button>
  )
}

function TagResourceLink({ resourceType, resourceId }: { resourceType: string; resourceId: string }) {
  if (resourceType === 'application') {
    return (
      <Link
        to="/applications/$appId"
        params={{ appId: resourceId }}
        className="text-brand-600 hover:underline dark:text-brand-400"
      >
        Open
      </Link>
    )
  }
  if (resourceType === 'database') {
    return (
      <Link
        to="/databases/$dbId"
        params={{ dbId: resourceId }}
        className="text-brand-600 hover:underline dark:text-brand-400"
      >
        Open
      </Link>
    )
  }
  if (resourceType === 'service') {
    return (
      <Link
        to="/services/$svcId"
        params={{ svcId: resourceId }}
        className="text-brand-600 hover:underline dark:text-brand-400"
      >
        Open
      </Link>
    )
  }
  return null
}

/** Coolify-style tag browser: pick a tag, see everything it's attached to. */
export function TagsPage() {
  const tags = useQuery({ queryKey: ['tags'], queryFn: api.tags })
  const [selected, setSelected] = useState<string>('')
  const [search, setSearch] = useState('')

  const allTags = tags.data?.tags || []
  const activeTag = allTags.find((t) => t.id === selected) || allTags[0]

  const resources = useQuery({
    queryKey: ['tag-resources', activeTag?.id],
    queryFn: () => api.tagResources(activeTag!.id),
    enabled: Boolean(activeTag?.id),
  })

  const filteredTags = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return allTags
    return allTags.filter((t) => t.name.toLowerCase().includes(q))
  }, [allTags, search])

  if (tags.isLoading) return <PageSkeleton cards={2} />

  return (
    <div className="space-y-6">
      <Header title="Tags" />
      {!allTags.length && (
        <div className="panel-card p-8 text-center text-sm text-gray-500 dark:text-gray-400">
          No tags yet. Attach tags to applications, databases, or services from their detail
          pages.
        </div>
      )}
      {Boolean(allTags.length) && (
        <>
          <div className="relative max-w-md">
            <Search className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400" />
            <input
              type="search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Filter tags…"
              className="panel-field h-9 w-full rounded-md py-1.5 pr-3 pl-9 text-sm"
            />
          </div>
          <div className="flex flex-wrap gap-2">
            {filteredTags.map((t) => (
              <TagChipStatic
                key={t.id}
                tag={t}
                active={activeTag?.id === t.id}
                onClick={() => setSelected(t.id)}
              />
            ))}
            {!filteredTags.length && (
              <p className="text-sm text-gray-500 dark:text-gray-400">No tags match “{search}”.</p>
            )}
          </div>

          {activeTag && (
            <div className="panel-card overflow-hidden">
              <div className="flex items-center gap-2 border-b border-gray-200 px-4 py-3 dark:border-gray-800">
                <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: activeTag.color || '#14b8a6' }} />
                <h2 className="text-sm font-semibold text-gray-900 dark:text-white">{activeTag.name}</h2>
                <span className="text-xs text-gray-500 dark:text-gray-400">
                  {resources.data?.resources.length ?? 0} resource(s)
                </span>
              </div>
              <table className="panel-table">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Type</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {resources.isLoading ? (
                    <tr>
                      <td colSpan={3} className="p-0">
                        <TableSkeleton rows={3} cols={2} />
                      </td>
                    </tr>
                  ) : (
                    <>
                  {(resources.data?.resources || []).map((r) => (
                    <tr key={`${r.resource_type}:${r.resource_id}`}>
                      <td className="font-medium text-gray-900 dark:text-white">{r.name}</td>
                      <td className="text-gray-600 capitalize dark:text-gray-300">{r.resource_type}</td>
                      <td>
                        <TagResourceLink resourceType={r.resource_type} resourceId={r.resource_id} />
                      </td>
                    </tr>
                  ))}
                  {!resources.data?.resources?.length && (
                    <tr>
                      <td colSpan={3} className="panel-table-empty">
                        No resources tagged with “{activeTag.name}” yet.
                      </td>
                    </tr>
                  )}
                    </>
                  )}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  )
}

/** Server picker then an interactive terminal, without needing to open a resource detail page. */
export function TerminalPickerPage() {
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers })
  const [serverId, setServerId] = useState('')

  const usable = (servers.data?.servers || []).filter((s) => s.is_usable)
  const server = usable.find((s) => s.id === serverId)

  if (servers.isLoading) return <PageSkeleton cards={2} />

  return (
    <div className="space-y-6">
      <Header title="Terminal" />
      {!usable.length ? (
        <div className="panel-card p-8 text-center text-sm text-gray-500 dark:text-gray-400">
          No servers with a working Docker connection yet.{' '}
          <Link to="/servers" className="text-brand-600 hover:underline dark:text-brand-400">
            Add a server
          </Link>
          .
        </div>
      ) : (
        <div className="panel-card space-y-4 p-5">
          <div className="flex flex-wrap items-end gap-3">
            <label className="block min-w-[16rem] text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Server</span>
              <select
                value={serverId}
                onChange={(e) => setServerId(e.target.value)}
                className="panel-field w-full rounded-lg px-3 py-2 text-sm"
              >
                <option value="">Select a server…</option>
                {usable.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name} ({s.ip})
                  </option>
                ))}
              </select>
            </label>
            {server && (
              <Link
                to="/servers/$serverId"
                params={{ serverId: server.id }}
                className="pb-2 text-xs text-brand-600 hover:underline dark:text-brand-400"
              >
                Open server details →
              </Link>
            )}
          </div>
          {server ? (
            <ServerTerminal key={server.id} serverId={server.id} />
          ) : (
            <p className="text-sm text-gray-500 dark:text-gray-400">Pick a server to connect.</p>
          )}
        </div>
      )}
    </div>
  )
}