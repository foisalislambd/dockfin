import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import { Btn, Header } from './Servers'

const CHANNELS = [
  { id: 'discord', label: 'Discord', placeholder: '{"webhook_url":"https://discord.com/api/webhooks/..."}' },
  { id: 'slack', label: 'Slack', placeholder: '{"webhook_url":"https://hooks.slack.com/services/..."}' },
  { id: 'webhook', label: 'Webhook', placeholder: '{"url":"https://example.com/hook","headers":{}}' },
] as const

export function NotificationsPage() {
  const qc = useQueryClient()
  const list = useQuery({ queryKey: ['notifications'], queryFn: api.notifications })
  const [local, setLocal] = useState<
    Record<string, { enabled: boolean; config: string }>
  >({
    discord: { enabled: false, config: '{\n  "webhook_url": ""\n}' },
    slack: { enabled: false, config: '{\n  "webhook_url": ""\n}' },
    webhook: { enabled: false, config: '{\n  "url": ""\n}' },
  })
  const [error, setError] = useState('')
  const [saved, setSaved] = useState('')

  useEffect(() => {
    if (!list.data?.notifications) return
    setLocal((prev) => {
      const next = { ...prev }
      for (const n of list.data!.notifications) {
        if (n.channel in next) {
          next[n.channel] = {
            enabled: n.enabled,
            config: prev[n.channel]?.config || '{\n}\n',
          }
        }
      }
      return next
    })
  }, [list.data])

  const save = useMutation({
    mutationFn: async (channel: string) => {
      const row = local[channel]
      let config: unknown
      try {
        config = JSON.parse(row.config)
      } catch {
        throw new Error('Config must be valid JSON')
      }
      return api.upsertNotification(channel, { enabled: row.enabled, config })
    },
    onSuccess: (_d, channel) => {
      setError('')
      setSaved(`Saved ${channel}`)
      void qc.invalidateQueries({ queryKey: ['notifications'] })
    },
    onError: (e: Error) => {
      setSaved('')
      setError(e.message)
    },
  })

  return (
    <div className="space-y-6">
      <Header
        title="Notifications"
        subtitle="Discord, Slack, and generic webhooks for deploy events."
      />
      {error && <p className="text-sm text-error-500">{error}</p>}
      {saved && <p className="text-sm text-brand-600 dark:text-brand-400">{saved}</p>}
      <div className="space-y-4">
        {CHANNELS.map((ch) => {
          const row = local[ch.id]
          return (
            <div
              key={ch.id}
              className="rounded-xl border border-gray-200 dark:border-gray-800 panel-card bg-white dark:bg-white/3/60 p-5"
            >
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 className="text-lg font-medium">{ch.label}</h2>
                  <p className="text-sm text-gray-500 dark:text-gray-400">Channel: {ch.id}</p>
                </div>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={row.enabled}
                    onChange={(e) =>
                      setLocal({
                        ...local,
                        [ch.id]: { ...row, enabled: e.target.checked },
                      })
                    }
                    className="accent-[var(--color-accent)]"
                  />
                  Enabled
                </label>
              </div>
              <label className="mt-4 block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">Config (JSON)</span>
                <textarea
                  rows={4}
                  value={row.config}
                  placeholder={ch.placeholder}
                  onChange={(e) =>
                    setLocal({
                      ...local,
                      [ch.id]: { ...row, config: e.target.value },
                    })
                  }
                  className="w-full rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2 font-mono text-xs"
                />
              </label>
              <div className="mt-3">
                <Btn primary onClick={() => save.mutate(ch.id)}>
                  Save {ch.label}
                </Btn>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
