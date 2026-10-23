import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { api, type ServiceDomain, type ServiceUnit } from '../lib/api'
import { useToast } from './Toast'
import { Modal } from './ui/Modal'
import { Btn } from './ui/Button'

type Draft = {
  unit_name: string
  host: string
  scheme: string
  internal_port: string
  force_https: boolean
  redirect: string
  noindex: boolean
}

const emptyDraft = (unit: string): Draft => ({
  unit_name: unit,
  host: '',
  scheme: 'https',
  internal_port: '',
  force_https: true,
  redirect: 'both',
  noindex: false,
})

function isMagic(host: string) {
  return /\.(sslip\.io|nip\.io)$/i.test(host)
}

export function ServiceDomainsPanel({
  serviceId,
  serviceName,
  units,
  serverId,
  destinationId,
}: {
  serviceId: string
  serviceName: string
  units: ServiceUnit[]
  serverId?: string
  destinationId?: string
}) {
  const qc = useQueryClient()
  const toast = useToast()
  const [search, setSearch] = useState('')
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ServiceDomain | null>(null)
  const [draft, setDraft] = useState<Draft>(emptyDraft(units[0]?.name || ''))
  const [dns, setDns] = useState<Record<string, 'ok' | 'failed' | 'checking' | 'pending'>>({})

  const q = useQuery({
    queryKey: ['service-domains', serviceId],
    queryFn: () => api.serviceDomains(serviceId),
  })
  const rows = q.data?.domains || []

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ['service-domains', serviceId] })
    void qc.invalidateQueries({ queryKey: ['service', serviceId] })
  }

  const save = useMutation({
    mutationFn: async () => {
      const rawPort = draft.internal_port.trim()
      let port = 0
      if (rawPort !== '') {
        port = Number(rawPort)
        if (!Number.isInteger(port) || port < 1 || port > 65535) {
          throw new Error('Internal port must be a number from 1 to 65535')
        }
      }
      const magic = isMagic(draft.host.trim())
      const body = {
        unit_name: draft.unit_name,
        host: draft.host.trim(),
        scheme: magic ? 'http' : 'https',
        internal_port: port,
        force_https: magic ? false : draft.force_https,
        redirect: magic ? 'both' : draft.redirect,
        noindex: draft.noindex,
      }
      if (editing) return api.updateServiceDomain(serviceId, editing.id, body)
      return api.createServiceDomain(serviceId, body)
    },
    onSuccess: () => {
      setOpen(false)
      setEditing(null)
      refresh()
      toast.success(editing ? 'Domain updated' : 'Domain added')
    },
    onError: (e: Error) => toast.error(e.message || 'Could not save domain'),
  })

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteServiceDomain(serviceId, id),
    onSuccess: () => {
      refresh()
      toast.success('Domain removed')
    },
    onError: (e: Error) => toast.error(e.message || 'Could not remove domain'),
  })

  const checkDns = async (hosts: string[]) => {
    if (!hosts.length) return
    setDns((prev) => {
      const next = { ...prev }
      for (const h of hosts) next[h] = 'checking'
      return next
    })
    try {
      const res = await api.checkDomainDNS({
        domains: hosts.join(','),
        server_id: serverId,
        destination_id: destinationId,
      })
      setDns((prev) => {
        const next = { ...prev }
        for (const row of res.results || []) {
          next[row.host] = row.skip_validation ? 'pending' : row.matched ? 'ok' : 'failed'
        }
        return next
      })
    } catch (e) {
      setDns((prev) => {
        const next = { ...prev }
        for (const h of hosts) next[h] = 'failed'
        return next
      })
      toast.error(e instanceof Error ? e.message : 'DNS check failed')
    }
  }

  const groups = useMemo(() => {
    const qtext = search.trim().toLowerCase()
    const map = new Map<string, ServiceDomain[]>()
    for (const row of rows) {
      const key = row.unit_name || 'Stack'
      const blob = `${key} ${row.host}`.toLowerCase()
      if (qtext && !blob.includes(qtext)) continue
      map.set(key, [...(map.get(key) || []), row])
    }
    return [...map.entries()]
  }, [rows, search])

  const openNew = () => {
    setEditing(null)
    setDraft(emptyDraft(units[0]?.name || ''))
    setOpen(true)
  }
  const openEdit = (row: ServiceDomain) => {
    setEditing(row)
    setDraft({
      unit_name: row.unit_name || units[0]?.name || '',
      host: row.host,
      scheme: row.scheme || 'https',
      internal_port: row.internal_port ? String(row.internal_port) : '',
      force_https: row.force_https,
      redirect: row.redirect || 'both',
      noindex: row.noindex,
    })
    setOpen(true)
  }

  const generate = () => {
    void api
      .generateDomain({
        name: draft.unit_name || serviceName || 'app',
        server_id: serverId,
        destination_id: destinationId,
        resource_id: serviceId,
      })
      .then((d) => {
        const host = (d.fqdn || '').split(',')[0].replace(/^https?:\/\//, '')
        setDraft((prev) => ({ ...prev, host, scheme: 'http', force_https: false, redirect: 'both' }))
      })
      .catch((e: unknown) => toast.error(e instanceof Error ? e.message : 'Could not generate domain'))
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search services or domains"
          aria-label="Search services or domains"
          className="panel-field h-8 w-full rounded-md px-3 text-[13px] sm:w-64"
        />
        <div className="ml-auto flex flex-wrap gap-2">
          {rows.length > 0 && (
            <Btn type="button" onClick={() => void checkDns(rows.map((r) => r.host))}>
              Check all DNS
            </Btn>
          )}
          <Btn primary type="button" onClick={openNew}>
            Add domain
          </Btn>
        </div>
      </div>

      {q.isLoading ? (
        <div className="panel-card px-4 py-8 text-sm text-gray-500">Loading domains…</div>
      ) : !rows.length ? (
        <div className="panel-card px-4 py-10 text-center">
          <p className="text-sm font-medium text-gray-900 dark:text-white">No domains configured</p>
          <p className="mx-auto mt-1 max-w-md text-sm text-gray-500 dark:text-gray-400">
            Add a domain and choose which container in this stack receives it. Redeploy to publish it.
          </p>
        </div>
      ) : !groups.length ? (
        <div className="panel-card px-4 py-8 text-center text-sm text-gray-500">No domains match that search.</div>
      ) : (
        groups.map(([unit, list]) => (
          <section key={unit} className="panel-card overflow-hidden">
            <div className="flex flex-wrap items-center gap-3 border-b border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-800 dark:bg-white/5">
              <span className="min-w-0 flex-1 truncate text-sm font-medium text-gray-900 dark:text-white">{unit}</span>
              <span className="text-xs text-gray-500">{list.length} domain{list.length === 1 ? '' : 's'}</span>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full table-fixed text-left text-[13px]">
                <thead className="text-[11px] font-medium uppercase tracking-wide text-gray-500">
                  <tr>
                    <th className="w-[34%] px-4 py-2">Domain</th>
                    <th className="hidden px-3 py-2 lg:table-cell">Protocol</th>
                    <th className="hidden px-3 py-2 xl:table-cell">Redirect</th>
                    <th className="hidden w-16 px-3 py-2 sm:table-cell">Port</th>
                    <th className="hidden px-3 py-2 md:table-cell">Indexing</th>
                    <th className="w-24 px-3 py-2">DNS</th>
                    <th className="w-28 px-4 py-2 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {list.map((row) => {
                    const url = `${row.scheme || 'https'}://${row.host}`
                    const dnsState = dns[row.host] || 'pending'
                    const dnsLabel =
                      dnsState === 'ok' ? 'DNS matches' : dnsState === 'failed' ? 'DNS mismatch' : dnsState === 'checking' ? 'Checking…' : 'Not checked'
                    return (
                      <tr key={row.id} className="border-t border-gray-100 dark:border-white/8">
                        <td className="max-w-0 px-4 py-3">
                          <a
                            href={url}
                            target="_blank"
                            rel="noreferrer"
                            title={url}
                            className="block truncate text-brand-600 hover:underline dark:text-brand-400"
                          >
                            {url}
                          </a>
                        </td>
                        <td className="hidden px-3 py-3 text-gray-600 lg:table-cell dark:text-gray-300">
                          {row.scheme === 'https' && row.force_https ? 'HTTP → HTTPS' : 'Disabled'}
                        </td>
                        <td className="hidden px-3 py-3 text-gray-600 xl:table-cell dark:text-gray-300">
                          {row.redirect === 'www' ? 'non-www → www' : row.redirect === 'non-www' ? 'www → non-www' : 'Disabled'}
                        </td>
                        <td className="hidden px-3 py-3 font-mono text-xs sm:table-cell">{row.internal_port || '—'}</td>
                        <td className="hidden px-3 py-3 md:table-cell">{row.noindex ? 'Blocked' : 'Allowed'}</td>
                        <td className="px-3 py-3">
                          <button type="button" className="text-xs hover:underline" onClick={() => void checkDns([row.host])}>
                            {dnsLabel}
                          </button>
                        </td>
                        <td className="px-4 py-3 text-right whitespace-nowrap">
                          <button type="button" className="text-xs text-brand-600 hover:underline" onClick={() => openEdit(row)}>
                            Edit
                          </button>
                          <button
                            type="button"
                            className="ml-3 text-xs text-error-500 hover:underline"
                            onClick={() => remove.mutate(row.id)}
                          >
                            Remove
                          </button>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </section>
        ))
      )}

      {open && (
        <Modal title={editing ? 'Edit domain' : 'Add domain'} onClose={() => setOpen(false)} wide>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              save.mutate()
            }}
          >
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500">Container</span>
              <select
                value={draft.unit_name}
                onChange={(e) => setDraft({ ...draft, unit_name: e.target.value })}
                className="panel-field w-full rounded-lg px-3 py-2 text-sm"
              >
                {!units.length && <option value="">Stack</option>}
                {units.map((u) => (
                  <option key={u.name} value={u.name}>
                    {u.name}
                    {u.image ? ` (${u.image})` : ''}
                  </option>
                ))}
              </select>
            </label>
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500">Domain</span>
              <input
                required
                value={draft.host}
                onChange={(e) => setDraft({ ...draft, host: e.target.value })}
                placeholder="app.example.com"
                className="panel-field w-full rounded-lg px-3 py-2 font-mono text-sm"
              />
            </label>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Custom domains are published as HTTPS. Magic domains stay HTTP and are left off the proxy while a custom domain is set.
            </p>
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500">Internal port</span>
              <input
                value={draft.internal_port}
                onChange={(e) => setDraft({ ...draft, internal_port: e.target.value })}
                placeholder="80"
                inputMode="numeric"
                className="panel-field w-full rounded-lg px-3 py-2 text-sm"
              />
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={draft.force_https}
                onChange={(e) => setDraft({ ...draft, force_https: e.target.checked })}
              />
              Redirect HTTP to HTTPS
            </label>
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500">Domain redirect</span>
              <select
                value={draft.redirect}
                onChange={(e) => setDraft({ ...draft, redirect: e.target.value })}
                className="panel-field w-full rounded-lg px-3 py-2 text-sm"
              >
                <option value="both">Both www and non-www</option>
                <option value="www">Redirect to www</option>
                <option value="non-www">Redirect to non-www</option>
              </select>
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={draft.noindex}
                onChange={(e) => setDraft({ ...draft, noindex: e.target.checked })}
              />
              Block search indexing
            </label>
            <div className="flex flex-wrap items-center justify-between gap-2 pt-2">
              <Btn type="button" onClick={generate}>
                Generate domain
              </Btn>
              <Btn primary type="submit">
                {save.isPending ? 'Saving…' : 'Save'}
              </Btn>
            </div>
          </form>
        </Modal>
      )}
    </div>
  )
}
