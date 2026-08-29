import type { Application, Deployment, Destination, Service } from '../lib/api'
import type { ResourceLink } from './LinksMenu'
import { StatusBadge } from './StatusBadge'
import { ServiceLogo } from './ServiceLogo'
import { Btn } from '../pages/Servers'
import { safeExternalHref } from '../lib/url'
import { Box, ExternalLink, GitBranch, Globe, Server, Variable } from 'lucide-react'

type VisitTarget = { fqdn?: string; links?: { url: string }[] }

export function formatRelativeTime(iso: string | undefined) {
  if (!iso) return '—'
  const then = new Date(iso).getTime()
  if (!Number.isFinite(then)) return '—'
  const sec = Math.max(0, Math.round((Date.now() - then) / 1000))
  if (sec < 45) return 'just now'
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`
  if (sec < 86400 * 30) return `${Math.floor(sec / 86400)}d ago`
  return new Date(iso).toLocaleDateString()
}

export function gitRepoLabel(repo?: string) {
  const raw = (repo || '').replace(/\.git$/, '')
  const m = raw.match(/([^/:]+)\/([^/]+)$/)
  return m ? `${m[1]}/${m[2]}` : raw || ''
}

export function primaryVisitUrl(app: VisitTarget): string | undefined {
  const fromLink = app.links?.map((l) => l.url).find((u) => safeExternalHref(u))
  if (fromLink) return safeExternalHref(fromLink)
  const raw = (app.fqdn || '').split(',')[0]?.trim()
  if (!raw) return undefined
  if (/^https?:\/\//i.test(raw)) return safeExternalHref(raw)
  const host = raw.replace(/^https?:\/\//i, '')
  const magic = /\.(sslip\.io|nip\.io)(:\d+)?$/i.test(host)
  return safeExternalHref(`${magic ? 'http' : 'https'}://${host}`)
}

function PreviewChrome({
  host,
  logoSrc,
  name,
  href,
}: {
  host: string
  logoSrc: string
  name: string
  href?: string
}) {
  const inner = (
    <div className="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-950">
      <div className="flex items-center gap-1.5 border-b border-gray-200 px-3 py-2 dark:border-gray-800">
        <span className="h-2 w-2 rounded-full bg-gray-300 dark:bg-gray-600" />
        <span className="h-2 w-2 rounded-full bg-gray-300 dark:bg-gray-600" />
        <span className="h-2 w-2 rounded-full bg-gray-300 dark:bg-gray-600" />
        <span className="ml-2 min-w-0 truncate font-mono text-[11px] text-gray-500">
          {host || 'No production URL yet'}
        </span>
      </div>
      <div className="flex h-36 items-center justify-center bg-gradient-to-br from-brand-500/10 via-transparent to-gray-100 dark:to-white/5">
        <ServiceLogo src={logoSrc} name={name} className="h-14 w-14 opacity-90" />
      </div>
    </div>
  )
  if (!href) return inner
  return (
    <a href={href} target="_blank" rel="noreferrer" className="block rounded-lg ring-brand-500/0 transition hover:ring-2 hover:ring-brand-500/40">
      {inner}
    </a>
  )
}

function gitWebHref(repo?: string) {
  const raw = (repo || '').trim().replace(/\.git$/, '')
  if (!raw) return undefined
  if (/^https?:\/\//i.test(raw)) return safeExternalHref(raw)
  const ssh = raw.match(/^git@([^:]+):(.+)$/)
  if (ssh) return safeExternalHref(`https://${ssh[1]}/${ssh[2]}`)
  return undefined
}

export function DeploymentRows({
  deployments,
  onOpen,
  onCancel,
  empty = 'No deployments yet.',
}: {
  deployments: Deployment[]
  onOpen: (id: string) => void
  onCancel?: (id: string) => void
  empty?: string
}) {
  if (!deployments.length) {
    return (
      <div className="px-5 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{empty}</div>
    )
  }
  return (
    <div className="divide-y divide-gray-200 dark:divide-gray-800">
      {deployments.map((d) => {
        const busy = d.status === 'queued' || d.status === 'in_progress'
        const title = (d.commit_message || '').trim() || d.current_stage || d.status
        return (
          <div key={d.id} className="flex flex-wrap items-center gap-3 px-5 py-3.5 hover:bg-gray-50 dark:hover:bg-white/[0.03]">
            <button
              type="button"
              className="min-w-0 flex-1 text-left"
              onClick={() => onOpen(d.id)}
            >
              <div className="flex flex-wrap items-center gap-2">
                <StatusBadge status={d.status} />
                <span className="truncate text-sm font-medium text-gray-900 dark:text-white">
                  {title}
                </span>
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                {d.commit_sha ? (
                  <span className="font-mono">{d.commit_sha.slice(0, 7)}</span>
                ) : (
                  <span className="font-mono">{d.id.slice(0, 8)}</span>
                )}
                <span>·</span>
                <span>{formatRelativeTime(d.created_at)}</span>
                {d.error_message ? (
                  <>
                    <span>·</span>
                    <span className="truncate text-error-500">{d.error_message}</span>
                  </>
                ) : null}
              </div>
            </button>
            <div className="flex shrink-0 items-center gap-2">
              {busy && onCancel ? (
                <button
                  type="button"
                  className="text-xs font-medium text-error-500 hover:underline"
                  onClick={() => onCancel(d.id)}
                >
                  Cancel
                </button>
              ) : null}
              <button
                type="button"
                className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400"
                onClick={() => onOpen(d.id)}
              >
                Logs
              </button>
            </div>
          </div>
        )
      })}
    </div>
  )
}

export function AppOverview({
  app,
  logoSrc,
  latest,
  recent,
  destination,
  emptyEnvCount,
  envTotal,
  links,
  onOpenDeployment,
  onCancelDeployment,
  onRedeploy,
  onOpenSettings,
  onViewAllDeployments,
  deployBusy,
  showGit = true,
}: {
  app: Application
  logoSrc: string
  latest?: Deployment
  recent: Deployment[]
  destination?: Destination
  emptyEnvCount: number
  envTotal: number
  links: ResourceLink[]
  onOpenDeployment: (id: string) => void
  onCancelDeployment?: (id: string) => void
  onRedeploy: () => void
  onOpenSettings: (side: 'general' | 'environment' | 'servers' | 'git') => void
  onViewAllDeployments?: () => void
  deployBusy?: boolean
  showGit?: boolean
}) {
  const visit = primaryVisitUrl(app)
  const host = visit ? visit.replace(/^https?:\/\//, '').replace(/\/$/, '') : ''
  const repo = gitRepoLabel(app.git_repository)
  const domains = [
    ...new Set(
      [
        ...(app.fqdn || '')
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
        ...links.map((l) => l.url),
      ].filter(Boolean),
    ),
  ]
  const hasLocalhost = domains.some(
    (u) => u.includes('127.0.0.1') || u.includes('localhost') || u.includes('.0.0.0.0.'),
  )

  return (
    <div className="space-y-6">
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.6fr)_minmax(18rem,1fr)]">
        <section className="panel-card overflow-hidden">
          <div className="flex items-center justify-between gap-3 border-b border-gray-200 px-5 py-3 dark:border-gray-800">
            <div>
              <p className="text-[11px] font-medium tracking-wide text-gray-500 uppercase dark:text-gray-400">
                Production
              </p>
              <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                Production Deployment
              </h2>
            </div>
            <StatusBadge status={latest?.status || app.status} />
          </div>

          <div className="border-b border-gray-200 bg-gray-50 px-5 py-4 dark:border-gray-800 dark:bg-white/[0.03]">
            <PreviewChrome
              host={host}
              logoSrc={logoSrc}
              name={app.name}
              href={visit && safeExternalHref(visit) ? safeExternalHref(visit) : undefined}
            />
          </div>

          <div className="space-y-4 px-5 py-4">
            <dl className="grid gap-3 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-xs text-gray-500 dark:text-gray-400">Last deployed</dt>
                <dd className="mt-0.5 text-gray-900 dark:text-white">
                  {latest ? formatRelativeTime(latest.created_at) : 'Not deployed yet'}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-gray-500 dark:text-gray-400">Source</dt>
                <dd className="mt-0.5 flex items-center gap-1.5 text-gray-900 dark:text-white">
                  <GitBranch className="h-3.5 w-3.5 shrink-0 text-gray-400" />
                  {(() => {
                    const href = gitWebHref(app.git_repository)
                    const label = repo || 'No git source'
                    return href ? (
                      <a href={href} target="_blank" rel="noreferrer" className="truncate hover:text-brand-600 dark:hover:text-brand-400">
                        {label}
                        {app.git_branch ? <span className="text-gray-500"> · {app.git_branch}</span> : null}
                      </a>
                    ) : (
                      <span className="truncate">
                        {label}
                        {app.git_branch ? <span className="text-gray-500"> · {app.git_branch}</span> : null}
                      </span>
                    )
                  })()}
                </dd>
              </div>
              {latest?.commit_sha || latest?.commit_message ? (
                <div className="sm:col-span-2">
                  <dt className="text-xs text-gray-500 dark:text-gray-400">Commit</dt>
                  <dd className="mt-0.5 truncate text-gray-900 dark:text-white">
                    {latest.commit_sha ? (
                      <span className="mr-2 font-mono text-xs">{latest.commit_sha.slice(0, 7)}</span>
                    ) : null}
                    {latest.commit_message || '—'}
                  </dd>
                </div>
              ) : null}
            </dl>
            <div className="flex flex-wrap gap-2">
              {visit && safeExternalHref(visit) ? null : (
                <Btn onClick={() => onOpenSettings('general')}>Add domain</Btn>
              )}
              <Btn primary onClick={onRedeploy} disabled={deployBusy}>
                {deployBusy ? 'Queueing…' : 'Redeploy'}
              </Btn>
              {latest ? (
                <Btn onClick={() => onOpenDeployment(latest.id)}>Deployment logs</Btn>
              ) : null}
            </div>
          </div>
        </section>

        <div className="space-y-4">
          <section className="panel-card p-5">
            <div className="mb-3 flex items-center justify-between gap-2">
              <h3 className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <Globe className="h-3.5 w-3.5 text-gray-400" />
                Domains
              </h3>
              <button
                type="button"
                className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400"
                onClick={() => onOpenSettings('general')}
              >
                Edit
              </button>
            </div>
            {hasLocalhost ? (
              <p className="mb-2 text-xs text-amber-700 dark:text-amber-300">
                A link uses localhost — set the server public IP, then redeploy.
              </p>
            ) : null}
            {domains.length ? (
              <ul className="space-y-2">
                {domains.slice(0, 6).map((d) => {
                  const href = safeExternalHref(d) || (/^https?:\/\//i.test(d) ? undefined : safeExternalHref(`http://${d}`))
                  return (
                    <li key={d} className="min-w-0 truncate font-mono text-xs text-gray-700 dark:text-gray-300">
                      {href ? (
                        <a href={href} target="_blank" rel="noreferrer" className="hover:text-brand-600 dark:hover:text-brand-400">
                          {d.replace(/^https?:\/\//, '')}
                        </a>
                      ) : (
                        d
                      )}
                    </li>
                  )
                })}
              </ul>
            ) : (
              <p className="text-sm text-gray-500 dark:text-gray-400">No domains assigned.</p>
            )}
          </section>

          <section className="panel-card space-y-3 p-5">
            <button
              type="button"
              className="flex w-full items-start justify-between gap-2 text-left"
              onClick={() => onOpenSettings('environment')}
            >
              <span className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <Variable className="h-3.5 w-3.5 text-gray-400" />
                Environment
              </span>
              <span className="text-xs text-brand-600 dark:text-brand-400">Edit</span>
            </button>
            {emptyEnvCount > 0 ? (
              <p className="text-sm text-amber-700 dark:text-amber-300">
                {emptyEnvCount} {emptyEnvCount === 1 ? 'variable' : 'variables'} still need a value.
              </p>
            ) : (
              <p className="text-sm text-gray-500 dark:text-gray-400">
                {envTotal ? `${envTotal} variables configured` : 'No variables yet'}
              </p>
            )}
          </section>

          <section className="panel-card p-5">
            <button
              type="button"
              className="flex w-full items-start justify-between gap-2 text-left"
              onClick={() => onOpenSettings('servers')}
            >
              <span className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <Server className="h-3.5 w-3.5 text-gray-400" />
                Destination
              </span>
              <span className="text-xs text-brand-600 dark:text-brand-400">Edit</span>
            </button>
            <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
              {destination?.name || (app.destination_id ? 'Assigned' : 'No destination')}
            </p>
            <p className="mt-0.5 text-xs capitalize text-gray-500 dark:text-gray-400">{app.build_pack}</p>
          </section>

          {showGit ? (
          <section className="panel-card p-5">
            <button
              type="button"
              className="flex w-full items-start justify-between gap-2 text-left"
              onClick={() => onOpenSettings('git')}
            >
              <span className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <GitBranch className="h-3.5 w-3.5 text-gray-400" />
                Git
              </span>
              <span className="text-xs text-brand-600 dark:text-brand-400">Edit</span>
            </button>
            <p className="mt-2 truncate text-sm text-gray-600 dark:text-gray-300">
              {repo || 'No repository connected'}
            </p>
            {app.git_branch ? (
              <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{app.git_branch}</p>
            ) : null}
          </section>
          ) : null}
        </div>
      </div>

      <section className="panel-card overflow-hidden">
        <div className="flex items-center justify-between gap-2 border-b border-gray-200 px-5 py-3 dark:border-gray-800">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Deployments</h3>
          {onViewAllDeployments ? (
            <button
              type="button"
              className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400"
              onClick={onViewAllDeployments}
            >
              View all
            </button>
          ) : null}
        </div>
        <DeploymentRows
          deployments={recent}
          onOpen={onOpenDeployment}
          onCancel={onCancelDeployment}
          empty="No deployments yet. Redeploy to create the first production build."
        />
      </section>
    </div>
  )
}

export function ServiceOverview({
  service,
  logoSrc,
  destinationName,
  emptyEnvCount,
  envTotal,
  onRedeploy,
  onOpenLogs,
  onOpenSettings,
  deployBusy,
}: {
  service: Service
  logoSrc: string
  destinationName?: string
  emptyEnvCount: number
  envTotal: number
  onRedeploy: () => void
  onOpenLogs: () => void
  onOpenSettings: (side: 'general' | 'domains' | 'environment') => void
  deployBusy?: boolean
}) {
  const visit = primaryVisitUrl(service)
  const host = visit ? visit.replace(/^https?:\/\//, '').replace(/\/$/, '') : ''
  const links = service.links || []
  const domains = [
    ...new Set(
      [
        ...(service.fqdn || '')
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
        ...links.map((l) => l.url),
      ].filter(Boolean),
    ),
  ]
  const hasLocalhost = domains.some(
    (u) => u.includes('127.0.0.1') || u.includes('localhost') || u.includes('.0.0.0.0.'),
  )
  const units = service.units || []

  return (
    <div className="space-y-6">
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.6fr)_minmax(18rem,1fr)]">
        <section className="panel-card overflow-hidden">
          <div className="flex items-center justify-between gap-3 border-b border-gray-200 px-5 py-3 dark:border-gray-800">
            <div>
              <p className="text-[11px] font-medium tracking-wide text-gray-500 uppercase dark:text-gray-400">
                Production
              </p>
              <h2 className="text-sm font-semibold text-gray-900 dark:text-white">
                Production Deployment
              </h2>
            </div>
            <StatusBadge status={service.status} />
          </div>

          <div className="border-b border-gray-200 bg-gray-50 px-5 py-4 dark:border-gray-800 dark:bg-white/[0.03]">
            <PreviewChrome
              host={host}
              logoSrc={logoSrc}
              name={service.name}
              href={visit && safeExternalHref(visit) ? safeExternalHref(visit) : undefined}
            />
          </div>

          <div className="space-y-4 px-5 py-4">
            <dl className="grid gap-3 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-xs text-gray-500 dark:text-gray-400">Type</dt>
                <dd className="mt-0.5 capitalize text-gray-900 dark:text-white">
                  {(service.service_type || '').replace(/[-_]+/g, ' ')}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-gray-500 dark:text-gray-400">Containers</dt>
                <dd className="mt-0.5 text-gray-900 dark:text-white">
                  {units.length ? `${units.length} unit${units.length === 1 ? '' : 's'}` : 'None listed'}
                </dd>
              </div>
            </dl>
            <div className="flex flex-wrap gap-2">
              {visit && safeExternalHref(visit) ? (
                <a
                  href={safeExternalHref(visit)}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex h-8 items-center gap-1.5 rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white hover:bg-brand-600"
                >
                  Visit
                  <ExternalLink className="h-3.5 w-3.5" />
                </a>
              ) : (
                <Btn onClick={() => onOpenSettings('domains')}>Add domain</Btn>
              )}
              <Btn primary onClick={onRedeploy} disabled={deployBusy}>
                {deployBusy ? 'Deploying…' : 'Redeploy'}
              </Btn>
              <Btn onClick={onOpenLogs}>Logs</Btn>
            </div>
          </div>
        </section>

        <div className="space-y-4">
          <section className="panel-card p-5">
            <div className="mb-3 flex items-center justify-between gap-2">
              <h3 className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <Globe className="h-3.5 w-3.5 text-gray-400" />
                Domains
              </h3>
              <button
                type="button"
                className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400"
                onClick={() => onOpenSettings('domains')}
              >
                Edit
              </button>
            </div>
            {hasLocalhost ? (
              <p className="mb-2 text-xs text-amber-700 dark:text-amber-300">
                A link uses localhost — set the server public IP, then redeploy.
              </p>
            ) : null}
            {domains.length ? (
              <ul className="space-y-2">
                {domains.slice(0, 6).map((d) => {
                  const href =
                    safeExternalHref(d) ||
                    (/^https?:\/\//i.test(d) ? undefined : safeExternalHref(`http://${d}`))
                  return (
                    <li key={d} className="min-w-0 truncate font-mono text-xs text-gray-700 dark:text-gray-300">
                      {href ? (
                        <a
                          href={href}
                          target="_blank"
                          rel="noreferrer"
                          className="hover:text-brand-600 dark:hover:text-brand-400"
                        >
                          {d.replace(/^https?:\/\//, '')}
                        </a>
                      ) : (
                        d
                      )}
                    </li>
                  )
                })}
              </ul>
            ) : (
              <p className="text-sm text-gray-500 dark:text-gray-400">No domains assigned.</p>
            )}
          </section>

          <section className="panel-card space-y-3 p-5">
            <button
              type="button"
              className="flex w-full items-start justify-between gap-2 text-left"
              onClick={() => onOpenSettings('environment')}
            >
              <span className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <Variable className="h-3.5 w-3.5 text-gray-400" />
                Environment
              </span>
              <span className="text-xs text-brand-600 dark:text-brand-400">Edit</span>
            </button>
            {emptyEnvCount > 0 ? (
              <p className="text-sm text-amber-700 dark:text-amber-300">
                {emptyEnvCount} {emptyEnvCount === 1 ? 'variable' : 'variables'} still need a value.
              </p>
            ) : (
              <p className="text-sm text-gray-500 dark:text-gray-400">
                {envTotal ? `${envTotal} variables configured` : 'No variables yet'}
              </p>
            )}
          </section>

          <section className="panel-card p-5">
            <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
              <Server className="h-3.5 w-3.5 text-gray-400" />
              Destination
            </div>
            <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
              {destinationName || (service.destination_id || service.server_id ? 'Assigned' : 'No destination')}
            </p>
          </section>
        </div>
      </div>

      <section className="panel-card overflow-hidden">
        <div className="border-b border-gray-200 px-5 py-3 dark:border-gray-800">
          <h3 className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
            <Box className="h-3.5 w-3.5 text-gray-400" />
            Containers
          </h3>
        </div>
        {units.length ? (
          <div className="divide-y divide-gray-200 dark:divide-gray-800">
            {units.map((u) => (
              <div key={u.name} className="flex flex-wrap items-center justify-between gap-3 px-5 py-3.5">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-gray-900 dark:text-white">{u.name}</p>
                  <p className="mt-0.5 truncate font-mono text-xs text-gray-500 dark:text-gray-400">
                    {u.image}
                  </p>
                </div>
                <StatusBadge status={u.status || service.status} />
              </div>
            ))}
          </div>
        ) : (
          <div className="px-5 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
            No compose units yet. Deploy to create containers.
          </div>
        )}
      </section>
    </div>
  )
}
