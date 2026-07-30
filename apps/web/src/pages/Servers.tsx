import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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
        subtitle="SSH-managed Docker hosts. No agent required."
        actions={
          <>
            <Btn onClick={() => setShowKey(true)}>Add SSH key</Btn>
            <Btn primary onClick={() => setShow(true)}>
              Add server
            </Btn>
          </>
        }
      />

      <div className="overflow-hidden rounded-xl border border-[var(--color-line)]">
        <table className="w-full text-left text-sm">
          <thead className="bg-[var(--color-panel)] text-[var(--color-muted)]">
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
              <tr key={s.id} className="border-t border-[var(--color-line)]">
                <td className="px-4 py-3">{s.name}</td>
                <td className="px-4 py-3 font-mono text-xs">
                  {s.user_name}@{s.ip}:{s.port}
                </td>
                <td className="px-4 py-3">
                  <Status ok={s.is_usable} label={s.is_usable ? `Docker ${s.docker_version || 'ok'}` : s.is_reachable ? 'No Docker' : 'Unreachable'} />
                </td>
                <td className="px-4 py-3">{s.proxy_status}</td>
                <td className="px-4 py-3 space-x-2">
                  <button
                    className="text-[var(--color-accent)]"
                    type="button"
                    onClick={() => void api.validateServer(s.id).then(() => qc.invalidateQueries({ queryKey: ['servers'] }))}
                  >
                    Validate
                  </button>
                  <button
                    className="text-[var(--color-accent)]"
                    type="button"
                    onClick={() => void api.startProxy(s.id).then(() => qc.invalidateQueries({ queryKey: ['servers'] }))}
                  >
                    Start proxy
                  </button>
                </td>
              </tr>
            ))}
            {!servers.data?.servers?.length && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-[var(--color-muted)]">
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
  onSubmit: (b: { name: string; ip: string; port: number; user_name: string; private_key_id?: string }) => void
  error?: string
}) {
  const [name, setName] = useState('')
  const [ip, setIp] = useState('')
  const [port, setPort] = useState(22)
  const [user, setUser] = useState('root')
  const [keyId, setKeyId] = useState(keys[0]?.id || '')
  return (
    <form
      className="space-y-3"
      onSubmit={(e: FormEvent) => {
        e.preventDefault()
        onSubmit({ name, ip, port, user_name: user, private_key_id: keyId || undefined })
      }}
    >
      <Input label="Name" value={name} onChange={setName} />
      <Input label="IP / hostname" value={ip} onChange={setIp} />
      <Input label="SSH user" value={user} onChange={setUser} />
      <label className="block text-sm">
        <span className="mb-1 block text-[var(--color-muted)]">Port</span>
        <input
          type="number"
          value={port}
          onChange={(e) => setPort(Number(e.target.value))}
          className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2"
        />
      </label>
      <label className="block text-sm">
        <span className="mb-1 block text-[var(--color-muted)]">SSH key</span>
        <select
          value={keyId}
          onChange={(e) => setKeyId(e.target.value)}
          className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2"
        >
          <option value="">None</option>
          {keys.map((k) => (
            <option key={k.id} value={k.id}>
              {k.name}
            </option>
          ))}
        </select>
      </label>
      {error && <p className="text-sm text-[var(--color-danger)]">{error}</p>}
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
        <span className="mb-1 block text-[var(--color-muted)]">Private key (PEM)</span>
        <textarea
          required
          rows={6}
          value={key}
          onChange={(e) => setKey(e.target.value)}
          className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2 font-mono text-xs"
        />
      </label>
      {error && <p className="text-sm text-[var(--color-danger)]">{error}</p>}
      <Btn primary type="submit">
        Save key
      </Btn>
    </form>
  )
}

export function Header({
  title,
  subtitle,
  actions,
}: {
  title: string
  subtitle: string
  actions?: React.ReactNode
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 className="text-3xl font-semibold tracking-tight">{title}</h1>
        <p className="mt-2 text-[var(--color-muted)]">{subtitle}</p>
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
      className={`rounded-lg px-3 py-2 text-sm font-medium transition ${
        primary
          ? 'bg-[var(--color-accent)] text-[var(--color-ink)] hover:bg-[var(--color-accent-2)]'
          : 'border border-[var(--color-line)] hover:border-[var(--color-accent)]'
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
        className="w-full max-w-md rounded-xl border border-[var(--color-line)] bg-[var(--color-panel)] p-5"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-medium">{title}</h3>
          <button type="button" onClick={onClose} className="text-[var(--color-muted)]">
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
      <span className="mb-1 block text-[var(--color-muted)]">{label}</span>
      <input
        required={required}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2 outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
      />
    </label>
  )
}

export function Status({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span className={`inline-flex items-center gap-1.5 ${ok ? 'text-[var(--color-accent)]' : 'text-[var(--color-warn)]'}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${ok ? 'bg-[var(--color-accent)]' : 'bg-[var(--color-warn)]'}`} />
      {label}
    </span>
  )
}
