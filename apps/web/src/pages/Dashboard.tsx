import type { ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Database, FolderKanban, Rocket, Server, Snowflake } from 'lucide-react'
import { DashboardSkeleton } from '../components/ui/Skeleton'
import { api, type AttentionItem, type OverviewDeployment, type Project } from '../lib/api'
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

function ago(iso?: string) {
  if (!iso) return ''
  const t = Date.parse(iso)
  if (!t) return ''
  const s = Math.max(0, Math.round((Date.now() - t) / 1000))
  if (s < 60) return `${s}s ago`
  const m = Math.round(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.round(m / 60)
  if (h < 48) return `${h}h ago`
  return `${Math.round(h / 24)}d ago`
}

function statusClass(status: string) {
  const s = status.toLowerCase()
  if (s === 'finished' || s === 'running' || s === 'healthy') return 'bg-success-500'
  if (s === 'queued' || s === 'in_progress') return 'bg-warning-500'
  if (s === 'failed' || s === 'error' || s === 'unhealthy') return 'bg-error-500'
  return 'bg-gray-400'
}

function ResourceLink({
  item,
  children,
  className,
}: {
  item: { resource_type: string; resource_id: string; project_id?: string; environment_id?: string }
  children: ReactNode
  className?: string
}) {
  const classNames = className || 'font-medium text-gray-900 hover:text-brand-600 dark:text-white dark:hover:text-brand-400'
  if (item.project_id && item.environment_id && item.resource_type === 'application') {
    return (
      <Link
        to="/projects/$projectId/environments/$envId/applications/$appId"
        params={{ projectId: item.project_id, envId: item.environment_id, appId: item.resource_id }}
        className={classNames}
      >
        {children}
      </Link>
    )
  }
  if (item.project_id && item.environment_id && item.resource_type === 'database') {
    return (
      <Link
        to="/projects/$projectId/environments/$envId/databases/$dbId"
        params={{ projectId: item.project_id, envId: item.environment_id, dbId: item.resource_id }}
        className={classNames}
      >
        {children}
      </Link>
    )
  }
  if (item.project_id && item.environment_id && item.resource_type === 'service') {
    return (
      <Link
        to="/projects/$projectId/environments/$envId/services/$svcId"
        params={{ projectId: item.project_id, envId: item.environment_id, svcId: item.resource_id }}
        className={classNames}
      >
        {children}
      </Link>
    )
  }
  if (item.resource_type === 'application') {
    return (
      <Link to="/applications/$appId" params={{ appId: item.resource_id }} className={classNames}>
        {children}
      </Link>
    )
  }
  if (item.resource_type === 'service') {
    return (
      <Link to="/services/$svcId" params={{ svcId: item.resource_id }} className={classNames}>
        {children}
      </Link>
    )
  }
  return <span className={classNames}>{children}</span>
}

function DeploymentLink({ dep, children }: { dep: OverviewDeployment; children: ReactNode }) {
  if (dep.project_id && dep.environment_id && dep.resource_type === 'application') {
    return (
      <Link
        to="/projects/$projectId/environments/$envId/applications/$appId/deployments/$deploymentId"
        params={{
          projectId: dep.project_id,
          envId: dep.environment_id,
          appId: dep.resource_id,
          deploymentId: dep.id,
        }}
        className="truncate font-medium text-gray-900 hover:text-brand-600 dark:text-white dark:hover:text-brand-400"
      >
        {children}
      </Link>
    )
  }
  return (
    <ResourceLink item={dep} className="truncate font-medium text-gray-900 hover:text-brand-600 dark:text-white dark:hover:text-brand-400">
      {children}
    </ResourceLink>
  )
}

function attentionLabel(item: AttentionItem) {
  if (item.kind === 'backup_failed') return 'Backup failed'
  if (item.kind === 'deploy_failed') return 'Deploy failed'
  return 'Needs a look'
}

export function DashboardPage() {
  const { user } = useAuth()
  const overview = useQuery({ queryKey: ['overview'], queryFn: api.overview })

  if (overview.isLoading) return <DashboardSkeleton />

  const data = overview.data
  const counts = data?.counts
  const hasServers = (counts?.servers ?? 0) > 0
  const recent = data?.recent_projects ?? []
  const attention = data?.attention ?? []
  const deployments = data?.deployments ?? []
  const frozen = data?.frozen_environments ?? []
  const deployWindow = data?.deploy_window
  const queue = data?.queue

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">
          Welcome{user?.name ? `, ${user.name}` : ''}
        </h1>
        {deployWindow ? (
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Last 7 days: {deployWindow.finished} finished
            {deployWindow.failed ? `, ${deployWindow.failed} failed` : ''}
            {deployWindow.active ? `, ${deployWindow.active} still running` : ''}
            {queue && (queue.queued > 0 || queue.in_progress > 0)
              ? ` · queue ${queue.in_progress} active, ${queue.queued} waiting`
              : ''}
          </p>
        ) : null}
      </div>

      {overview.error ? (
        <div className="panel-card border-error-200 p-4 text-sm text-error-500">
          Could not load the fleet pulse. {overview.error.message}
        </div>
      ) : null}

      {!overview.error && (counts?.projects ?? 0) === 0 && (
        <section className="panel-card border-brand-200 bg-brand-50/60 p-6 dark:border-brand-500/30 dark:bg-brand-500/10">
          <h2 className="text-lg font-medium text-gray-900 dark:text-white">Get the first deploy out</h2>
          <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
            Dockfin deploys to a Docker host you control. Three steps, then the pulse on this page starts filling in.
          </p>
          <ol className="mt-4 space-y-3 text-sm">
            <li className="flex items-start gap-3">
              <span className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-xs font-semibold ${hasServers ? 'bg-success-500 text-white' : 'bg-white text-gray-700 dark:bg-gray-900 dark:text-gray-200'}`}>
                {hasServers ? '✓' : '1'}
              </span>
              <div>
                <p className="font-medium text-gray-900 dark:text-white">Add a server</p>
                <p className="text-gray-600 dark:text-gray-300">
                  {hasServers
                    ? 'This host is already registered.'
                    : 'A VPS install usually adds itself. Otherwise add the host under Servers.'}
                </p>
                {!hasServers && (
                  <Link to="/servers" className="mt-1 inline-flex font-medium text-brand-600 hover:text-brand-700 dark:text-brand-400">
                    Go to Servers
                  </Link>
                )}
              </div>
            </li>
            <li className="flex items-start gap-3">
              <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-white text-xs font-semibold text-gray-700 dark:bg-gray-900 dark:text-gray-200">
                2
              </span>
              <div>
                <p className="font-medium text-gray-900 dark:text-white">Create a project</p>
                <p className="text-gray-600 dark:text-gray-300">Projects group environments, applications, databases, and services.</p>
                <Link to="/projects" className="mt-1 inline-flex font-medium text-brand-600 hover:text-brand-700 dark:text-brand-400">
                  Create a project
                </Link>
              </div>
            </li>
            <li className="flex items-start gap-3">
              <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-white text-xs font-semibold text-gray-700 dark:bg-gray-900 dark:text-gray-200">
                3
              </span>
              <div>
                <p className="font-medium text-gray-900 dark:text-white">Deploy something</p>
                <p className="text-gray-600 dark:text-gray-300">
                  Add an application from Git, or start a one-click service. Freeze the environment later if you need a quiet window.
                </p>
              </div>
            </li>
          </ol>
        </section>
      )}

      {hasServers === false && (counts?.projects ?? 0) > 0 && !overview.error && (
        <div className="panel-card border-brand-200 bg-brand-50/60 p-6 dark:border-brand-500/30 dark:bg-brand-500/10">
          <h2 className="text-lg font-medium text-gray-900 dark:text-white">No servers yet</h2>
          <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
            On a VPS install the first account should auto-add this host with its public IP. You can also
            add any host under Servers.
          </p>
          <Link
            to="/servers"
            className="mt-4 inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white hover:bg-brand-600"
          >
            Go to Servers
          </Link>
        </div>
      )}

      <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-5">
        <StatCard
          label="Servers"
          value={counts?.servers ?? '—'}
          to="/servers"
          icon={<Server className="h-4 w-4 text-brand-600 dark:text-brand-400" />}
          iconBg="bg-brand-50 dark:bg-brand-500/15"
        />
        <StatCard
          label="Projects"
          value={counts?.projects ?? '—'}
          to="/projects"
          icon={<FolderKanban className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />}
          iconBg="bg-emerald-50 dark:bg-emerald-500/15"
        />
        <StatCard
          label="Applications"
          value={counts?.applications ?? '—'}
          to="/projects"
          icon={<Rocket className="h-4 w-4 text-violet-600 dark:text-violet-400" />}
          iconBg="bg-violet-50 dark:bg-violet-500/15"
        />
        <StatCard
          label="Databases"
          value={counts?.databases ?? '—'}
          to="/projects"
          icon={<Database className="h-4 w-4 text-amber-600 dark:text-amber-400" />}
          iconBg="bg-amber-50 dark:bg-amber-500/15"
        />
        <StatCard
          label="Services"
          value={counts?.services ?? '—'}
          to="/projects"
          icon={<FolderKanban className="h-4 w-4 text-sky-600 dark:text-sky-400" />}
          iconBg="bg-sky-50 dark:bg-sky-500/15"
        />
      </div>

      {(attention.length > 0 || frozen.length > 0 || deployments.length > 0) && (
        <div className="grid gap-4 lg:grid-cols-2">
          <section className="panel-card p-4">
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Needs attention</h2>
            {frozen.length > 0 && (
              <ul className="mt-3 space-y-2">
                {frozen.map((env) => (
                  <li key={env.id} className="flex items-start gap-2 text-sm">
                    <Snowflake className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
                    <div className="min-w-0">
                      <Link
                        to="/projects/$projectId/environments/$envId"
                        params={{ projectId: env.project_id, envId: env.id }}
                        className="font-medium text-gray-900 hover:text-brand-600 dark:text-white dark:hover:text-brand-400"
                      >
                        {env.project_name} / {env.name}
                      </Link>
                      <p className="text-gray-500 dark:text-gray-400">
                        Deployments paused{env.reason ? `: ${env.reason}` : ''}
                      </p>
                    </div>
                  </li>
                ))}
              </ul>
            )}
            {attention.length === 0 ? (
              <p className="mt-3 text-sm text-gray-500 dark:text-gray-400">
                {frozen.length ? 'Nothing else looks broken.' : 'No failed deploys, unhealthy resources, or backup errors.'}
              </p>
            ) : (
              <ul className="mt-3 space-y-3">
                {attention.map((item) => (
                  <li key={`${item.kind}:${item.resource_type}:${item.resource_id}:${item.at || ''}`} className="flex items-start gap-2 text-sm">
                    <span className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${statusClass(item.status || 'failed')}`} />
                    <div className="min-w-0">
                      <ResourceLink item={item}>{item.resource_name || item.resource_type}</ResourceLink>
                      <p className="truncate text-gray-500 dark:text-gray-400">
                        {attentionLabel(item)}
                        {item.message && item.message !== attentionLabel(item) ? ` — ${item.message}` : ''}
                        {item.at ? ` · ${ago(item.at)}` : ''}
                      </p>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="panel-card p-4">
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Recent deploys</h2>
            {deployments.length === 0 ? (
              <p className="mt-3 text-sm text-gray-500 dark:text-gray-400">No deployments yet.</p>
            ) : (
              <ul className="mt-3 space-y-3">
                {deployments.map((dep) => (
                  <li key={dep.id} className="flex items-start gap-2 text-sm">
                    <span className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${statusClass(dep.status)}`} title={dep.status} />
                    <div className="min-w-0">
                      <DeploymentLink dep={dep}>{dep.resource_name || 'Deployment'}</DeploymentLink>
                      <p className="truncate text-gray-500 dark:text-gray-400">
                        {dep.status.replace('_', ' ')}
                        {dep.commit_message ? ` — ${dep.commit_message}` : ''}
                        {typeof dep.duration_seconds === 'number' ? ` · ${dep.duration_seconds}s` : ''}
                        {` · ${ago(dep.created_at)}`}
                      </p>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>
      )}

      <div>
        <h2 className="mb-3 text-lg font-medium text-gray-900 dark:text-white">Projects</h2>
        {recent.length === 0 ? (
          <div className="panel-card rounded-lg border border-dashed border-gray-200 px-4 py-8 text-center dark:border-gray-700">
            <p className="text-sm text-gray-500 dark:text-gray-400">No projects yet.</p>
            <Link
              to="/projects"
              className="mt-3 inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white hover:bg-brand-600"
            >
              Create a project
            </Link>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {recent.map((p: Project) => (
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
                  ) : null}
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
