import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Database, FolderKanban, Rocket, Server } from 'lucide-react'
import type { ReactNode } from 'react'
import { PageSkeleton } from '../components/ui/Skeleton'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'

function StatCard({
  label,
  value,
  to,
  icon,
  iconBg,
}: {
  label: string
  value: string | number
  to: string
  icon: ReactNode
  iconBg: string
}) {
  return (
    <Link to={to} className="panel-card overflow-hidden transition hover:border-brand-300 dark:hover:border-brand-500/40">
      <div className="flex items-center justify-between gap-3 p-4">
        <div className="min-w-0">
          <p className="truncate text-xs font-medium tracking-wide text-gray-500 uppercase dark:text-gray-400">
            {label}
          </p>
          <p className="mt-1 text-lg font-semibold tracking-tight text-gray-900 sm:text-xl dark:text-white">
            {value}
          </p>
        </div>
        <div className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${iconBg}`}>{icon}</div>
      </div>
    </Link>
  )
}

export function DashboardPage() {
  const { user } = useAuth()
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const apps = useQuery({ queryKey: ['applications'], queryFn: () => api.applications() })
  const dbs = useQuery({ queryKey: ['databases'], queryFn: () => api.databases() })

  const hasServers = (servers.data?.servers?.length ?? 0) > 0
  const loading =
    servers.isLoading || projects.isLoading || apps.isLoading || dbs.isLoading

  if (loading) return <PageSkeleton cards={2} />

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">
          Welcome{user?.name ? `, ${user.name}` : ''}
        </h1>
      </div>

      {!hasServers && (
        <div className="panel-card border-brand-200 bg-brand-50/60 p-6 dark:border-brand-500/30 dark:bg-brand-500/10">
          <h2 className="text-lg font-medium text-gray-900 dark:text-white">Welcome — no servers yet</h2>
          <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
            Add this VPS (or another host) under Servers, then create a project to deploy.
          </p>
          <Link
            to="/servers"
            className="mt-4 inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white hover:bg-brand-600"
          >
            Go to Servers
          </Link>
        </div>
      )}

      <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
        <StatCard
          label="Servers"
          value={servers.data?.servers?.length ?? '—'}
          to="/servers"
          icon={<Server className="h-4 w-4 text-brand-600 dark:text-brand-400" />}
          iconBg="bg-brand-50 dark:bg-brand-500/15"
        />
        <StatCard
          label="Projects"
          value={projects.data?.projects?.length ?? '—'}
          to="/projects"
          icon={<FolderKanban className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />}
          iconBg="bg-emerald-50 dark:bg-emerald-500/15"
        />
        <StatCard
          label="Applications"
          value={apps.data?.applications?.length ?? '—'}
          to="/projects"
          icon={<Rocket className="h-4 w-4 text-violet-600 dark:text-violet-400" />}
          iconBg="bg-violet-50 dark:bg-violet-500/15"
        />
        <StatCard
          label="Databases"
          value={dbs.data?.databases?.length ?? '—'}
          to="/projects"
          icon={<Database className="h-4 w-4 text-amber-600 dark:text-amber-400" />}
          iconBg="bg-amber-50 dark:bg-amber-500/15"
        />
      </div>

      <div className="panel-card p-6">
        <h2 className="text-lg font-medium text-gray-900 dark:text-white">Quick start</h2>
        <ol className="mt-3 list-decimal space-y-2 pl-5 text-sm text-gray-600 dark:text-gray-400">
          <li>
            Add an SSH key and server under{' '}
            <Link to="/servers" className="text-brand-600 dark:text-brand-400">
              Servers
            </Link>
          </li>
          <li>
            Create a{' '}
            <Link to="/projects" className="text-brand-600 dark:text-brand-400">
              project
            </Link>{' '}
            and deploy an application or one-click service
          </li>
        </ol>
      </div>
    </div>
  )
}
