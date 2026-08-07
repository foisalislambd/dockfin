import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bell, Hash, Mail, MessageCircle, Send, Webhook } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { FormPageSkeleton } from '../components/ui/Skeleton'
import { ResourceTabs, TabPanel } from '../components/ui/tabs'
import { api, type NotificationSetting } from '../lib/api'
import { Btn, Header, Input, Modal } from './Servers'

const CHANNELS = [
  { id: 'email', label: 'Email', icon: Mail },
  { id: 'discord', label: 'Discord', icon: Hash },
  { id: 'telegram', label: 'Telegram', icon: Send },
  { id: 'slack', label: 'Slack', icon: MessageCircle },
  { id: 'pushover', label: 'Pushover', icon: Bell },
  { id: 'webhook', label: 'Webhook', icon: Webhook },
] as const

type ChannelId = (typeof CHANNELS)[number]['id']

const EVENT_GROUPS: { title: string; events: { id: string; label: string; help?: string }[] }[] = [
  {
    title: 'Deployments',
    events: [
      { id: 'deployment_success', label: 'Deployment Success' },
      { id: 'deployment_failure', label: 'Deployment Failure' },
      {
        id: 'status_change',
        label: 'Container Status Changes',
        help: 'Notify for Stopped and Restarted container events.',
      },
    ],
  },
  {
    title: 'Backups',
    events: [
      { id: 'backup_success', label: 'Backup Success' },
      { id: 'backup_failure', label: 'Backup Failure' },
    ],
  },
  {
    title: 'Scheduled Tasks',
    events: [
      { id: 'scheduled_task_success', label: 'Scheduled Task Success' },
      { id: 'scheduled_task_failure', label: 'Scheduled Task Failure' },
    ],
  },
  {
    title: 'Server',
    events: [
      { id: 'docker_cleanup_success', label: 'Docker Cleanup Success' },
      { id: 'docker_cleanup_failure', label: 'Docker Cleanup Failure' },
      { id: 'server_disk_usage', label: 'Server Disk Usage' },
      { id: 'server_reachable', label: 'Server Reachable' },
      { id: 'server_unreachable', label: 'Server Unreachable' },
      { id: 'server_patch', label: 'Server Patching' },
      { id: 'traefik_outdated', label: 'Traefik Proxy Outdated' },
    ],
  },
]

const DEFAULT_EVENTS = [
  'deployment_failure',
  'backup_failure',
  'scheduled_task_failure',
  'docker_cleanup_failure',
  'server_disk_usage',
  'server_unreachable',
  'server_patch',
  'traefik_outdated',
]

type EmailCfg = {
  use_instance_email_settings: boolean
  smtp_from_name: string
  smtp_from_address: string
  smtp_enabled: boolean
  smtp_host: string
  smtp_port: number
  smtp_encryption: string
  smtp_username: string
  smtp_password: string
  smtp_timeout: number
  resend_enabled: boolean
  resend_api_key: string
  recipients: string
}

type DiscordCfg = { webhook_url: string; ping_enabled: boolean }
type SlackCfg = { webhook_url: string }
type TelegramCfg = { bot_token: string; chat_id: string; thread_ids: Record<string, string> }
type PushoverCfg = { user_key: string; api_token: string }
type WebhookCfg = { url: string; headers: Record<string, string> }

function defaultConfig(ch: ChannelId): Record<string, unknown> {
  switch (ch) {
    case 'email':
      return {
        use_instance_email_settings: true,
        smtp_from_name: '',
        smtp_from_address: '',
        smtp_enabled: false,
        smtp_host: '',
        smtp_port: 587,
        smtp_encryption: 'starttls',
        smtp_username: '',
        smtp_password: '',
        smtp_timeout: 30,
        resend_enabled: false,
        resend_api_key: '',
        recipients: '',
      }
    case 'discord':
      return { webhook_url: '', ping_enabled: false }
    case 'telegram':
      return { bot_token: '', chat_id: '', thread_ids: {} }
    case 'slack':
      return { webhook_url: '' }
    case 'pushover':
      return { user_key: '', api_token: '' }
    case 'webhook':
      return { url: '', headers: {} }
  }
}

function parseConfig(ch: ChannelId, raw: unknown): Record<string, unknown> {
  const base = defaultConfig(ch)
  if (!raw || typeof raw !== 'object') return base
  return { ...base, ...(raw as Record<string, unknown>) }
}

export function NotificationsPage() {
  const qc = useQueryClient()
  const list = useQuery({ queryKey: ['notifications'], queryFn: api.notifications })
  const [tab, setTab] = useState<ChannelId>('email')
  const [toast, setToast] = useState('')
  const [error, setError] = useState('')
  const [testEmailOpen, setTestEmailOpen] = useState(false)
  const [testEmail, setTestEmail] = useState('')

  const byChannel = useMemo(() => {
    const map: Record<string, NotificationSetting> = {}
    for (const n of list.data?.notifications || []) map[n.channel] = n
    return map
  }, [list.data])

  const [enabled, setEnabled] = useState(false)
  const [events, setEvents] = useState<string[]>(DEFAULT_EVENTS)
  const [config, setConfig] = useState<Record<string, unknown>>(defaultConfig('email'))

  useEffect(() => {
    const row = byChannel[tab]
    setEnabled(!!row?.enabled)
    // Saved rows keep their events (including empty). Unsaved channels get defaults.
    setEvents(row?.id ? (row.events ?? []) : DEFAULT_EVENTS)
    setConfig(parseConfig(tab, row?.config))
    setError('')
    setToast('')
  }, [tab, byChannel])

  const savedEnabled = !!byChannel[tab]?.enabled && !!byChannel[tab]?.id

  const save = useMutation({
    mutationFn: () => api.upsertNotification(tab, { enabled, config, events }),
    onSuccess: () => {
      setError('')
      setToast('Settings saved.')
      void qc.invalidateQueries({ queryKey: ['notifications'] })
    },
    onError: (e: Error) => {
      setToast('')
      setError(e.message)
    },
  })

  const test = useMutation({
    mutationFn: (email?: string) => api.testNotification(tab, email ? { email } : undefined),
    onSuccess: () => {
      setError('')
      setToast(tab === 'email' ? 'Test Email sent.' : 'Test notification sent.')
      setTestEmailOpen(false)
    },
    onError: (e: Error) => {
      setToast('')
      setError(e.message)
    },
  })

  const setCfg = (patch: Record<string, unknown>) => setConfig((c) => ({ ...c, ...patch }))

  const toggleEvent = (id: string) => {
    setEvents((prev) => (prev.includes(id) ? prev.filter((e) => e !== id) : [...prev, id]))
  }

  if (list.isLoading) return <FormPageSkeleton />

  return (
    <div className="space-y-6">
      <Header title="Notifications" />

      <ResourceTabs
        tabs={CHANNELS.map((c) => ({ id: c.id, label: c.label, icon: c.icon }))}
        active={tab}
        onChange={(id) => setTab(id as ChannelId)}
      />

      <TabPanel>
        <div className="space-y-6">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
              {CHANNELS.find((c) => c.id === tab)?.label}
            </h2>
            <Btn primary type="button" disabled={save.isPending} onClick={() => save.mutate()}>
              {save.isPending ? 'Saving…' : 'Save'}
            </Btn>
            {tab === 'email' ? (
              <Btn
                type="button"
                disabled={!savedEnabled || test.isPending}
                onClick={() => setTestEmailOpen(true)}
              >
                Send Test Email
              </Btn>
            ) : (
              <Btn
                type="button"
                disabled={!savedEnabled || test.isPending}
                onClick={() => test.mutate(undefined)}
              >
                {test.isPending ? 'Sending…' : 'Send Test Notification'}
              </Btn>
            )}
          </div>

          {error && <p className="text-sm text-error-500">{error}</p>}
          {toast && <p className="text-sm text-success-600 dark:text-success-400">{toast}</p>}

          <label className="flex w-fit items-center gap-2 text-sm">
            <input
              type="checkbox"
              className="accent-[var(--color-accent)]"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
            />
            Enabled
          </label>

          {tab === 'email' && (
            <EmailFields cfg={config as unknown as EmailCfg} setCfg={setCfg} />
          )}
          {tab === 'discord' && (
            <DiscordFields cfg={config as unknown as DiscordCfg} setCfg={setCfg} />
          )}
          {tab === 'telegram' && (
            <TelegramFields cfg={config as unknown as TelegramCfg} setCfg={setCfg} />
          )}
          {tab === 'slack' && <SlackFields cfg={config as unknown as SlackCfg} setCfg={setCfg} />}
          {tab === 'pushover' && (
            <PushoverFields cfg={config as unknown as PushoverCfg} setCfg={setCfg} />
          )}
          {tab === 'webhook' && (
            <WebhookFields cfg={config as unknown as WebhookCfg} setCfg={setCfg} />
          )}

          <div>
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
              Notification Settings
            </h2>
            <p className="mt-1 mb-4 text-sm text-gray-500 dark:text-gray-400">
              Select events for which you would like to receive{' '}
              {CHANNELS.find((c) => c.id === tab)?.label.toLowerCase()} notifications.
            </p>
            <div className={`flex flex-col gap-4 ${tab === 'telegram' ? '' : 'max-w-2xl'}`}>
              {EVENT_GROUPS.map((g) => (
                <div
                  key={g.title}
                  className="rounded-lg border border-gray-200 p-4 dark:border-gray-800"
                >
                  <h3 className="mb-3 font-medium text-gray-900 dark:text-white">{g.title}</h3>
                  <div className="flex flex-col gap-2 pl-1">
                    {g.events.map((ev) => (
                      <div key={ev.id} className="flex flex-wrap items-start gap-3">
                        <label className="flex min-w-[220px] flex-1 items-start gap-2 text-sm">
                          <input
                            type="checkbox"
                            className="mt-0.5 accent-[var(--color-accent)]"
                            checked={events.includes(ev.id)}
                            onChange={() => toggleEvent(ev.id)}
                          />
                          <span>
                            <span className="text-gray-800 dark:text-gray-200">{ev.label}</span>
                            {ev.help && (
                              <span className="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">
                                {ev.help}
                              </span>
                            )}
                          </span>
                        </label>
                        {tab === 'telegram' && (
                          <input
                            type="password"
                            placeholder="Custom Telegram Thread ID"
                            value={
                              ((config.thread_ids as Record<string, string>) || {})[ev.id] || ''
                            }
                            onChange={(e) => {
                              const thread_ids = {
                                ...((config.thread_ids as Record<string, string>) || {}),
                                [ev.id]: e.target.value,
                              }
                              setCfg({ thread_ids })
                            }}
                            className="panel-field w-56 rounded-lg px-2 py-1.5 text-xs"
                          />
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </TabPanel>

      {testEmailOpen && (
        <Modal title="Send Test Email" onClose={() => setTestEmailOpen(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              test.mutate(testEmail)
            }}
          >
            <Input label="Recipient" value={testEmail} onChange={setTestEmail} />
            {test.error && <p className="text-sm text-error-500">{test.error.message}</p>}
            <Btn primary type="submit" disabled={test.isPending || !testEmail.trim()}>
              Send Email
            </Btn>
          </form>
        </Modal>
      )}
    </div>
  )
}

function FieldHelp({ children }: { children: React.ReactNode }) {
  return <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">{children}</p>
}

function EmailFields({
  cfg,
  setCfg,
}: {
  cfg: EmailCfg
  setCfg: (p: Record<string, unknown>) => void
}) {
  return (
    <div className="max-w-2xl space-y-4">
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          className="accent-[var(--color-accent)]"
          checked={!!cfg.use_instance_email_settings}
          onChange={(e) => setCfg({ use_instance_email_settings: e.target.checked })}
        />
        Use system wide (transactional) email settings
      </label>
      <FieldHelp>
        Uses SMTP / Resend from Settings → Email. Configure those first for team notifications.
      </FieldHelp>

      {!cfg.use_instance_email_settings && (
        <>
          <div className="grid gap-3 sm:grid-cols-2">
            <Input
              label="From Name"
              value={cfg.smtp_from_name || ''}
              onChange={(v) => setCfg({ smtp_from_name: v })}
            />
            <Input
              label="From Address"
              value={cfg.smtp_from_address || ''}
              onChange={(v) => setCfg({ smtp_from_address: v })}
            />
          </div>

          <div className="rounded-lg border border-gray-200 p-4 dark:border-gray-800">
            <h3 className="mb-3 font-medium">SMTP Server</h3>
            <label className="mb-3 flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={!!cfg.smtp_enabled}
                onChange={(e) =>
                  setCfg({
                    smtp_enabled: e.target.checked,
                    resend_enabled: e.target.checked ? false : cfg.resend_enabled,
                  })
                }
              />
              Enabled
            </label>
            <div className="grid gap-3 sm:grid-cols-2">
              <Input label="Host" value={cfg.smtp_host || ''} onChange={(v) => setCfg({ smtp_host: v })} />
              <Input
                label="Port"
                value={String(cfg.smtp_port || 587)}
                onChange={(v) => setCfg({ smtp_port: Number(v) || 587 })}
              />
              <label className="block text-sm sm:col-span-2">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">Encryption</span>
                <select
                  value={cfg.smtp_encryption || 'starttls'}
                  onChange={(e) => setCfg({ smtp_encryption: e.target.value })}
                  className="panel-field w-full rounded-lg px-3 py-2"
                >
                  <option value="starttls">STARTTLS</option>
                  <option value="tls">TLS</option>
                  <option value="none">None</option>
                </select>
              </label>
              <Input
                label="SMTP Username"
                value={cfg.smtp_username || ''}
                onChange={(v) => setCfg({ smtp_username: v })}
                required={false}
              />
              <label className="block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">SMTP Password</span>
                <input
                  type="password"
                  value={cfg.smtp_password || ''}
                  onChange={(e) => setCfg({ smtp_password: e.target.value })}
                  className="panel-field w-full rounded-lg px-3 py-2"
                  placeholder="Leave blank to keep"
                />
              </label>
            </div>
          </div>

          <div className="rounded-lg border border-gray-200 p-4 dark:border-gray-800">
            <h3 className="mb-3 font-medium">Resend</h3>
            <label className="mb-3 flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={!!cfg.resend_enabled}
                onChange={(e) =>
                  setCfg({
                    resend_enabled: e.target.checked,
                    smtp_enabled: e.target.checked ? false : cfg.smtp_enabled,
                  })
                }
              />
              Enabled
            </label>
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">API Key</span>
              <input
                type="password"
                value={cfg.resend_api_key || ''}
                onChange={(e) => setCfg({ resend_api_key: e.target.value })}
                className="panel-field w-full rounded-lg px-3 py-2"
                placeholder="Leave blank to keep"
              />
            </label>
          </div>
        </>
      )}

      <Input
        label="Recipients (optional)"
        value={cfg.recipients || ''}
        onChange={(v) => setCfg({ recipients: v })}
        required={false}
      />
      <FieldHelp>Comma-separated emails. Leave empty to notify all team members.</FieldHelp>
    </div>
  )
}

function DiscordFields({
  cfg,
  setCfg,
}: {
  cfg: DiscordCfg
  setCfg: (p: Record<string, unknown>) => void
}) {
  return (
    <div className="max-w-2xl space-y-3">
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={!!cfg.ping_enabled}
          onChange={(e) => setCfg({ ping_enabled: e.target.checked })}
        />
        Ping Enabled
      </label>
      <FieldHelp>
        If enabled, a ping (@here) will be sent when a critical event happens.
      </FieldHelp>
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">Webhook</span>
        <input
          type="password"
          value={cfg.webhook_url || ''}
          onChange={(e) => setCfg({ webhook_url: e.target.value })}
          className="panel-field w-full rounded-lg px-3 py-2"
          placeholder="Leave blank to keep existing webhook"
        />
      </label>
      <FieldHelp>
        Create a Discord Server and generate a Webhook URL.{' '}
        <a
          className="underline"
          href="https://support.discord.com/hc/en-us/articles/228383668-Intro-to-Webhooks"
          target="_blank"
          rel="noreferrer"
        >
          Webhook Documentation
        </a>
      </FieldHelp>
    </div>
  )
}

function TelegramFields({
  cfg,
  setCfg,
}: {
  cfg: TelegramCfg
  setCfg: (p: Record<string, unknown>) => void
}) {
  return (
    <div className="max-w-2xl space-y-3">
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">Bot API Token</span>
        <input
          type="password"
          value={cfg.bot_token || ''}
          onChange={(e) => setCfg({ bot_token: e.target.value })}
          className="panel-field w-full rounded-lg px-3 py-2"
          placeholder="Leave blank to keep"
        />
      </label>
      <FieldHelp>
        Create a bot with{' '}
        <a className="underline" href="https://t.me/BotFather" target="_blank" rel="noreferrer">
          BotFather
        </a>
        .
      </FieldHelp>
      <Input
        label="Chat ID"
        value={cfg.chat_id || ''}
        onChange={(v) => setCfg({ chat_id: v })}
      />
      <FieldHelp>Add your bot to a group chat and add its Chat ID here.</FieldHelp>
    </div>
  )
}

function SlackFields({
  cfg,
  setCfg,
}: {
  cfg: SlackCfg
  setCfg: (p: Record<string, unknown>) => void
}) {
  return (
    <div className="max-w-2xl space-y-3">
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">Webhook</span>
        <input
          type="password"
          value={cfg.webhook_url || ''}
          onChange={(e) => setCfg({ webhook_url: e.target.value })}
          className="panel-field w-full rounded-lg px-3 py-2"
          placeholder="Leave blank to keep existing webhook"
        />
      </label>
      <FieldHelp>
        Create a Slack app with an Incoming Webhook —{' '}
        <a className="underline" href="https://api.slack.com/apps" target="_blank" rel="noreferrer">
          api.slack.com/apps
        </a>
      </FieldHelp>
    </div>
  )
}

function PushoverFields({
  cfg,
  setCfg,
}: {
  cfg: PushoverCfg
  setCfg: (p: Record<string, unknown>) => void
}) {
  return (
    <div className="max-w-2xl space-y-3">
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">User Key</span>
        <input
          type="password"
          value={cfg.user_key || ''}
          onChange={(e) => setCfg({ user_key: e.target.value })}
          className="panel-field w-full rounded-lg px-3 py-2"
          placeholder="Leave blank to keep"
        />
      </label>
      <FieldHelp>
        From your{' '}
        <a className="underline" href="https://pushover.net" target="_blank" rel="noreferrer">
          Pushover dashboard
        </a>
        .
      </FieldHelp>
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">API Token</span>
        <input
          type="password"
          value={cfg.api_token || ''}
          onChange={(e) => setCfg({ api_token: e.target.value })}
          className="panel-field w-full rounded-lg px-3 py-2"
          placeholder="Leave blank to keep"
        />
      </label>
    </div>
  )
}

function WebhookFields({
  cfg,
  setCfg,
}: {
  cfg: WebhookCfg
  setCfg: (p: Record<string, unknown>) => void
}) {
  return (
    <div className="max-w-2xl space-y-3">
      <Input
        label="Webhook URL (POST)"
        value={cfg.url || ''}
        onChange={(v) => setCfg({ url: v })}
      />
      <FieldHelp>
        Enter a valid HTTP or HTTPS URL. Dockfin will send POST requests to this endpoint when
        events occur.
      </FieldHelp>
    </div>
  )
}
