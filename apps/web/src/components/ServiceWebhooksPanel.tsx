import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { api } from '../lib/api'
import { Btn } from '../pages/Servers'
import { PanelSkeleton, Skeleton } from './ui/Skeleton'

type Props = {
  serviceId: string
}

export function ServiceWebhooksPanel({ serviceId }: Props) {
  const qc = useQueryClient()
  const info = useQuery({
    queryKey: ['service-webhook', serviceId],
    queryFn: () => api.serviceWebhook(serviceId),
  })
  const [secret, setSecret] = useState<string | null>(null)
  const [copied, setCopied] = useState('')

  const gen = useMutation({
    mutationFn: () => api.setServiceWebhookSecret(serviceId),
    onSuccess: (data) => {
      setSecret(data.secret)
      void qc.invalidateQueries({ queryKey: ['service-webhook', serviceId] })
    },
  })

  const copy = async (label: string, value: string) => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(label)
      setTimeout(() => setCopied(''), 1500)
    } catch {
      /* ignore */
    }
  }

  const deployUrl = info.data?.deploy_url || ''
  const webhookUrl = info.data?.deploy_webhook_url || ''
  const tokenHint = secret || '<DEPLOY_TOKEN>'

  const curlApi = deployUrl
    ? `curl --request GET '${deployUrl}' \\\n  --header 'Authorization: Bearer <API_TOKEN_WITH_DEPLOY>'`
    : ''
  const curlSecret = webhookUrl
    ? `curl --request POST '${webhookUrl}' \\\n  --header 'Authorization: Bearer ${tokenHint}'`
    : ''

  if (info.isLoading) {
    return (
      <div className="space-y-5" role="status" aria-label="Loading">
        <div>
          <Skeleton className="h-4 w-24" />
          <Skeleton className="mt-2 h-3 w-72 max-w-full" />
        </div>
        <div className="panel-card p-5">
          <PanelSkeleton rows={3} />
        </div>
        <div className="panel-card p-5">
          <PanelSkeleton rows={2} />
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Webhooks</h2>
        <p className="mt-1 max-w-2xl text-sm text-gray-500 dark:text-gray-400">
          Trigger a redeploy of this service from CI or an external system.
        </p>
      </div>

      <section className="panel-card space-y-3 p-5">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Deploy webhook</h3>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Authenticate with an{' '}
          <Link
            to="/security"
            search={{ tab: 'api-tokens' }}
            className="text-brand-600 hover:underline dark:text-brand-400"
          >
            API token
          </Link>{' '}
          that has the <code className="font-mono text-xs">deploy</code> ability, or with the
          resource deploy token below.
        </p>
        <div>
          <div className="mb-1 text-xs text-gray-500 dark:text-gray-400">Deploy URL</div>
          <div className="flex flex-wrap items-start gap-2">
            <code className="block flex-1 break-all rounded-lg bg-gray-50 px-3 py-2 font-mono text-xs dark:bg-white/5">
              {deployUrl}
            </code>
            <Btn type="button" disabled={!deployUrl} onClick={() => void copy('url', deployUrl)}>
              {copied === 'url' ? 'Copied' : 'Copy'}
            </Btn>
          </div>
        </div>
        <div>
          <div className="mb-1 text-xs text-gray-500 dark:text-gray-400">Example (API token)</div>
          <pre className="overflow-x-auto rounded-lg bg-gray-50 p-3 font-mono text-[11px] text-gray-700 dark:bg-white/5 dark:text-gray-300">
            {curlApi || '—'}
          </pre>
        </div>
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Append <code className="font-mono">&amp;force=true</code> to force recreate. GET and POST
          are both accepted.
        </p>
      </section>

      <section className="panel-card space-y-3 p-5">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Resource deploy token</h3>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Optional dedicated token for this service only. Shown once when generated — store it in
          your CI secrets.
        </p>
        <div>
          <div className="mb-1 text-xs text-gray-500 dark:text-gray-400">Webhook URL</div>
          <div className="flex flex-wrap items-start gap-2">
            <code className="block flex-1 break-all rounded-lg bg-gray-50 px-3 py-2 font-mono text-xs dark:bg-white/5">
              {webhookUrl}
            </code>
            <Btn type="button" disabled={!webhookUrl} onClick={() => void copy('hook', webhookUrl)}>
              {copied === 'hook' ? 'Copied' : 'Copy'}
            </Btn>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Btn primary type="button" onClick={() => gen.mutate()} disabled={gen.isPending}>
            {gen.isPending
              ? 'Generating…'
              : info.data?.has_secret
                ? 'Rotate deploy token'
                : 'Generate deploy token'}
          </Btn>
          {info.data?.has_secret && !secret && (
            <span className="text-xs text-gray-500 dark:text-gray-400">
              A token is already configured (value hidden).
            </span>
          )}
        </div>
        {secret && (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3">
            <div className="text-xs font-medium text-amber-800 dark:text-amber-200">
              Copy this token now — it will not be shown again
            </div>
            <code className="mt-1 block break-all font-mono text-sm">{secret}</code>
            <div className="mt-2">
              <Btn type="button" onClick={() => void copy('secret', secret)}>
                {copied === 'secret' ? 'Copied' : 'Copy token'}
              </Btn>
            </div>
          </div>
        )}
        {(secret || info.data?.has_secret) && (
          <div>
            <div className="mb-1 text-xs text-gray-500 dark:text-gray-400">Example (resource token)</div>
            <pre className="overflow-x-auto rounded-lg bg-gray-50 p-3 font-mono text-[11px] text-gray-700 dark:bg-white/5 dark:text-gray-300">
              {curlSecret}
            </pre>
          </div>
        )}
        {gen.error && <p className="text-sm text-error-500">{gen.error.message}</p>}
      </section>
    </div>
  )
}
