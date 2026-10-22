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
      <div className="space-y-4" role="status" aria-label="Loading">
        <Skeleton className="h-6 w-28" />
        <div className="panel-card p-5">
          <PanelSkeleton rows={3} showHeader={false} />
        </div>
        <div className="panel-card p-5">
          <PanelSkeleton rows={2} showHeader={false} />
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white">Webhooks</h2>

      <section className="panel-card space-y-4 p-4 sm:p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Deploy webhook</h3>
          <Link
            to="/security"
            search={{ tab: 'api-tokens' }}
            className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400"
          >
            API tokens
          </Link>
        </div>
        <CopyRow
          label="Deploy URL"
          value={deployUrl}
          copied={copied === 'url'}
          onCopy={() => void copy('url', deployUrl)}
        />
        <ExampleBlock label="Example (API token)" text={curlApi} />
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Use an API token with the <code className="font-mono">deploy</code> ability. GET and POST both work.
          Add <code className="font-mono">&amp;force=true</code> to recreate containers.
        </p>
      </section>

      <section className="panel-card space-y-4 p-4 sm:p-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Resource deploy token</h3>
          <Btn primary type="button" onClick={() => gen.mutate()} disabled={gen.isPending}>
            {gen.isPending ? 'Generating…' : info.data?.has_secret ? 'Rotate token' : 'Generate token'}
          </Btn>
        </div>
        <CopyRow
          label="Webhook URL"
          value={webhookUrl}
          copied={copied === 'hook'}
          onCopy={() => void copy('hook', webhookUrl)}
        />
        {info.data?.has_secret && !secret ? (
          <p className="text-xs text-gray-500 dark:text-gray-400">A token is already set. Its value stays hidden.</p>
        ) : null}
        {secret ? (
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3">
            <p className="text-xs font-medium text-amber-800 dark:text-amber-200">
              Copy this token now. It will not be shown again.
            </p>
            <code className="mt-2 block break-all font-mono text-xs sm:text-sm">{secret}</code>
            <div className="mt-3">
              <Btn type="button" onClick={() => void copy('secret', secret)}>
                {copied === 'secret' ? 'Copied' : 'Copy token'}
              </Btn>
            </div>
          </div>
        ) : null}
        {(secret || info.data?.has_secret) && <ExampleBlock label="Example (resource token)" text={curlSecret} />}
        {gen.error ? <p className="text-sm text-error-500">{gen.error.message}</p> : null}
      </section>
    </div>
  )
}

function CopyRow({
  label,
  value,
  copied,
  onCopy,
}: {
  label: string
  value: string
  copied: boolean
  onCopy: () => void
}) {
  return (
    <div className="min-w-0">
      <div className="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">{label}</div>
      <div className="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
        <code
          title={value}
          className="block min-w-0 flex-1 truncate rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 font-mono text-xs text-gray-800 dark:border-gray-800 dark:bg-white/5 dark:text-gray-200"
        >
          {value || '—'}
        </code>
        <div className="shrink-0">
          <Btn type="button" disabled={!value} onClick={onCopy}>
            {copied ? 'Copied' : 'Copy'}
          </Btn>
        </div>
      </div>
    </div>
  )
}

function ExampleBlock({ label, text }: { label: string; text: string }) {
  return (
    <div className="min-w-0">
      <div className="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">{label}</div>
      <pre className="max-w-full overflow-x-auto whitespace-pre-wrap break-all rounded-lg border border-gray-200 bg-gray-50 p-3 font-mono text-[11px] leading-5 text-gray-700 dark:border-gray-800 dark:bg-white/5 dark:text-gray-300">
        {text || '—'}
      </pre>
    </div>
  )
}
