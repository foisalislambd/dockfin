import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bell, Hash, Mail, MessageCircle, Send, Webhook } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useToast } from '../components/Toast'
import { TabbedPageSkeleton } from '../components/ui/Skeleton'
import { ResourceTabs } from '../components/ui/tabs'
import { api, type NotificationSetting } from '../lib/api'
import { Btn, Header, Input, Modal } from './Servers'

const CHANNELS = [
  { id: 'email', label: 'Email', hint: 'Team inbox', icon: Mail },
  { id: 'discord', label: 'Discord', hint: 'Channel webhook', icon: Hash },
  { id: 'telegram', label: 'Telegram', hint: 'Bot + chat', icon: Send },
  { id: 'slack', label: 'Slack', hint: 'Incoming webhook', icon: MessageCircle },
  { id: 'pushover', label: 'Pushover', hint: 'Phone alerts', icon: Bell },
  { id: 'webhook', label: 'Webhook', hint: 'HTTP POST', icon: Webhook },
] as const

type ChannelId = (typeof CHANNELS)[number]['id']

const EVENT_GROUPS: { title: string; events: { id: string; label: string; help?: string }[] }[] = [
  {
    title: 'Deployments',
    events: [
      { id: 'deployment_success', label: 'Deployment success' },
      { id: 'deployment_failure', label: 'Deployment failure' },
      {
        id: 'status_change',
        label: 'Container status changes',
        help: 'Stopped and restarted containers.',
      },
    ],
  },
  {
    title: 'Backups',
    events: [
      { id: 'backup_success', label: 'Backup success' },
      { id: 'backup_failure', label: 'Backup failure' },
    ],
  },
  {
    title: 'Scheduled tasks',
    events: [
      { id: 'scheduled_task_success', label: 'Task success' },
      { id: 'scheduled_task_failure', label: 'Task failure' },
    ],
  },
  {
    title: 'Server',
    events: [
      { id: 'docker_cleanup_success', label: 'Docker cleanup success' },
      { id: 'docker_cleanup_failure', label: 'Docker cleanup failure' },
      { id: 'server_disk_usage', label: 'Disk usage' },
      { id: 'server_reachable', label: 'Server reachable' },
      { id: 'server_unreachable', label: 'Server unreachable' },
      { id: 'server_patch', label: 'Server patching' },
      { id: 'traefik_outdated', label: 'Traefik outdated' },
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

const fieldClass =
  'panel-field w-full rounded-lg px-3 py-2 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20'

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

function Switch({
  checked,
  onChange,
  label,
}: {
  checked: boolean
  onChange: (v: boolean) => void
  label?: string
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      onClick={() => onChange(!checked)}
      className={`relative h-6 w-11 shrink-0 rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500/40 ${
        checked ? 'bg-brand-500' : 'bg-gray-200 dark:bg-gray-700'
      }`}
    >
      <span
        className={`absolute top-0.5 left-0.5 h-5 w-5 rounded-full bg-white shadow-sm transition-transform ${
          checked ? 'translate-x-5' : 'translate-x-0'
        }`}
      />
    </button>
  )
}

function SwitchRow({
  label,
  help,
  checked,
  onChange,
}: {
  label: string
  help?: string
  checked: boolean
  onChange: (v: boolean) => void
}) {
  return (
    <div className="flex items-start justify-between gap-4 py-2.5">
      <div className="min-w-0">
        <p className="text-sm text-gray-800 dark:text-gray-200">{label}</p>
        {help ? <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{help}</p> : null}
      </div>
      <Switch checked={checked} onChange={onChange} label={label} />
    </div>
  )
}

function ExtLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <a
      className="text-brand-600 hover:underline dark:text-brand-400"
      href={href}
      target="_blank"
      rel="noreferrer"
    >
      {children}
    </a>
  )
}

export function NotificationsPage() {
  const qc = useQueryClient()
  const toast = useToast()
  const list = useQuery({ queryKey: ['notifications'], queryFn: api.notifications })
  const [tab, setTab] = useState<ChannelId>('email')
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
    setEvents(row?.id ? (row.events ?? []) : DEFAULT_EVENTS)
    setConfig(parseConfig(tab, row?.config))
    setError('')
  }, [tab, byChannel])

  const savedEnabled = !!byChannel[tab]?.enabled && !!byChannel[tab]?.id
  const channel = CHANNELS.find((c) => c.id === tab)!
  const ChannelIcon = channel.icon

  const save = useMutation({
    mutationFn: () => api.upsertNotification(tab, { enabled, config, events }),
    onSuccess: () => {
      setError('')
      toast.success('Settings saved.')
      void qc.invalidateQueries({ queryKey: ['notifications'] })
    },
    onError: (e: Error) => {
      setError(e.message)
    },
  })

  const test = useMutation({
    mutationFn: (email?: string) => api.testNotification(tab, email ? { email } : undefined),
    onSuccess: () => {
      setError('')
      toast.success(tab === 'email' ? 'Test email sent.' : 'Test notification sent.')
      setTestEmailOpen(false)
    },
    onError: (e: Error) => {
      setError(e.message)
    },
  })

  const setCfg = (patch: Record<string, unknown>) => setConfig((c) => ({ ...c, ...patch }))

  const toggleEvent = (id: string) => {
    setEvents((prev) => (prev.includes(id) ? prev.filter((e) => e !== id) : [...prev, id]))
  }

  if (list.isLoading) return <TabbedPageSkeleton />

  return (
    <div className="space-y-6">
      <Header title="Notifications" />

      <ResourceTabs
        tabs={CHANNELS.map((c) => ({ id: c.id, label: c.label, icon: c.icon }))}
        active={tab}
        onChange={(id) => setTab(id as ChannelId)}
      />

      <div className="space-y-4">
        <div className="panel-card flex flex-wrap items-center gap-3 px-5 py-4">
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-brand-50 text-brand-600 dark:bg-brand-500/15 dark:text-brand-400">
                <ChannelIcon className="h-5 w-5" strokeWidth={1.75} />
              </div>
              <div className="min-w-0">
                <div className="text-base font-semibold text-gray-900 dark:text-white">
                  {channel.label}
                </div>
                <div className="text-sm text-gray-500 dark:text-gray-400">{channel.hint}</div>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <div className="flex items-center gap-2">
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  {enabled ? 'On' : 'Off'}
                </span>
                <Switch checked={enabled} onChange={setEnabled} label="Enable channel" />
              </div>
              <Btn
                type="button"
                disabled={!savedEnabled || test.isPending}
                onClick={() => (tab === 'email' ? setTestEmailOpen(true) : test.mutate(undefined))}
              >
                {test.isPending ? 'Sending…' : 'Send test'}
              </Btn>
              <Btn primary type="button" disabled={save.isPending} onClick={() => save.mutate()}>
                {save.isPending ? 'Saving…' : 'Save'}
              </Btn>
            </div>
          </div>

          {error ? <p className="text-sm text-error-500">{error}</p> : null}

          <div className="panel-card space-y-4 p-5">
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Connection</h2>
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
          </div>

          <div className="panel-card p-5">
            <div className="mb-3 flex items-baseline justify-between gap-3">
              <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Events</h2>
              <span className="text-xs text-gray-400 dark:text-gray-500">
                {events.length} selected
              </span>
            </div>
            <div className={tab === 'telegram' ? 'space-y-6' : 'grid gap-6 xl:grid-cols-2'}>
              {EVENT_GROUPS.map((g) => (
                <div key={g.title}>
                  <h3 className="mb-1 text-xs font-semibold tracking-wider text-gray-400 uppercase dark:text-gray-500">
                    {g.title}
                  </h3>
                  <div className="divide-y divide-gray-100 dark:divide-gray-800">
                    {g.events.map((ev) => (
                      <div key={ev.id}>
                        <SwitchRow
                          label={ev.label}
                          help={ev.help}
                          checked={events.includes(ev.id)}
                          onChange={() => toggleEvent(ev.id)}
                        />
                        {tab === 'telegram' ? (
                          <input
                            type="password"
                            placeholder="Custom thread ID (optional)"
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
                            className={`${fieldClass} mb-2 text-xs`}
                          />
                        ) : null}
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

      {testEmailOpen && (
        <Modal title="Send test email" onClose={() => setTestEmailOpen(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              test.mutate(testEmail)
            }}
          >
            <Input label="Recipient" value={testEmail} onChange={setTestEmail} />
            {test.error ? <p className="text-sm text-error-500">{test.error.message}</p> : null}
            <Btn primary type="submit" disabled={test.isPending || !testEmail.trim()}>
              Send
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
    <div className="space-y-4">
      <SwitchRow
        label="Use instance email settings"
        help="SMTP / Resend from Settings → Email."
        checked={!!cfg.use_instance_email_settings}
        onChange={(v) => setCfg({ use_instance_email_settings: v })}
      />

      {!cfg.use_instance_email_settings && (
        <>
          <div className="grid gap-3 sm:grid-cols-2">
            <Input
              label="From name"
              value={cfg.smtp_from_name || ''}
              onChange={(v) => setCfg({ smtp_from_name: v })}
            />
            <Input
              label="From address"
              value={cfg.smtp_from_address || ''}
              onChange={(v) => setCfg({ smtp_from_address: v })}
            />
          </div>

          <div className="rounded-lg border border-gray-100 p-4 dark:border-gray-800">
            <SwitchRow
              label="SMTP server"
              checked={!!cfg.smtp_enabled}
              onChange={(v) =>
                setCfg({
                  smtp_enabled: v,
                  resend_enabled: v ? false : cfg.resend_enabled,
                })
              }
            />
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
                  className={fieldClass}
                >
                  <option value="starttls">STARTTLS</option>
                  <option value="tls">TLS</option>
                  <option value="none">None</option>
                </select>
              </label>
              <Input
                label="SMTP username"
                value={cfg.smtp_username || ''}
                onChange={(v) => setCfg({ smtp_username: v })}
                required={false}
              />
              <label className="block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">SMTP password</span>
                <input
                  type="password"
                  value={cfg.smtp_password || ''}
                  onChange={(e) => setCfg({ smtp_password: e.target.value })}
                  className={fieldClass}
                  placeholder="Leave blank to keep"
                />
              </label>
            </div>
          </div>

          <div className="rounded-lg border border-gray-100 p-4 dark:border-gray-800">
            <SwitchRow
              label="Resend"
              checked={!!cfg.resend_enabled}
              onChange={(v) =>
                setCfg({
                  resend_enabled: v,
                  smtp_enabled: v ? false : cfg.smtp_enabled,
                })
              }
            />
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">API key</span>
              <input
                type="password"
                value={cfg.resend_api_key || ''}
                onChange={(e) => setCfg({ resend_api_key: e.target.value })}
                className={fieldClass}
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
      <FieldHelp>Comma-separated. Empty means all team members.</FieldHelp>
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
    <div className="space-y-3">
      <SwitchRow
        label="Ping @here on critical events"
        checked={!!cfg.ping_enabled}
        onChange={(v) => setCfg({ ping_enabled: v })}
      />
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">Webhook URL</span>
        <input
          type="password"
          value={cfg.webhook_url || ''}
          onChange={(e) => setCfg({ webhook_url: e.target.value })}
          className={fieldClass}
          placeholder="Leave blank to keep existing webhook"
        />
      </label>
      <FieldHelp>
        Generate a webhook in Discord.{' '}
        <ExtLink href="https://support.discord.com/hc/en-us/articles/228383668-Intro-to-Webhooks">
          Docs
        </ExtLink>
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
    <div className="space-y-3">
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">Bot API token</span>
        <input
          type="password"
          value={cfg.bot_token || ''}
          onChange={(e) => setCfg({ bot_token: e.target.value })}
          className={fieldClass}
          placeholder="Leave blank to keep"
        />
      </label>
      <FieldHelp>
        Create a bot with <ExtLink href="https://t.me/BotFather">BotFather</ExtLink>.
      </FieldHelp>
      <Input label="Chat ID" value={cfg.chat_id || ''} onChange={(v) => setCfg({ chat_id: v })} />
      <FieldHelp>Add the bot to a group and paste the chat ID.</FieldHelp>
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
    <div className="space-y-3">
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">Webhook URL</span>
        <input
          type="password"
          value={cfg.webhook_url || ''}
          onChange={(e) => setCfg({ webhook_url: e.target.value })}
          className={fieldClass}
          placeholder="Leave blank to keep existing webhook"
        />
      </label>
      <FieldHelp>
        Incoming Webhook app — <ExtLink href="https://api.slack.com/apps">api.slack.com/apps</ExtLink>
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
    <div className="space-y-3">
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">User key</span>
        <input
          type="password"
          value={cfg.user_key || ''}
          onChange={(e) => setCfg({ user_key: e.target.value })}
          className={fieldClass}
          placeholder="Leave blank to keep"
        />
      </label>
      <FieldHelp>
        From your <ExtLink href="https://pushover.net">Pushover dashboard</ExtLink>.
      </FieldHelp>
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">API token</span>
        <input
          type="password"
          value={cfg.api_token || ''}
          onChange={(e) => setCfg({ api_token: e.target.value })}
          className={fieldClass}
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
    <div className="space-y-3">
      <Input
        label="Webhook URL (POST)"
        value={cfg.url || ''}
        onChange={(v) => setCfg({ url: v })}
      />
      <FieldHelp>Dockfin sends a POST to this URL when a selected event fires.</FieldHelp>
    </div>
  )
}
