import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Check, Database, FolderKanban, Rocket, Server } from 'lucide-react'
import type { ReactNode } from 'react'
import { UseThisHostButton } from '../components/UseThisHostButton'
import { DashboardSkeleton } from '../components/ui/Skeleton'
import { Header } from '../components/ui/Header'
import { api, fetchAllEnvironments, type Application, type Project, type Service } from '../lib/api'
import { useAuth } from '../lib/auth'

function StatCard({
  label,
  value,
  hint,
  to,
  icon,
  iconBg,
}: {
  label: string
  value: string | number
  hint?: string
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
          {hint ? <p className="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">{hint}</p> : null}
        </div>
        <div className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg ${iconBg}`}>{icon}</div>
      </div>
    </Link>
  )
}

function latestProjects(list: Project[], limit = 6): Project[] {
  return [...list]
    .sort((a, b) => {
      const ta = a.created_at ? Date.parse(a.created_at) : 0
      const tb = b.created_at ? Date.parse(b.created_at) : 0
      if (tb !== ta) return tb - ta
      return a.name.localeCompare(b.name)
    })
    .slice(0, limit)
}

function runningCount(items: Array<{ status?: string }>) {
  return items.filter((item) => {
    const s = (item.status || '').toLowerCase()
    return s === 'running' || s === 'healthy' || s === 'started' || s === 'exited:0'
  }).length
}

function Step({
  done,
  index,
  title,
  body,
  actions,
}: {
  done: boolean
  index: number
  title: string
  body: string
  actions?: ReactNode
}) {
  return (
    <li className="flex gap-3">
      <span
        className={`mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold ${
          done
            ? 'bg-success-50 text-success-600 dark:bg-success-500/15'
            : 'bg-brand-50 text-brand-700 dark:bg-brand-500/15 dark:text-brand-300'
        }`}
      >
        {done ? <Check className="h-3.5 w-3.5" /> : index}
      </span>
      <div className="min-w-0">
        <p className="text-sm font-medium text-gray-900 dark:text-white">{title}</p>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{body}</p>
        {!done && actions ? <div className="mt-2 flex flex-wrap items-center gap-2">{actions}</div> : null}
      </div>
    </li>
  )
}

export function DashboardPage() {
  const { user, team } = useAuth()
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const apps = useQuery({ queryKey: ['applications'], queryFn: () => api.applications() })
  const dbs = useQuery({ queryKey: ['databases'], queryFn: () => api.databases() })
  const services = useQuery({ queryKey: ['services'], queryFn: () => api.services() })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments })

  const serverList = servers.data?.servers ?? []
  const projectList = projects.data?.projects ?? []
  const appList: Application[] = apps.data?.applications ?? []
  const dbList = dbs.data?.databases ?? []
  const serviceList: Service[] = services.data?.services ?? []

  const loading =
    servers.isLoading || projects.isLoading || apps.isLoading || dbs.isLoading || services.isLoading

  const recent = useMemo(() => latestProjects(projectList, 6), [projectList])
  const unreachable = serverList.filter((s) => !s.is_reachable).length
  const firstEnv = envs.data?.[0]
  const resourceCount = appList.length + dbList.length + serviceList.length
  const ready = serverList.length > 0 && projectList.length > 0 && resourceCount > 0

  if (loading) return <DashboardSkeleton />

  const failed =
    servers.isError || projects.isError || apps.isError || dbs.isError || services.isError

  return (
    <div className="space-y-6">
      <Header
        title={user?.name ? `Welcome, ${user.name}` : 'Overview'}
        subtitle={team ? `${team.name}${team.personal ? ' · personal team' : ''}` : 'Workspace health and first-run setup'}
        actions={
          <Link
            to="/projects"
            search={{ new: '1' }}
            className="inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white hover:bg-brand-600"
          >
            New project
          </Link>
        }
      />

      {failed && (
        <div className="rounded-lg border border-error-500/30 bg-error-50 px-4 py-3 text-sm text-error-500 dark:bg-error-500/10">
          Some workspace data could not be loaded. Refresh the page to try again.
        </div>
      )}

      {!ready && (
        <section className="panel-card p-5">
          <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Get the first deploy out</h2>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Three steps. You can leave and come back — Dockfin keeps the workspace.
          </p>
          <ol className="mt-4 space-y-4">
            <Step
              index={1}
              done={serverList.length > 0}
              title="Add a server"
              body="Use this machine, or connect another host over SSH. Docker must be installed there."
              actions={
                <>
                  <UseThisHostButton />
                  <Link
                    to="/servers/new"
                    className="inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white hover:bg-brand-600"
                  >
                    Add a different server
                  </Link>
                </>
              }
            />
            <Step
              index={2}
              done={projectList.length > 0}
              title="Create a project"
              body="A project groups environments. The first one is production."
              actions={
                <Link
                  to="/projects"
                  search={{ new: '1' }}
                  className="inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white hover:bg-brand-600"
                >
                  New project
                </Link>
              }
            />
            <Step
              index={3}
              done={resourceCount > 0}
              title="Deploy something"
              body="An application from Git or a Dockerfile, a database, or one of the one-click services."
              actions={
                firstEnv ? (
                  <Link
                    to="/projects/$projectId/environments/$envId/new"
                    params={{ projectId: firstEnv.project_id, envId: firstEnv.id }}
                    className="inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white hover:bg-brand-600"
                  >
                    Add a resource
                  </Link>
                ) : (
                  <span className="text-xs text-gray-400">Create a project first.</span>
                )
              }
            />
          </ol>
        </section>
      )}

      <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
        <StatCard
          label="Servers"
          value={serverList.length}
          hint={unreachable ? `${unreachable} unreachable` : serverList.length ? 'Reachable hosts' : 'None yet'}
          to="/servers"
          icon={<Server className="h-4 w-4 text-brand-600 dark:text-brand-400" />}
          iconBg="bg-brand-50 dark:bg-brand-500/15"
        />
        <StatCard
          label="Projects"
          value={projectList.length}
          to="/projects"
          icon={<FolderKanban className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />}
          iconBg="bg-emerald-50 dark:bg-emerald-500/15"
        />
        <StatCard
          label="Applications"
          value={appList.length}
          hint={appList.length ? `${runningCount(appList)} running` : 'From Git, image, or Dockerfile'}
          to="/projects"
          icon={<Rocket className="h-4 w-4 text-violet-600 dark:text-violet-400" />}
          iconBg="bg-violet-50 dark:bg-violet-500/15"
        />
        <StatCard
          label="Data & services"
          value={dbList.length + serviceList.length}
          hint={`${dbList.length} databases · ${serviceList.length} services`}
          to="/projects"
          icon={<Database className="h-4 w-4 text-amber-600 dark:text-amber-400" />}
          iconBg="bg-amber-50 dark:bg-amber-500/15"
        />
      </div>

      <div>
        <div className="mb-3 flex items-baseline justify-between gap-3">
          <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Projects</h2>
          <Link to="/projects" className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400">
            View all
          </Link>
        </div>

        {recent.length === 0 ? (
          <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-gray-200 px-4 py-10 text-center dark:border-gray-700">
            <span className="inline-block rounded-md border border-gray-200 px-2.5 py-1 text-xs text-gray-500 dark:border-gray-700 dark:text-gray-400">
              No projects yet
            </span>
            <p className="max-w-sm text-xs text-gray-500 dark:text-gray-400">
              A project is where applications, databases, and one-click services are deployed.
            </p>
            <Link
              to="/projects"
              search={{ new: '1' }}
              className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400"
            >
              Create a project
            </Link>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {recent.map((p) => (
              <Link
                key={p.id}
                to="/projects/$projectId"
                params={{ projectId: p.id }}
                className="panel-card flex items-center gap-3 p-4 transition hover:border-brand-300 dark:hover:border-brand-500/40"
              >
                <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-emerald-50 dark:bg-emerald-500/15">
                  <FolderKanban className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                </div>
                <div className="min-w-0">
                  <p className="truncate font-medium text-gray-900 dark:text-white">{p.name}</p>
                  {p.description ? (
                    <p className="truncate text-sm text-gray-500 dark:text-gray-400">{p.description}</p>
                  ) : (
                    <p className="truncate text-sm text-gray-400">Open environments</p>
                  )}
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
