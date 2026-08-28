import { useQuery } from '@tanstack/react-query'
import { Check, Copy, Info } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../lib/api'
import { Btn } from '../pages/Servers'

/** Extract bare hostname from a domain entry (scheme/path/port stripped). */
export function hostFromDomainEntry(entry: string): string {
  let s = entry.trim()
  if (!s) return ''
  const lower = s.toLowerCase()
  if (lower.startsWith('https://')) s = s.slice('https://'.length)
  else if (lower.startsWith('http://')) s = s.slice('http://'.length)
  const slash = s.indexOf('/')
  if (slash >= 0) s = s.slice(0, slash)
  const colon = s.indexOf(':')
  if (colon >= 0) s = s.slice(0, colon)
  return s.toLowerCase()
}

export function splitDomainEntries(domains: string): string[] {
  return domains
    .split(',')
    .map((p) => p.trim())
    .filter(Boolean)
}

function isMagicHost(host: string): boolean {
  const h = host.toLowerCase()
  return h.endsWith('.sslip.io') || h === 'sslip.io' || h.endsWith('.nip.io') || h === 'nip.io'
}

/** Matches Go proxy.WantAutoHTTPS: custom hosts only; magic/localhost stay HTTP. */
export function domainsWantAutoHttps(domains: string): boolean {
  const hosts = splitDomainEntries(domains).map(hostFromDomainEntry).filter(Boolean)
  if (!hosts.length) return false
  for (const h of hosts) {
    if (h === 'localhost' || h === '127.0.0.1' || isMagicHost(h)) return false
  }
  return true
}

function dnsRecordName(host: string): string {
  const parts = host.split('.').filter(Boolean)
  if (parts.length <= 2) return '@'
  return parts.slice(0, -2).join('.')
}

/** Registrable-looking zone for wildcard tip (last two labels). */
function dnsZone(host: string): string {
  const parts = host.split('.').filter(Boolean)
  if (parts.length < 2) return host
  return parts.slice(-2).join('.')
}

function pickUsableIP(...candidates: Array<string | undefined | null>): string {
  for (const c of candidates) {
    const ip = (c || '').trim()
    if (!ip || ip === '127.0.0.1' || ip === '0.0.0.0' || ip === 'localhost') continue
    return ip
  }
  return ''
}

function CopyBtn({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  if (!text) return null
  return (
    <button
      type="button"
      title={copied ? 'Copied' : `Copy ${text}`}
      aria-label={copied ? 'Copied' : `Copy ${text}`}
      className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded border border-gray-200 bg-white text-gray-600 hover:border-brand-400 hover:text-brand-600 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300 dark:hover:border-brand-500 dark:hover:text-brand-400"
      onClick={(e) => {
        e.preventDefault()
        e.stopPropagation()
        void navigator.clipboard.writeText(text).then(() => {
          setCopied(true)
          setTimeout(() => setCopied(false), 1200)
        })
      }}
    >
      {copied ? <Check className="h-3 w-3 text-emerald-500" /> : <Copy className="h-3 w-3" />}
    </button>
  )
}

type DNSCheckResponse = Awaited<ReturnType<typeof api.checkDomainDNS>>

type DnsRow = { type: string; name: string; value: string; note?: string }

function buildDnsRows(hosts: string[], serverIp: string): DnsRow[] {
  const rows: DnsRow[] = []
  const seen = new Set<string>()
  const zones = new Set<string>()

  for (const host of hosts) {
    const name = dnsRecordName(host)
    const key = `A:${name}`
    if (!seen.has(key)) {
      seen.add(key)
      rows.push({ type: 'A', name, value: serverIp })
    }
    zones.add(dnsZone(host))
  }

  for (const zone of zones) {
    const key = `A:*:${zone}`
    if (seen.has(key)) continue
    seen.add(key)
    rows.push({
      type: 'A',
      name: '*',
      value: serverIp,
      note: `all of *.${zone}`,
    })
  }

  return rows
}

/** Shared Type / Name / Value table with clear copy buttons. */
function DnsRecordsTable({ rows }: { rows: DnsRow[] }) {
  if (rows.length === 0) return null
  return (
    <div className="overflow-hidden rounded-md border border-gray-200 dark:border-gray-700">
      <table className="w-full text-left text-xs">
        <thead className="bg-gray-50 text-[10px] uppercase tracking-wide text-gray-500 dark:bg-white/5 dark:text-gray-400">
          <tr>
            <th className="px-2 py-1.5 font-medium">Type</th>
            <th className="px-2 py-1.5 font-medium">Name</th>
            <th className="px-2 py-1.5 font-medium">Value</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr
              key={`${row.type}-${row.name}-${row.note || ''}`}
              className="border-t border-gray-200 dark:border-gray-800"
            >
              <td className="px-2 py-2 font-mono text-[11px]">{row.type}</td>
              <td className="px-2 py-2">
                <div className="flex items-center gap-1.5">
                  <code className="font-mono text-[11px] text-gray-900 dark:text-white">{row.name}</code>
                  <CopyBtn text={row.name} />
                </div>
                {row.note ? (
                  <div className="mt-0.5 text-[10px] text-gray-500 dark:text-gray-400">{row.note}</div>
                ) : null}
              </td>
              <td className="px-2 py-2">
                <div className="flex items-center gap-1.5">
                  <code className="font-mono text-[11px] text-gray-900 dark:text-white">
                    {row.value || '—'}
                  </code>
                  <CopyBtn text={row.value} />
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

/** Minimal DNS rows + Check DNS (for tooltip only). */
function DnsTipBody({
  domains,
  serverIp,
  serverId,
  destinationId,
}: {
  domains: string
  serverIp: string
  serverId?: string
  destinationId?: string
}) {
  const hosts = useMemo(
    () =>
      splitDomainEntries(domains)
        .map(hostFromDomainEntry)
        .filter((h) => h && !isMagicHost(h)),
    [domains],
  )
  const [busy, setBusy] = useState(false)
  const [check, setCheck] = useState<DNSCheckResponse | null>(null)
  const ip = pickUsableIP(serverIp, check?.expected_ip)
  const rows = useMemo(() => buildDnsRows(hosts, ip), [hosts, ip])

  const runCheck = async () => {
    if (!domains.trim() || hosts.length === 0) return
    setBusy(true)
    try {
      setCheck(
        await api.checkDomainDNS({
          domains,
          server_id: serverId,
          destination_id: destinationId,
          expected_ip: ip || undefined,
        }),
      )
    } catch {
      setCheck(null)
    } finally {
      setBusy(false)
    }
  }

  if (hosts.length === 0) {
    return <p className="text-xs text-gray-500 dark:text-gray-400">Enter a domain first.</p>
  }

  return (
    <div className="space-y-2">
      {!ip ? (
        <p className="text-xs text-amber-700 dark:text-amber-300">Set Public IPv4 first.</p>
      ) : null}
      <DnsRecordsTable rows={rows} />
      <p className="text-[10px] text-gray-500 dark:text-gray-400">
        <code className="font-mono">*</code> = all subdomains. <code className="font-mono">@</code> =
        root domain.
      </p>
      <div className="flex items-center justify-between gap-2">
        <button
          type="button"
          className="rounded-md bg-brand-500/10 px-2 py-1 text-xs font-medium text-brand-700 hover:bg-brand-500/20 dark:text-brand-300"
          disabled={busy}
          onClick={() => void runCheck()}
        >
          {busy ? 'Checking…' : 'Check DNS'}
        </button>
        {check?.results?.[0] ? (
          <span
            className={`text-xs font-medium ${
              check.results[0].matched || check.results[0].skip_validation
                ? 'text-emerald-600 dark:text-emerald-400'
                : 'text-error-500'
            }`}
          >
            {check.results[0].skip_validation
              ? 'Skipped'
              : check.results[0].matched
                ? 'OK'
                : 'Mismatch'}
          </span>
        ) : null}
      </div>
    </div>
  )
}

/** Info-icon tooltip: DNS records to add (exact + wildcard). Resolves IP if missing. */
export function DnsGuideTooltip({
  domains,
  serverIp,
  serverId,
  destinationId,
}: {
  domains: string
  serverIp?: string
  serverId?: string
  destinationId?: string
}) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

  const needResolve = open && !pickUsableIP(serverIp)
  const settings = useQuery({
    queryKey: ['instance-settings'],
    queryFn: api.instanceSettings,
    enabled: needResolve,
  })
  const servers = useQuery({
    queryKey: ['servers'],
    queryFn: api.servers,
    enabled: needResolve && !serverId,
  })
  const server = useQuery({
    queryKey: ['server', serverId],
    queryFn: () => api.getServer(serverId!),
    enabled: open && Boolean(serverId),
  })

  const resolvedIp = useMemo(() => {
    const fromProp = pickUsableIP(serverIp)
    if (fromProp) return fromProp
    const s = server.data
    if (s) {
      const fromServer = pickUsableIP(s.public_ip, s.ip)
      if (fromServer) return fromServer
    }
    const fromSettings = pickUsableIP(settings.data?.settings?.public_ipv4)
    if (fromSettings) return fromSettings
    const list = servers.data?.servers || []
    for (const srv of list) {
      const ip = pickUsableIP(srv.public_ip, srv.ip)
      if (ip) return ip
    }
    return ''
  }, [serverIp, server.data, settings.data, servers.data])

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <div className="relative inline-flex" ref={rootRef}>
      <button
        type="button"
        className={`rounded p-0.5 transition ${
          open
            ? 'text-brand-600 dark:text-brand-400'
            : 'text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'
        }`}
        aria-label="DNS records"
        aria-expanded={open}
        onClick={(e) => {
          e.preventDefault()
          setOpen((v) => !v)
        }}
      >
        <Info className="h-3.5 w-3.5" />
      </button>
      {open ? (
        <div
          role="tooltip"
          className="absolute top-full left-0 z-40 mt-1.5 w-80 rounded-lg border border-gray-200 bg-white p-3 shadow-lg dark:border-gray-700 dark:bg-gray-950"
        >
          <p className="mb-2 text-xs font-semibold text-gray-900 dark:text-white">Add these DNS records</p>
          <DnsTipBody
            domains={domains}
            serverIp={resolvedIp}
            serverId={serverId}
            destinationId={destinationId}
          />
        </div>
      ) : null}
    </div>
  )
}

/** Normalize a bare domain to http(s)://… — custom → https, magic → http. */
export function normalizeDomainEntry(entry: string): string {
  const raw = entry.trim()
  if (!raw) return ''
  const lower = raw.toLowerCase()
  if (lower.startsWith('https://') || lower.startsWith('http://')) {
    return raw.replace(/\/$/, '')
  }
  const host = hostFromDomainEntry(raw)
  if (!host) return raw
  const rest = raw.replace(/\/$/, '')
  if (host === 'localhost' || host === '127.0.0.1' || isMagicHost(host)) {
    return `http://${rest}`
  }
  return `https://${rest}`
}

export function normalizeDomains(domains: string): string {
  return splitDomainEntries(domains).map(normalizeDomainEntry).filter(Boolean).join(',')
}

/**
 * Inline DNS health under a domain field. Red when A records don't point at
 * this server — so users know to fix DNS (Settings + project resources).
 */
export function DomainDNSAlert({
  domains,
  serverIp,
  serverId,
  destinationId,
}: {
  domains: string
  serverIp?: string
  serverId?: string
  destinationId?: string
}) {
  const customHosts = useMemo(
    () =>
      splitDomainEntries(domains)
        .map(hostFromDomainEntry)
        .filter((h) => h && !isMagicHost(h) && h !== 'localhost' && h !== '127.0.0.1'),
    [domains],
  )
  const [debounced, setDebounced] = useState(domains)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(domains), 600)
    return () => clearTimeout(t)
  }, [domains])

  const needResolve = customHosts.length > 0 && !pickUsableIP(serverIp)
  const settings = useQuery({
    queryKey: ['instance-settings'],
    queryFn: api.instanceSettings,
    enabled: needResolve,
  })
  const servers = useQuery({
    queryKey: ['servers'],
    queryFn: api.servers,
    enabled: needResolve && !serverId,
  })
  const resolvedIp = useMemo(() => {
    const fromProp = pickUsableIP(serverIp)
    if (fromProp) return fromProp
    const fromSettings = pickUsableIP(settings.data?.settings?.public_ipv4)
    if (fromSettings) return fromSettings
    for (const srv of servers.data?.servers || []) {
      const ip = pickUsableIP(srv.public_ip, srv.ip)
      if (ip) return ip
    }
    return ''
  }, [serverIp, settings.data, servers.data])

  const ip = pickUsableIP(resolvedIp)

  const check = useQuery({
    queryKey: ['domain-dns-alert', debounced, resolvedIp, serverId, destinationId],
    queryFn: () =>
      api.checkDomainDNS({
        domains: debounced,
        server_id: serverId,
        destination_id: destinationId,
        expected_ip: resolvedIp || undefined,
      }),
    enabled: customHosts.length > 0 && Boolean(debounced.trim()),
    staleTime: 15_000,
    retry: false,
  })

  // Silent while first check runs — only show after a completed mismatch.
  if (customHosts.length === 0) return null
  if (!check.data || check.isError) return null

  const data = check.data
  if (!data.validation_enabled) {
    return null
  }

  const bad = (data.results || []).filter((r) => !r.matched && !r.skip_validation)
  if (bad.length === 0 || data.ok) {
    return null
  }

  const alertIp = pickUsableIP(data.expected_ip, ip)
  const alertRows = buildDnsRows(customHosts, alertIp)

  return (
    <div className="space-y-2 rounded-lg border border-error-500/40 bg-error-500/10 px-3 py-2.5">
      <p className="text-xs font-semibold text-error-700 dark:text-error-400">DNS mismatch — add these</p>
      <DnsRecordsTable rows={alertRows} />
      <div className="space-y-1 text-[11px] leading-relaxed text-error-700 dark:text-error-400">
        <p>
          At your DNS provider, create the A records above (copy Name + Value). Wait 1–5 minutes,
          then save again.
        </p>
        {bad[0]?.resolved_ips?.length ? (
          <p>
            Now points to <code className="font-mono">{bad[0].resolved_ips.join(', ')}</code>
            {alertIp ? (
              <>
                ; must be <code className="font-mono">{alertIp}</code>
              </>
            ) : null}
            .
          </p>
        ) : (
          <p>Domain is not resolving to this server yet.</p>
        )}
      </div>
    </div>
  )
}

export function DomainsPanel({
  value,
  onChange,
  onSave,
  saveBusy,
  serverId,
  destinationId,
  resourceId,
  resourceName,
  title = 'Domains',
}: {
  value: string
  onChange: (v: string) => void
  onSave?: (normalized: string) => void
  saveBusy?: boolean
  serverId?: string
  destinationId?: string
  resourceId?: string
  resourceName?: string
  title?: string
  /** @deprecated unused — kept for call-site compatibility */
  hint?: string
}) {
  const [local, setLocal] = useState(value)
  const [draft, setDraft] = useState('')
  const [error, setError] = useState('')
  const [advanced, setAdvanced] = useState(false)

  useEffect(() => {
    setLocal(value)
    setError('')
  }, [value])

  const server = useQuery({
    queryKey: ['server', serverId],
    queryFn: () => api.getServer(serverId!),
    enabled: Boolean(serverId),
  })

  const serverIp = useMemo(() => {
    const s = server.data
    if (!s) return ''
    return pickUsableIP(s.public_ip, s.ip)
  }, [server.data])

  const chips = useMemo(() => splitDomainEntries(local), [local])

  const applyNormalized = (raw: string) => {
    const next = normalizeDomains(raw)
    setLocal(next)
    onChange(next)
    return next
  }

  const setFromChips = (nextChips: string[]) => {
    applyNormalized(nextChips.join(','))
  }

  const addChip = () => {
    const entry = normalizeDomainEntry(draft.trim())
    if (!entry) return
    if (chips.includes(entry)) {
      setDraft('')
      return
    }
    setFromChips([...chips, entry])
    setDraft('')
  }

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-1.5">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">{title}</h3>
          <DnsGuideTooltip
            domains={local}
            serverIp={serverIp}
            serverId={serverId}
            destinationId={destinationId}
          />
        </div>
        {onSave ? (
          <Btn
            primary
            type="button"
            onClick={() => onSave(applyNormalized(local))}
            disabled={saveBusy}
          >
            {saveBusy ? 'Saving…' : 'Save'}
          </Btn>
        ) : null}
      </div>

      <div className="flex min-h-10 flex-wrap items-center gap-2 rounded-lg border border-gray-200 bg-white px-2 py-1.5 dark:border-gray-800 dark:bg-gray-950">
        {chips.map((host) => (
          <span
            key={host}
            className="inline-flex items-center gap-1 rounded-md bg-gray-100 px-2 py-1 font-mono text-xs text-gray-800 dark:bg-white/10 dark:text-gray-100"
          >
            {host}
            <button
              type="button"
              className="text-gray-500 hover:text-error-500"
              aria-label={`Remove ${host}`}
              onClick={() => setFromChips(chips.filter((c) => c !== host))}
            >
              ×
            </button>
          </span>
        ))}
        <input
          value={draft}
          placeholder={chips.length ? 'Add domain…' : 'app.example.com'}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ',') {
              e.preventDefault()
              addChip()
            }
            if (e.key === 'Backspace' && !draft && chips.length) {
              setFromChips(chips.slice(0, -1))
            }
          }}
          onBlur={() => addChip()}
          className="min-w-[10rem] flex-1 border-0 bg-transparent px-1 py-1 text-sm outline-none"
        />
      </div>

      <button
        type="button"
        className="text-xs text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
        onClick={() => setAdvanced((v) => !v)}
      >
        {advanced ? 'Hide raw editor' : 'Edit as comma-separated list'}
      </button>
      {advanced && (
        <input
          value={local}
          placeholder="app.example.com, api.example.com"
          onChange={(e) => {
            setLocal(e.target.value)
            onChange(e.target.value)
          }}
          onBlur={() => applyNormalized(local)}
          className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
        />
      )}

      <DomainDNSAlert
        domains={local}
        serverIp={serverIp}
        serverId={serverId}
        destinationId={destinationId}
      />
      {error && (
        <p className="text-sm text-error-500" role="alert">
          {error}
        </p>
      )}
      <button
        type="button"
        className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400"
        onClick={() => {
          setError('')
          void api
            .generateDomain({
              name: resourceName || 'app',
              destination_id: destinationId || undefined,
              server_id: serverId || undefined,
              resource_id: resourceId || undefined,
            })
            .then((d) => {
              applyNormalized(d.fqdn || d.url || '')
            })
            .catch((e: unknown) => {
              const msg = e instanceof Error ? e.message : 'Could not generate free domain'
              setError(msg)
            })
        }}
      >
        Generate free domain
      </button>
    </section>
  )
}
