import { AlertTriangle, Circle } from 'lucide-react'

export type SetupCheck = {
  id: string
  ok: boolean
  title: string
  hint: string
  actionLabel?: string
  onAction?: () => void
}

export function ResourceSetupBanner({ checks }: { checks: SetupCheck[] }) {
  const pending = checks.filter((c) => !c.ok)
  if (!pending.length) return null

  return (
    <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-500/30 dark:bg-amber-500/10">
      <div className="flex items-start gap-3">
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
        <div className="min-w-0 flex-1">
          <h3 className="text-sm font-semibold text-amber-950 dark:text-amber-100">
            Finish setup before you deploy
          </h3>
          <p className="mt-0.5 text-xs text-amber-800/80 dark:text-amber-200/80">
            {pending.length === 1
              ? 'One step is still open.'
              : `${pending.length} steps still need attention.`}
          </p>
          <ul className="mt-3 space-y-2">
            {pending.map((c) => (
              <li key={c.id} className="flex items-start gap-2 text-sm">
                <Circle className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
                <div className="min-w-0 flex-1">
                  <div className="font-medium text-gray-900 dark:text-white">{c.title}</div>
                  <p className="text-xs text-amber-900/70 dark:text-amber-100/70">{c.hint}</p>
                </div>
                {c.onAction && c.actionLabel ? (
                  <button
                    type="button"
                    onClick={c.onAction}
                    className="shrink-0 text-xs font-medium text-brand-700 hover:underline dark:text-brand-300"
                  >
                    {c.actionLabel}
                  </button>
                ) : null}
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  )
}
