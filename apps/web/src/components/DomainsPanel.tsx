import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
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

export function DnsGuideModal({
  open,
  onClose,
  domains,
  serverIp,
}: {
  open: boolean
  onClose: () => void
  domains: string
  serverIp: string
}) {
  const hosts = useMemo(
    () =>
      splitDomainEntries(domains)
        .map(hostFromDomainEntry)
        .filter((h) => h && !isMagicHost(h)),
    [domains],
  )

  if (!open) return null

  return (
    <Modal title="DNS setup" onClose={onClose} wide>
      <div className="space-y-4 text-sm text-gray-700 dark:text-gray-300">
        <p>
          Point your domain at this server so Traefik can route traffic and issue HTTPS certificates.
          Create these records at your DNS provider (Cloudflare, Namecheap, Route53, etc.).
        </p>

        {!serverIp ? (
          <p className="rounded-lg border border-amber-300/60 bg-amber-50 px-3 py-2 text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
            Server public IP is unknown. Open the server → set <strong>Public IP</strong> (or run
            Validate), then come back.
          </p>
        ) : (
          <div className="flex flex-wrap items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-gray-800 dark:bg-black/30">
            <span className="text-xs text-gray-500 dark:text-gray-400">Server IP</span>
            <code className="font-mono text-sm text-gray-900 dark:text-white">{serverIp}</code>
            <CopyBtn text={serverIp} />
          </div>
        )}

        {hosts.length === 0 ? (
          <p className="text-xs text-gray-500 dark:text-gray-400">
            Add a custom domain above (not sslip.io / nip.io) to see exact DNS rows. Magic free
            domains need no DNS changes.
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
            Apex domain (<code className="font-mono">example.com</code>): Name ={' '}
            <code className="font-mono">@</code> (or blank).
          </li>
          <li>
            Subdomain (<code className="font-mono">app.example.com</code>): Name ={' '}
            <code className="font-mono">app</code>.
          </li>
          <li>
            Wildcard for all subdomains: Name = <code className="font-mono">*</code>, Value = server
            IP.
          </li>
          <li>
            Cloudflare: use <strong>DNS only</strong> (grey cloud) until Let&apos;s Encrypt works;
            orange proxy can block HTTP-01.
          </li>
          <li>After DNS propagates, save domains here and redeploy the resource.</li>
        </ul>

        <div className="flex justify-end">
          <Btn type="button" onClick={onClose}>
            Got it
          </Btn>
        </div>
      </div>
    </Modal>
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
  hint,
}: {
  value: string
  onChange: (v: string) => void
  onSave?: () => void
  saveBusy?: boolean
  serverId?: string
  destinationId?: string
  resourceId?: string
  resourceName?: string
  title?: string
  hint?: string
}) {
  const [dnsOpen, setDnsOpen] = useState(false)
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

  const customHosts = splitDomainEntries(local)
    .map(hostFromDomainEntry)
    .filter((h) => h && !isMagicHost(h))

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">{title}</h3>
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {hint ||
              'Custom domains use https:// by default; free sslip.io / nip.io domains stay http://.'}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Btn type="button" onClick={() => setDnsOpen(true)}>
            DNS instructions
          </Btn>
          {onSave ? (
            <Btn primary type="button" onClick={onSave} disabled={saveBusy}>
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
                  setLocal(d.fqdn)
                  onChange(d.fqdn)
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
              onClick={() => setDnsOpen(true)}
            >
              What to add in DNS →
            </button>
          ) : null}
        </div>
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Examples:{' '}
          <code className="font-mono">https://app.example.com</code>,{' '}
          <code className="font-mono">https://example.com,https://www.example.com</code>. Empty =
          auto free domain on next deploy. Redeploy after saving for Traefik to pick up changes.
        </p>
        {customHosts.length > 0 && serverIp ? (
          <p className="rounded-lg border border-brand-500/20 bg-brand-500/5 px-3 py-2 text-xs text-gray-700 dark:text-gray-300">
            DNS: create an <strong>A</strong> record for{' '}
            <code className="font-mono">{customHosts[0]}</code> →{' '}
            <code className="font-mono">{serverIp}</code>
            {customHosts.length > 1 ? ` (and ${customHosts.length - 1} more)` : ''}.{' '}
            <button
              type="button"
              className="font-medium text-brand-600 hover:underline dark:text-brand-400"
              onClick={() => setDnsOpen(true)}
            >
              Full guide
            </button>
          </p>
        ) : null}
      </div>

      <DnsGuideModal
        open={dnsOpen}
        onClose={() => setDnsOpen(false)}
        domains={local}
        serverIp={serverIp}
      />
    </section>
  )
}
