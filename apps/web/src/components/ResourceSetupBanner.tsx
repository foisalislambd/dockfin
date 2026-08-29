import { AlertTriangle, CheckCircle2, Circle } from 'lucide-react'

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
            {checks.map((c) => (
              <li key={c.id} className="flex items-start gap-2 text-sm">
                {c.ok ? (
                  <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
                ) : (
                  <Circle className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
                )}
                <div className="min-w-0 flex-1">
                  <div className={`font-medium ${c.ok ? 'text-gray-500 line-through dark:text-gray-500' : 'text-gray-900 dark:text-white'}`}>
                    {c.title}
                  </div>
                  {!c.ok ? (
                    <p className="text-xs text-amber-900/70 dark:text-amber-100/70">{c.hint}</p>
                  ) : null}
                </div>
                {!c.ok && c.onAction && c.actionLabel ? (
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
