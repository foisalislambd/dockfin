import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'
import { Btn, Header, Input, Modal } from './Servers'

export function StoragesPage() {
  const qc = useQueryClient()
  const storages = useQuery({ queryKey: ['s3-storages'], queryFn: api.s3Storages })
  const [show, setShow] = useState(false)
  const [form, setForm] = useState({
    name: '',
    endpoint: '',
    bucket: '',
    region: 'us-east-1',
    access_key: '',
    secret_key: '',
  })
  const create = useMutation({
    mutationFn: () => api.createS3Storage(form),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['s3-storages'] })
      setShow(false)
      setForm({ name: '', endpoint: '', bucket: '', region: 'us-east-1', access_key: '', secret_key: '' })
    },
  })

  return (
    <div className="space-y-6">
      <Header
        title="S3 Storages"
        actions={
          <Btn primary onClick={() => setShow(true)}>
            + Add
          </Btn>
        }
      />
      <p className="text-sm text-gray-500 dark:text-gray-400">Backup destinations for databases and volumes.</p>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {(storages.data?.s3_storages || []).map((s) => (
          <div key={s.id} className="panel-card p-5">
            <div className="font-medium text-gray-900 dark:text-white">{s.name}</div>
            <div className="mt-1 truncate font-mono text-xs text-gray-500 dark:text-gray-400">{s.endpoint}</div>
            <div className="mt-2 text-sm text-gray-600 dark:text-gray-300">
              {s.bucket} · {s.region}
            </div>
            <div className="mt-4">
              <Btn
                onClick={() => {
                  if (confirm(`Delete storage ${s.name}?`)) {
                    void api.deleteS3Storage(s.id).then(() => qc.invalidateQueries({ queryKey: ['s3-storages'] }))
                  }
                }}
              >
                Delete
              </Btn>
            </div>
          </div>
        ))}
        {!storages.data?.s3_storages?.length && (
          <div className="panel-card col-span-full p-8 text-center text-sm text-gray-500">No S3 storages yet.</div>
        )}
      </div>

      {show && (
        <Modal title="Add S3 storage" onClose={() => setShow(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <Input label="Name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} />
            <Input label="Endpoint" value={form.endpoint} onChange={(v) => setForm({ ...form, endpoint: v })} />
            <Input label="Bucket" value={form.bucket} onChange={(v) => setForm({ ...form, bucket: v })} />
            <Input label="Region" value={form.region} onChange={(v) => setForm({ ...form, region: v })} required={false} />
            <Input label="Access key" value={form.access_key} onChange={(v) => setForm({ ...form, access_key: v })} />
            <Input label="Secret key" value={form.secret_key} onChange={(v) => setForm({ ...form, secret_key: v })} />
            {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
            <Btn primary type="submit">
              Save
            </Btn>
          </form>
        </Modal>
      )}
    </div>
  )
}

export function TeamPage() {
  const { user, team, teams, refresh } = useAuth()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const switchTo = async (teamId: string) => {
    setError('')
    setBusy(true)
    try {
      await api.switchTeam(teamId)
      await refresh()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to switch team')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-6">
      <Header title="Team" />
      <div className="panel-card p-5">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Current team</h2>
        <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
          {team?.name || '—'}
          {team?.personal ? ' (personal)' : ''}
        </p>
        <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">Signed in as {user?.email}</p>
      </div>

      <div className="space-y-3">
        <h2 className="text-lg font-medium text-gray-900 dark:text-white">Your teams</h2>
        {error && <p className="text-sm text-error-500">{error}</p>}
        <div className="grid gap-3 sm:grid-cols-2">
          {teams.map((t) => (
            <div key={t.id} className="panel-card flex items-center justify-between gap-3 p-4">
              <div>
                <div className="font-medium text-gray-900 dark:text-white">{t.name}</div>
                <div className="text-xs text-gray-500 dark:text-gray-400">
                  {t.personal ? 'Personal' : 'Shared'}
                  {t.role ? ` · ${t.role}` : ''}
                  {team?.id === t.id ? ' · active' : ''}
                </div>
              </div>
              {team?.id !== t.id && (
                <Btn
                  primary
                  onClick={() => {
                    if (!busy) void switchTo(t.id)
                  }}
                >
                  Switch
                </Btn>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export function SharedVariablesPage() {
  const qc = useQueryClient()
  const vars = useQuery({
    queryKey: ['shared-env', 'team'],
    queryFn: () => api.sharedEnvVars('team'),
  })
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const upsert = useMutation({
    mutationFn: () =>
      api.upsertSharedEnvVar({
        scope_type: 'team',
        key,
        value,
      }),
    onSuccess: () => {
      setKey('')
      setValue('')
      void qc.invalidateQueries({ queryKey: ['shared-env', 'team'] })
    },
  })

  return (
    <div className="space-y-6">
      <Header title="Shared Variables" />
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Team-scoped variables available as {'{{team.KEY}}'} in deployments.
      </p>

      <div className="panel-card overflow-hidden">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Key</th>
              <th className="px-3 py-2">Value</th>
            </tr>
          </thead>
          <tbody>
            {(vars.data?.shared_environment_variables || []).map((v) => (
              <tr key={v.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-mono text-xs">{v.key}</td>
                <td className="px-3 py-2 font-mono text-xs text-gray-500 dark:text-gray-400">{v.value ?? '••••'}</td>
              </tr>
            ))}
            {!vars.data?.shared_environment_variables?.length && (
              <tr>
                <td colSpan={2} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  No shared variables yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <form
        className="flex flex-wrap items-end gap-3"
        onSubmit={(e) => {
          e.preventDefault()
          upsert.mutate()
        }}
      >
        <div className="min-w-[140px] flex-1">
          <Input label="Key" value={key} onChange={setKey} />
        </div>
        <div className="min-w-[180px] flex-1">
          <Input label="Value" value={value} onChange={setValue} />
        </div>
        <Btn primary type="submit">
          Save
        </Btn>
        {upsert.error && <p className="w-full text-sm text-error-500">{upsert.error.message}</p>}
      </form>
    </div>
  )
}

export function SettingsPage() {
  const { user, team } = useAuth()
  const version = useQuery({ queryKey: ['version'], queryFn: api.version })

  return (
    <div className="space-y-6">
      <Header title="Settings" />

      <div className="panel-card p-6">
        <h2 className="text-lg font-medium text-gray-900 dark:text-white">Instance</h2>
        <dl className="mt-6 grid gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs text-gray-500 dark:text-gray-400">Name</dt>
            <dd className="mt-1 text-sm">{version.data?.name || 'Goolify'}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 dark:text-gray-400">Version</dt>
            <dd className="mt-1 font-mono text-sm">{version.data?.version || '—'}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 dark:text-gray-400">API</dt>
            <dd className="mt-1 font-mono text-sm">/api/v1</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 dark:text-gray-400">Config path (hosts)</dt>
            <dd className="mt-1 font-mono text-sm">/data/goolify</dd>
          </div>
        </dl>
      </div>

      <div className="panel-card p-6">
        <h2 className="text-lg font-medium text-gray-900 dark:text-white">Profile</h2>
        <dl className="mt-6 grid gap-4 sm:grid-cols-2">
          <div>
            <dt className="text-xs text-gray-500 dark:text-gray-400">Name</dt>
            <dd className="mt-1 text-sm">{user?.name || '—'}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 dark:text-gray-400">Email</dt>
            <dd className="mt-1 text-sm">{user?.email || '—'}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 dark:text-gray-400">Current team</dt>
            <dd className="mt-1 text-sm">{team?.name || '—'}</dd>
          </div>
        </dl>
      </div>

      <div className="panel-card p-6">
        <h2 className="text-lg font-medium text-gray-900 dark:text-white">Quick links</h2>
        <ul className="mt-4 space-y-2 text-sm">
          <li>
            <Link to="/team" className="text-brand-600 hover:underline dark:text-brand-400">
              Team
            </Link>
          </li>
          <li>
            <Link to="/storages" className="text-brand-600 hover:underline dark:text-brand-400">
              S3 Storages
            </Link>
          </li>
          <li>
            <Link to="/shared-variables" className="text-brand-600 hover:underline dark:text-brand-400">
              Shared variables
            </Link>
          </li>
          <li>
            <Link to="/security/private-keys" className="text-brand-600 hover:underline dark:text-brand-400">
              SSH private keys
            </Link>
          </li>
          <li>
            <Link to="/onboarding" className="text-brand-600 hover:underline dark:text-brand-400">
              Onboarding wizard
            </Link>
          </li>
          <li>
            <Link to="/notifications" className="text-brand-600 hover:underline dark:text-brand-400">
              Notifications
            </Link>
          </li>
        </ul>
      </div>
    </div>
  )
}

export function PrivateKeysPage() {
  const qc = useQueryClient()
  const keys = useQuery({ queryKey: ['keys'], queryFn: api.keys })
  const [show, setShow] = useState(false)
  const [name, setName] = useState('')
  const [key, setKey] = useState('')
  const create = useMutation({
    mutationFn: () => api.createKey(name, key),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['keys'] })
      setShow(false)
      setName('')
      setKey('')
    },
  })

  return (
    <div className="space-y-6">
      <Header
        title="Private Keys"
        actions={
          <Btn primary onClick={() => setShow(true)}>
            + Add
          </Btn>
        }
      />
      <div className="panel-card overflow-hidden">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Fingerprint</th>
            </tr>
          </thead>
          <tbody>
            {(keys.data?.private_keys || []).map((k) => (
              <tr key={k.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-medium">{k.name}</td>
                <td className="px-3 py-2 font-mono text-xs text-gray-500 dark:text-gray-400">{k.fingerprint}</td>
              </tr>
            ))}
            {!keys.data?.private_keys?.length && (
              <tr>
                <td colSpan={2} className="px-4 py-8 text-center text-gray-500">
                  No private keys yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {show && (
        <Modal title="Add SSH key" onClose={() => setShow(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
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
                className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 font-mono text-xs dark:border-gray-800 dark:bg-gray-900"
              />
            </label>
            {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
            <Btn primary type="submit">
              Save key
            </Btn>
          </form>
        </Modal>
      )}
    </div>
  )
}
