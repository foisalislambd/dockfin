import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../lib/api'
import { Btn, Input, Modal } from '../pages/Servers'

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
      className="rounded px-1.5 py-0.5 text-[11px] font-medium text-brand-600 hover:bg-brand-500/10 dark:text-brand-400"
      onClick={() => {
        void navigator.clipboard.writeText(text).then(() => {
          setCopied(true)
          setTimeout(() => setCopied(false), 1500)
        })
      }}
    >
      {copied ? 'Copied' : 'Copy'}
    </button>
  )
}

type DNSCheckResponse = Awaited<ReturnType<typeof api.checkDomainDNS>>

export function DnsGuideModal({
  open,
  onClose,
  domains,
  serverIp,
  serverId,
  destinationId,
  autoCheck,
}: {
  open: boolean
  onClose: () => void
  domains: string
  serverIp: string
  serverId?: string
  destinationId?: string
  autoCheck?: boolean
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
  const [checkError, setCheckError] = useState('')

  const runCheck = async () => {
    if (!domains.trim() || hosts.length === 0) return
    setBusy(true)
    setCheckError('')
    try {
      const res = await api.checkDomainDNS({
        domains,
        server_id: serverId,
        destination_id: destinationId,
        // Prefer server lookup on API when UI IP unknown (destination-only flows).
        expected_ip: serverIp || undefined,
      })
      setCheck(res)
    } catch (e) {
      setCheck(null)
      setCheckError(e instanceof Error ? e.message : 'DNS check failed')
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => {
    if (!open) return
    setCheck(null)
    setCheckError('')
    if (autoCheck && hosts.length > 0) {
      void runCheck()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, domains])

  if (!open) return null

  return (
    <Modal title="DNS setup" onClose={onClose} wide>
      <div className="space-y-4 text-sm text-gray-700 dark:text-gray-300">
        <p>
          Add these DNS records at your provider (Cloudflare, Namecheap, Route53, GoDaddy, etc.) so
          traffic and Let&apos;s Encrypt reach this server.
        </p>

        {!serverIp ? (
          <p className="rounded-lg border border-amber-300/60 bg-amber-50 px-3 py-2 text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
            Server public IP is unknown. Open the server → set <strong>Public IP</strong> (or run
            Validate), then come back.
          </p>
        ) : (
          <div className="flex flex-wrap items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-gray-800 dark:bg-black/30">
            <span className="text-xs text-gray-500 dark:text-gray-400">Point A records to</span>
            <code className="font-mono text-sm text-gray-900 dark:text-white">{serverIp}</code>
            <CopyBtn text={serverIp} />
          </div>
        )}

        {hosts.length === 0 ? (
          <p className="text-xs text-gray-500 dark:text-gray-400">
            Enter a custom domain (not sslip.io / nip.io) to see exact DNS rows. Free magic domains
            need no DNS changes.
          </p>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-800">
            <table className="min-w-full text-left text-xs">
              <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
                <tr>
                  <th className="px-3 py-2 font-medium">Type</th>
                  <th className="px-3 py-2 font-medium">Name</th>
                  <th className="px-3 py-2 font-medium">Value</th>
                  <th className="px-3 py-2 font-medium">Host</th>
                </tr>
              </thead>
              <tbody>
                {hosts.map((host) => (
                  <tr key={host} className="border-t border-gray-200 dark:border-gray-800">
                    <td className="px-3 py-2 font-mono">A</td>
                    <td className="px-3 py-2 font-mono">
                      <span className="inline-flex items-center gap-1">
                        {dnsRecordName(host)}
                        <CopyBtn text={dnsRecordName(host)} />
                      </span>
                    </td>
                    <td className="px-3 py-2 font-mono">
                      <span className="inline-flex items-center gap-1">
                        {serverIp || '—'}
                        {serverIp ? <CopyBtn text={serverIp} /> : null}
                      </span>
                    </td>
                    <td className="px-3 py-2 font-mono text-gray-500 dark:text-gray-400">{host}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <ul className="list-disc space-y-1 pl-5 text-xs text-gray-500 dark:text-gray-400">
          <li>
            Apex (<code className="font-mono">example.com</code>): Name ={' '}
            <code className="font-mono">@</code> (or blank), Value = server IP.
          </li>
          <li>
            Subdomain (<code className="font-mono">app.example.com</code>): Name ={' '}
            <code className="font-mono">app</code>, Value = server IP.
          </li>
          <li>
            Cloudflare: use <strong>DNS only</strong> (grey cloud) until SSL works; orange proxy can
            block Let&apos;s Encrypt HTTP-01.
          </li>
          <li>Wait for DNS to propagate, then click Check DNS and Redeploy.</li>
        </ul>

        {hosts.length > 0 ? (
          <div className="space-y-2 rounded-lg border border-gray-200 p-3 dark:border-gray-800">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <span className="text-xs font-medium text-gray-900 dark:text-white">DNS validation</span>
              <Btn type="button" onClick={() => void runCheck()} disabled={busy}>
                {busy ? 'Checking…' : 'Check DNS'}
              </Btn>
            </div>
            {checkError ? <p className="text-xs text-error-500">{checkError}</p> : null}
            {check ? (
              <div className="space-y-2">
                {!check.validation_enabled ? (
                  <p className="text-xs text-amber-700 dark:text-amber-300">
                    DNS validation is disabled in Settings → Advanced. Records are not verified.
                  </p>
                ) : null}
                {check.results.map((r) => (
                  <div
                    key={r.host}
                    className={`rounded-md px-2.5 py-2 text-xs ${
                      r.matched || r.skip_validation
                        ? 'bg-emerald-500/10 text-emerald-800 dark:text-emerald-300'
                        : 'bg-error-500/10 text-error-600 dark:text-error-400'
                    }`}
                  >
                    <div className="font-medium">
                      {r.host}:{' '}
                      {r.skip_validation
                        ? 'skipped'
                        : r.matched
                          ? r.cloudflare
                            ? 'OK (Cloudflare proxy)'
                            : 'OK — points to server'
                          : 'Not pointing to this server yet'}
                    </div>
                    {r.resolved_ips?.length ? (
                      <div className="mt-0.5 font-mono opacity-80">
                        Resolved: {r.resolved_ips.join(', ')}
                        {r.expected_ip ? ` · expected ${r.expected_ip}` : ''}
                      </div>
                    ) : null}
                    {r.error && !r.matched ? <div className="mt-0.5 opacity-90">{r.error}</div> : null}
                  </div>
                ))}
                {!check.ok && check.validation_enabled ? (
                  <p className="text-xs text-gray-500 dark:text-gray-400">
                    Add the A record(s) above, wait a minute, then Check DNS again. You can still
                    save the domain and redeploy after DNS is correct.
                  </p>
                ) : null}
              </div>
            ) : (
              <p className="text-xs text-gray-500 dark:text-gray-400">
                After you create the A record(s), click Check DNS to verify they resolve to{' '}
                <code className="font-mono">{serverIp || 'your server IP'}</code>.
              </p>
            )}
          </div>
        ) : null}

        <div className="flex justify-end gap-2">
          <Btn type="button" onClick={onClose}>
            Got it
          </Btn>
        </div>
      </div>
    </Modal>
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
  hint,
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
  hint?: string
}) {
  const [dnsOpen, setDnsOpen] = useState(false)
  const [autoCheck, setAutoCheck] = useState(false)
  const [local, setLocal] = useState(value)
  const prevCustom = useRef(false)

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

  const customHosts = splitDomainEntries(local)
    .map(hostFromDomainEntry)
    .filter((h) => h && !isMagicHost(h))

  const applyNormalized = (raw: string) => {
    const next = normalizeDomains(raw)
    setLocal(next)
    onChange(next)
    const hosts = splitDomainEntries(next)
      .map(hostFromDomainEntry)
      .filter((h) => h && !isMagicHost(h))
    // Coolify-style: when user finishes typing a custom domain, open DNS topic popup.
    if (hosts.length > 0 && !prevCustom.current) {
      setAutoCheck(true)
      setDnsOpen(true)
    }
    prevCustom.current = hosts.length > 0
    return next
  }

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">{title}</h3>
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {hint ||
              'Type domain.com — https:// is added automatically. We show what to add in DNS and can check it.'}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Btn
            type="button"
            onClick={() => {
              setAutoCheck(true)
              setDnsOpen(true)
            }}
          >
            DNS instructions
          </Btn>
          {onSave ? (
            <Btn
              primary
              type="button"
              onClick={() => {
                const next = applyNormalized(local)
                if (customHosts.length > 0 || splitDomainEntries(next).some((e) => !isMagicHost(hostFromDomainEntry(e)))) {
                  setAutoCheck(true)
                  setDnsOpen(true)
                }
                onSave(next)
              }}
              disabled={saveBusy}
            >
              {saveBusy ? 'Saving…' : 'Save'}
            </Btn>
          ) : null}
        </div>
      </div>

      <div className="space-y-2">
        <Input
          label="Domains"
          value={local}
          onChange={(v) => {
            setLocal(v)
            onChange(v)
          }}
          onBlur={() => applyNormalized(local)}
          required={false}
        />
        <div className="flex flex-wrap items-center gap-3">
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
            Generate free domain (sslip.io / nip.io)
          </button>
          {customHosts.length > 0 ? (
            <button
              type="button"
              className="text-xs font-medium text-gray-600 hover:underline dark:text-gray-400"
              onClick={() => {
                setAutoCheck(true)
                setDnsOpen(true)
              }}
            >
              What to add in DNS →
            </button>
          ) : null}
        </div>
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Examples:{' '}
          <code className="font-mono">app.example.com</code>,{' '}
          <code className="font-mono">example.com,www.example.com</code>. After DNS is set, Redeploy
          for Traefik + SSL.
        </p>
        {customHosts.length > 0 && serverIp ? (
          <p className="rounded-lg border border-brand-500/20 bg-brand-500/5 px-3 py-2 text-xs text-gray-700 dark:text-gray-300">
            Add an <strong>A</strong> record: <code className="font-mono">{customHosts[0]}</code> →{' '}
            <code className="font-mono">{serverIp}</code>
            {customHosts.length > 1 ? ` (+${customHosts.length - 1} more)` : ''}.{' '}
            <button
              type="button"
              className="font-medium text-brand-600 hover:underline dark:text-brand-400"
              onClick={() => {
                setAutoCheck(true)
                setDnsOpen(true)
              }}
            >
              Open DNS guide &amp; check
            </button>
          </p>
        ) : null}
      </div>

      <DnsGuideModal
        open={dnsOpen}
        onClose={() => setDnsOpen(false)}
        domains={local}
        serverIp={serverIp}
        serverId={serverId}
        destinationId={destinationId}
        autoCheck={autoCheck}
      />
    </section>
  )
}
