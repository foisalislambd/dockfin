import { Header } from './Servers'

export function SettingsPage() {
  return (
    <div className="space-y-6">
      <Header
        title="Settings"
      />
      <div className="rounded-xl border border-gray-200 dark:border-gray-800 panel-card bg-white dark:bg-white/3/60 p-6">
        <h2 className="text-lg font-medium">Instance settings</h2>
        <dl className="mt-6 grid gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs text-gray-500 dark:text-gray-400">API</dt>
            <dd className="mt-1 font-mono text-sm">/api/v1</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 dark:text-gray-400">Config path (hosts)</dt>
            <dd className="mt-1 font-mono text-sm">/data/goolify</dd>
          </div>
        </dl>
      </div>
    </div>
  )
}
