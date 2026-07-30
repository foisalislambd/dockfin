import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { api } from '../lib/api'

export function DashboardPage() {
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers })
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const apps = useQuery({ queryKey: ['applications'], queryFn: () => api.applications() })
  const dbs = useQuery({ queryKey: ['databases'], queryFn: api.databases })

  const hasServers = (servers.data?.servers?.length ?? 0) > 0

  const cards = [
    { label: 'Servers', value: servers.data?.servers.length ?? '—', to: '/servers' },
    { label: 'Projects', value: projects.data?.projects.length ?? '—', to: '/projects' },
    { label: 'Applications', value: apps.data?.applications.length ?? '—', to: '/applications' },
    { label: 'Databases', value: dbs.data?.databases.length ?? '—', to: '/databases' },
  ]

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-semibold tracking-tight">Dashboard</h1>
        <p className="mt-2 text-[var(--color-muted)]">
          Your self-hosted PaaS at a glance. Add a server, then deploy.
        </p>
      </div>

      {!servers.isLoading && !hasServers && (
        <div className="rounded-xl border border-[var(--color-accent)]/40 bg-[var(--color-panel)]/70 p-6">
          <h2 className="text-lg font-medium">Welcome — no servers yet</h2>
          <p className="mt-2 text-sm text-[var(--color-muted)]">
            Run the onboarding wizard to add an SSH key, connect a host, start the proxy, and create your first project.
          </p>
          <Link
            to="/onboarding"
            className="mt-4 inline-flex rounded-lg bg-[var(--color-accent)] px-4 py-2 text-sm font-medium text-[var(--color-ink)] hover:bg-[var(--color-accent-2)]"
          >
            Start onboarding
          </Link>
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {cards.map((c) => (
          <Link
            key={c.label}
            to={c.to}
            className="rounded-xl border border-[var(--color-line)] bg-[var(--color-panel)]/70 p-5 transition hover:border-[var(--color-accent)]"
          >
            <div className="text-sm text-[var(--color-muted)]">{c.label}</div>
            <div className="mt-2 text-3xl font-semibold text-[var(--color-accent)]">{c.value}</div>
          </Link>
        ))}
      </div>
      <div className="rounded-xl border border-[var(--color-line)] bg-[var(--color-panel)]/50 p-6">
        <h2 className="text-lg font-medium">Quick start</h2>
        <ol className="mt-3 list-decimal space-y-2 pl-5 text-sm text-[var(--color-muted)]">
          <li>Upload an SSH private key</li>
          <li>Add a server and validate Docker</li>
          <li>Start Traefik proxy</li>
          <li>Create a project and deploy an application</li>
        </ol>
        <Link to="/onboarding" className="mt-4 inline-block text-sm text-[var(--color-accent)] hover:underline">
          Open onboarding wizard →
        </Link>
      </div>
    </div>
  )
}
