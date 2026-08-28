import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'
import { BrandLogo } from '../components/BrandLogo'
import { ThemeToggle } from '../components/theme/ThemeToggle'
import { PanelSkeleton } from '../components/ui/Skeleton'
import { Btn } from './Servers'

function inviteToken() {
  try {
    return new URLSearchParams(window.location.search).get('token') || ''
  } catch {
    return ''
  }
}

export function InvitePage() {
  const nav = useNavigate()
  const { user, loading, refresh } = useAuth()
  const token = inviteToken()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const preview = useQuery({
    queryKey: ['invite-preview', token],
    queryFn: () => api.previewInvitation(token),
    enabled: !!token,
    retry: false,
  })

  useEffect(() => {
    if (!token) setError('Missing invite token')
  }, [token])

  const onAccept = async (e: FormEvent) => {
    e.preventDefault()
    if (!token) return
    setBusy(true)
    setError('')
    try {
      await api.acceptInvitation(token)
      await refresh()
      void nav({ to: '/dashboard' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not accept invitation')
    } finally {
      setBusy(false)
    }
  }

  const loginHref = `/login?redirect=${encodeURIComponent(`/invite?token=${token}`)}`

  return (
    <div className="relative flex min-h-dvh items-center justify-center bg-white px-5 py-12 dark:bg-gray-900">
      <div className="absolute top-4 right-4">
        <ThemeToggle />
      </div>
      <div className="w-full max-w-md space-y-5">
        <BrandLogo className="h-10 w-10 rounded-lg" />
        <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">
          Team invitation
        </h1>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Confirm below to join. Opening this page does not use up the invite — email previews and
          chat unfurls are safe.
        </p>
        {preview.isLoading ? (
          <PanelSkeleton rows={3} showHeader={false} />
        ) : null}
        {preview.error && (
          <p className="text-sm text-error-500">
            {preview.error.message || 'This invite is invalid or expired.'}
          </p>
        )}
        {preview.data?.invitation && (
          <div className="panel-card space-y-1 p-4 text-sm">
            <p>
              <span className="text-gray-500">Team</span>{' '}
              <span className="font-medium text-gray-900 dark:text-white">
                {preview.data.invitation.team_name}
              </span>
            </p>
            <p>
              <span className="text-gray-500">Role</span> {preview.data.invitation.role}
            </p>
            <p>
              <span className="text-gray-500">Invited email</span> {preview.data.invitation.email}
            </p>
          </div>
        )}
        {error && <p className="text-sm text-error-500">{error}</p>}
        {loading ? (
          preview.isLoading ? null : <PanelSkeleton rows={1} showHeader={false} />
        ) : user ? (
          <form onSubmit={(e) => void onAccept(e)}>
            <Btn primary type="submit" disabled={busy || !preview.data?.invitation}>
              {busy ? 'Joining…' : 'Accept invitation'}
            </Btn>
          </form>
        ) : (
          <p className="text-sm text-gray-600 dark:text-gray-300">
            Sign in with the invited email, then come back to confirm.{' '}
            <a href={loginHref} className="text-brand-600 hover:underline dark:text-brand-400">
              Sign in
            </a>
          </p>
        )}
      </div>
    </div>
  )
}
