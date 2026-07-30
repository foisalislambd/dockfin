import { useQueries, useQuery } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { Boxes, Database, Rocket } from 'lucide-react'
import { api, LAST_ENV_KEY } from '../lib/api'
import { Header } from './Servers'

type ResourceRow = {
  id: string
  name: string
  kind: 'application' | 'database' | 'service'
  meta: string
  status: string
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

  const resources: ResourceRow[] = [
    ...(appsQ.data?.applications || []).map((a) => ({
      id: a.id,
      name: a.name,
      kind: 'application' as const,
      meta: a.build_pack + (a.fqdn ? ` · ${a.fqdn}` : ''),
      status: a.status,
    })),
    ...(dbsQ.data?.databases || []).map((d) => ({
      id: d.id,
      name: d.name,
      kind: 'database' as const,
      meta: d.engine,
      status: d.status,
    })),
    ...(svcsQ.data?.services || []).map((s) => ({
      id: s.id,
      name: s.name,
      kind: 'service' as const,
      meta: s.service_type,
      status: s.status,
    })),
  ].sort((a, b) => a.name.localeCompare(b.name))

  const loading = appsQ.isLoading || dbsQ.isLoading || svcsQ.isLoading

  return (
    <div className="space-y-6">
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
            <Link
              to="/projects/$projectId/environments/$envId/new"
              params={{ projectId, envId }}
              onClick={() => localStorage.setItem(LAST_ENV_KEY, envId)}
              className="inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white hover:bg-brand-600"
            >
              + New
            </Link>
          }
        />
      </div>

      {loading && <p className="text-sm text-gray-500 dark:text-gray-400">Loading resources…</p>}

      {!loading && !resources.length && (
        <div className="panel-card p-10 text-center">
          <p className="text-sm text-gray-500 dark:text-gray-400">No resources in this environment yet.</p>
          <Link
            to="/projects/$projectId/environments/$envId/new"
            params={{ projectId, envId }}
            className="mt-4 inline-flex h-8 items-center rounded-md bg-brand-500 px-3 text-xs font-medium text-white hover:bg-brand-600"
          >
            + New resource
          </Link>
        </div>
      )}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {resources.map((r) => {
          const Icon = r.kind === 'application' ? Rocket : r.kind === 'database' ? Database : Boxes
          const card = (
            <>
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-brand-50 dark:bg-brand-500/15">
                <Icon className="h-4 w-4 text-brand-600 dark:text-brand-400" />
              </div>
              <div className="min-w-0">
                <div className="truncate font-medium text-gray-900 dark:text-white">{r.name}</div>
                <div className="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">
                  {r.kind} · {r.meta}
                </div>
                <div className="mt-2 text-xs text-gray-600 dark:text-gray-300">{r.status}</div>
              </div>
            </>
          )
          const className =
            'panel-card flex gap-3 p-5 transition hover:border-brand-300 dark:hover:border-brand-500/40'
          if (r.kind === 'application') {
            return (
              <Link
                key={`${r.kind}-${r.id}`}
                to="/projects/$projectId/environments/$envId/applications/$appId"
                params={{ projectId, envId, appId: r.id }}
                className={className}
              >
                {card}
              </Link>
            )
          }
          if (r.kind === 'database') {
            return (
              <Link
                key={`${r.kind}-${r.id}`}
                to="/projects/$projectId/environments/$envId/databases/$dbId"
                params={{ projectId, envId, dbId: r.id }}
                className={className}
              >
                {card}
              </Link>
            )
          }
          return (
            <Link
              key={`${r.kind}-${r.id}`}
              to="/projects/$projectId/environments/$envId/services/$svcId"
              params={{ projectId, envId, svcId: r.id }}
              className={className}
            >
              {card}
            </Link>
          )
        })}
      </div>
    </div>
  )
}
