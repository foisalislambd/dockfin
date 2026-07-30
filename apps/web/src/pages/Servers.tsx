import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import type { FormEvent } from 'react'
import { api } from '../lib/api'

export function ServersPage() {
  const qc = useQueryClient()
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers })
  const keys = useQuery({ queryKey: ['keys'], queryFn: api.keys })
  const [show, setShow] = useState(false)
  const [showKey, setShowKey] = useState(false)

  const create = useMutation({
    mutationFn: api.createServer,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['servers'] })
      setShow(false)
    },
  })
  const createKey = useMutation({
    mutationFn: ({ name, key }: { name: string; key: string }) => api.createKey(name, key),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['keys'] })
      setShowKey(false)
    },
  })

  return (
    <div className="space-y-6">
      <Header
        title="Servers"
        actions={
          <>
            <Btn onClick={() => setShowKey(true)}>Add SSH key</Btn>
            <Btn primary onClick={() => setShow(true)}>
              Add server
            </Btn>
          </>
        }
      />

      <div className="overflow-hidden rounded-xl border border-gray-200 dark:border-gray-800">
        <table className="w-full text-left text-sm">
          <thead className="panel-card bg-white dark:bg-white/3 text-gray-500 dark:text-gray-400">
            <tr>
              <th className="px-4 py-3 font-medium">Name</th>
              <th className="px-4 py-3 font-medium">Host</th>
              <th className="px-4 py-3 font-medium">Status</th>
              <th className="px-4 py-3 font-medium">Proxy</th>
              <th className="px-4 py-3 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {(servers.data?.servers || []).map((s) => (
              <tr key={s.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-4 py-3">
                  <Link
                    to="/servers/$serverId"
                    params={{ serverId: s.id }}
                    className="font-medium text-brand-600 hover:underline dark:text-brand-400"
                  >
                    {s.name}
                  </Link>
                </td>
                <td className="px-4 py-3 font-mono text-xs">
                  {s.user_name}@{s.ip}:{s.port}
                </td>
                <td className="px-4 py-3">
                  <Status ok={s.is_usable} label={s.is_usable ? `Docker ${s.docker_version || 'ok'}` : s.is_reachable ? 'No Docker' : 'Unreachable'} />
                </td>
                <td className="px-4 py-3">
                  <span className="text-xs text-gray-500">{s.proxy_type || 'traefik'}</span>
                  {s.proxy_status ? ` · ${s.proxy_status}` : ''}
                </td>
                <td className="px-4 py-3 space-x-2">
                  <Link
                    to="/servers/$serverId"
                    params={{ serverId: s.id }}
                    className="text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
                  >
                    Open
                  </Link>
                  <button
                    className="text-brand-600 dark:text-brand-400"
                    type="button"
                    onClick={() => void api.validateServer(s.id).then(() => qc.invalidateQueries({ queryKey: ['servers'] }))}
                  >
                    Validate
                  </button>
                  <button
                    className="text-brand-600 dark:text-brand-400"
                    type="button"
                    disabled={s.proxy_type === 'none'}
                    onClick={() => void api.startProxy(s.id).then(() => qc.invalidateQueries({ queryKey: ['servers'] }))}
                  >
                    Start proxy
                  </button>
                </td>
              </tr>
            ))}
            {!servers.data?.servers?.length && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  No servers yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {show && (
        <Modal title="Add server" onClose={() => setShow(false)}>
          <ServerForm
            keys={keys.data?.private_keys || []}
            onSubmit={(body) => create.mutate(body)}
            error={create.error?.message}
          />
        </Modal>
      )}
      {showKey && (
        <Modal title="Add SSH key" onClose={() => setShowKey(false)}>
          <KeyForm onSubmit={(name, key) => createKey.mutate({ name, key })} error={createKey.error?.message} />
        </Modal>
      )}
    </div>
  )
}

function ServerForm({
  keys,
  onSubmit,
  error,
}: {
  keys: { id: string; name: string }[]
  onSubmit: (b: {
    name: string
    ip: string
    port: number
    user_name: string
    private_key_id?: string
    proxy_type?: string
  }) => void
  error?: string
}) {
  const [name, setName] = useState('')
  const [ip, setIp] = useState('')
  const [port, setPort] = useState(22)
  const [user, setUser] = useState('root')
  const [keyId, setKeyId] = useState(keys[0]?.id || '')
  const [proxyType, setProxyType] = useState('traefik')
  return (
    <form
      className="space-y-3"
      onSubmit={(e: FormEvent) => {
        e.preventDefault()
        onSubmit({
          name,
          ip,
          port,
          user_name: user,
          private_key_id: keyId || undefined,
          proxy_type: proxyType,
        })
      }}
    >
      <Input label="Name" value={name} onChange={setName} />
      <Input label="IP / hostname" value={ip} onChange={setIp} />
      <Input label="SSH user" value={user} onChange={setUser} />
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">Port</span>
        <input
          type="number"
          value={port}
          onChange={(e) => setPort(Number(e.target.value))}
          className="w-full rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2"
        />
      </label>
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">Proxy</span>
        <select
          value={proxyType}
          onChange={(e) => setProxyType(e.target.value)}
          className="w-full rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2"
        >
          <option value="traefik">Traefik</option>
          <option value="caddy">Caddy</option>
          <option value="none">None</option>
        </select>
      </label>
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">SSH key</span>
        <select
          value={keyId}
          onChange={(e) => setKeyId(e.target.value)}
          className="w-full rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2"
        >
          <option value="">None</option>
          {keys.map((k) => (
            <option key={k.id} value={k.id}>
              {k.name}
            </option>
          ))}
        </select>
      </label>
      {error && <p className="text-sm text-error-500">{error}</p>}
      <Btn primary type="submit">
        Create
      </Btn>
    </form>
  )
}

function KeyForm({ onSubmit, error }: { onSubmit: (name: string, key: string) => void; error?: string }) {
  const [name, setName] = useState('')
  const [key, setKey] = useState('')
  return (
    <form
      className="space-y-3"
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit(name, key)
      }}
    >
      <Input label="Name" value={name} onChange={setName} />
      <label className="block text-sm">
        <span className="mb-1 block text-gray-500 dark:text-gray-400">Private key (PEM)</span>
        <textarea
          required
          rows={6}
          value={key}
          onChange={(e) => setKey(e.target.value)}
          className="w-full rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2 font-mono text-xs"
        />
      </label>
      {error && <p className="text-sm text-error-500">{error}</p>}
      <Btn primary type="submit">
        Save key
      </Btn>
    </form>
  )
}

export function Header({
  title,
  actions,
}: {
  title: string
  subtitle?: string
  actions?: React.ReactNode
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">{title}</h1>
      </div>
      <div className="flex gap-2">{actions}</div>
    </div>
  )
}

export function Btn({
  children,
  onClick,
  primary,
  type = 'button',
}: {
  children: React.ReactNode
  onClick?: () => void
  primary?: boolean
  type?: 'button' | 'submit'
}) {
  return (
    <button
      type={type}
      onClick={onClick}
      className={`inline-flex h-8 items-center rounded-md px-2.5 text-xs font-medium transition ${
        primary
          ? 'bg-brand-500 text-white hover:bg-brand-600'
          : 'border border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-white/5'
      }`}
    >
      {children}
    </button>
  )
}

export function Modal({
  title,
  children,
  onClose,
}: {
  title: string
  children: React.ReactNode
  onClose: () => void
}) {
  return (
    <div className="fixed inset-0 z-40 grid place-items-center bg-black/50 p-4" onClick={onClose}>
      <div
        className="w-full max-w-md rounded-xl border border-gray-200 dark:border-gray-800 panel-card bg-white dark:bg-white/3 p-5"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-medium">{title}</h3>
          <button type="button" onClick={onClose} className="text-gray-500 dark:text-gray-400">
            ✕
          </button>
        </div>
        {children}
      </div>
    </div>
  )
}

export function Input({
  label,
  value,
  onChange,
  required = true,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  required?: boolean
}) {
  return (
    <label className="block text-sm">
      <span className="mb-1 block text-gray-500 dark:text-gray-400">{label}</span>
      <input
        required={required}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-3 py-2 outline-none focus:ring-1 focus:ring-brand-500"
      />
    </label>
  )
}

export function Status({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span className={`inline-flex items-center gap-1.5 ${ok ? 'text-brand-600 dark:text-brand-400' : 'text-[var(--color-warn)]'}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${ok ? 'bg-brand-500' : 'bg-[var(--color-warn)]'}`} />
      {label}
    </span>
  )
}
