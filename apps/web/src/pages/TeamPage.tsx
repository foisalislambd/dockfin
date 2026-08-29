import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Copy, Mail, Plus, Users } from 'lucide-react'
import { useState } from 'react'
import { useConfirm } from '../components/ConfirmDialog'
import { useToast } from '../components/Toast'
import { PageSkeleton } from '../components/ui/Skeleton'
import { api, type Team } from '../lib/api'
import { useAuth } from '../lib/auth'
import { Btn } from './Servers'

function initials(name: string, email: string) {
  const src = (name || email || '?').trim()
  const parts = src.split(/[\s@._-]+/).filter(Boolean)
  const a = (parts[0]?.[0] || '?').toUpperCase()
  const b = (parts[1]?.[0] || '').toUpperCase()
  return (a + b).slice(0, 2)
}

function roleClass(role: string) {
  if (role === 'owner')
    return 'bg-brand-50 text-brand-700 dark:bg-brand-500/15 dark:text-brand-300'
  if (role === 'admin') return 'bg-sky-50 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300'
  return 'bg-gray-100 text-gray-600 dark:bg-white/10 dark:text-gray-300'
}

function RoleChip({ role }: { role: string }) {
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-[11px] font-medium capitalize ${roleClass(role)}`}>
      {role}
    </span>
  )
}

export function TeamPage() {
  const { user, team, teams, refresh } = useAuth()
  const qc = useQueryClient()
  const toast = useToast()
  const confirm = useConfirm()
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
    if (busy || teamId === team?.id) return
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
    onSuccess: (res) => {
      setInviteEmail('')
      setCreatedInvite(res.invite_url || res.token || null)
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
  const memberList = members.data?.members || []
  const pending = invitations.data?.invitations || []

  if (members.isLoading || invitations.isLoading) return <PageSkeleton cards={2} />

  return (
    <div className="space-y-5">
      <div className="flex min-w-0 items-center gap-2.5">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-brand-500/10 text-brand-600 dark:text-brand-400">
          <Users className="h-5 w-5" />
        </span>
        <div className="min-w-0">
          <h1 className="text-lg font-semibold tracking-tight text-gray-900 dark:text-white">Team</h1>
          <p className="truncate text-xs text-gray-500 dark:text-gray-400">
            {team?.name || '—'}
            {team?.personal ? ' · personal' : ''}
            {team?.role ? ` · ${team.role}` : ''}
            {user?.email ? ` · ${user.email}` : ''}
          </p>
        </div>
      </div>

      {error ? <p className="text-sm text-error-500">{error}</p> : null}

      <div className="flex gap-2 overflow-x-auto pb-0.5 [-ms-overflow-style:none] [scrollbar-width:none] sm:flex-wrap sm:overflow-visible [&::-webkit-scrollbar]:hidden">
        {teams.map((t) => (
          <TeamChip
            key={t.id}
            team={t}
            active={team?.id === t.id}
            disabled={busy}
            onSelect={() => void switchTo(t.id)}
          />
        ))}
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <div className="panel-card overflow-hidden lg:col-span-2">
          <div className="flex items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-gray-800">
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Members</h2>
            <span className="text-xs text-gray-500 dark:text-gray-400">{memberList.length}</span>
          </div>
          <ul className="divide-y divide-gray-200 dark:divide-gray-800">
            {memberList.map((m) => (
              <li key={m.user_id} className="flex items-center gap-3 px-4 py-3">
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-brand-500/15 text-xs font-semibold text-brand-700 dark:text-brand-300">
                  {initials(m.name, m.email)}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium text-gray-900 dark:text-white">
                    {m.name || m.email}
                    {m.user_id === user?.id ? (
                      <span className="ml-1.5 text-xs font-normal text-gray-400">you</span>
                    ) : null}
                  </div>
                  <div className="truncate text-xs text-gray-500 dark:text-gray-400">{m.email}</div>
                </div>
                <RoleChip role={m.role} />
                {canManage && m.user_id !== user?.id ? (
                  <button
                    type="button"
                    className="shrink-0 text-xs text-error-500 hover:underline"
                    onClick={async () => {
                      const ok = await confirm({
                        title: 'Remove member',
                        message: `Remove ${m.email} from this team?`,
                        confirmLabel: 'Remove',
                        danger: true,
                      })
                      if (!ok) return
                      await api.removeTeamMember(m.user_id)
                      void qc.invalidateQueries({ queryKey: ['team-members'] })
                    }}
                  >
                    Remove
                  </button>
                ) : null}
              </li>
            ))}
            {!memberList.length ? (
              <li className="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">No members.</li>
            ) : null}
          </ul>
        </div>

        <div className="space-y-4">
          <form
            className="panel-card space-y-3 p-4"
            onSubmit={(e) => {
              e.preventDefault()
              if (!newTeamName.trim()) return
              createTeam.mutate()
            }}
          >
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">New team</h2>
            <input
              value={newTeamName}
              onChange={(e) => setNewTeamName(e.target.value)}
              placeholder="Name"
              required
              className="panel-field h-9 w-full rounded-md px-3 text-sm"
            />
            <input
              value={newTeamDesc}
              onChange={(e) => setNewTeamDesc(e.target.value)}
              placeholder="Description (optional)"
              className="panel-field h-9 w-full rounded-md px-3 text-sm"
            />
            {createTeam.error ? (
              <p className="text-sm text-error-500">{createTeam.error.message}</p>
            ) : null}
            <Btn primary type="submit">
              <span className="inline-flex items-center gap-1">
                <Plus className="h-3.5 w-3.5" />
                {createTeam.isPending ? 'Creating…' : 'Create'}
              </span>
            </Btn>
          </form>

          <form
            className="panel-card space-y-3 p-4"
            onSubmit={(e) => {
              e.preventDefault()
              if (!acceptToken.trim()) return
              accept.mutate()
            }}
          >
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Join with token</h2>
            <input
              value={acceptToken}
              onChange={(e) => setAcceptToken(e.target.value)}
              placeholder="Invite token"
              className="panel-field h-9 w-full rounded-md px-3 font-mono text-sm"
            />
            {accept.error ? <p className="text-sm text-error-500">{accept.error.message}</p> : null}
            <Btn primary type="submit">
              {accept.isPending ? 'Joining…' : 'Accept'}
            </Btn>
          </form>
        </div>
      </div>

      {canManage ? (
        <div className="panel-card overflow-hidden">
          <div className="border-b border-gray-200 px-4 py-3 dark:border-gray-800">
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Invitations</h2>
            <form
              className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center"
              onSubmit={(e) => {
                e.preventDefault()
                invite.mutate()
              }}
            >
              <label className="relative min-w-0 flex-1">
                <Mail className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-gray-400" />
                <input
                  type="email"
                  required
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  placeholder="email@example.com"
                  className="panel-field h-9 w-full rounded-md py-1.5 pr-3 pl-8 text-sm"
                />
              </label>
              <select
                value={inviteRole}
                onChange={(e) => setInviteRole(e.target.value)}
                className="panel-field h-9 rounded-md px-2 text-sm sm:w-28"
              >
                <option value="member">Member</option>
                <option value="admin">Admin</option>
              </select>
              <Btn primary type="submit">
                {invite.isPending ? 'Sending…' : 'Invite'}
              </Btn>
            </form>
            {invite.error ? <p className="mt-2 text-sm text-error-500">{invite.error.message}</p> : null}
            {createdInvite ? (
              <div className="mt-3 flex items-start gap-2 rounded-lg border border-brand-200 bg-brand-50 px-3 py-2 dark:border-brand-500/30 dark:bg-brand-500/10">
                <code className="min-w-0 flex-1 break-all font-mono text-xs text-gray-800 dark:text-gray-200">
                  {createdInvite}
                </code>
                <button
                  type="button"
                  className="inline-flex shrink-0 items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-brand-700 hover:bg-white/60 dark:text-brand-300 dark:hover:bg-white/5"
                  onClick={() => {
                    void navigator.clipboard.writeText(createdInvite).then(
                      () => toast.success('Invite link copied'),
                      () => toast.error('Could not copy'),
                    )
                  }}
                >
                  <Copy className="h-3.5 w-3.5" />
                  Copy
                </button>
              </div>
            ) : null}
          </div>
          {pending.length ? (
            <ul className="divide-y divide-gray-200 dark:divide-gray-800">
              {pending.map((inv) => (
                <li key={inv.id} className="flex flex-wrap items-center gap-2 px-4 py-3">
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-sm text-gray-900 dark:text-white">{inv.email}</div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                      Expires {new Date(inv.expires_at).toLocaleDateString()}
                    </div>
                  </div>
                  <RoleChip role={inv.role} />
                  <button
                    type="button"
                    className="text-xs text-error-500 hover:underline"
                    onClick={async () => {
                      const ok = await confirm({
                        title: 'Revoke invitation',
                        message: `Revoke the invite for ${inv.email}?`,
                        confirmLabel: 'Revoke',
                        danger: true,
                      })
                      if (!ok) return
                      await api.deleteInvitation(inv.id)
                      void qc.invalidateQueries({ queryKey: ['team-invitations'] })
                    }}
                  >
                    Revoke
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">No pending invitations.</p>
          )}
        </div>
      ) : null}
    </div>
  )
}

function TeamChip({
  team: t,
  active,
  disabled,
  onSelect,
}: {
  team: Team
  active: boolean
  disabled: boolean
  onSelect: () => void
}) {
  return (
    <button
      type="button"
      disabled={disabled || active}
      onClick={onSelect}
      className={`inline-flex max-w-[85vw] shrink-0 items-center gap-2 rounded-lg border px-3 py-2 text-left transition sm:max-w-xs ${
        active
          ? 'border-brand-500 bg-brand-50 dark:border-brand-400 dark:bg-brand-500/10'
          : 'border-gray-200 hover:bg-gray-50 disabled:opacity-50 dark:border-gray-800 dark:hover:bg-white/5'
      }`}
    >
      <span
        className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-[11px] font-semibold ${
          active
            ? 'bg-brand-500 text-white'
            : 'bg-gray-100 text-gray-600 dark:bg-white/10 dark:text-gray-300'
        }`}
      >
        {active ? <Check className="h-3.5 w-3.5" /> : initials(t.name, t.name)}
      </span>
      <span className="min-w-0">
        <span className="block truncate text-sm font-medium text-gray-900 dark:text-white">{t.name}</span>
        <span className="block truncate text-[11px] text-gray-500 dark:text-gray-400">
          {t.personal ? 'Personal' : 'Shared'}
          {t.role ? ` · ${t.role}` : ''}
        </span>
      </span>
    </button>
  )
}
