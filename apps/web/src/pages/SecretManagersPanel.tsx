import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useToast } from '../components/Toast'
import { Btn } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { api, type IntegrationToken } from '../lib/api'

type Provider = 'doppler' | 'infisical' | 'vault'

export function SecretManagersPanel() {
  const toast = useToast()
  const qc = useQueryClient()
  const tokens = useQuery({ queryKey: ['integration-tokens'], queryFn: api.integrationTokens })
  const [open, setOpen] = useState(false)
  const [provider, setProvider] = useState<Provider>('doppler')
  const [name, setName] = useState('')
  const [token, setToken] = useState('')
  const [baseURL, setBaseURL] = useState('')
  const [clientID, setClientID] = useState('')
  const [namespace, setNamespace] = useState('')

  const reset = () => {
    setProvider('doppler')
    setName('')
    setToken('')
    setBaseURL('')
    setClientID('')
    setNamespace('')
  }

  const create = useMutation({
    mutationFn: () =>
      api.createIntegrationToken({
        provider,
        name,
        token,
        metadata:
          provider === 'infisical'
            ? { base_url: baseURL || 'https://app.infisical.com', client_id: clientID }
            : provider === 'vault'
              ? { base_url: baseURL, namespace }
              : {},
      }),
    onSuccess: () => {
      toast.success('Token saved. The secret value is encrypted and is not shown again.')
      setOpen(false)
      reset()
      void qc.invalidateQueries({ queryKey: ['integration-tokens'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const validate = useMutation({
    mutationFn: (id: string) => api.validateIntegrationToken(id),
    onSuccess: () => toast.success('Token is valid.'),
    onError: (err: Error) => toast.error(err.message),
  })

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteIntegrationToken(id),
    onSuccess: () => {
      toast.success('Token deleted.')
      void qc.invalidateQueries({ queryKey: ['integration-tokens'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const list = tokens.data?.integration_tokens || []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <p className="max-w-2xl text-sm text-gray-500 dark:text-gray-400">
          Connect Doppler, Infisical, or HashiCorp Vault. Resources reference keys as {'{{vault.KEY}}'}. Dockfin fetches
          the value during deploy and does not store it.
        </p>
        <Btn primary onClick={() => setOpen(true)}>
          Add token
        </Btn>
      </div>

      {tokens.isLoading ? (
        <p className="text-sm text-gray-500">Loading…</p>
      ) : list.length === 0 ? (
        <p className="text-sm text-gray-500">No secret manager tokens yet.</p>
      ) : (
        <ul className="divide-y divide-gray-200 rounded-lg border border-gray-200 dark:divide-gray-800 dark:border-gray-800">
          {list.map((item) => (
            <TokenRow
              key={item.id}
              item={item}
              onValidate={() => validate.mutate(item.id)}
              onDelete={() => remove.mutate(item.id)}
            />
          ))}
        </ul>
      )}

      {open ? (
        <Modal title="Add secret manager" onClose={() => setOpen(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500">Provider</span>
              <select
                value={provider}
                onChange={(e) => setProvider(e.target.value as Provider)}
                className="panel-field w-full rounded-lg px-3 py-2"
              >
                <option value="doppler">Doppler</option>
                <option value="infisical">Infisical</option>
                <option value="vault">HashiCorp Vault</option>
              </select>
            </label>
            <Text label="Name" value={name} onChange={setName} />
            <Text
              label={provider === 'infisical' ? 'Client secret' : 'Token'}
              value={token}
              onChange={setToken}
              secret
            />
            {provider === 'infisical' ? (
              <>
                <Text label="Client ID" value={clientID} onChange={setClientID} />
                <Text
                  label="Base URL"
                  value={baseURL}
                  onChange={setBaseURL}
                  placeholder="https://app.infisical.com"
                  required={false}
                />
              </>
            ) : null}
            {provider === 'vault' ? (
              <>
                <Text label="Vault address" value={baseURL} onChange={setBaseURL} placeholder="https://vault.example.com" />
                <Text label="Namespace (optional)" value={namespace} onChange={setNamespace} required={false} />
              </>
            ) : null}
            <div className="flex justify-end gap-2 pt-2">
              <Btn type="button" onClick={() => setOpen(false)}>
                Cancel
              </Btn>
              <Btn primary type="submit" disabled={create.isPending || !name.trim() || !token.trim()}>
                Save and validate
              </Btn>
            </div>
          </form>
        </Modal>
      ) : null}
    </div>
  )
}

function TokenRow({
  item,
  onValidate,
  onDelete,
}: {
  item: IntegrationToken
  onValidate: () => void
  onDelete: () => void
}) {
  const meta = item.metadata || {}
  const detail =
    item.provider === 'infisical'
      ? [meta.base_url, meta.client_id].filter(Boolean).join(' · ')
      : item.provider === 'vault'
        ? [meta.base_url, meta.namespace].filter(Boolean).join(' · ')
        : 'Service token'
  return (
    <li className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
      <div>
        <div className="font-medium text-gray-900 dark:text-white">{item.name}</div>
        <div className="text-xs text-gray-500">
          {item.provider}
          {detail ? ` · ${detail}` : ''}
        </div>
      </div>
      <div className="flex gap-2">
        <Btn onClick={onValidate}>Validate</Btn>
        <Btn onClick={onDelete}>Delete</Btn>
      </div>
    </li>
  )
}

function Text({
  label,
  value,
  onChange,
  secret,
  placeholder,
  required = true,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  secret?: boolean
  placeholder?: string
  required?: boolean
}) {
  return (
    <label className="block text-sm">
      <span className="mb-1 block text-gray-500">{label}</span>
      <input
        required={required}
        type={secret ? 'password' : 'text'}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="panel-field w-full rounded-lg px-3 py-2"
        autoComplete="off"
      />
    </label>
  )
}
