import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { ApiError, api, type SecretManagerSettings } from '../lib/api'
import { useToast } from './Toast'
import { Btn } from './ui/Button'

type Props = {
  resourceType: string
  resourceId: string
}

const emptySettings = (): SecretManagerSettings => ({
  project: '',
  config: '',
  project_id: '',
  environment: '',
  secret_path: '/',
  mount: 'secret',
  path: '',
})

export function SecretManagerLink({ resourceType, resourceId }: Props) {
  const toast = useToast()
  const qc = useQueryClient()
  const tokens = useQuery({ queryKey: ['integration-tokens'], queryFn: api.integrationTokens })
  const link = useQuery({
    queryKey: ['secret-manager', resourceType, resourceId],
    queryFn: async () => {
      try {
        return await api.secretManager(resourceType, resourceId)
      } catch (err) {
        if (err instanceof ApiError && err.status === 404) return null
        throw err
      }
    },
  })
  const [tokenId, setTokenId] = useState('')
  const [settings, setSettings] = useState<SecretManagerSettings>(emptySettings)
  const [keys, setKeys] = useState<string[] | null>(null)

  useEffect(() => {
    if (link.isLoading) return
    if (!link.data) {
      setTokenId('')
      setSettings(emptySettings())
      setKeys(null)
      return
    }
    setTokenId(link.data.integration_token_id)
    setSettings({ ...emptySettings(), ...link.data.settings })
  }, [resourceId, link.isLoading, link.data])

  const selected = (tokens.data?.integration_tokens || []).find((t) => t.id === tokenId)
  const provider = selected?.provider || ''

  const save = useMutation({
    mutationFn: () =>
      api.saveSecretManager({
        resource_type: resourceType,
        resource_id: resourceId,
        integration_token_id: tokenId,
        settings: currentSettings(),
      }),
    onSuccess: () => {
      toast.success('Secret manager linked. Values are read at deploy time.')
      setKeys(null)
      void qc.invalidateQueries({ queryKey: ['secret-manager', resourceType, resourceId] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const remove = useMutation({
    mutationFn: () => api.deleteSecretManager(resourceType, resourceId),
    onSuccess: () => {
      toast.success('Secret manager disconnected.')
      setTokenId('')
      setSettings(emptySettings())
      setKeys(null)
      void qc.invalidateQueries({ queryKey: ['secret-manager', resourceType, resourceId] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const loadKeys = useMutation({
    mutationFn: () => api.integrationTokenKeys(tokenId, currentSettings()),
    onSuccess: (data) => setKeys(data.keys || []),
    onError: (err: Error) => toast.error(err.message),
  })

  const importAll = useMutation({
    mutationFn: () => api.importSecretManager(resourceType, resourceId),
    onSuccess: (data) => {
      const n = data.imported?.length || 0
      const skipped = data.skipped?.length || 0
      toast.success(
        n === 0
          ? 'No new keys to import.'
          : `Imported ${n} reference${n === 1 ? '' : 's'}${skipped ? `, skipped ${skipped}` : ''}.`,
      )
      void qc.invalidateQueries({ queryKey: ['env-vars', resourceType, resourceId] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const addRef = useMutation({
    mutationFn: (key: string) =>
      api.upsertEnvVar({
        resource_type: resourceType,
        resource_id: resourceId,
        key,
        value: `{{vault.${key}}}`,
        is_runtime: true,
        is_buildtime: true,
        comment: 'Remote secret',
      }),
    onSuccess: (_data, key) => {
      toast.success(`Added {{vault.${key}}}`)
      void qc.invalidateQueries({ queryKey: ['env-vars', resourceType, resourceId] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  function currentSettings(): SecretManagerSettings {
    return scopedSettings(provider, settings)
  }

  const list = tokens.data?.integration_tokens || []
  const formDirty =
    !link.data ||
    tokenId !== link.data.integration_token_id ||
    JSON.stringify(currentSettings()) !==
      JSON.stringify(scopedSettings(link.data.provider, { ...emptySettings(), ...link.data.settings }))

  return (
    <section className="panel-card space-y-3 p-4">
      <div>
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Secret manager</h3>
        <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
          Store <code className="text-[11px]">{'{{vault.KEY}}'}</code> references. Dockfin reads the value from Doppler,
          Infisical, or Vault when the resource is deployed. The secret is not saved in Dockfin.
        </p>
      </div>

      {tokens.isLoading ? (
        <p className="text-sm text-gray-500">Loading tokens…</p>
      ) : list.length === 0 ? (
        <p className="text-sm text-gray-600 dark:text-gray-300">
          No tokens yet.{' '}
          <Link to="/security" search={{ tab: 'secret-managers' }} className="text-brand-600 hover:underline dark:text-brand-400">
            Add one in Security
          </Link>
          .
        </p>
      ) : (
        <>
          <label className="block text-sm">
            <span className="mb-1 block text-gray-500 dark:text-gray-400">Token</span>
            <select
              value={tokenId}
              onChange={(e) => {
                const id = e.target.value
                setTokenId(id)
                setKeys(null)
                if (link.data && id === link.data.integration_token_id) {
                  setSettings({ ...emptySettings(), ...link.data.settings })
                } else {
                  setSettings(emptySettings())
                }
              }}
              className="panel-field w-full rounded-lg px-3 py-2 text-sm"
            >
              <option value="">Select a token</option>
              {list.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name} ({t.provider})
                </option>
              ))}
            </select>
          </label>

          {provider === 'doppler' && (
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="Project (optional)" value={settings.project || ''} onChange={(v) => setSettings({ ...settings, project: v })} />
              <Field label="Config (optional)" value={settings.config || ''} onChange={(v) => setSettings({ ...settings, config: v })} />
            </div>
          )}
          {provider === 'infisical' && (
            <div className="grid gap-3 sm:grid-cols-3">
              <Field label="Project ID" value={settings.project_id || ''} onChange={(v) => setSettings({ ...settings, project_id: v })} />
              <Field label="Environment" value={settings.environment || ''} onChange={(v) => setSettings({ ...settings, environment: v })} />
              <Field label="Secret path" value={settings.secret_path || '/'} onChange={(v) => setSettings({ ...settings, secret_path: v })} />
            </div>
          )}
          {provider === 'vault' && (
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="KV mount" value={settings.mount || 'secret'} onChange={(v) => setSettings({ ...settings, mount: v })} />
              <Field label="Path" value={settings.path || ''} onChange={(v) => setSettings({ ...settings, path: v })} />
            </div>
          )}

          <div className="flex flex-wrap gap-2">
            <Btn primary disabled={!tokenId || save.isPending} onClick={() => save.mutate()}>
              Save link
            </Btn>
            <Btn disabled={!tokenId || loadKeys.isPending} onClick={() => loadKeys.mutate()}>
              Load key names
            </Btn>
            {link.data ? (
              <>
                <Btn disabled={importAll.isPending || formDirty} onClick={() => importAll.mutate()}>
                  Import missing
                </Btn>
                <Btn disabled={remove.isPending} onClick={() => remove.mutate()}>
                  Disconnect
                </Btn>
              </>
            ) : null}
          </div>

          {link.data ? (
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Linked to {link.data.token_name} ({link.data.provider}).
            </p>
          ) : null}

          {keys ? (
            keys.length === 0 ? (
              <p className="text-sm text-gray-500">No keys at that path.</p>
            ) : (
              <ul className="max-h-48 space-y-1 overflow-auto text-sm">
                {keys.map((key) => (
                  <li key={key} className="flex items-center justify-between gap-2 rounded-md px-2 py-1 hover:bg-gray-50 dark:hover:bg-white/5">
                    <code className="text-xs">{key}</code>
                    {/^[A-Za-z_][A-Za-z0-9_]*$/.test(key) ? (
                      <Btn disabled={addRef.isPending} onClick={() => addRef.mutate(key)}>
                        Add {'{{vault.' + key + '}}'}
                      </Btn>
                    ) : (
                      <span className="text-xs text-gray-400">Not a valid env name</span>
                    )}
                  </li>
                ))}
              </ul>
            )
          ) : null}
        </>
      )}
    </section>
  )
}

function scopedSettings(provider: string, raw: SecretManagerSettings): SecretManagerSettings {
  if (provider === 'doppler') return { project: raw.project || '', config: raw.config || '' }
  if (provider === 'infisical') {
    return {
      project_id: raw.project_id || '',
      environment: raw.environment || '',
      secret_path: raw.secret_path || '/',
    }
  }
  if (provider === 'vault') return { mount: raw.mount || 'secret', path: raw.path || '' }
  return {}
}

function Field({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <label className="block text-sm">
      <span className="mb-1 block text-gray-500 dark:text-gray-400">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="panel-field w-full rounded-lg px-3 py-2 text-sm"
      />
    </label>
  )
}
