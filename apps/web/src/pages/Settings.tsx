import { Header } from './Servers'

export function SettingsPage() {
  return (
    <div className="space-y-6">
      <Header
        title="Settings"
        subtitle="Instance configuration for this Goolify installation."
      />
      <div className="rounded-xl border border-[var(--color-line)] bg-[var(--color-panel)]/60 p-6">
        <h2 className="text-lg font-medium">Instance settings</h2>
        <p className="mt-2 text-sm text-[var(--color-muted)]">
          Coming soon — domain defaults, registry credentials, and team preferences will live here.
        </p>
        <dl className="mt-6 grid gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs text-[var(--color-muted)]">API</dt>
            <dd className="mt-1 font-mono text-sm">/api/v1</dd>
          </div>
          <div>
            <dt className="text-xs text-[var(--color-muted)]">Config path (hosts)</dt>
            <dd className="mt-1 font-mono text-sm">/data/goolify</dd>
          </div>
        </dl>
      </div>
    </div>
  )
}
