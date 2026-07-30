import { Link, useParams } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Boxes, Database, Rocket } from 'lucide-react'
import { api, LAST_ENV_KEY } from '../lib/api'

const TYPES = [
  {
    id: 'application',
    title: 'Application',
    description: 'Deploy from Git, Dockerfile, Compose, Nixpacks, or a Docker image.',
    icon: Rocket,
    href: 'applications/new' as const,
  },
  {
    id: 'database',
    title: 'Database',
    description: 'PostgreSQL, MySQL, MariaDB, MongoDB, Redis, KeyDB, Dragonfly, ClickHouse.',
    icon: Database,
    href: 'databases/new' as const,
  },
  {
    id: 'service',
    title: 'One-click Service',
    description: 'Deploy from the service catalog (WordPress, Plausible, n8n, …).',
    icon: Boxes,
    href: 'services/new' as const,
  },
]

export function NewResourcePage() {
  const { projectId, envId } = useParams({ strict: false }) as { projectId: string; envId: string }
  const project = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => api.getProject(projectId),
  })

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
          <Link
            to="/projects/$projectId/environments/$envId"
            params={{ projectId, envId }}
            className="hover:text-brand-600 dark:hover:text-brand-400"
          >
            Resources
          </Link>
          <span>/</span>
          <span className="text-gray-900 dark:text-white">New</span>
        </nav>
        <h1 className="mt-3 text-xl font-semibold tracking-tight text-gray-900 dark:text-white">New Resource</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
          Deploy resources, like Applications, Databases, Services…
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {TYPES.map((t) => {
          const Icon = t.icon
          if (t.href === 'applications/new') {
            return (
              <Link
                key={t.id}
                to="/projects/$projectId/environments/$envId/applications/new"
                params={{ projectId, envId }}
                onClick={() => localStorage.setItem(LAST_ENV_KEY, envId)}
                className="panel-card flex flex-col gap-3 p-5 transition hover:border-brand-300 dark:hover:border-brand-500/40"
              >
                <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-brand-50 dark:bg-brand-500/15">
                  <Icon className="h-5 w-5 text-brand-600 dark:text-brand-400" />
                </div>
                <div>
                  <div className="font-semibold text-gray-900 dark:text-white">{t.title}</div>
                  <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{t.description}</p>
                </div>
              </Link>
            )
          }
          if (t.href === 'databases/new') {
            return (
              <Link
                key={t.id}
                to="/projects/$projectId/environments/$envId/databases/new"
                params={{ projectId, envId }}
                onClick={() => localStorage.setItem(LAST_ENV_KEY, envId)}
                className="panel-card flex flex-col gap-3 p-5 transition hover:border-brand-300 dark:hover:border-brand-500/40"
              >
                <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-brand-50 dark:bg-brand-500/15">
                  <Icon className="h-5 w-5 text-brand-600 dark:text-brand-400" />
                </div>
                <div>
                  <div className="font-semibold text-gray-900 dark:text-white">{t.title}</div>
                  <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{t.description}</p>
                </div>
              </Link>
            )
          }
          return (
            <Link
              key={t.id}
              to="/projects/$projectId/environments/$envId/services/new"
              params={{ projectId, envId }}
              onClick={() => localStorage.setItem(LAST_ENV_KEY, envId)}
              className="panel-card flex flex-col gap-3 p-5 transition hover:border-brand-300 dark:hover:border-brand-500/40"
            >
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-brand-50 dark:bg-brand-500/15">
                <Icon className="h-5 w-5 text-brand-600 dark:text-brand-400" />
              </div>
              <div>
                <div className="font-semibold text-gray-900 dark:text-white">{t.title}</div>
                <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{t.description}</p>
              </div>
            </Link>
          )
        })}
      </div>
    </div>
  )
}
