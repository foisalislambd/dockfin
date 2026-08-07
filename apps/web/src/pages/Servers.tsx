import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { CreatePageShell, FormActions, FormInput, FormSelect } from '../components/ui/forms'
import { PageSkeleton } from '../components/ui/Skeleton'
import { api } from '../lib/api'

export { Btn } from '../components/ui/Button'
export { Header } from '../components/ui/Header'
export { Input } from '../components/ui/Input'
export { Modal } from '../components/ui/Modal'

import { Btn } from '../components/ui/Button'
import { Header } from '../components/ui/Header'
import { Input } from '../components/ui/Input'
import { Modal } from '../components/ui/Modal'

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
  const [mode, setMode] = useState<'ssh' | 'cloud'>('ssh')
  return (
    <CreatePageShell title="Add server" backTo="/servers" backLabel="Back to Servers">
      <div className="mb-5 flex gap-2">
        {(
          [
            ['ssh', 'Existing server (SSH)'],
            ['cloud', 'From cloud provider'],
          ] as const
        ).map(([id, label]) => (
          <button
            key={id}
            type="button"
            onClick={() => setMode(id)}
            className={`inline-flex h-8 items-center rounded-md px-2.5 text-xs font-medium ${
              mode === id
                ? 'bg-brand-500 text-white'
                : 'border border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-white/5'
            }`}
          >
            {label}
          </button>
        ))}
      </div>
      {mode === 'ssh' ? <SshServerForm /> : <CloudServerForm />}
    </CreatePageShell>
  )
}

function SshServerForm() {
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
  )
}

function CloudServerForm() {
  const qc = useQueryClient()
  const nav = useNavigate()
  const keys = useQuery({ queryKey: ['keys'], queryFn: api.keys })
  const tokens = useQuery({ queryKey: ['cloud-tokens'], queryFn: api.cloudTokens })
  const scripts = useQuery({ queryKey: ['cloud-init-scripts'], queryFn: api.cloudInitScripts })
  const [tokenId, setTokenId] = useState('')
  const [name, setName] = useState('')
  const [region, setRegion] = useState('')
  const [size, setSize] = useState('')
  const [image, setImage] = useState('')
  const [keyId, setKeyId] = useState('')
  const [scriptId, setScriptId] = useState('')
  const [proxyType, setProxyType] = useState('traefik')

  useEffect(() => {
    if (!tokenId && tokens.data?.cloud_tokens?.[0]?.id) setTokenId(tokens.data.cloud_tokens[0].id)
  }, [tokens.data, tokenId])
  useEffect(() => {
    if (!keyId && keys.data?.private_keys?.[0]?.id) setKeyId(keys.data.private_keys[0].id)
  }, [keys.data, keyId])

  const defaults = useQuery({
    queryKey: ['cloud-defaults', tokenId],
    queryFn: () => api.cloudProviderDefaults(tokenId),
    enabled: Boolean(tokenId),
  })

  const provision = useMutation({
    mutationFn: () =>
      api.provisionServer({
        cloud_token_id: tokenId,
        name,
        region: region || undefined,
        size: size || undefined,
        image: image || undefined,
        private_key_id: keyId,
        cloud_init_script_id: scriptId || undefined,
        proxy_type: proxyType,
      }),
    onSuccess: (res) => {
      void qc.invalidateQueries({ queryKey: ['servers'] })
      void nav({ to: '/servers/$serverId', params: { serverId: res.server.id } })
    },
  })

  if (!tokens.isLoading && !tokens.data?.cloud_tokens?.length) {
    return (
      <p className="text-sm text-gray-500 dark:text-gray-400">
        No cloud provider tokens yet. Add a Hetzner, DigitalOcean, or Vultr API token under{' '}
        <Link to="/security" className="text-brand-600 dark:text-brand-400">
          Security → Cloud tokens
        </Link>
        .
      </p>
    )
  }

  return (
    <form
      className="space-y-4"
      onSubmit={(e: FormEvent) => {
        e.preventDefault()
        provision.mutate()
      }}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <FormSelect label="Cloud token" value={tokenId} onChange={setTokenId}>
          {(tokens.data?.cloud_tokens || []).map((t) => (
            <option key={t.id} value={t.id}>
              {t.name} ({t.provider})
            </option>
          ))}
        </FormSelect>
        <FormInput label="Name" value={name} onChange={setName} />
        <FormInput
          label="Region / location"
          value={region}
          onChange={setRegion}
          required={false}
          placeholder={defaults.data?.region}
        />
        <FormInput
          label="Size / plan"
          value={size}
          onChange={setSize}
          required={false}
          placeholder={defaults.data?.size}
        />
        <FormInput
          label="Image"
          value={image}
          onChange={setImage}
          required={false}
          placeholder={defaults.data?.image}
        />
        <FormSelect label="SSH key" value={keyId} onChange={setKeyId}>
          {(keys.data?.private_keys || []).map((k) => (
            <option key={k.id} value={k.id}>
              {k.name}
            </option>
          ))}
        </FormSelect>
        <FormSelect label="Cloud-init script" value={scriptId} onChange={setScriptId} required={false}>
          <option value="">Default (install Docker)</option>
          {(scripts.data?.cloud_init_scripts || []).map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </FormSelect>
        <FormSelect label="Proxy" value={proxyType} onChange={setProxyType}>
          <option value="traefik">Traefik</option>
          <option value="caddy">Caddy</option>
          <option value="none">None</option>
        </FormSelect>
      </div>
      <p className="text-xs text-gray-500 dark:text-gray-400">
        The selected SSH key is uploaded to the provider if missing, the instance boots with
        cloud-init, and Dockfin registers it as <code>root@IP:22</code> once a public IP is
        assigned. Leave region/size/image empty to use the provider defaults shown as placeholders.
      </p>
      {provision.error && <p className="text-sm text-error-500">{provision.error.message}</p>}
      <FormActions busy={provision.isPending} submitLabel="Provision server" cancelTo="/servers" />
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
