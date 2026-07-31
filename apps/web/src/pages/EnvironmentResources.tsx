import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { useMemo, useState, type FormEvent, type MouseEvent } from 'react'
import { PageSkeleton } from '../components/ui/Skeleton'
import { api, LAST_ENV_KEY, type Tag } from '../lib/api'
import { Btn, Header, Input, Modal } from './Servers'

type ResourceKind = 'application' | 'database' | 'service'

type ResourceRow = {
  id: string
  name: string
  kind: ResourceKind
  subtitle: string
  status: string
  description?: string
  tags: Tag[]
}

function statusDotClass(status: string) {
  const s = status.toLowerCase()
  if (s.includes('running') || s.includes('healthy') || s === 'online') return 'bg-success-500'
  if (s.includes('deploy') || s.includes('progress') || s.includes('starting')) return 'bg-warning-500'
  if (s.includes('error') || s.includes('fail') || s.includes('exited') || s.includes('stopped'))
    return 'bg-error-500'
  return 'bg-gray-400'
}

function ResourceCard({
  row,
  to,
  params,
  onAddTag,
}: {
  row: ResourceRow
  to: string
  params: Record<string, string>
  onAddTag: (row: ResourceRow) => void
}) {
  return (
    <Link
      to={to}
      params={params}
      className="panel-card flex min-h-[7.5rem] flex-col justify-between gap-3 p-4 transition hover:border-brand-300 dark:hover:border-brand-500/40"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 truncate text-sm font-semibold text-gray-900 dark:text-white">
          {row.name}
        </div>
        <span
          className={`mt-1 h-2.5 w-2.5 shrink-0 rounded-full ${statusDotClass(row.status)}`}
          title={row.status}
          aria-label={row.status}
        />
      </div>
      <div className="truncate text-xs text-gray-500 dark:text-gray-400">{row.subtitle}</div>
      <div className="flex flex-wrap items-center gap-1.5">
        {row.tags.map((t) => (
          <span
            key={t.id}
            className="rounded px-1.5 py-0.5 text-[10px] font-medium text-white"
            style={{ backgroundColor: t.color || '#14b8a6' }}
          >
            {t.name}
          </span>
        ))}
        <button
          type="button"
          className="text-xs text-gray-400 hover:text-brand-600 dark:hover:text-brand-400"
          onClick={(e: MouseEvent) => {
            e.preventDefault()
            e.stopPropagation()
            onAddTag(row)
          }}
        >
          {row.tags.length ? '+ Tag' : 'Add tag'}
        </button>
      </div>
    </Link>
  )
}

export function EnvironmentResourcesPage() {
  const { projectId, envId } = useParams({ strict: false }) as { projectId: string; envId: string }
  const nav = useNavigate()
  const qc = useQueryClient()
  const [search, setSearch] = useState('')
  const [cloneOpen, setCloneOpen] = useState(false)
  const [cloneName, setCloneName] = useState('')
  const [tagTarget, setTagTarget] = useState<ResourceRow | null>(null)
  const [tagName, setTagName] = useState('')

  const project = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => api.getProject(projectId),
  })
  const envs = useQuery({
    queryKey: ['environments', projectId],
    queryFn: () => api.environments(projectId),
  })
  const env = (envs.data?.environments || []).find((e) => e.id === envId)

  const destinations = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers })
  const envTags = useQuery({
    queryKey: ['environment-tags', projectId, envId],
    queryFn: () => api.environmentTags(projectId, envId),
  })
  const allTags = useQuery({ queryKey: ['tags'], queryFn: api.tags })

  const [appsQ, dbsQ, svcsQ] = useQueries({
    queries: [
      { queryKey: ['applications', envId], queryFn: () => api.applications(envId) },
      { queryKey: ['databases', envId], queryFn: () => api.databases(envId) },
      { queryKey: ['services', envId], queryFn: () => api.services(envId) },
    ],
  })

  const tagsByKey = useMemo(() => {
    const m = new Map<string, Tag[]>()
    for (const row of envTags.data?.resource_tags || []) {
      m.set(`${row.resource_type}:${row.resource_id}`, row.tags)
    }
    return m
  }, [envTags.data])

  const serverLabel = (destinationId?: string | null, serverId?: string | null) => {
    if (serverId) {
      const srv = (servers.data?.servers || []).find((s) => s.id === serverId)
      if (srv) return `Server: ${srv.name}`
    }
    if (!destinationId) return 'Server: —'
    const dest = (destinations.data?.destinations || []).find((d) => d.id === destinationId)
    if (!dest) return 'Server: —'
    const srv = (servers.data?.servers || []).find((s) => s.id === dest.server_id)
    return `Server: ${srv?.name || dest.name || 'unknown'}`
  }

  const applications: ResourceRow[] = (appsQ.data?.applications || []).map((a) => ({
    id: a.id,
    name: a.name,
    kind: 'application' as const,
    subtitle: a.fqdn ? a.fqdn : serverLabel(a.destination_id),
    status: a.status,
    description: a.description,
    tags: tagsByKey.get(`application:${a.id}`) || [],
  }))

  const databases: ResourceRow[] = (dbsQ.data?.databases || []).map((d) => ({
    id: d.id,
    name: d.name,
    kind: 'database' as const,
    subtitle: serverLabel(d.destination_id),
    status: d.status,
    description: d.description,
    tags: tagsByKey.get(`database:${d.id}`) || [],
  }))

  const services: ResourceRow[] = (svcsQ.data?.services || []).map((s) => ({
    id: s.id,
    name: s.name,
    kind: 'service' as const,
    subtitle: s.fqdn ? s.fqdn : serverLabel(s.destination_id, s.server_id),
    status: s.status,
    description: s.description,
    tags: tagsByKey.get(`service:${s.id}`) || [],
  }))

  const q = search.trim().toLowerCase()
  const match = (r: ResourceRow) => {
    if (!q) return true
    return (
      r.name.toLowerCase().includes(q) ||
      r.subtitle.toLowerCase().includes(q) ||
      (r.description || '').toLowerCase().includes(q) ||
      r.tags.some((t) => t.name.toLowerCase().includes(q))
    )
  }

  const filteredApps = applications.filter(match).sort((a, b) => a.name.localeCompare(b.name))
  const filteredDbs = databases.filter(match).sort((a, b) => a.name.localeCompare(b.name))
  const filteredSvcs = services.filter(match).sort((a, b) => a.name.localeCompare(b.name))

  const loading = appsQ.isLoading || dbsQ.isLoading || svcsQ.isLoading
  const empty = !applications.length && !databases.length && !services.length
  const noMatch = !empty && !filteredApps.length && !filteredDbs.length && !filteredSvcs.length

  const clone = useMutation({
    mutationFn: () => api.cloneEnvironment(projectId, envId, cloneName.trim()),
    onSuccess: (res) => {
      setCloneOpen(false)
      setCloneName('')
      void qc.invalidateQueries({ queryKey: ['environments', projectId] })
      void nav({
        to: '/projects/$projectId/environments/$envId',
        params: { projectId, envId: res.environment.id },
      })
    },
  })

  const attachTag = useMutation({
    mutationFn: async () => {
      if (!tagTarget || !tagName.trim()) return
      await api.attachTag({
        name: tagName.trim(),
        resource_type: tagTarget.kind,
        resource_id: tagTarget.id,
      })
    },
    onSuccess: () => {
      setTagTarget(null)
      setTagName('')
      void qc.invalidateQueries({ queryKey: ['environment-tags', projectId, envId] })
      void qc.invalidateQueries({ queryKey: ['tags'] })
    },
  })

  if (loading) return <PageSkeleton cards={2} />

  return (
    <div className="space-y-8">
      <div>
        <nav className="flex flex-wrap items-center gap-1 text-sm text-gray-500 dark:text-gray-400">
          <Link to="/projects" className="hover:text-brand-600 dark:hover:text-brand-400">
            Projects
          </Link>
          <span>/</span>
          <Link
            to="/projects/$projectId"
            params={{ projectId }}
            className="hover:text-brand-600 dark:hover:text-brand-400"
          >
            {project.data?.name || '…'}
          </Link>
          <span>/</span>
          <select
            className="rounded border-0 bg-transparent py-0 pr-6 text-sm font-medium text-gray-900 dark:text-white"
            value={envId}
            aria-label="Environment"
            onChange={(e) => {
              const next = e.target.value
              localStorage.setItem(LAST_ENV_KEY, next)
              void nav({
                to: '/projects/$projectId/environments/$envId',
                params: { projectId, envId: next },
              })
            }}
          >
            {(envs.data?.environments || []).map((e) => (
              <option key={e.id} value={e.id}>
                {e.name}
              </option>
            ))}
          </select>
        </nav>
        <Header
          title="Resources"
          actions={
            <div className="flex flex-wrap items-center gap-2">
              <Link
                to="/projects/$projectId/environments/$envId/shared-variables"
                params={{ projectId, envId }}
                className="inline-flex h-8 items-center rounded-md border border-gray-200 px-2.5 text-xs font-medium text-gray-600 hover:bg-gray-50 dark:border-gray-800 dark:text-gray-300 dark:hover:bg-white/5"
              >
                Env Shared Vars
              </Link>
              <Link
                to="/projects/$projectId/shared-variables"
                params={{ projectId }}
                className="inline-flex h-8 items-center rounded-md border border-gray-200 px-2.5 text-xs font-medium text-gray-600 hover:bg-gray-50 dark:border-gray-800 dark:text-gray-300 dark:hover:bg-white/5"
              >
                Project Shared Vars
              </Link>
              <Link
                to="/projects/$projectId/environments/$envId/edit"
                params={{ projectId, envId }}
                className="inline-flex h-8 items-center rounded-md border border-gray-200 px-2.5 text-xs font-medium text-gray-600 hover:bg-gray-50 dark:border-gray-800 dark:text-gray-300 dark:hover:bg-white/5"
              >
                Settings
              </Link>
              <Btn type="button" onClick={() => setCloneOpen(true)}>
                Clone
              </Btn>
              <Link
                to="/projects/$projectId/environments/$envId/new"
                params={{ projectId, envId }}
                onClick={() => localStorage.setItem(LAST_ENV_KEY, envId)}
                className="inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white hover:bg-brand-600"
              >
                + New
              </Link>
            </div>
          }
        />
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {env?.name || 'Environment'} — applications, databases, and one-click services.
        </p>
      </div>

      {!empty && (
        <div className="relative max-w-xl">
          <Search className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400" />
          <input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search for name, fqdn, tags…"
            className="panel-field h-9 w-full rounded-md py-1.5 pr-3 pl-9 text-sm"
          />
        </div>
      )}

      {empty && (
        <div className="panel-card p-10 text-center">
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Deploy resources, like Applications, Databases, Services…
          </p>
          <div className="mt-4 flex flex-wrap justify-center gap-2">
            <Link
              to="/projects/$projectId/environments/$envId/new"
              params={{ projectId, envId }}
              onClick={() => localStorage.setItem(LAST_ENV_KEY, envId)}
              className="inline-flex h-8 items-center rounded-md bg-brand-500 px-3 text-xs font-medium text-white hover:bg-brand-600"
            >
              + Add Resource
            </Link>
            <Btn type="button" onClick={() => setCloneOpen(true)}>
              Clone from another setup
            </Btn>
          </div>
        </div>
      )}

      {noMatch && (
        <div className="panel-card p-8 text-center text-sm text-gray-500 dark:text-gray-400">
          No resource found with the search term “{search}”.
        </div>
      )}

      {filteredApps.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Applications</h2>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {filteredApps.map((r) => (
              <ResourceCard
                key={r.id}
                row={r}
                to="/projects/$projectId/environments/$envId/applications/$appId"
                params={{ projectId, envId, appId: r.id }}
                onAddTag={setTagTarget}
              />
            ))}
          </div>
        </section>
      )}

      {filteredDbs.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Databases</h2>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {filteredDbs.map((r) => (
              <ResourceCard
                key={r.id}
                row={r}
                to="/projects/$projectId/environments/$envId/databases/$dbId"
                params={{ projectId, envId, dbId: r.id }}
                onAddTag={setTagTarget}
              />
            ))}
          </div>
        </section>
      )}

      {filteredSvcs.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Services</h2>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {filteredSvcs.map((r) => (
              <ResourceCard
                key={r.id}
                row={r}
                to="/projects/$projectId/environments/$envId/services/$svcId"
                params={{ projectId, envId, svcId: r.id }}
                onAddTag={setTagTarget}
              />
            ))}
          </div>
        </section>
      )}

      {cloneOpen && (
        <Modal title="Clone Environment" onClose={() => setCloneOpen(false)}>
          <p className="mb-3 text-sm text-gray-500 dark:text-gray-400">
            Duplicates resource configuration (env vars, settings, tasks, tags) into a new
            environment. Runtime data and domains are not copied.
          </p>
          <form
            className="space-y-3"
            onSubmit={(e: FormEvent) => {
              e.preventDefault()
              clone.mutate()
            }}
          >
            <Input
              label="New environment name"
              value={cloneName}
              onChange={setCloneName}
            />
            {clone.error && <p className="text-sm text-error-500">{clone.error.message}</p>}
            <div className="flex justify-end gap-2">
              <Btn type="button" onClick={() => setCloneOpen(false)}>
                Cancel
              </Btn>
              <Btn primary type="submit" disabled={!cloneName.trim() || clone.isPending}>
                {clone.isPending ? 'Cloning…' : 'Clone'}
              </Btn>
            </div>
          </form>
        </Modal>
      )}

      {tagTarget && (
        <Modal title={`Tag · ${tagTarget.name}`} onClose={() => setTagTarget(null)}>
          <form
            className="space-y-3"
            onSubmit={(e: FormEvent) => {
              e.preventDefault()
              attachTag.mutate()
            }}
          >
            <Input label="Tag name" value={tagName} onChange={setTagName} />
            {(allTags.data?.tags || []).length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {(allTags.data?.tags || []).map((t) => (
                  <button
                    key={t.id}
                    type="button"
                    className="rounded px-2 py-0.5 text-xs text-white"
                    style={{ backgroundColor: t.color }}
                    onClick={() => {
                      void api
                        .attachTag({
                          tag_id: t.id,
                          resource_type: tagTarget.kind,
                          resource_id: tagTarget.id,
                        })
                        .then(() => {
                          setTagTarget(null)
                          void qc.invalidateQueries({
                            queryKey: ['environment-tags', projectId, envId],
                          })
                        })
                    }}
                  >
                    {t.name}
                  </button>
                ))}
              </div>
            )}
            {attachTag.error && <p className="text-sm text-error-500">{attachTag.error.message}</p>}
            <div className="flex justify-end gap-2">
              <Btn type="button" onClick={() => setTagTarget(null)}>
                Cancel
              </Btn>
              <Btn primary type="submit" disabled={!tagName.trim() || attachTag.isPending}>
                Add tag
              </Btn>
            </div>
          </form>
        </Modal>
      )}
    </div>
  )
}
