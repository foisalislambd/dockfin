import type { ReactNode } from 'react'

export function DeployFreezeNotice({
  reason,
  action,
}: {
  reason?: string
  action?: ReactNode
}) {
  return (
    <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-950 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-100">
      <p className="font-medium">Deployments are paused</p>
      <p className="mt-1 text-amber-900/80 dark:text-amber-100/80">
        {reason?.trim()
          ? reason
          : 'New deploys from the dashboard, webhooks, and previews are skipped until this environment is unfrozen.'}{' '}
        Restarts, stops, and scheduled commands still run.
      </p>
      {action ? <div className="mt-3 flex flex-wrap items-center gap-2">{action}</div> : null}
    </div>
  )
}
