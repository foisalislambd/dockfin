import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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
          <div className="panel-card col-span-full p-8 text-center text-sm text-gray-500 dark:text-gray-400">No S3 storages yet.</div>
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
  const qc = useQueryClient()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState('member')
  const [acceptToken, setAcceptToken] = useState('')
  const [createdInvite, setCreatedInvite] = useState<string | null>(null)

  const members = useQuery({ queryKey: ['team-members'], queryFn: api.teamMembers })
  const invitations = useQuery({ queryKey: ['team-invitations'], queryFn: api.teamInvitations })

  const switchTo = async (teamId: string) => {
    setError('')
    setBusy(true)
    try {
      await api.switchTeam(teamId)
      await refresh()
      void qc.invalidateQueries({ queryKey: ['team-members'] })
      void qc.invalidateQueries({ queryKey: ['team-invitations'] })
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to switch team')
    } finally {
      setBusy(false)
    }
  }

  const invite = useMutation({
    mutationFn: () => api.createInvitation(inviteEmail, inviteRole),
    onSuccess: (inv) => {
      setInviteEmail('')
      setCreatedInvite(inv.token || null)
      void qc.invalidateQueries({ queryKey: ['team-invitations'] })
    },
  })

  const accept = useMutation({
    mutationFn: () => api.acceptInvitation(acceptToken.trim()),
    onSuccess: async () => {
      setAcceptToken('')
      await refresh()
      void qc.invalidateQueries({ queryKey: ['team-members'] })
      void qc.invalidateQueries({ queryKey: ['team-invitations'] })
    },
  })

  const canManage = team?.role === 'owner' || team?.role === 'admin'

  return (
    <div className="space-y-6">
      <Header title="Team" />
      <div className="panel-card p-5">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Current team</h2>
        <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
          {team?.name || '—'}
          {team?.personal ? ' (personal)' : ''}
          {team?.role ? ` · ${team.role}` : ''}
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

      <div className="space-y-3">
        <h2 className="text-lg font-medium text-gray-900 dark:text-white">Members</h2>
        <div className="panel-card overflow-hidden">
          <table className="w-full text-left text-sm">
            <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
              <tr>
                <th className="px-3 py-2">Name</th>
                <th className="px-3 py-2">Email</th>
                <th className="px-3 py-2">Role</th>
                <th className="px-3 py-2">Actions</th>
              </tr>
            </thead>
            <tbody>
              {(members.data?.members || []).map((m) => (
                <tr key={m.user_id} className="border-t border-gray-200 dark:border-gray-800">
                  <td className="px-3 py-2">{m.name}</td>
                  <td className="px-3 py-2 text-gray-500 dark:text-gray-400">{m.email}</td>
                  <td className="px-3 py-2">{m.role}</td>
                  <td className="px-3 py-2">
                    {canManage && m.user_id !== user?.id && (
                      <button
                        type="button"
                        className="text-error-500"
                        onClick={() => {
                          if (confirm(`Remove ${m.email}?`)) {
                            void api.removeTeamMember(m.user_id).then(() =>
                              qc.invalidateQueries({ queryKey: ['team-members'] }),
                            )
                          }
                        }}
                      >
                        Remove
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {canManage && (
        <div className="space-y-3">
          <h2 className="text-lg font-medium text-gray-900 dark:text-white">Invitations</h2>
          <form
            className="flex flex-wrap items-end gap-3"
            onSubmit={(e) => {
              e.preventDefault()
              invite.mutate()
            }}
          >
            <div className="min-w-[200px] flex-1">
              <Input label="Email" value={inviteEmail} onChange={setInviteEmail} />
            </div>
            <label className="block text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Role</span>
              <select
                value={inviteRole}
                onChange={(e) => setInviteRole(e.target.value)}
                className="panel-field h-9 rounded-lg px-2"
              >
                <option value="member">member</option>
                <option value="admin">admin</option>
              </select>
            </label>
            <Btn primary type="submit">
              Invite
            </Btn>
          </form>
          {invite.error && <p className="text-sm text-error-500">{invite.error.message}</p>}
          {createdInvite && (
            <div className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm dark:border-amber-500/30 dark:bg-amber-500/10">
              <p className="font-medium text-amber-800 dark:text-amber-200">Invite token (share once)</p>
              <code className="mt-1 block break-all font-mono text-xs">{createdInvite}</code>
            </div>
          )}
          <div className="panel-card overflow-hidden">
            <table className="w-full text-left text-sm">
              <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
                <tr>
                  <th className="px-3 py-2">Email</th>
                  <th className="px-3 py-2">Role</th>
                  <th className="px-3 py-2">Expires</th>
                  <th className="px-3 py-2">Actions</th>
                </tr>
              </thead>
              <tbody>
                {(invitations.data?.invitations || []).map((inv) => (
                  <tr key={inv.id} className="border-t border-gray-200 dark:border-gray-800">
                    <td className="px-3 py-2">{inv.email}</td>
                    <td className="px-3 py-2">{inv.role}</td>
                    <td className="px-3 py-2 text-gray-500 dark:text-gray-400">
                      {new Date(inv.expires_at).toLocaleDateString()}
                    </td>
                    <td className="px-3 py-2">
                      <button
                        type="button"
                        className="text-error-500"
                        onClick={() =>
                          void api.deleteInvitation(inv.id).then(() =>
                            qc.invalidateQueries({ queryKey: ['team-invitations'] }),
                          )
                        }
                      >
                        Revoke
                      </button>
                    </td>
                  </tr>
                ))}
                {!invitations.data?.invitations?.length && (
                  <tr>
                    <td colSpan={4} className="px-4 py-6 text-center text-gray-500 dark:text-gray-400">
                      No pending invitations.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <div className="panel-card space-y-3 p-5">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Accept invitation</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Paste an invite token (email must match your account).
        </p>
        <form
          className="flex flex-wrap items-end gap-3"
          onSubmit={(e) => {
            e.preventDefault()
            accept.mutate()
          }}
        >
          <div className="min-w-[220px] flex-1">
            <Input label="Token" value={acceptToken} onChange={setAcceptToken} />
          </div>
          <Btn primary type="submit">
            Accept
          </Btn>
        </form>
        {accept.error && <p className="text-sm text-error-500">{accept.error.message}</p>}
      </div>
    </div>
  )
}

export function ApiTokensPage() {
  const qc = useQueryClient()
  const tokens = useQuery({ queryKey: ['api-tokens'], queryFn: api.apiTokens })
  const [show, setShow] = useState(false)
  const [name, setName] = useState('')
  const [plain, setPlain] = useState<string | null>(null)
  const create = useMutation({
    mutationFn: () => api.createApiToken(name, ['*']),
    onSuccess: (data) => {
      setPlain(data.token)
      setName('')
      setShow(false)
      void qc.invalidateQueries({ queryKey: ['api-tokens'] })
    },
  })

  return (
    <div className="space-y-6">
      <Header
        title="API Tokens"
        actions={
          <Btn primary onClick={() => setShow(true)}>
            + New token
          </Btn>
        }
      />
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Use as <code className="font-mono text-xs">Authorization: Bearer glfy_…</code> for CLI and
        automation. Bound to the current team.
      </p>

      {plain && (
        <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-500/30 dark:bg-amber-500/10">
          <p className="text-sm font-medium text-amber-800 dark:text-amber-200">Copy now — shown once</p>
          <code className="mt-2 block break-all font-mono text-sm">{plain}</code>
          <button type="button" className="mt-2 text-xs text-brand-600" onClick={() => setPlain(null)}>
            Dismiss
          </button>
        </div>
      )}

      <div className="panel-card overflow-hidden">
        <table className="w-full text-left text-sm">
          <thead className="bg-gray-50 text-gray-500 dark:bg-white/5 dark:text-gray-400">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Prefix</th>
              <th className="px-3 py-2">Abilities</th>
              <th className="px-3 py-2">Created</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {(tokens.data?.api_tokens || []).map((t) => (
              <tr key={t.id} className="border-t border-gray-200 dark:border-gray-800">
                <td className="px-3 py-2 font-medium">{t.name}</td>
                <td className="px-3 py-2 font-mono text-xs">{t.token_prefix}…</td>
                <td className="px-3 py-2 text-xs">{(t.abilities || []).join(', ')}</td>
                <td className="px-3 py-2 text-gray-500 dark:text-gray-400">{new Date(t.created_at).toLocaleString()}</td>
                <td className="px-3 py-2">
                  <button
                    type="button"
                    className="text-error-500"
                    onClick={() => {
                      if (confirm(`Delete token ${t.name}?`)) {
                        void api.deleteApiToken(t.id).then(() =>
                          qc.invalidateQueries({ queryKey: ['api-tokens'] }),
                        )
                      }
                    }}
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
            {!tokens.data?.api_tokens?.length && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
                  No API tokens yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {show && (
        <Modal title="New API token" onClose={() => setShow(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <Input label="Name" value={name} onChange={setName} />
            {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
            <Btn primary type="submit">
              Create
            </Btn>
          </form>
        </Modal>
      )}
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
                <td colSpan={2} className="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
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
                className="panel-field w-full rounded-lg px-3 py-2 font-mono text-xs"
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
