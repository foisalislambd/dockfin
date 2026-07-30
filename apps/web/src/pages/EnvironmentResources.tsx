import { useQueries, useQuery } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { PageSkeleton } from '../components/ui/Skeleton'
import { api, LAST_ENV_KEY } from '../lib/api'
import { Header } from './Servers'

type ResourceKind = 'application' | 'database' | 'service'

type ResourceRow = {
  id: string
  name: string
  kind: ResourceKind
  subtitle: string
  status: string
  destinationId?: string | null
}

function statusDotClass(status: string) {
  const s = status.toLowerCase()
  if (s.includes('running') || s.includes('healthy') || s === 'online') {
    return 'bg-success-500'
  }
  if (s.includes('deploy') || s.includes('progress') || s.includes('starting')) {
    return 'bg-warning-500'
  }
  if (s.includes('error') || s.includes('fail') || s.includes('exited') || s.includes('stopped')) {
    return 'bg-error-500'
  }
  return 'bg-gray-400'
}

function ResourceCard({
  name,
  subtitle,
  status,
  to,
  params,
}: {
  name: string
  subtitle: string
  status: string
  to: string
  params: Record<string, string>
}) {
  return (
    <Link
      to={to}
      params={params}
      className="panel-card flex min-h-[7.5rem] flex-col justify-between gap-3 p-4 transition hover:border-brand-300 dark:hover:border-brand-500/40"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 truncate text-sm font-semibold text-gray-900 dark:text-white">
          {name}
        </div>
        <span
          className={`mt-1 h-2.5 w-2.5 shrink-0 rounded-full ${statusDotClass(status)}`}
          title={status}
          aria-label={status}
        />
      </div>
      <div className="text-xs text-gray-500 dark:text-gray-400">{subtitle}</div>
      <div className="text-xs text-gray-400 dark:text-gray-500">Add tag</div>
    </Link>
  )
}

export function EnvironmentResourcesPage() {
  const { projectId, envId } = useParams({ strict: false }) as { projectId: string; envId: string }

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

  const [appsQ, dbsQ, svcsQ] = useQueries({
    queries: [
      {
        queryKey: ['applications', envId],
        queryFn: () => api.applications(envId),
      },
      {
        queryKey: ['databases', envId],
        queryFn: () => api.databases(envId),
      },
      {
        queryKey: ['services', envId],
        queryFn: () => api.services(envId),
      },
    ],
  })

  const serverLabel = (destinationId?: string | null) => {
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
    destinationId: a.destination_id,
  }))

  const databases: ResourceRow[] = (dbsQ.data?.databases || []).map((d) => ({
    id: d.id,
    name: d.name,
    kind: 'database' as const,
    subtitle: serverLabel(d.destination_id),
    status: d.status,
    destinationId: d.destination_id,
  }))

  const services: ResourceRow[] = (svcsQ.data?.services || []).map((s) => ({
    id: s.id,
    name: s.name,
    kind: 'service' as const,
    subtitle: s.fqdn ? s.fqdn : serverLabel(s.destination_id),
    status: s.status,
    destinationId: s.destination_id,
  }))

  const loading = appsQ.isLoading || dbsQ.isLoading || svcsQ.isLoading
  const empty =
    !applications.length && !databases.length && !services.length

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
          <span className="text-gray-900 dark:text-white">{env?.name || '…'}</span>
        </nav>
        <Header
          title="Resources"
          actions={
            <div className="flex items-center gap-2">
              <Link
                to="/projects/$projectId/environments/$envId/edit"
                params={{ projectId, envId }}
                className="inline-flex h-8 items-center rounded-md border border-gray-200 px-2.5 text-xs font-medium text-gray-600 hover:bg-gray-50 dark:border-gray-800 dark:text-gray-300 dark:hover:bg-white/5"
              >
                Settings
              </Link>
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
      </div>

      {empty && (
        <div className="panel-card p-10 text-center">
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Deploy resources, like Applications, Databases, Services…
          </p>
          <Link
            to="/projects/$projectId/environments/$envId/new"
            params={{ projectId, envId }}
            onClick={() => localStorage.setItem(LAST_ENV_KEY, envId)}
            className="mt-4 inline-flex h-8 items-center rounded-md bg-brand-500 px-3 text-xs font-medium text-white hover:bg-brand-600"
          >
            + New resource
          </Link>
        </div>
      )}

      {applications.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Applications</h2>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {applications.map((r) => (
              <ResourceCard
                key={r.id}
                name={r.name}
                subtitle={r.subtitle}
                status={r.status}
                to="/projects/$projectId/environments/$envId/applications/$appId"
                params={{ projectId, envId, appId: r.id }}
              />
            ))}
          </div>
        </section>
      )}

      {databases.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Databases</h2>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {databases.map((r) => (
              <ResourceCard
                key={r.id}
                name={r.name}
                subtitle={r.subtitle}
                status={r.status}
                to="/projects/$projectId/environments/$envId/databases/$dbId"
                params={{ projectId, envId, dbId: r.id }}
              />
            ))}
          </div>
        </section>
      )}

      {services.length > 0 && (
        <section className="space-y-3">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Services</h2>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {services.map((r) => (
              <ResourceCard
                key={r.id}
                name={r.name}
                subtitle={r.subtitle}
                status={r.status}
                to="/projects/$projectId/environments/$envId/services/$svcId"
                params={{ projectId, envId, svcId: r.id }}
              />
            ))}
          </div>
        </section>
      )}
    </div>
  )
}
