import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'
import { PageSkeleton } from '../components/ui/Skeleton'
import { ResourceTabs, TabPanel } from '../components/ui/tabs'
import {
  api,
  type CloudInitScript,
  type CloudProviderToken,
  type Key,
} from '../lib/api'
import { Btn, Header, Input, Modal } from './Servers'

const TABS = [
  { id: 'private-keys', label: 'Private Keys' },
  { id: 'cloud-tokens', label: 'Cloud Tokens' },
  { id: 'cloud-init', label: 'Cloud-Init Scripts' },
  { id: 'api-tokens', label: 'API Tokens' },
] as const

type TabId = (typeof TABS)[number]['id']

const API_ABILITIES = [
  { id: 'root', label: 'root', help: 'Root access, be careful!' },
  { id: 'write', label: 'write', help: 'Write access to all resources.' },
  { id: 'deploy', label: 'deploy', help: 'Can trigger deploy webhooks.' },
  { id: 'read', label: 'read', help: 'Read access to resources.' },
  { id: 'read:sensitive', label: 'read:sensitive', help: 'Includes secrets, logs, passwords.' },
] as const

const EXPIRY_OPTIONS: { days: number | null; label: string }[] = [
  { days: 7, label: '7 days' },
  { days: 30, label: '30 days' },
  { days: 60, label: '60 days' },
  { days: 90, label: '90 days' },
  { days: 365, label: '1 year' },
  { days: null, label: 'Never' },
]

function isTabId(v: string): v is TabId {
  return TABS.some((t) => t.id === v)
}

export function SecurityPage({ initialTab }: { initialTab?: TabId }) {
  const nav = useNavigate()
  const search = useSearch({ strict: false }) as { tab?: string }
  const tabFromUrl = search.tab && isTabId(search.tab) ? search.tab : undefined
  const [tab, setTab] = useState<TabId>(initialTab || tabFromUrl || 'private-keys')

  useEffect(() => {
    if (initialTab) setTab(initialTab)
    else if (tabFromUrl) setTab(tabFromUrl)
  }, [initialTab, tabFromUrl])

  const changeTab = (id: string) => {
    if (!isTabId(id)) return
    setTab(id)
    void nav({
      to: '/security',
      search: id === 'private-keys' ? {} : { tab: id },
      replace: true,
    })
  }

  return (
    <div className="space-y-6">
      <Header title="Security" />
      <ResourceTabs tabs={[...TABS]} active={tab} onChange={changeTab} />
      {tab === 'private-keys' && (
        <TabPanel>
          <PrivateKeysPanel />
        </TabPanel>
      )}
      {tab === 'cloud-tokens' && (
        <TabPanel>
          <CloudTokensPanel />
        </TabPanel>
      )}
      {tab === 'cloud-init' && (
        <TabPanel>
          <CloudInitPanel />
        </TabPanel>
      )}
      {tab === 'api-tokens' && (
        <TabPanel>
          <ApiTokensPanel />
        </TabPanel>
      )}
    </div>
  )
}

/** @deprecated use SecurityPage */
export function PrivateKeysPage() {
  return <SecurityPage initialTab="private-keys" />
}

/** @deprecated use SecurityPage */
export function ApiTokensPage() {
  return <SecurityPage initialTab="api-tokens" />
}

function PrivateKeysPanel() {
  const qc = useQueryClient()
  const keys = useQuery({ queryKey: ['keys'], queryFn: api.keys })
  const [addOpen, setAddOpen] = useState(false)
  const [addMenu, setAddMenu] = useState(false)
  const [selected, setSelected] = useState<Key | null>(null)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [pem, setPem] = useState('')

  const create = useMutation({
    mutationFn: () => api.createKey(name, pem, description),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['keys'] })
      setAddOpen(false)
      setName('')
      setDescription('')
      setPem('')
    },
  })
  const generate = useMutation({
    mutationFn: (type: 'ed25519' | 'rsa') => api.generateKey(type),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['keys'] }),
  })
  const cleanup = useMutation({
    mutationFn: () => api.cleanupUnusedKeys(),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['keys'] }),
  })
  const update = useMutation({
    mutationFn: () =>
      api.updateKey(selected!.id, name, description),
    onSuccess: (k) => {
      setSelected(k)
      void qc.invalidateQueries({ queryKey: ['keys'] })
    },
  })
  const remove = useMutation({
    mutationFn: () => api.deleteKey(selected!.id),
    onSuccess: () => {
      setSelected(null)
      void qc.invalidateQueries({ queryKey: ['keys'] })
    },
  })

  useEffect(() => {
    if (!selected) return
    setName(selected.name)
    setDescription(selected.description || '')
  }, [selected])

  if (keys.isLoading) return <PageSkeleton cards={2} />

  const list = keys.data?.private_keys || []

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Private Keys</h2>
        <div className="relative">
          <Btn
            primary
            type="button"
            onClick={() => setAddMenu((v) => !v)}
          >
            + Add
          </Btn>
          {addMenu && (
            <div className="absolute left-0 z-20 mt-1 min-w-[200px] rounded-lg border border-gray-200 bg-white p-1 shadow-lg dark:border-gray-700 dark:bg-gray-900">
              <button
                type="button"
                className="block w-full rounded-md px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-white/5"
                onClick={() => {
                  setAddMenu(false)
                  generate.mutate('ed25519')
                }}
              >
                Generate ED25519
              </button>
              <button
                type="button"
                className="block w-full rounded-md px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-white/5"
                onClick={() => {
                  setAddMenu(false)
                  generate.mutate('rsa')
                }}
              >
                Generate RSA
              </button>
              <button
                type="button"
                className="block w-full rounded-md px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-white/5"
                onClick={() => {
                  setAddMenu(false)
                  setAddOpen(true)
                }}
              >
                Add manually
              </button>
            </div>
          )}
        </div>
        <Btn
          type="button"
          disabled={cleanup.isPending || !list.some((k) => !k.in_use)}
          onClick={() => {
            if (confirm('Delete all unused SSH keys? This cannot be undone.')) cleanup.mutate()
          }}
        >
          {cleanup.isPending ? 'Cleaning…' : 'Delete unused SSH Keys'}
        </Btn>
      </div>
      {(generate.error || cleanup.error) && (
        <p className="text-sm text-error-500">
          {(generate.error || cleanup.error)?.message}
        </p>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        {list.map((k) => (
          <button
            key={k.id}
            type="button"
            onClick={() => setSelected(k)}
            className="panel-card p-5 text-left transition hover:border-brand-400"
          >
            <div className="font-medium text-gray-900 dark:text-white">{k.name}</div>
            <div className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {k.description || 'No description'}
              {!k.in_use && (
                <span className="ml-2 inline-flex items-center rounded bg-amber-400 px-1.5 py-0.5 text-xs font-medium text-black">
                  Unused
                </span>
              )}
            </div>
          </button>
        ))}
        {!list.length && (
          <p className="text-sm text-gray-500 dark:text-gray-400">No private keys found.</p>
        )}
      </div>

      {addOpen && (
        <Modal title="Add Private Key Manually" onClose={() => setAddOpen(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Private Keys are used to connect to your servers without passwords. Do not use
              passphrase-protected keys.
            </p>
            <Input label="Name" value={name} onChange={setName} />
            <Input label="Description" value={description} onChange={setDescription} required={false} />
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Private Key</span>
              <textarea
                required
                rows={6}
                value={pem}
                onChange={(e) => setPem(e.target.value)}
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
              />
            </label>
            {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
            <Btn primary type="submit" disabled={create.isPending}>
              Continue
            </Btn>
          </form>
        </Modal>
      )}

      {selected && (
        <Modal title={selected.name} onClose={() => setSelected(null)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              update.mutate()
            }}
          >
            <Input label="Name" value={name} onChange={setName} />
            <Input label="Description" value={description} onChange={setDescription} required={false} />
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Public Key</span>
              <textarea
                readOnly
                rows={3}
                value={selected.public_key}
                className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
              />
            </label>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Fingerprint: <span className="font-mono">{selected.fingerprint}</span>
            </p>
            {(update.error || remove.error) && (
              <p className="text-sm text-error-500">
                {(update.error || remove.error)?.message}
              </p>
            )}
            <div className="flex flex-wrap gap-2">
              <Btn primary type="submit" disabled={update.isPending}>
                Save
              </Btn>
              <Btn
                type="button"
                disabled={!!selected.in_use || remove.isPending}
                onClick={() => {
                  if (confirm(`Delete private key "${selected.name}"?`)) remove.mutate()
                }}
              >
                {selected.in_use ? 'In use' : 'Delete'}
              </Btn>
            </div>
          </form>
        </Modal>
      )}
    </div>
  )
}

function CloudTokensPanel() {
  const qc = useQueryClient()
  const tokens = useQuery({ queryKey: ['cloud-tokens'], queryFn: api.cloudTokens })
  const [addOpen, setAddOpen] = useState(false)
  const [addMenu, setAddMenu] = useState(false)
  const [provider, setProvider] = useState<'hetzner' | 'digitalocean' | 'vultr'>('hetzner')
  const [selected, setSelected] = useState<CloudProviderToken | null>(null)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [token, setToken] = useState('')
  const [msg, setMsg] = useState('')

  const create = useMutation({
    mutationFn: () =>
      api.createCloudToken({ provider, name, description, token }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['cloud-tokens'] })
      setAddOpen(false)
      setName('')
      setDescription('')
      setToken('')
    },
  })
  const update = useMutation({
    mutationFn: () =>
      api.updateCloudToken(selected!.id, {
        name,
        description,
        token: token || undefined,
      }),
    onSuccess: (t) => {
      setSelected(t)
      setToken('')
      setMsg('Saved.')
      void qc.invalidateQueries({ queryKey: ['cloud-tokens'] })
    },
  })
  const validate = useMutation({
    mutationFn: () => api.validateCloudToken(selected!.id),
    onSuccess: () => setMsg('Token is valid.'),
    onError: (e: Error) => setMsg(e.message),
  })
  const remove = useMutation({
    mutationFn: () => api.deleteCloudToken(selected!.id),
    onSuccess: () => {
      setSelected(null)
      void qc.invalidateQueries({ queryKey: ['cloud-tokens'] })
    },
  })

  useEffect(() => {
    if (!selected) return
    setName(selected.name)
    setDescription(selected.description || '')
    setToken('')
    setMsg('')
  }, [selected])

  if (tokens.isLoading) return <PageSkeleton cards={2} />
  const list = tokens.data?.cloud_tokens || []

  const openAdd = (p: 'hetzner' | 'digitalocean' | 'vultr') => {
    setProvider(p)
    setAddMenu(false)
    setAddOpen(true)
    setName('')
    setDescription('')
    setToken('')
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <div className="min-w-0 flex-1">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            Cloud Provider Tokens
          </h2>
        </div>
        <div className="relative">
          <Btn primary type="button" onClick={() => setAddMenu((v) => !v)}>
            + Add
          </Btn>
          {addMenu && (
            <div className="absolute right-0 z-20 mt-1 min-w-[180px] rounded-lg border border-gray-200 bg-white p-1 shadow-lg dark:border-gray-700 dark:bg-gray-900">
              {(['hetzner', 'digitalocean', 'vultr'] as const).map((p) => (
                <button
                  key={p}
                  type="button"
                  className="block w-full rounded-md px-3 py-2 text-left text-sm capitalize hover:bg-gray-100 dark:hover:bg-white/5"
                  onClick={() => openAdd(p)}
                >
                  {p === 'digitalocean' ? 'DigitalOcean' : p[0].toUpperCase() + p.slice(1)}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        {list.map((t) => (
          <button
            key={t.id}
            type="button"
            onClick={() => setSelected(t)}
            className="panel-card p-5 text-left transition hover:border-brand-400"
          >
            <div className="font-medium text-gray-900 dark:text-white">{t.name}</div>
            <div className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              <span className="uppercase">{t.provider}</span>
              {t.description ? ` · ${t.description}` : ''}
            </div>
          </button>
        ))}
        {!list.length && (
          <p className="text-sm text-gray-500 dark:text-gray-400">
            No cloud provider tokens found.
          </p>
        )}
      </div>

      {addOpen && (
        <Modal
          title={`Add ${provider === 'digitalocean' ? 'DigitalOcean' : provider[0].toUpperCase() + provider.slice(1)} Token`}
          onClose={() => setAddOpen(false)}
        >
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <Input label="Token Name" value={name} onChange={setName} />
            <Input label="Description" value={description} onChange={setDescription} required={false} />
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">API Token</span>
              <input
                type="password"
                required
                value={token}
                onChange={(e) => setToken(e.target.value)}
                className="panel-field w-full rounded-lg px-3 py-2"
              />
            </label>
            {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
            <Btn primary type="submit" disabled={create.isPending}>
              {create.isPending ? 'Validating…' : 'Validate & Add Token'}
            </Btn>
          </form>
        </Modal>
      )}

      {selected && (
        <Modal title={selected.name} onClose={() => setSelected(null)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              update.mutate()
            }}
          >
            <Input label="Name" value={name} onChange={setName} />
            <Input label="Description" value={description} onChange={setDescription} required={false} />
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Provider: <span className="uppercase">{selected.provider}</span>
            </p>
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">
                Rotate API Token (optional)
              </span>
              <input
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                className="panel-field w-full rounded-lg px-3 py-2"
                placeholder="Leave blank to keep current"
              />
            </label>
            {msg && <p className="text-sm text-success-600 dark:text-success-400">{msg}</p>}
            {(update.error || remove.error) && (
              <p className="text-sm text-error-500">
                {(update.error || remove.error)?.message}
              </p>
            )}
            <div className="flex flex-wrap gap-2">
              <Btn primary type="submit" disabled={update.isPending}>
                Save
              </Btn>
              <Btn type="button" onClick={() => validate.mutate()} disabled={validate.isPending}>
                Validate
              </Btn>
              <Btn
                type="button"
                disabled={remove.isPending}
                onClick={() => {
                  if (confirm(`Delete token "${selected.name}"?`)) remove.mutate()
                }}
              >
                Delete
              </Btn>
            </div>
          </form>
        </Modal>
      )}
    </div>
  )
}

function CloudInitPanel() {
  const qc = useQueryClient()
  const scripts = useQuery({ queryKey: ['cloud-init'], queryFn: api.cloudInitScripts })
  const [addOpen, setAddOpen] = useState(false)
  const [selected, setSelected] = useState<CloudInitScript | null>(null)
  const [name, setName] = useState('')
  const [script, setScript] = useState('')

  const create = useMutation({
    mutationFn: () => api.createCloudInitScript(name, script),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['cloud-init'] })
      setAddOpen(false)
      setName('')
      setScript('')
    },
  })
  const detail = useQuery({
    queryKey: ['cloud-init', selected?.id],
    queryFn: () => api.getCloudInitScript(selected!.id),
    enabled: !!selected,
  })
  const update = useMutation({
    mutationFn: () => api.updateCloudInitScript(selected!.id, name, script),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['cloud-init'] })
      void qc.invalidateQueries({ queryKey: ['cloud-init', selected!.id] })
    },
  })
  const remove = useMutation({
    mutationFn: () => api.deleteCloudInitScript(selected!.id),
    onSuccess: () => {
      setSelected(null)
      void qc.invalidateQueries({ queryKey: ['cloud-init'] })
    },
  })

  useEffect(() => {
    if (!detail.data) return
    setName(detail.data.name)
    setScript(detail.data.script || '')
  }, [detail.data])

  if (scripts.isLoading) return <PageSkeleton cards={2} />
  const list = scripts.data?.cloud_init_scripts || []

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <div className="flex-1">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Cloud-Init Scripts</h2>
        </div>
        <Btn primary type="button" onClick={() => setAddOpen(true)}>
          + Add
        </Btn>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        {list.map((s) => (
          <button
            key={s.id}
            type="button"
            onClick={() => setSelected(s)}
            className="panel-card p-5 text-left transition hover:border-brand-400"
          >
            <div className="font-medium text-gray-900 dark:text-white">{s.name}</div>
            <div className="mt-1 text-sm text-gray-500 dark:text-gray-400">Cloud-init script</div>
          </button>
        ))}
        {!list.length && (
          <p className="text-sm text-gray-500 dark:text-gray-400">
            No cloud-init scripts found. Create one to get started.
          </p>
        )}
      </div>

      {addOpen && (
        <Modal title="New Cloud-Init Script" onClose={() => setAddOpen(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <Input label="Script Name" value={name} onChange={setName} />
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Script Content</span>
              <textarea
                required
                rows={12}
                value={script}
                onChange={(e) => setScript(e.target.value)}
                placeholder={'#cloud-config\npackages:\n  - curl'}
                className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
              />
            </label>
            {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
            <Btn primary type="submit" disabled={create.isPending}>
              Create Script
            </Btn>
          </form>
        </Modal>
      )}

      {selected && (
        <Modal title={selected.name} onClose={() => setSelected(null)}>
          {detail.isLoading ? (
            <p className="text-sm text-gray-500">Loading…</p>
          ) : (
            <form
              className="space-y-3"
              onSubmit={(e) => {
                e.preventDefault()
                update.mutate()
              }}
            >
              <Input label="Script Name" value={name} onChange={setName} />
              <label className="block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">Script Content</span>
                <textarea
                  required
                  rows={12}
                  value={script}
                  onChange={(e) => setScript(e.target.value)}
                  className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
                />
              </label>
              {(update.error || remove.error) && (
                <p className="text-sm text-error-500">
                  {(update.error || remove.error)?.message}
                </p>
              )}
              <div className="flex flex-wrap gap-2">
                <Btn primary type="submit" disabled={update.isPending}>
                  Update Script
                </Btn>
                <Btn
                  type="button"
                  disabled={remove.isPending}
                  onClick={() => {
                    if (confirm(`Delete script "${selected.name}"?`)) remove.mutate()
                  }}
                >
                  Delete
                </Btn>
              </div>
            </form>
          )}
        </Modal>
      )}
    </div>
  )
}

function ApiTokensPanel() {
  const qc = useQueryClient()
  const settings = useQuery({ queryKey: ['instance-settings'], queryFn: api.instanceSettings })
  const tokens = useQuery({ queryKey: ['api-tokens'], queryFn: api.apiTokens })
  const [name, setName] = useState('')
  const [expiresIn, setExpiresIn] = useState<number | null>(30)
  const [abilities, setAbilities] = useState<string[]>(['read'])
  const [plain, setPlain] = useState<string | null>(null)
  const [filter, setFilter] = useState('')

  const apiEnabled = settings.data?.settings.is_api_enabled !== false

  const create = useMutation({
    mutationFn: () =>
      api.createApiToken(
        name,
        abilities.includes('root') ? ['root'] : abilities,
        expiresIn ?? undefined,
      ),
    onSuccess: (data) => {
      setPlain(data.token)
      setName('')
      setAbilities(['read'])
      setExpiresIn(30)
      void qc.invalidateQueries({ queryKey: ['api-tokens'] })
    },
  })

  const filtered = useMemo(() => {
    const list = tokens.data?.api_tokens || []
    const q = filter.trim().toLowerCase()
    if (!q) return list
    return list.filter((t) => t.name.toLowerCase().includes(q))
  }, [tokens.data, filter])

  const toggleAbility = (id: string) => {
    if (id === 'root') {
      setAbilities((prev) => (prev.includes('root') ? ['read'] : ['root']))
      return
    }
    setAbilities((prev) => {
      const withoutRoot = prev.filter((a) => a !== 'root')
      if (withoutRoot.includes(id)) {
        const next = withoutRoot.filter((a) => a !== id)
        return next.length ? next : ['read']
      }
      return [...withoutRoot, id]
    })
  }

  if (tokens.isLoading || settings.isLoading) return <PageSkeleton cards={2} />

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white">API Tokens</h2>
        {!apiEnabled ? (
          <p className="mt-1 text-sm text-amber-600 dark:text-amber-400">
            API is disabled. Enable it in Settings → Advanced if you want to use the API.
          </p>
        ) : (
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Tokens are created with the current team as scope. Use{' '}
            <code className="font-mono text-xs">Authorization: Bearer dfin_…</code>
          </p>
        )}
      </div>

      {apiEnabled && (
        <div className="panel-card space-y-4 p-5">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">New Token</h3>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <div className="flex flex-wrap items-end gap-3">
              <div className="min-w-[200px] flex-1">
                <Input label="Description" value={name} onChange={setName} />
              </div>
              <label className="block text-sm">
                <span className="mb-1 block text-gray-500 dark:text-gray-400">Expires in</span>
                <select
                  value={expiresIn === null ? '' : String(expiresIn)}
                  onChange={(e) =>
                    setExpiresIn(e.target.value === '' ? null : Number(e.target.value))
                  }
                  className="panel-field rounded-lg px-3 py-2"
                >
                  {EXPIRY_OPTIONS.map((o) => (
                    <option key={o.label} value={o.days === null ? '' : o.days}>
                      {o.label}
                    </option>
                  ))}
                </select>
              </label>
              <Btn primary type="submit" disabled={create.isPending || !name.trim()}>
                Create
              </Btn>
            </div>

            <div>
              <p className="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                Token Permissions
              </p>
              <div className="space-y-2">
                {API_ABILITIES.map((a) => {
                  const disabled = a.id !== 'root' && abilities.includes('root')
                  return (
                    <label
                      key={a.id}
                      className={`flex items-start gap-2 text-sm ${disabled ? 'opacity-50' : ''}`}
                    >
                      <input
                        type="checkbox"
                        className="mt-0.5"
                        checked={abilities.includes(a.id)}
                        disabled={disabled}
                        onChange={() => toggleAbility(a.id)}
                      />
                      <span>
                        <span className="font-medium">{a.label}</span>
                        <span className="ml-2 text-gray-500 dark:text-gray-400">{a.help}</span>
                      </span>
                    </label>
                  )
                })}
              </div>
              {abilities.includes('root') && (
                <p className="mt-2 text-sm font-medium text-amber-600 dark:text-amber-400">
                  Root access, be careful!
                </p>
              )}
            </div>
            {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
          </form>
        </div>
      )}

      {plain && (
        <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-500/30 dark:bg-amber-500/10">
          <p className="text-sm font-medium text-amber-800 dark:text-amber-200">
            Please copy this token now. For your security, it won&apos;t be shown again.
          </p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <code className="block flex-1 break-all font-mono text-sm">{plain}</code>
            <Btn
              type="button"
              onClick={() => void navigator.clipboard.writeText(plain)}
            >
              Copy
            </Btn>
            <button
              type="button"
              className="text-xs text-brand-600"
              onClick={() => setPlain(null)}
            >
              Dismiss
            </button>
          </div>
        </div>
      )}

      <div>
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Issued Tokens</h3>
          {(tokens.data?.api_tokens?.length || 0) > 1 && (
            <input
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter tokens..."
              className="panel-field w-64 rounded-lg px-3 py-1.5 text-sm"
            />
          )}
        </div>
        <div className="panel-card overflow-hidden">
          <table className="w-full text-left text-sm">
            <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
              <tr>
                <th className="px-3 py-2">Description</th>
                <th className="px-3 py-2">Permissions</th>
                <th className="px-3 py-2">Last used</th>
                <th className="px-3 py-2">Created</th>
                <th className="px-3 py-2">Expires</th>
                <th className="px-3 py-2">Actions</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((t) => (
                <tr key={t.id} className="border-t border-gray-200 dark:border-gray-800">
                  <td className="px-3 py-2 font-medium">{t.name}</td>
                  <td className="px-3 py-2 text-xs">{(t.abilities || []).join(', ')}</td>
                  <td className="px-3 py-2 text-gray-500 dark:text-gray-400">
                    {t.last_used_at ? new Date(t.last_used_at).toLocaleString() : 'Never'}
                  </td>
                  <td className="px-3 py-2 text-gray-500 dark:text-gray-400">
                    {new Date(t.created_at).toLocaleString()}
                  </td>
                  <td className="px-3 py-2 text-gray-500 dark:text-gray-400">
                    {t.expires_at ? new Date(t.expires_at).toLocaleString() : 'Never'}
                  </td>
                  <td className="px-3 py-2">
                    <button
                      type="button"
                      className="text-error-500"
                      onClick={() => {
                        if (confirm(`Revoke token "${t.name}"?`)) {
                          void api.deleteApiToken(t.id).then(() =>
                            qc.invalidateQueries({ queryKey: ['api-tokens'] }),
                          )
                        }
                      }}
                    >
                      Revoke token
                    </button>
                  </td>
                </tr>
              ))}
              {!filtered.length && (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                    No API tokens found.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
