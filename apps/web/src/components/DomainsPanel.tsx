import { useQuery } from '@tanstack/react-query'
import { Info } from 'lucide-react'
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

function dnsRecordName(host: string): string {
  const parts = host.split('.').filter(Boolean)
  if (parts.length <= 2) return '@'
  return parts.slice(0, -2).join('.')
}

function CopyBtn({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      type="button"
      className="rounded px-1 py-0.5 text-[10px] font-medium text-brand-600 hover:bg-brand-500/10 dark:text-brand-400"
      onClick={(e) => {
        e.preventDefault()
        e.stopPropagation()
        void navigator.clipboard.writeText(text).then(() => {
          setCopied(true)
          setTimeout(() => setCopied(false), 1200)
        })
      }}
    >
      {copied ? '✓' : 'Copy'}
    </button>
  )
}

type DNSCheckResponse = Awaited<ReturnType<typeof api.checkDomainDNS>>

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

  const runCheck = async () => {
    if (!domains.trim() || hosts.length === 0) return
    setBusy(true)
    try {
      setCheck(
        await api.checkDomainDNS({
          domains,
          server_id: serverId,
          destination_id: destinationId,
          expected_ip: serverIp || undefined,
        }),
      )
    } catch {
      setCheck(null)
    } finally {
      setBusy(false)
    }
  }

  if (hosts.length === 0) {
    return <p className="text-[11px] text-gray-500 dark:text-gray-400">Enter a custom domain first.</p>
  }

  return (
    <div className="space-y-2">
      {!serverIp ? (
        <p className="text-[11px] text-amber-700 dark:text-amber-300">Set public IP first.</p>
      ) : null}
      <table className="w-full text-left text-[11px]">
        <thead className="text-gray-500 dark:text-gray-400">
          <tr>
            <th className="pb-1 font-medium">Type</th>
            <th className="pb-1 font-medium">Name</th>
            <th className="pb-1 font-medium">Value</th>
          </tr>
        </thead>
        <tbody>
          {hosts.map((host) => (
            <tr key={host} className="border-t border-gray-100 dark:border-gray-800">
              <td className="py-1.5 font-mono">A</td>
              <td className="py-1.5 font-mono">
                <span className="inline-flex items-center gap-0.5">
                  {dnsRecordName(host)}
                  <CopyBtn text={dnsRecordName(host)} />
                </span>
              </td>
              <td className="py-1.5 font-mono">
                <span className="inline-flex items-center gap-0.5">
                  {serverIp || '—'}
                  {serverIp ? <CopyBtn text={serverIp} /> : null}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="flex items-center justify-between gap-2 pt-1">
        <button
          type="button"
          className="rounded-md bg-brand-500/10 px-2 py-1 text-[11px] font-medium text-brand-700 hover:bg-brand-500/20 dark:text-brand-300"
          disabled={busy}
          onClick={() => void runCheck()}
        >
          {busy ? 'Checking…' : 'Check DNS'}
        </button>
        {check?.results?.[0] ? (
          <span
            className={`text-[11px] ${
              check.results[0].matched || check.results[0].skip_validation
                ? 'text-emerald-600 dark:text-emerald-400'
                : 'text-error-500'
            }`}
          >
            {check.results[0].skip_validation
              ? 'Skipped'
              : check.results[0].matched
                ? check.results[0].cloudflare
                  ? 'OK (CF)'
                  : 'OK'
                : 'Mismatch'}
          </span>
        ) : null}
      </div>
    </div>
  )
}

/** Info-icon tooltip: only shows what DNS records to add. No modal. */
export function DnsGuideTooltip({
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
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

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
          className="absolute top-full left-0 z-40 mt-1.5 w-64 rounded-lg border border-gray-200 bg-white p-2.5 shadow-lg dark:border-gray-700 dark:bg-gray-950 sm:w-72"
        >
          <p className="mb-2 text-[11px] font-medium text-gray-900 dark:text-white">Add in DNS</p>
          <DnsTipBody
            domains={domains}
            serverIp={serverIp}
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

  useEffect(() => {
    setLocal(value)
  }, [value])

  const server = useQuery({
    queryKey: ['server', serverId],
    queryFn: () => api.getServer(serverId!),
    enabled: Boolean(serverId),
  })

  const serverIp = useMemo(() => {
    const s = server.data
    if (!s) return ''
    const pub = (s.public_ip || '').trim()
    if (pub && pub !== '127.0.0.1' && pub !== '0.0.0.0') return pub
    const ip = (s.ip || '').trim()
    if (ip && ip !== '127.0.0.1' && ip !== '0.0.0.0' && ip !== 'localhost') return ip
    return pub || ip || ''
  }, [server.data])

  const applyNormalized = (raw: string) => {
    const next = normalizeDomains(raw)
    setLocal(next)
    onChange(next)
    return next
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

      <input
        value={local}
        placeholder="app.example.com"
        onChange={(e) => {
          setLocal(e.target.value)
          onChange(e.target.value)
        }}
        onBlur={() => applyNormalized(local)}
        className="panel-field w-full rounded-lg px-3 py-2 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
      />
      <button
        type="button"
        className="text-xs font-medium text-brand-600 hover:underline dark:text-brand-400"
        onClick={() => {
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
            .catch(() => undefined)
        }}
      >
        Generate free domain
      </button>
    </section>
  )
}
