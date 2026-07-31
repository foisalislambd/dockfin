import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { EnvSecretCell, SecretInput } from '../components/SecretValue'
import { CreatePageShell, FormActions, FormInput } from '../components/ui/forms'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'
import { isSecretEnvKey } from '../lib/secrets'
import { Btn, Header, Input } from './Servers'

export function StoragesPage() {
  const qc = useQueryClient()
  const storages = useQuery({ queryKey: ['s3-storages'], queryFn: api.s3Storages })

  return (
    <div className="space-y-6">
      <Header
        title="S3 Storages"
        actions={
          <Link
            to="/storages/new"
            className="inline-flex h-8 items-center rounded-md bg-brand-500 px-2.5 text-xs font-medium text-white transition hover:bg-brand-600"
          >
            + Add
          </Link>
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
    </div>
  )
}

export function CreateStoragePage() {
  const qc = useQueryClient()
  const nav = useNavigate()
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
      void nav({ to: '/storages' })
    },
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    create.mutate()
  }

  return (
    <CreatePageShell title="Add S3 storage" backTo="/storages" backLabel="Back to S3 Storages">
      <form className="space-y-4" onSubmit={onSubmit}>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Configure an S3-compatible bucket for database and volume backups.
        </p>
        <div className="grid gap-4 sm:grid-cols-2">
          <FormInput label="Name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} />
          <FormInput
            label="Endpoint"
            value={form.endpoint}
            onChange={(v) => setForm({ ...form, endpoint: v })}
            placeholder="https://s3.amazonaws.com"
          />
          <FormInput label="Bucket" value={form.bucket} onChange={(v) => setForm({ ...form, bucket: v })} />
          <FormInput
            label="Region"
            value={form.region}
            onChange={(v) => setForm({ ...form, region: v })}
            required={false}
            hint="optional"
          />
          <FormInput
            label="Access key"
            value={form.access_key}
            onChange={(v) => setForm({ ...form, access_key: v })}
          />
          <FormInput
            label="Secret key"
            type="password"
            value={form.secret_key}
            onChange={(v) => setForm({ ...form, secret_key: v })}
          />
        </div>
        {create.error && <p className="text-sm text-error-500">{create.error.message}</p>}
        <FormActions busy={create.isPending} submitLabel="Save" cancelTo="/storages" />
      </form>
    </CreatePageShell>
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

export function SharedVariablesPage() {
  const qc = useQueryClient()
  const vars = useQuery({
    queryKey: ['shared-env', 'team'],
    queryFn: () => api.sharedEnvVars('team', undefined, true),
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
        <table className="panel-table">
          <thead>
            <tr>
              <th>Key</th>
              <th>Value</th>
            </tr>
          </thead>
          <tbody>
            {(vars.data?.shared_environment_variables || []).map((v) => (
              <tr key={v.id}>
                <td className="font-mono text-xs text-gray-900 dark:text-gray-100">{v.key}</td>
                <td>
                  <EnvSecretCell envKey={v.key} value={v.value} />
                </td>
              </tr>
            ))}
            {!vars.data?.shared_environment_variables?.length && (
              <tr>
                <td colSpan={2} className="panel-table-empty">
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
          {isSecretEnvKey(key) ? (
            <SecretInput label="Value" value={value} onChange={setValue} required />
          ) : (
            <Input label="Value" value={value} onChange={setValue} />
          )}
        </div>
        <Btn primary type="submit">
          Save
        </Btn>
        {upsert.error && <p className="w-full text-sm text-error-500">{upsert.error.message}</p>}
      </form>
    </div>
  )
}
