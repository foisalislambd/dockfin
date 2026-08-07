import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  Archive,
  CalendarClock,
  Eye,
  EyeOff,
  KeyRound,
  Mail,
  RefreshCw,
  Settings2,
  SlidersHorizontal,
  User,
} from 'lucide-react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { DnsGuideTooltip, DomainDNSAlert, normalizeDomainEntry } from '../components/DomainsPanel'
import { InfoHint } from '../components/ui/forms'
import { MIT_LICENSE_TEXT } from '../config/app.config'
import {
  api,
  type InstanceSettings,
  type InstanceSettingsPatch,
  type OauthSetting,
} from '../lib/api'
import { useAuth } from '../lib/auth'
import { Btn, Header } from './Servers'

type TopTab = 'configuration' | 'backup' | 'email' | 'oauth' | 'scheduled' | 'profile'
type ConfigSub = 'general' | 'advanced' | 'updates'

const TIMEZONES =
  typeof Intl !== 'undefined' && 'supportedValuesOf' in Intl
    ? (Intl as unknown as { supportedValuesOf: (k: string) => string[] }).supportedValuesOf('timeZone')
    : ['UTC', 'America/New_York', 'Europe/London', 'Asia/Dhaka', 'Asia/Tokyo']

function Helper({ text }: { text: string }) {
  return <InfoHint text={text} />
}

function FieldLabel({ label, helper }: { label: string; helper?: string }) {
  return (
    <div className="mb-1 flex items-center gap-1.5">
      <span className="text-sm text-gray-500 dark:text-gray-400">{label}</span>
      {helper ? <Helper text={helper} /> : null}
    </div>
  )
}

function TextField({
  label,
  helper,
  value,
  onChange,
  placeholder,
  type = 'text',
  required,
  disabled,
}: {
  label: string
  helper?: string
  value: string
  onChange: (v: string) => void
  placeholder?: string
  type?: string
  required?: boolean
  disabled?: boolean
}) {
  return (
    <label className="block w-full text-sm">
      <FieldLabel label={label} helper={helper} />
      <input
        type={type}
        required={required}
        disabled={disabled}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="panel-field w-full rounded-lg px-3 py-2 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20 disabled:opacity-50"
      />
    </label>
  )
}

function SecretField({
  label,
  helper,
  value,
  onChange,
  placeholder,
}: {
  label: string
  helper?: string
  value: string
  onChange: (v: string) => void
  placeholder?: string
}) {
  const [show, setShow] = useState(false)
  return (
    <label className="block w-full text-sm">
      <FieldLabel label={label} helper={helper} />
      <div className="relative">
        <input
          type={show ? 'text' : 'password'}
          autoComplete="new-password"
          value={value}
          placeholder={placeholder}
          onChange={(e) => onChange(e.target.value)}
          className="panel-field w-full rounded-lg px-3 py-2 pr-10 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
        />
        <button
          type="button"
          onClick={() => setShow((s) => !s)}
          className="absolute top-1/2 right-2 -translate-y-1/2 rounded p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
          aria-label={show ? 'Hide' : 'Show'}
        >
          {show ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
        </button>
      </div>
    </label>
  )
}

function Toggle({
  label,
  helper,
  checked,
  onChange,
  disabled,
}: {
  label: string
  helper?: string
  checked: boolean
  onChange: (v: boolean) => void
  disabled?: boolean
}) {
  return (
    <label className="flex max-w-md items-center justify-between gap-3 py-1.5 text-sm">
      <span className="flex items-center gap-1.5 text-gray-700 dark:text-gray-200">
        {label}
        {helper ? <Helper text={helper} /> : null}
      </span>
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        className="h-4 w-4 accent-[var(--color-accent)]"
      />
    </label>
  )
}

function SectionHead({
  title,
  actions,
}: {
  title: string
  actions?: ReactNode
}) {
  return (
    <div className="mb-4">
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="text-lg font-medium text-gray-900 dark:text-white">{title}</h2>
        {actions}
      </div>
    </div>
  )
}

function emptyForm(): InstanceSettings {
  return {
    id: 1,
    public_url: '',
    instance_name: 'Dockfin',
    instance_timezone: 'UTC',
    public_ipv4: '',
    public_ipv6: '',
    is_registration_enabled: true,
    do_not_track: false,
    is_dns_validation_enabled: true,
    custom_dns_servers: '1.1.1.1',
    is_api_enabled: true,
    allowed_ips: '',
    webhook_allowed_internal_hosts: '',
    webhook_allow_localhost: false,
    is_mcp_server_enabled: false,
    disable_two_step_confirmation: false,
    is_sponsorship_popup_enabled: true,
    update_channel: 'stable',
    is_auto_update_enabled: true,
    auto_update_frequency: '0 0 * * *',
    update_check_frequency: '0 * * * *',
    docker_registry_url: 'ghcr.io',
    smtp_enabled: false,
    smtp_from_name: '',
    smtp_from_address: '',
    smtp_host: '',
    smtp_port: 587,
    smtp_encryption: 'starttls',
    smtp_username: '',
    smtp_password_set: false,
    smtp_timeout: null,
    resend_enabled: false,
    resend_api_key_set: false,
    updated_at: '',
  }
}

export function SettingsPage() {
  const { user, team } = useAuth()
  const qc = useQueryClient()
  const canEdit = team?.role === 'owner' || team?.role === 'admin'
  const [topTab, setTopTab] = useState<TopTab>('configuration')
  const [sub, setSub] = useState<ConfigSub>('general')
  const [form, setForm] = useState<InstanceSettings>(emptyForm)
  const [smtpPassword, setSmtpPassword] = useState('')
  const [resendKey, setResendKey] = useState('')
  const [tzSearch, setTzSearch] = useState('')
  const [tzOpen, setTzOpen] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  const settings = useQuery({ queryKey: ['instance-settings'], queryFn: api.instanceSettings })
  const version = useQuery({ queryKey: ['version'], queryFn: api.version })
  const oauth = useQuery({
    queryKey: ['oauth-settings'],
    queryFn: api.oauthSettings,
    enabled: topTab === 'oauth',
  })
  const tasks = useQuery({
    queryKey: ['scheduled-tasks'],
    queryFn: () => api.scheduledTasks(),
    enabled: topTab === 'scheduled',
  })
  const backups = useQuery({
    queryKey: ['instance-backup'],
    queryFn: api.instanceBackup,
    enabled: topTab === 'backup',
    refetchInterval: topTab === 'backup' ? 5000 : false,
  })
  const storages = useQuery({
    queryKey: ['s3-storages'],
    queryFn: api.s3Storages,
    enabled: topTab === 'backup',
  })
  const [backupFreq, setBackupFreq] = useState('0 0 * * *')
  const [backupRetention, setBackupRetention] = useState('7')
  const [backupDesc, setBackupDesc] = useState('Dockfin database')

  useEffect(() => {
    if (backups.data?.backup) {
      setBackupFreq(backups.data.backup.frequency || '0 0 * * *')
      setBackupRetention(String(backups.data.backup.retention ?? 7))
      setBackupDesc(backups.data.backup.description || 'Dockfin database')
    }
  }, [backups.data])

  useEffect(() => {
    if (settings.data?.settings) {
      setForm(settings.data.settings)
      setTzSearch(settings.data.settings.instance_timezone || 'UTC')
    }
  }, [settings.data])

  const filteredTz = useMemo(() => {
    const q = tzSearch.toLowerCase()
    return TIMEZONES.filter((tz) => tz.toLowerCase().includes(q)).slice(0, 80)
  }, [tzSearch])

  const save = useMutation({
    mutationFn: (body: InstanceSettingsPatch) => api.patchInstanceSettings(body),
    onSuccess: (data) => {
      setForm(data.settings)
      setSmtpPassword('')
      setResendKey('')
      setError('')
      if (data.panel_route_warning) {
        setMessage(`Settings saved — panel domain route: ${data.panel_route_warning}`)
      } else {
        setMessage('Settings saved')
      }
      void qc.invalidateQueries({ queryKey: ['instance-settings'] })
    },
    onError: (e: Error) => {
      setMessage('')
      setError(e.message)
    },
  })

  const configureBackup = useMutation({
    mutationFn: () => api.configureInstanceBackup(),
    onSuccess: () => {
      setError('')
      setMessage('Instance backup configured')
      void qc.invalidateQueries({ queryKey: ['instance-backup'] })
    },
    onError: (e: Error) => {
      setMessage('')
      setError(e.message)
    },
  })

  const saveBackup = useMutation({
    mutationFn: (body: Parameters<typeof api.patchInstanceBackup>[0]) => api.patchInstanceBackup(body),
    onSuccess: () => {
      setError('')
      setMessage('Backup settings saved')
      void qc.invalidateQueries({ queryKey: ['instance-backup'] })
    },
    onError: (e: Error) => {
      setMessage('')
      setError(e.message)
    },
  })

  const runBackup = useMutation({
    mutationFn: () => api.runInstanceBackup(),
    onSuccess: () => {
      setError('')
      setMessage('Backup started')
      void qc.invalidateQueries({ queryKey: ['instance-backup'] })
    },
    onError: (e: Error) => {
      setMessage('')
      setError(e.message)
    },
  })

  const patchOauth = useMutation({
    mutationFn: ({ provider, body }: { provider: string; body: Partial<OauthSetting> & { client_secret?: string } }) =>
      api.patchOauthSetting(provider, body),
    onSuccess: () => {
      setError('')
      setMessage('OAuth provider saved')
      void qc.invalidateQueries({ queryKey: ['oauth-settings'] })
    },
    onError: (e: Error) => {
      setMessage('')
      setError(e.message)
    },
  })

  const set = <K extends keyof InstanceSettings>(key: K, value: InstanceSettings[K]) => {
    setForm((f) => ({ ...f, [key]: value }))
  }

  const saveGeneral = () => {
    if (!canEdit) return
    const public_url = form.public_url.trim()
      ? normalizeDomainEntry(form.public_url.trim())
      : ''
    if (public_url !== form.public_url) set('public_url', public_url)
    save.mutate({
      public_url,
      instance_name: form.instance_name,
      instance_timezone: form.instance_timezone,
      public_ipv4: form.public_ipv4,
      public_ipv6: form.public_ipv6,
    })
  }

  const saveAdvanced = () => {
    if (!canEdit) return
    save.mutate({
      is_registration_enabled: form.is_registration_enabled,
      do_not_track: form.do_not_track,
      is_dns_validation_enabled: form.is_dns_validation_enabled,
      custom_dns_servers: form.custom_dns_servers,
      is_api_enabled: form.is_api_enabled,
      allowed_ips: form.allowed_ips,
      webhook_allowed_internal_hosts: form.webhook_allowed_internal_hosts,
      webhook_allow_localhost: form.webhook_allow_localhost,
      is_mcp_server_enabled: form.is_mcp_server_enabled,
      disable_two_step_confirmation: form.disable_two_step_confirmation,
      is_sponsorship_popup_enabled: form.is_sponsorship_popup_enabled,
    })
  }

  const saveUpdates = () => {
    if (!canEdit) return
    save.mutate({
      update_check_frequency: form.update_check_frequency,
      is_auto_update_enabled: form.is_auto_update_enabled,
      auto_update_frequency: form.auto_update_frequency,
      docker_registry_url: form.docker_registry_url,
      update_channel: form.update_channel,
    })
  }

  const saveEmail = () => {
    if (!canEdit) return
    const body: InstanceSettingsPatch = {
      smtp_enabled: form.smtp_enabled,
      smtp_from_name: form.smtp_from_name,
      smtp_from_address: form.smtp_from_address,
      smtp_host: form.smtp_host,
      smtp_port: form.smtp_port,
      smtp_encryption: form.smtp_encryption,
      smtp_username: form.smtp_username || '',
      resend_enabled: form.resend_enabled,
    }
    if (smtpPassword) body.smtp_password = smtpPassword
    if (resendKey) body.resend_api_key = resendKey
    if (form.smtp_timeout != null) body.smtp_timeout = form.smtp_timeout
    save.mutate(body)
  }

  const topTabs = [
    { id: 'configuration' as const, label: 'Configuration', icon: Settings2 },
    { id: 'backup' as const, label: 'Backup', icon: Archive },
    { id: 'email' as const, label: 'Transactional Email', icon: Mail },
    { id: 'oauth' as const, label: 'OAuth', icon: KeyRound },
    { id: 'scheduled' as const, label: 'Scheduled Jobs', icon: CalendarClock },
    { id: 'profile' as const, label: 'Profile & License', icon: User },
  ]

  const sideItems = [
    { id: 'general' as const, label: 'General', icon: Settings2 },
    { id: 'advanced' as const, label: 'Advanced', icon: SlidersHorizontal },
    { id: 'updates' as const, label: 'Updates', icon: RefreshCw },
  ]

  return (
    <div className="space-y-6">
      <Header title="Settings" />

      <nav className="flex flex-wrap gap-5 border-b border-gray-200 dark:border-gray-800">
        {topTabs.map((t) => {
          const Icon = t.icon
          const active = topTab === t.id
          return (
            <button
              key={t.id}
              type="button"
              onClick={() => setTopTab(t.id)}
              className={`-mb-px inline-flex items-center gap-1.5 border-b-2 pb-2 text-sm transition ${
                active
                  ? 'border-gray-900 text-gray-900 dark:border-white dark:text-white'
                  : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
              }`}
            >
              <Icon
                className={`h-3.5 w-3.5 shrink-0 ${active ? 'opacity-100' : 'opacity-70'}`}
                aria-hidden
              />
              {t.label}
            </button>
          )
        })}
      </nav>

      {error && <p className="text-sm text-error-500">{error}</p>}
      {message && <p className="text-sm text-brand-600 dark:text-brand-400">{message}</p>}
      {!canEdit && (
        <p className="text-sm text-amber-600 dark:text-amber-400">
          View only — switch to a team where you are owner or admin to edit instance settings.
        </p>
      )}

      {topTab === 'configuration' && (
        <div className="flex flex-col gap-8 sm:flex-row">
          <aside className="w-full shrink-0 sm:w-44">
            <nav className="flex flex-row gap-2 sm:flex-col">
              {sideItems.map((item) => {
                const Icon = item.icon
                const active = sub === item.id
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setSub(item.id)}
                    className={`inline-flex items-center gap-2 rounded-md px-3 py-2 text-left text-sm transition ${
                      active
                        ? 'bg-brand-500 text-white'
                        : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-white/5'
                    }`}
                  >
                    <Icon
                      className={`h-3.5 w-3.5 shrink-0 ${active ? 'text-white' : 'opacity-70'}`}
                      aria-hidden
                    />
                    {item.label}
                  </button>
                )
              })}
            </nav>
          </aside>

          <div className="min-w-0 flex-1">
            {settings.isLoading && <p className="text-sm text-gray-500">Loading settings…</p>}

            {sub === 'general' && (
              <form
                className="space-y-4"
                onSubmit={(e) => {
                  e.preventDefault()
                  saveGeneral()
                }}
              >
                <SectionHead
                  title="General"
                  actions={
                    <Btn primary type="submit" disabled={!canEdit || save.isPending}>
                      Save
                    </Btn>
                  }
                />

                <div className="space-y-3">
                  <div className="flex items-center gap-1.5">
                    <h4 className="text-sm font-medium text-gray-900 dark:text-white">Domain</h4>
                    <DnsGuideTooltip domains={form.public_url} serverIp={(form.public_ipv4 || '').trim()} />
                  </div>
                  <input
                    type="text"
                    value={form.public_url}
                    placeholder="dash.example.com"
                    onChange={(e) => set('public_url', e.target.value)}
                    onBlur={() => {
                      const next = form.public_url.trim()
                        ? normalizeDomainEntry(form.public_url.trim())
                        : ''
                      if (next !== form.public_url) set('public_url', next)
                    }}
                    className="panel-field w-full rounded-lg px-3 py-2 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20 disabled:opacity-50"
                  />
                  <DomainDNSAlert domains={form.public_url} serverIp={(form.public_ipv4 || '').trim()} />
                </div>

                <div className="grid gap-3 md:grid-cols-2">
                  <TextField
                    label="Name"
                    helper="Instance display name."
                    value={form.instance_name}
                    onChange={(v) => set('instance_name', v)}
                    placeholder="Dockfin"
                    required={false}
                  />
                  <div className="relative w-full text-sm">
                    <FieldLabel label="Timezone" helper="Used for schedules and update checks." />
                    <input
                      value={tzSearch}
                      onFocus={() => setTzOpen(true)}
                      onChange={(e) => {
                        setTzSearch(e.target.value)
                        setTzOpen(true)
                      }}
                      onBlur={() => setTimeout(() => setTzOpen(false), 150)}
                      placeholder="Search timezone…"
                      className="panel-field w-full rounded-lg px-3 py-2 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
                    />
                    {tzOpen && (
                      <div className="absolute z-30 mt-1 max-h-60 w-full overflow-auto rounded-md border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-900">
                        {filteredTz.map((tz) => (
                          <button
                            key={tz}
                            type="button"
                            className="block w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-white/10"
                            onMouseDown={(e) => e.preventDefault()}
                            onClick={() => {
                              set('instance_timezone', tz)
                              setTzSearch(tz)
                              setTzOpen(false)
                            }}
                          >
                            {tz}
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
                <div className="grid gap-3 md:grid-cols-2">
                  <SecretField
                    label="Public IPv4"
                    helper="Used for DNS checks and magic domains."
                    value={form.public_ipv4}
                    onChange={(v) => set('public_ipv4', v)}
                    placeholder="1.2.3.4"
                  />
                  <SecretField
                    label="Public IPv6"
                    helper="Optional IPv6 override."
                    value={form.public_ipv6}
                    onChange={(v) => set('public_ipv6', v)}
                    placeholder="2001:db8::1"
                  />
                </div>
              </form>
            )}

            {sub === 'advanced' && (
              <form
                className="space-y-2"
                onSubmit={(e) => {
                  e.preventDefault()
                  saveAdvanced()
                }}
              >
                <SectionHead
                  title="Advanced"
                  actions={
                    <Btn primary type="submit" disabled={!canEdit || save.isPending}>
                      Save
                    </Btn>
                  }
                />
                <Toggle
                  label="Registration Allowed"
                  helper="Allow users to self-register. Turned off automatically after the first admin account; re-enable here to invite more people."
                  checked={form.is_registration_enabled}
                  onChange={(v) => set('is_registration_enabled', v)}
                  disabled={!canEdit}
                />
                <Toggle
                  label="Do Not Track"
                  helper="Opt out of anonymous usage tracking and error reports."
                  checked={form.do_not_track}
                  onChange={(v) => set('do_not_track', v)}
                  disabled={!canEdit}
                />
                <h3 className="pt-4 text-sm font-semibold text-gray-900 dark:text-white">DNS Settings</h3>
                <Toggle
                  label="DNS Validation"
                  helper="Verify custom domains in DNS before deployment."
                  checked={form.is_dns_validation_enabled}
                  onChange={(v) => set('is_dns_validation_enabled', v)}
                  disabled={!canEdit}
                />
                <TextField
                  label="Custom DNS Servers"
                  helper="Comma-separated DNS servers for domain validation (e.g. 1.1.1.1,8.8.8.8)."
                  value={form.custom_dns_servers}
                  onChange={(v) => set('custom_dns_servers', v)}
                  placeholder="1.1.1.1,8.8.8.8"
                  required={false}
                />
                <h3 className="pt-4 text-sm font-semibold text-gray-900 dark:text-white">API Settings</h3>
                <Toggle
                  label="API Access"
                  helper="Allow authenticated REST API requests. Configure tokens under Security → API Tokens."
                  checked={form.is_api_enabled}
                  onChange={(v) => set('is_api_enabled', v)}
                  disabled={!canEdit}
                />
                <TextField
                  label="Allowed IPs for API Access"
                  helper="Comma-separated IPs or CIDRs. Empty or 0.0.0.0 allows anywhere."
                  value={form.allowed_ips}
                  onChange={(v) => set('allowed_ips', v)}
                  placeholder="192.168.1.100,10.0.0.0/8"
                  required={false}
                />
                {(!form.allowed_ips || form.allowed_ips.split(',').some((p) => p.trim() === '0.0.0.0')) && (
                  <p className="rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300">
                    Empty / 0.0.0.0 allows API access from anywhere — not recommended for production.
                  </p>
                )}
                <h3 className="pt-4 text-sm font-semibold text-gray-900 dark:text-white">
                  Webhook / S3 Endpoint Controls
                </h3>
                <label className="block max-w-xl text-sm">
                  <FieldLabel
                    label="Allowed Internal Webhook/S3 Targets"
                    helper="Optional allowlist for webhook and S3 destinations (hostnames, IPs, CIDRs)."
                  />
                  <textarea
                    rows={3}
                    value={form.webhook_allowed_internal_hosts}
                    onChange={(e) => set('webhook_allowed_internal_hosts', e.target.value)}
                    placeholder="hooks.company.local, 10.50.0.0/16"
                    className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
                  />
                </label>
                <Toggle
                  label="Allow Localhost Webhook/S3 Targets"
                  helper="Allow localhost only when also listed above."
                  checked={form.webhook_allow_localhost}
                  onChange={(v) => set('webhook_allow_localhost', v)}
                  disabled={!canEdit}
                />
                <h3 className="pt-4 text-sm font-semibold text-gray-900 dark:text-white">MCP Server</h3>
                <Toggle
                  label="Enable MCP Server Instance-wide"
                  helper="Expose a Model Context Protocol endpoint for AI clients. Requires API Access."
                  checked={form.is_mcp_server_enabled}
                  onChange={(v) => set('is_mcp_server_enabled', v)}
                  disabled={!canEdit}
                />
                <h3 className="pt-4 text-sm font-semibold text-gray-900 dark:text-white">Confirmation</h3>
                <Toggle
                  label="Show Sponsorship Popup"
                  helper="Show occasional sponsorship reminders."
                  checked={form.is_sponsorship_popup_enabled}
                  onChange={(v) => set('is_sponsorship_popup_enabled', v)}
                  disabled={!canEdit}
                />
                <Toggle
                  label="Disable Two Step Confirmation"
                  helper="Skip text/password confirmation on destructive actions. Reduces safety."
                  checked={form.disable_two_step_confirmation}
                  onChange={(v) => set('disable_two_step_confirmation', v)}
                  disabled={!canEdit}
                />
              </form>
            )}

            {sub === 'updates' && (
              <form
                className="space-y-3"
                onSubmit={(e) => {
                  e.preventDefault()
                  saveUpdates()
                }}
              >
                <SectionHead
                  title="Updates"
                  actions={
                    <Btn primary type="submit" disabled={!canEdit || save.isPending}>
                      Save
                    </Btn>
                  }
                />
                <TextField
                  label="Update Check Frequency"
                  helper="Cron expression to check for new versions (default: every hour)."
                  value={form.update_check_frequency}
                  onChange={(v) => set('update_check_frequency', v)}
                  placeholder="0 * * * *"
                />
                <h3 className="pt-2 text-sm font-semibold text-gray-900 dark:text-white">Auto Update</h3>
                <Toggle
                  label="Enabled"
                  checked={form.is_auto_update_enabled}
                  onChange={(v) => set('is_auto_update_enabled', v)}
                  disabled={!canEdit}
                />
                <TextField
                  label="Frequency (cron expression)"
                  helper="When Dockfin auto-updates (default: daily at 00:00)."
                  value={form.auto_update_frequency}
                  onChange={(v) => set('auto_update_frequency', v)}
                  placeholder="0 0 * * *"
                  disabled={!form.is_auto_update_enabled}
                />
                <h3 className="pt-2 text-sm font-semibold text-gray-900 dark:text-white">Docker Registry</h3>
                <label className="block max-w-md text-sm">
                  <FieldLabel
                    label="Docker Registry"
                    helper="Registry used to pull Dockfin images during updates."
                  />
                  <select
                    value={form.docker_registry_url}
                    onChange={(e) => set('docker_registry_url', e.target.value)}
                    className="panel-field w-full rounded-lg px-3 py-2"
                    disabled={!canEdit}
                  >
                    <option value="ghcr.io">GitHub Container Registry (ghcr.io)</option>
                    <option value="docker.io">Docker Hub (docker.io)</option>
                  </select>
                </label>
                <label className="block max-w-md text-sm">
                  <FieldLabel label="Update Channel" helper="Release channel for updates." />
                  <select
                    value={form.update_channel}
                    onChange={(e) => set('update_channel', e.target.value)}
                    className="panel-field w-full rounded-lg px-3 py-2"
                    disabled={!canEdit}
                  >
                    <option value="stable">stable</option>
                    <option value="next">next</option>
                    <option value="nightly">nightly</option>
                  </select>
                </label>
                <div className="panel-card mt-4 p-4 text-sm text-gray-600 dark:text-gray-300">
                  Running <span className="font-mono">{version.data?.version || '—'}</span> (
                  {version.data?.name || 'Dockfin'})
                </div>
              </form>
            )}
          </div>
        </div>
      )}

      {topTab === 'email' && (
        <form
          className="max-w-3xl space-y-4"
          onSubmit={(e) => {
            e.preventDefault()
            saveEmail()
          }}
        >
          <SectionHead
            title="Transactional Email"
            actions={
              <Btn primary type="submit" disabled={!canEdit || save.isPending}>
                Save
              </Btn>
            }
          />
          <div className="grid gap-3 md:grid-cols-2">
            <TextField
              label="From Name"
              helper="Name used in emails."
              value={form.smtp_from_name}
              onChange={(v) => set('smtp_from_name', v)}
              required={false}
            />
            <TextField
              label="From Address"
              helper="Email address used in emails."
              value={form.smtp_from_address}
              onChange={(v) => set('smtp_from_address', v)}
              placeholder="noreply@example.com"
              required={false}
            />
          </div>

          <div className="panel-card space-y-3 p-5">
            <div className="flex items-center justify-between">
              <h3 className="font-medium text-gray-900 dark:text-white">SMTP Server</h3>
              <Toggle
                label="Enabled"
                checked={form.smtp_enabled}
                onChange={(v) => {
                  set('smtp_enabled', v)
                  if (v) set('resend_enabled', false)
                }}
                disabled={!canEdit}
              />
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              <TextField
                label="Host"
                value={form.smtp_host}
                onChange={(v) => set('smtp_host', v)}
                required={false}
              />
              <TextField
                label="Port"
                value={String(form.smtp_port)}
                onChange={(v) => set('smtp_port', Number(v) || 587)}
                required={false}
              />
              <label className="block text-sm">
                <FieldLabel label="Encryption" />
                <select
                  value={form.smtp_encryption}
                  onChange={(e) => set('smtp_encryption', e.target.value)}
                  className="panel-field w-full rounded-lg px-3 py-2"
                  disabled={!canEdit}
                >
                  <option value="starttls">STARTTLS</option>
                  <option value="tls">TLS</option>
                  <option value="none">None</option>
                </select>
              </label>
              <TextField
                label="Username"
                value={form.smtp_username || ''}
                onChange={(v) => set('smtp_username', v)}
                required={false}
              />
              <SecretField
                label="Password"
                value={smtpPassword}
                onChange={setSmtpPassword}
                placeholder={form.smtp_password_set ? '•••••••• (unchanged)' : ''}
              />
            </div>
          </div>

          <div className="panel-card space-y-3 p-5">
            <div className="flex items-center justify-between">
              <h3 className="font-medium text-gray-900 dark:text-white">Resend</h3>
              <Toggle
                label="Enabled"
                checked={form.resend_enabled}
                onChange={(v) => {
                  set('resend_enabled', v)
                  if (v) set('smtp_enabled', false)
                }}
                disabled={!canEdit}
              />
            </div>
            <SecretField
              label="API Key"
              value={resendKey}
              onChange={setResendKey}
              placeholder={form.resend_api_key_set ? '•••••••• (unchanged)' : 're_…'}
            />
          </div>
        </form>
      )}

      {topTab === 'oauth' && (
        <div className="space-y-4">
          <SectionHead
            title="OAuth"
          />
          {oauth.isLoading && <p className="text-sm text-gray-500">Loading…</p>}
          <div className="grid gap-4 lg:grid-cols-2">
            {(oauth.data?.oauth_settings || []).map((row) => (
              <OauthCard
                key={row.id}
                row={row}
                canEdit={canEdit}
                busy={patchOauth.isPending}
                onSave={(body) => patchOauth.mutate({ provider: row.provider, body })}
              />
            ))}
          </div>
        </div>
      )}

      {topTab === 'backup' && (
        <div className="space-y-4">
          <SectionHead
            title="Backup"
            actions={
              backups.data?.backup.configured ? (
                <div className="flex flex-wrap gap-2">
                  <Btn
                    primary
                    disabled={!canEdit || saveBackup.isPending}
                    onClick={() =>
                      saveBackup.mutate({
                        frequency: backupFreq,
                        retention: Number(backupRetention) || 0,
                        description: backupDesc,
                        enabled: backups.data?.backup.enabled,
                      })
                    }
                  >
                    Save
                  </Btn>
                  <Btn
                    disabled={!canEdit || runBackup.isPending}
                    onClick={() => runBackup.mutate()}
                  >
                    Backup Now
                  </Btn>
                </div>
              ) : null
            }
          />

          {backups.isLoading && <p className="text-sm text-gray-500">Loading backup settings…</p>}

          {backups.data && !backups.data.backup.configured && (
            <div className="panel-card space-y-3 p-5">
              <p className="text-sm text-gray-600 dark:text-gray-300">
                To configure automatic local backup for your Dockfin instance, add the instance database
                resource. Dumps are stored under{' '}
                <span className="font-mono text-xs">{backups.data.runtime.backup_dir}</span>.
              </p>
              {!backups.data.runtime.detected_container && (
                <p className="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">
                  No running Postgres container detected. Start the Dockfin database container, then try again.
                </p>
              )}
              <Btn
                primary
                disabled={!canEdit || configureBackup.isPending || !backups.data.runtime.detected_container}
                onClick={() => configureBackup.mutate()}
              >
                Configure Backup
              </Btn>
            </div>
          )}

          {backups.data?.backup.configured && (
            <>
              <div className="panel-card space-y-4 p-5">
                <div className="grid gap-3 md:grid-cols-3">
                  <TextField label="UUID" value={backups.data.backup.uuid} onChange={() => {}} disabled />
                  <TextField label="Name" value={backups.data.backup.name} onChange={() => {}} disabled />
                  <TextField
                    label="Description"
                    value={backupDesc}
                    onChange={setBackupDesc}
                    required={false}
                  />
                </div>
                <div className="grid gap-3 md:grid-cols-2">
                  <TextField label="User" value={backups.data.backup.db_user} onChange={() => {}} disabled />
                  <SecretField
                    label="Password"
                    value="••••••••"
                    onChange={() => {}}
                    placeholder="from DOCKFIN_DATABASE_URL"
                  />
                </div>
                <div className="grid gap-3 md:grid-cols-2">
                  <TextField
                    label="Container"
                    value={backups.data.runtime.container}
                    onChange={() => {}}
                    disabled
                    helper="Postgres Docker container used for pg_dump."
                  />
                  <TextField
                    label="Local backup directory"
                    value={backups.data.runtime.backup_dir}
                    onChange={() => {}}
                    disabled
                  />
                </div>
              </div>

              <div className="panel-card space-y-3 p-5">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <h3 className="font-medium text-gray-900 dark:text-white">Schedule</h3>
                  <Toggle
                    label="Enabled"
                    checked={backups.data.backup.enabled}
                    onChange={(v) => saveBackup.mutate({ enabled: v })}
                    disabled={!canEdit}
                  />
                </div>
                <div className="grid gap-3 md:grid-cols-2">
                  <TextField
                    label="Frequency (cron)"
                    helper="Default: every day at 00:00 (0 0 * * *)."
                    value={backupFreq}
                    onChange={setBackupFreq}
                  />
                  <TextField
                    label="Retention (local copies)"
                    helper="How many local dump files to keep. 0 = unlimited."
                    value={backupRetention}
                    onChange={setBackupRetention}
                  />
                </div>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Local-only backups. Optional S3 destinations remain under{' '}
                  <Link to="/storages" className="text-brand-600 hover:underline dark:text-brand-400">
                    S3 Storages
                  </Link>
                  {(storages.data?.s3_storages || []).length
                    ? ` (${storages.data!.s3_storages.length} configured)`
                    : ''}
                  .
                </p>
              </div>

              <div className="panel-card overflow-hidden">
                <div className="border-b border-gray-200 px-4 py-3 dark:border-gray-800">
                  <h3 className="text-sm font-medium text-gray-900 dark:text-white">Executions</h3>
                </div>
                <table className="w-full text-left text-sm">
                  <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
                    <tr>
                      <th className="px-3 py-2">Status</th>
                      <th className="px-3 py-2">Filename</th>
                      <th className="px-3 py-2">Size</th>
                      <th className="px-3 py-2">Started</th>
                      <th className="px-3 py-2">Error</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(backups.data.executions || []).map((ex) => (
                      <tr key={ex.id} className="border-t border-gray-200 dark:border-gray-800">
                        <td className="px-3 py-2 capitalize">{ex.status}</td>
                        <td className="max-w-xs truncate px-3 py-2 font-mono text-xs">{ex.filename}</td>
                        <td className="px-3 py-2 font-mono text-xs">
                          {ex.size_bytes ? `${Math.round(ex.size_bytes / 1024)} KB` : '—'}
                        </td>
                        <td className="px-3 py-2 text-xs text-gray-500">
                          {ex.started_at ? new Date(ex.started_at).toLocaleString() : '—'}
                        </td>
                        <td className="max-w-xs truncate px-3 py-2 text-xs text-error-500">
                          {ex.error_message || ''}
                        </td>
                      </tr>
                    ))}
                    {!backups.data.executions?.length && (
                      <tr>
                        <td colSpan={5} className="px-4 py-8 text-center text-gray-500">
                          No backup executions yet. Click Backup Now or wait for the schedule.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>
      )}

      {topTab === 'scheduled' && (
        <div className="space-y-4">
          <SectionHead
            title="Scheduled Jobs"
          />
          <div className="panel-card overflow-hidden">
            <table className="w-full text-left text-sm">
              <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
                <tr>
                  <th className="px-3 py-2">Name</th>
                  <th className="px-3 py-2">Command</th>
                  <th className="px-3 py-2">Frequency</th>
                  <th className="px-3 py-2">Enabled</th>
                </tr>
              </thead>
              <tbody>
                {(tasks.data?.scheduled_tasks || []).map((t) => (
                  <tr key={t.id} className="border-t border-gray-200 dark:border-gray-800">
                    <td className="px-3 py-2 font-medium">{t.name}</td>
                    <td className="max-w-xs truncate px-3 py-2 font-mono text-xs">{t.command}</td>
                    <td className="px-3 py-2 font-mono text-xs">{t.frequency}</td>
                    <td className="px-3 py-2">{t.enabled ? 'Yes' : 'No'}</td>
                  </tr>
                ))}
                {!tasks.data?.scheduled_tasks?.length && (
                  <tr>
                    <td colSpan={4} className="px-4 py-8 text-center text-gray-500">
                      No scheduled tasks yet.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {topTab === 'profile' && (
        <div className="space-y-4">
          <SectionHead
            title="Profile & License"
          />
          <div className="panel-card space-y-4 p-6">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Profile</h3>
            <dl className="grid gap-4 sm:grid-cols-2">
              <div>
                <dt className="text-xs text-gray-500 dark:text-gray-400">Signed in</dt>
                <dd className="mt-1 text-sm">
                  {user?.name} · {user?.email}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-gray-500 dark:text-gray-400">Team</dt>
                <dd className="mt-1 text-sm">{team?.name || '—'}</dd>
              </div>
            </dl>
          </div>
          <div className="panel-card space-y-4 p-6">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white">License</h3>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Dockfin is free and open source software licensed under the MIT License.
            </p>
            <pre className="max-h-72 overflow-auto rounded-lg border border-gray-200 bg-gray-50 p-3 font-mono text-xs whitespace-pre-wrap dark:border-gray-800 dark:bg-gray-950 dark:text-gray-300">
              {MIT_LICENSE_TEXT}
            </pre>
          </div>
        </div>
      )}
    </div>
  )
}

function OauthCard({
  row,
  canEdit,
  busy,
  onSave,
}: {
  row: OauthSetting
  canEdit: boolean
  busy: boolean
  onSave: (body: {
    enabled?: boolean
    client_id?: string
    client_secret?: string
    redirect_uri?: string
    tenant?: string
    base_url?: string
  }) => void
}) {
  const [clientId, setClientId] = useState(row.client_id)
  const [secret, setSecret] = useState('')
  const [redirect, setRedirect] = useState(row.redirect_uri)
  const [tenant, setTenant] = useState(row.tenant)
  const [baseUrl, setBaseUrl] = useState(row.base_url)
  const [enabled, setEnabled] = useState(row.enabled)

  useEffect(() => {
    setClientId(row.client_id)
    setRedirect(row.redirect_uri)
    setTenant(row.tenant)
    setBaseUrl(row.base_url)
    setEnabled(row.enabled)
    setSecret('')
  }, [row])

  const needsTenant = row.provider === 'azure'
  const needsBase = row.provider === 'authentik' || row.provider === 'clerk' || row.provider === 'zitadel' || row.provider === 'gitlab'

  return (
    <div className="panel-card space-y-3 p-5">
      <div className="flex items-center justify-between gap-2">
        <h3 className="font-medium capitalize text-gray-900 dark:text-white">{row.provider}</h3>
        <Toggle label="Enabled" checked={enabled} onChange={setEnabled} disabled={!canEdit} />
      </div>
      <TextField label="Client ID" value={clientId} onChange={setClientId} required={false} />
      <SecretField
        label="Client Secret"
        value={secret}
        onChange={setSecret}
        placeholder={row.client_secret_set ? '•••••••• (unchanged)' : ''}
      />
      <TextField label="Redirect URI" value={redirect} onChange={setRedirect} required={false} />
      {needsTenant && <TextField label="Tenant" value={tenant} onChange={setTenant} required={false} />}
      {needsBase && <TextField label="Base URL" value={baseUrl} onChange={setBaseUrl} required={false} />}
      <Btn
        primary
        disabled={!canEdit || busy}
        onClick={() => {
          const body: {
            enabled: boolean
            client_id: string
            redirect_uri: string
            tenant: string
            base_url: string
            client_secret?: string
          } = {
            enabled,
            client_id: clientId,
            redirect_uri: redirect,
            tenant,
            base_url: baseUrl,
          }
          if (secret) body.client_secret = secret
          onSave(body)
        }}
      >
        Save {row.provider}
      </Btn>
    </div>
  )
}
