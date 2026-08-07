import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams, useSearch } from '@tanstack/react-router'
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
  const [newTeamName, setNewTeamName] = useState('')
  const [newTeamDesc, setNewTeamDesc] = useState('')

  const members = useQuery({ queryKey: ['team-members'], queryFn: api.teamMembers })
  const invitations = useQuery({ queryKey: ['team-invitations'], queryFn: api.teamInvitations })

  const createTeam = useMutation({
    mutationFn: () => api.createTeam(newTeamName.trim(), newTeamDesc.trim()),
    onSuccess: async (res) => {
      setNewTeamName('')
      setNewTeamDesc('')
      await api.switchTeam(res.team.id)
      await refresh()
      void qc.invalidateQueries({ queryKey: ['team-members'] })
      void qc.invalidateQueries({ queryKey: ['team-invitations'] })
    },
  })

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

      <form
        className="panel-card space-y-3 p-5"
        onSubmit={(e) => {
          e.preventDefault()
          if (!newTeamName.trim()) return
          createTeam.mutate()
        }}
      >
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Create team</h2>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Create a shared organization team. You become the owner.
        </p>
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Name</span>
          <input
            value={newTeamName}
            onChange={(e) => setNewTeamName(e.target.value)}
            className="w-full panel-field rounded-lg px-3 py-2"
            required
          />
        </label>
        <label className="block text-sm">
          <span className="mb-1 block text-gray-500 dark:text-gray-400">Description</span>
          <input
            value={newTeamDesc}
            onChange={(e) => setNewTeamDesc(e.target.value)}
            className="w-full panel-field rounded-lg px-3 py-2"
          />
        </label>
        {createTeam.error && (
          <p className="text-sm text-error-500">{createTeam.error.message}</p>
        )}
        <Btn primary type="submit">
          {createTeam.isPending ? 'Creating…' : 'Create team'}
        </Btn>
      </form>

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

export function SharedVariablesPage({
  scopeType,
  scopeId,
  title = 'Shared Variables',
}: {
  scopeType?: 'team' | 'project' | 'environment' | 'server'
  scopeId?: string
  title?: string
} = {}) {
  const qc = useQueryClient()
  const nav = useNavigate()
  // When rendered directly on /shared-variables (no props), support ?scope=server&server_id=...
  // so links like Server detail settings can deep-link into the server scope hub.
  const search = useSearch({ strict: false }) as { scope?: string; server_id?: string }
  const usingProps = scopeType !== undefined
  const effectiveScopeType: 'team' | 'project' | 'environment' | 'server' = usingProps
    ? scopeType
    : search.scope === 'server'
      ? 'server'
      : 'team'
  const [serverId, setServerId] = useState(search.server_id || '')
  const effectiveScopeId = usingProps ? scopeId : effectiveScopeType === 'server' ? serverId || undefined : undefined

  const servers = useQuery({
    queryKey: ['servers'],
    queryFn: api.servers,
    enabled: !usingProps && effectiveScopeType === 'server',
  })

  const vars = useQuery({
    queryKey: ['shared-env', effectiveScopeType, effectiveScopeId || ''],
    queryFn: () => api.sharedEnvVars(effectiveScopeType, effectiveScopeId, true),
    enabled: effectiveScopeType !== 'server' || Boolean(effectiveScopeId),
  })
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const upsert = useMutation({
    mutationFn: () =>
      api.upsertSharedEnvVar({
        scope_type: effectiveScopeType,
        scope_id: effectiveScopeId,
        key,
        value,
      }),
    onSuccess: () => {
      setKey('')
      setValue('')
      void qc.invalidateQueries({ queryKey: ['shared-env', effectiveScopeType, effectiveScopeId || ''] })
    },
  })

  return (
    <div className="space-y-6">
      <Header title={title} />

      {!usingProps && (
        <div className="flex flex-wrap items-end gap-3">
          <div className="inline-flex rounded-lg border border-gray-200 p-1 dark:border-gray-800">
            <button
              type="button"
              className={`rounded-md px-3 py-1 text-sm ${
                effectiveScopeType === 'team'
                  ? 'bg-brand-500 text-white'
                  : 'text-gray-600 dark:text-gray-300'
              }`}
              onClick={() => void nav({ to: '/shared-variables', search: {} })}
            >
              Team
            </button>
            <button
              type="button"
              className={`rounded-md px-3 py-1 text-sm ${
                effectiveScopeType === 'server'
                  ? 'bg-brand-500 text-white'
                  : 'text-gray-600 dark:text-gray-300'
              }`}
              onClick={() =>
                void nav({
                  to: '/shared-variables',
                  search: { scope: 'server', server_id: serverId || undefined },
                })
              }
            >
              Server
            </button>
          </div>
          {effectiveScopeType === 'server' && (
            <label className="block min-w-[220px] text-sm">
              <span className="mb-1 block text-gray-500 dark:text-gray-400">Server</span>
              <select
                value={serverId}
                onChange={(e) => {
                  setServerId(e.target.value)
                  void nav({
                    to: '/shared-variables',
                    search: { scope: 'server', server_id: e.target.value || undefined },
                  })
                }}
                className="panel-field w-full rounded-lg px-3 py-2 text-sm"
              >
                <option value="">Select a server…</option>
                {(servers.data?.servers || []).map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </label>
          )}
        </div>
      )}

      {effectiveScopeType === 'server' && !effectiveScopeId ? (
        <div className="panel-card p-8 text-center text-sm text-gray-500 dark:text-gray-400">
          Select a server above to view or edit its shared variables.
        </div>
      ) : (
        <>
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
        </>
      )}
    </div>
  )
}

export function EnvironmentSharedVariablesPage() {
  const { projectId, envId } = useParams({ strict: false }) as { projectId: string; envId: string }
  const project = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => api.getProject(projectId),
  })
  const envs = useQuery({
    queryKey: ['environments', projectId],
    queryFn: () => api.environments(projectId),
  })
  const env = (envs.data?.environments || []).find((e) => e.id === envId)

  return (
    <div className="space-y-4">
      <nav className="flex flex-wrap items-center gap-1 text-sm text-gray-500 dark:text-gray-400">
        <Link to="/projects" className="hover:text-brand-600 dark:hover:text-brand-400">
          Projects
        </Link>
        <span>/</span>
        <Link
          to="/projects/$projectId"
          params={{ projectId }}
          className="hover:text-brand-600 dark:hover:text-brand-400"
        >
          {project.data?.name || '…'}
        </Link>
        <span>/</span>
        <Link
          to="/projects/$projectId/environments/$envId"
          params={{ projectId, envId }}
          className="hover:text-brand-600 dark:hover:text-brand-400"
        >
          {env?.name || '…'}
        </Link>
        <span>/</span>
        <span className="text-gray-900 dark:text-white">Shared Variables</span>
      </nav>
      <SharedVariablesPage
        scopeType="environment"
        scopeId={envId}
        title="Environment Shared Variables"
      />
    </div>
  )
}

export function ProjectSharedVariablesPage() {
  const { projectId } = useParams({ strict: false }) as { projectId: string }
  const project = useQuery({
    queryKey: ['project', projectId],
    queryFn: () => api.getProject(projectId),
  })

  return (
    <div className="space-y-4">
      <nav className="flex flex-wrap items-center gap-1 text-sm text-gray-500 dark:text-gray-400">
        <Link to="/projects" className="hover:text-brand-600 dark:hover:text-brand-400">
          Projects
        </Link>
        <span>/</span>
        <Link
          to="/projects/$projectId"
          params={{ projectId }}
          className="hover:text-brand-600 dark:hover:text-brand-400"
        >
          {project.data?.name || '…'}
        </Link>
        <span>/</span>
        <span className="text-gray-900 dark:text-white">Shared Variables</span>
      </nav>
      <SharedVariablesPage
        scopeType="project"
        scopeId={projectId}
        title="Project Shared Variables"
      />
    </div>
  )
}
