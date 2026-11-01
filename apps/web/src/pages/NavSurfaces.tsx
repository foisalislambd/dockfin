import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { PageSkeleton, TableSkeleton } from '../components/ui/Skeleton'
import { api, type Server, type Tag } from '../lib/api'
import { Header } from '../components/ui/Header'

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

export function AuditLogPage() {
  const [q, setQ] = useState('')
  const [category, setCategory] = useState('')
  const [offset, setOffset] = useState(0)
  const limit = 50
  const logs = useQuery({
    queryKey: ['audit-logs', q, category, offset],
    queryFn: () => api.auditLogs({ q, category, limit, offset }),
    refetchInterval: 15000,
  })
  const rows = logs.data?.audit_logs || []
  const total = logs.data?.total || 0
  return (
    <div className="space-y-6">
      <Header title="Audit log" />
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Sign-ins, deploys, freezes, member changes, and resource create or delete actions. Owners and admins only.
      </p>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <label className="block flex-1 text-xs font-medium text-gray-600 dark:text-gray-300">
          Search
          <input
            value={q}
            onChange={(e) => {
              setOffset(0)
              setQ(e.target.value)
            }}
            className="mt-1 h-9 w-full rounded-md border border-gray-200 bg-white px-2 text-sm dark:border-gray-700 dark:bg-gray-900"
            placeholder="Email, path, or summary"
          />
        </label>
        <label className="block text-xs font-medium text-gray-600 dark:text-gray-300">
          Category
          <select
            value={category}
            onChange={(e) => {
              setOffset(0)
              setCategory(e.target.value)
            }}
            className="mt-1 h-9 w-full rounded-md border border-gray-200 bg-white px-2 text-sm sm:w-40 dark:border-gray-700 dark:bg-gray-900"
          >
            <option value="">All</option>
            <option value="auth">Auth</option>
            <option value="deploy">Deploy</option>
            <option value="access">Access</option>
            <option value="settings">Settings</option>
            <option value="resource">Resource</option>
          </select>
        </label>
      </div>
      {logs.isLoading ? <PageSkeleton cards={1} /> : null}
      {logs.error ? (
        <div className="panel-card border-error-200 p-4 text-sm text-error-500" role="alert">
          Could not load the audit log. {logs.error.message}
        </div>
      ) : null}
      {!logs.isLoading && !logs.error && (
        <div className="panel-card overflow-x-auto">
          <table className="panel-table min-w-[40rem]">
            <thead>
              <tr>
                <th>When</th>
                <th>Who</th>
                <th>What</th>
                <th>Resource</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.id}>
                  <td className="whitespace-nowrap text-xs text-gray-500">
                    {row.created_at ? new Date(row.created_at).toLocaleString() : '—'}
                  </td>
                  <td className="text-sm">{row.actor_email || row.user_email || '—'}</td>
                  <td className="text-sm">
                    <div>{row.summary || `${row.method} ${row.action}`}</div>
                    <div className="font-mono text-[11px] text-gray-500">{row.category || row.action}</div>
                  </td>
                  <td className="font-mono text-xs">
                    {row.resource_type}
                    {row.resource_id ? ` ${row.resource_id.slice(0, 8)}` : ''}
                  </td>
                  <td className="text-xs">{row.status_code || '—'}</td>
                </tr>
              ))}
              {!rows.length && (
                <tr>
                  <td colSpan={5} className="px-4 py-8 text-center text-gray-500">
                    No audit entries match.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
          <div className="flex items-center justify-between gap-3 border-t border-gray-200 px-4 py-3 text-xs dark:border-gray-800">
            <span className="text-gray-500">
              {total} event{total === 1 ? '' : 's'}
            </span>
            <div className="flex gap-2">
              <button
                type="button"
                className="rounded-md border border-gray-200 px-2 py-1 disabled:opacity-40 dark:border-gray-700"
                disabled={offset === 0}
                onClick={() => setOffset((n) => Math.max(0, n - limit))}
              >
                Previous
              </button>
              <button
                type="button"
                className="rounded-md border border-gray-200 px-2 py-1 disabled:opacity-40 dark:border-gray-700"
                disabled={offset + limit >= total}
                onClick={() => setOffset((n) => n + limit)}
              >
                Next
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}