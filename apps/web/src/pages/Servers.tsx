import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import type { FormEvent } from 'react'
import { CreatePageShell, FormActions, FormInput, FormSelect } from '../components/ui/forms'
import { PageSkeleton } from '../components/ui/Skeleton'
import { api } from '../lib/api'

export function ServersPage() {
  const qc = useQueryClient()
  const servers = useQuery({ queryKey: ['servers'], queryFn: api.servers })
  const [showKey, setShowKey] = useState(false)

  const createKey = useMutation({
    mutationFn: ({ name, key }: { name: string; key: string }) => api.createKey(name, key),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['keys'] })
      setShowKey(false)
    },
  })

  if (servers.isLoading) return <PageSkeleton cards={2} />

  return (
    <div className="space-y-6">
      <Header
        title="Servers"
        actions={
          <>
            <Btn onClick={() => setShowKey(true)}>Add SSH key</Btn>
            <Link
              to="/servers/new"
              className="inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white hover:bg-brand-600"
            >
              Add server
            </Link>
          </>
        }
      />

      <div className="panel-card overflow-hidden">
        <table className="panel-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Host</th>
              <th>Status</th>
              <th>Proxy</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {(servers.data?.servers || []).map((s) => (
              <tr key={s.id}>
                <td>
                  <Link
                    to="/servers/$serverId"
                    params={{ serverId: s.id }}
                    className="font-medium text-brand-600 hover:underline dark:text-brand-400"
                  >
                    {s.name}
                  </Link>
                </td>
                <td className="font-mono text-xs text-gray-600 dark:text-gray-400">
                  {s.user_name}@{s.ip}:{s.port}
                </td>
                <td>
                  <Status ok={s.is_usable} label={s.is_usable ? `Docker ${s.docker_version || 'ok'}` : s.is_reachable ? 'No Docker' : 'Unreachable'} />
                </td>
                <td className="text-xs text-gray-500 dark:text-gray-400">
                  {s.proxy_type || 'traefik'}
                  {s.proxy_status ? ` · ${s.proxy_status}` : ''}
                </td>
                <td className="space-x-2">
                  <Link
                    to="/servers/$serverId"
                    params={{ serverId: s.id }}
                    className="text-gray-500 hover:text-brand-600 dark:text-gray-400 dark:hover:text-brand-400"
                  >
                    Open
                  </Link>
                  <button
                    className="text-brand-600 hover:underline dark:text-brand-400"
                    type="button"
                    onClick={() => void api.validateServer(s.id).then(() => qc.invalidateQueries({ queryKey: ['servers'] }))}
                  >
                    Validate
                  </button>
                  <button
                    className="text-brand-600 hover:underline dark:text-brand-400 disabled:opacity-40"
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
                <td colSpan={5} className="panel-table-empty">
                  No servers yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {showKey && (
        <Modal title="Add SSH key" onClose={() => setShowKey(false)}>
          <KeyForm onSubmit={(name, key) => createKey.mutate({ name, key })} error={createKey.error?.message} />
        </Modal>
      )}
    </div>
  )
}

export function CreateServerPage() {
  const qc = useQueryClient()
  const nav = useNavigate()
  const keys = useQuery({ queryKey: ['keys'], queryFn: api.keys })
  const [name, setName] = useState('')
  const [ip, setIp] = useState('')
  const [port, setPort] = useState('22')
  const [user, setUser] = useState('root')
  const [keyId, setKeyId] = useState('')
  const [proxyType, setProxyType] = useState('traefik')

  useEffect(() => {
    if (!keyId && keys.data?.private_keys?.[0]?.id) {
      setKeyId(keys.data.private_keys[0].id)
    }
  }, [keys.data, keyId])

  const create = useMutation({
    mutationFn: api.createServer,
    onSuccess: (server) => {
      void qc.invalidateQueries({ queryKey: ['servers'] })
      void nav({ to: '/servers/$serverId', params: { serverId: server.id } })
    },
  })

  return (
    <CreatePageShell title="Add server" backTo="/servers" backLabel="Back to Servers">
      <form
        className="space-y-4"
        onSubmit={(e: FormEvent) => {
          e.preventDefault()
          create.mutate({
            name,
            ip,
            port: Number(port) || 22,
            user_name: user,
            private_key_id: keyId || undefined,
            proxy_type: proxyType,
          })
        }}
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <FormInput label="Name" value={name} onChange={setName} />
          <FormInput label="IP / hostname" value={ip} onChange={setIp} />
          <FormInput label="SSH user" value={user} onChange={setUser} />
          <FormInput label="Port" value={port} onChange={setPort} type="number" />
          <FormSelect label="Proxy" value={proxyType} onChange={setProxyType}>
            <option value="traefik">Traefik</option>
            <option value="caddy">Caddy</option>
            <option value="none">None</option>
          </FormSelect>
          <FormSelect label="SSH key" value={keyId} onChange={setKeyId} required={false}>
            <option value="">None</option>
            {(keys.data?.private_keys || []).map((k) => (
              <option key={k.id} value={k.id}>
                {k.name}
              </option>
            ))}
          </FormSelect>
        </div>
        {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
        <FormActions busy={create.isPending} submitLabel="Create" cancelTo="/servers" />
      </form>
    </CreatePageShell>
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
          className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
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
  disabled,
}: {
  children: React.ReactNode
  onClick?: () => void
  primary?: boolean
  type?: 'button' | 'submit'
  disabled?: boolean
}) {
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      className={`inline-flex h-8 items-center rounded-md px-2.5 text-xs font-medium transition disabled:cursor-not-allowed disabled:opacity-50 ${
        primary
          ? 'bg-brand-500 text-white hover:bg-brand-600'
          : 'border border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-gray-800 dark:text-gray-300 dark:hover:bg-white/5'
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
  wide,
}: {
  title: string
  children: React.ReactNode
  onClose: () => void
  wide?: boolean
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      window.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [onClose])

  // Portal above the shell so the overlay covers the sidebar (z-50) too.
  return createPortal(
    <div className="panel-modal-backdrop" onClick={onClose} role="presentation">
      <div
        className={`panel-modal w-full p-5 ${wide ? 'max-w-xl' : 'max-w-md'}`}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-medium text-gray-900 dark:text-white">{title}</h3>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-white/10"
          >
            ✕
          </button>
        </div>
        {children}
      </div>
    </div>,
    document.body,
  )
}

export function Input({
  label,
  value,
  onChange,
  onBlur,
  required = true,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  onBlur?: () => void
  required?: boolean
}) {
  return (
    <label className="block text-sm">
      <span className="mb-1 block text-gray-500 dark:text-gray-400">{label}</span>
      <input
        required={required}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onBlur={() => onBlur?.()}
        className="panel-field w-full rounded-lg px-3 py-2 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
      />
    </label>
  )
}

export function Status({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 ${
        ok ? 'text-success-600 dark:text-success-500' : 'text-warning-500'
      }`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${ok ? 'bg-success-500' : 'bg-warning-500'}`} />
      {label}
    </span>
  )
}
