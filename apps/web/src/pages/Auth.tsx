import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Link, Navigate, useNavigate } from '@tanstack/react-router'
import { Eye, EyeOff, Lock, Mail, User } from 'lucide-react'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'
import { appConfig } from '../config/app.config'
import { BrandLogo } from '../components/BrandLogo'
import { ThemeToggle } from '../components/theme/ThemeToggle'
import { AuthFormSkeleton } from '../components/ui/Skeleton'

const inputClass =
  'panel-field h-9 w-full rounded-md pl-9 pr-3 text-sm shadow-sm transition placeholder:text-gray-400 focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20 focus:outline-none'

function BrandPanel({ className = '' }: { className?: string }) {
  const { brand } = appConfig
  return (
    <div
      className={`relative flex flex-col items-center justify-center overflow-hidden bg-brand-950 px-8 py-12 text-center sm:px-10 ${className}`}
    >
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute -top-24 -left-24 h-72 w-72 rounded-full bg-brand-500/35 blur-3xl" />
        <div className="absolute -right-16 bottom-0 h-80 w-80 rounded-full bg-brand-600/25 blur-3xl" />
        <div className="absolute top-1/2 left-1/2 h-48 w-48 -translate-x-1/2 -translate-y-1/2 rounded-full bg-brand-400/10 blur-2xl" />
      </div>
      <div className="relative z-10 w-full max-w-md">
        <BrandLogo
          variant="wordmark"
          forceTheme="dark"
          className="mx-auto h-40 w-auto max-w-[280px] rounded-2xl drop-shadow-lg sm:h-48 sm:max-w-[320px]"
        />
        <h2 className="sr-only">{brand.name}</h2>
        <p className="mt-6 text-sm leading-relaxed text-brand-100/90 sm:text-base">{brand.loginDescription}</p>
        <ul className="mt-8 space-y-3 text-left text-sm text-brand-100/85">
          {brand.loginFeatures.map((item) => (
            <li key={item} className="flex items-center gap-3">
              <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-brand-500/30 text-xs text-brand-200">
                ✓
              </span>
              {item}
            </li>
          ))}
        </ul>
        <p className="mt-10 text-xs text-brand-200/70">
          {brand.copyright} · {brand.license} License
        </p>
      </div>
    </div>
  )
}

function AuthShell({ children }: { children: ReactNode }) {
  return (
    <div className="relative grid min-h-dvh w-full bg-white dark:bg-gray-900 lg:grid-cols-2">
      <div className="absolute top-4 right-4 z-10 sm:top-6 sm:right-6">
        <ThemeToggle />
      </div>
      <div className="flex min-h-dvh flex-col justify-center px-5 py-10 sm:px-10 lg:px-14 xl:px-16">
        <div className="mx-auto w-full max-w-[400px]">{children}</div>
      </div>
      <BrandPanel className="hidden min-h-dvh lg:flex" />
    </div>
  )
}

function useRegistrationEnabled() {
  const [enabled, setEnabled] = useState<boolean | null>(null)

  useEffect(() => {
    let cancelled = false
    void api
      .registrationStatus()
      .then((res) => {
        if (!cancelled) setEnabled(res.registration_enabled)
      })
      .catch(() => {
        // Fail open so a status blip never hides first-time signup; POST /register is the real gate.
        if (!cancelled) setEnabled(true)
      })
    return () => {
      cancelled = true
    }
  }, [])

  return enabled
}

export function LoginPage() {
  const nav = useNavigate()
  const { refresh, user, loading } = useAuth()
  const registrationEnabled = useRegistrationEnabled()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [challengeId, setChallengeId] = useState(() => {
    try {
      return new URLSearchParams(window.location.search).get('challenge_id') || ''
    } catch {
      return ''
    }
  })
  const [otp, setOtp] = useState('')
  const [recovery, setRecovery] = useState('')
  const [providers, setProviders] = useState<Array<{ provider: string; name?: string }>>([])

  useEffect(() => {
    let cancelled = false
    void api
      .oauthProviders()
      .then((res) => {
        if (!cancelled) setProviders(res.providers || [])
      })
      .catch(() => {
        if (!cancelled) setProviders([])
      })
    return () => {
      cancelled = true
    }
  }, [])

  const completeLogin = async () => {
    await refresh()
    let next = '/dashboard'
    try {
      const redir = new URLSearchParams(window.location.search).get('redirect') || ''
      if (redir.startsWith('/') && !redir.startsWith('//')) next = redir
    } catch {
      /* ignore */
    }
    if (next !== '/dashboard') {
      window.location.assign(next)
      return
    }
    void nav({ to: '/dashboard' })
  }

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      if (challengeId) {
        await api.login2FA({
          challenge_id: challengeId,
          code: otp || undefined,
          recovery_code: recovery || undefined,
        })
        await completeLogin()
        return
      }
      const res = await api.login(email, password)
      if (res.status === '2fa_required' && res.challenge_id) {
        setChallengeId(res.challenge_id)
        return
      }
      await completeLogin()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setBusy(false)
    }
  }

  if (loading) {
    return (
      <AuthShell>
        <AuthFormSkeleton />
      </AuthShell>
    )
  }
  if (user) {
    let next = '/dashboard'
    try {
      const redir = new URLSearchParams(window.location.search).get('redirect') || ''
      if (redir.startsWith('/') && !redir.startsWith('//')) next = redir
    } catch {
      /* ignore */
    }
    if (next !== '/dashboard') {
      window.location.assign(next)
      return null
    }
    return <Navigate to="/dashboard" />
  }

  return (
    <AuthShell>
      <BrandLogo className="mb-5 h-10 w-10 rounded-lg lg:hidden" />
      <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
        {challengeId ? 'Two-factor authentication' : 'Sign in'}
      </h1>
      {challengeId && (
        <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
          Enter a code from your authenticator app, or a recovery code.
        </p>
      )}
      <form onSubmit={onSubmit} className="mt-6 space-y-4">
        {!challengeId ? (
          <>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Email</label>
              <div className="relative">
                <Mail className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-gray-400" />
                <input
                  type="email"
                  required
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className={inputClass}
                />
              </div>
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Password</label>
              <div className="relative">
                <Lock className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-gray-400" />
                <input
                  type={showPassword ? 'text' : 'password'}
                  required
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className={`${inputClass} pr-11`}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((v) => !v)}
                  className="absolute top-1/2 right-3 -translate-y-1/2 rounded-md p-0.5 text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-white/10"
                  aria-label={showPassword ? 'Hide password' : 'Show password'}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </div>
          </>
        ) : (
          <>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">
                Authenticator code
              </label>
              <input
                inputMode="numeric"
                autoComplete="one-time-code"
                value={otp}
                onChange={(e) => setOtp(e.target.value)}
                className={inputClass.replace('pl-9', 'pl-3')}
                placeholder="123456"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">
                Or recovery code
              </label>
              <input
                value={recovery}
                onChange={(e) => setRecovery(e.target.value)}
                className={inputClass.replace('pl-9', 'pl-3')}
              />
            </div>
          </>
        )}
        {error && (
          <p className="text-sm text-error-500" role="alert">
            {error}
          </p>
        )}
        <button
          type="submit"
          disabled={busy}
          className="flex h-9 w-full items-center justify-center rounded-md bg-brand-500 text-sm font-medium text-white shadow-sm transition hover:bg-brand-600 focus:ring-2 focus:ring-brand-500/30 focus:outline-none disabled:cursor-not-allowed disabled:opacity-60"
        >
          {busy ? 'Signing in…' : challengeId ? 'Verify' : 'Sign in'}
        </button>
      </form>
      {!challengeId && (
        <>
          <p className="mt-3 text-center text-sm">
            <Link
              to="/forgot-password"
              className="font-medium text-brand-600 hover:text-brand-500 dark:text-brand-400"
            >
              Forgot password?
            </Link>
          </p>
          {providers.length > 0 && (
            <div className="mt-6 space-y-2">
              <p className="text-center text-xs uppercase tracking-wide text-gray-400">Or continue with</p>
              <div className="flex flex-col gap-2">
                {providers.map((p) => (
                  <a
                    key={p.provider}
                    href={`/api/v1/auth/oauth/${p.provider}/start`}
                    className="flex h-9 items-center justify-center rounded-md border border-gray-200 text-sm font-medium text-gray-800 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-100 dark:hover:bg-gray-800"
                  >
                    {p.name || p.provider}
                  </a>
                ))}
              </div>
            </div>
          )}
          {registrationEnabled ? (
            <p className="mt-6 text-center text-sm text-gray-500 dark:text-gray-400">
              No account?{' '}
              <Link
                to="/register"
                className="font-medium text-brand-600 hover:text-brand-500 dark:text-brand-400 dark:hover:text-brand-300"
              >
                Create one
              </Link>
            </p>
          ) : registrationEnabled === false ? (
            <p className="mt-6 text-center text-sm text-gray-500 dark:text-gray-400">
              Registration is closed. Ask an admin to invite you.
            </p>
          ) : null}
        </>
      )}
    </AuthShell>
  )
}

export function RegisterPage() {
  const nav = useNavigate()
  const { refresh, user, loading } = useAuth()
  const registrationEnabled = useRegistrationEnabled()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await api.register(name, email, password)
      await refresh()
      void nav({ to: '/dashboard' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed')
    } finally {
      setBusy(false)
    }
  }

  // Wait for session check so logged-in users never flash the register form
  if (loading || registrationEnabled === null) {
    return (
      <AuthShell>
        <AuthFormSkeleton />
      </AuthShell>
    )
  }
  if (user) return <Navigate to="/dashboard" />
  if (!registrationEnabled) return <Navigate to="/login" />

  return (
    <AuthShell>
      <BrandLogo className="mb-5 h-10 w-10 rounded-lg lg:hidden" />
      <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
        Create account
      </h1>
      <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
        First account becomes the admin. Registration closes automatically after signup.
      </p>
      <form onSubmit={onSubmit} className="mt-6 space-y-4">
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Name</label>
          <div className="relative">
            <User className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-gray-400" />
            <input
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              className={inputClass}
            />
          </div>
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Email</label>
          <div className="relative">
            <Mail className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-gray-400" />
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className={inputClass}
            />
          </div>
        </div>
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Password</label>
          <div className="relative">
            <Lock className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-gray-400" />
            <input
              type="password"
              required
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={inputClass}
            />
          </div>
        </div>
        {error && (
          <p className="text-sm text-error-500" role="alert">
            {error}
          </p>
        )}
        <button
          type="submit"
          disabled={busy}
          className="flex h-9 w-full items-center justify-center rounded-md bg-brand-500 text-sm font-medium text-white shadow-sm transition hover:bg-brand-600 disabled:opacity-60"
        >
          {busy ? 'Creating…' : 'Create account'}
        </button>
      </form>
      <p className="mt-6 text-center text-sm text-gray-500 dark:text-gray-400">
        Already have an account?{' '}
        <Link to="/login" className="font-medium text-brand-600 hover:text-brand-500 dark:text-brand-400 dark:hover:text-brand-300">
          Sign in
        </Link>
      </p>
    </AuthShell>
  )
}

export function ForgotPasswordPage() {
  const [email, setEmail] = useState('')
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)
  const [busy, setBusy] = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await api.forgotPassword(email)
      setDone(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Request failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthShell>
      <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
        Reset password
      </h1>
      <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
        We will email a reset link if that address has an account and mail is configured.
      </p>
      {done ? (
        <p className="mt-6 text-sm text-gray-700 dark:text-gray-200">
          If an account exists, a reset email is on the way.{' '}
          <Link to="/login" className="font-medium text-brand-600 dark:text-brand-400">
            Back to sign in
          </Link>
        </p>
      ) : (
        <form onSubmit={onSubmit} className="mt-6 space-y-4">
          <div>
            <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">Email</label>
            <div className="relative">
              <Mail className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-gray-400" />
              <input
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className={inputClass}
              />
            </div>
          </div>
          {error && <p className="text-sm text-error-500">{error}</p>}
          <button
            type="submit"
            disabled={busy}
            className="flex h-9 w-full items-center justify-center rounded-md bg-brand-500 text-sm font-medium text-white disabled:opacity-60"
          >
            {busy ? 'Sending…' : 'Send reset link'}
          </button>
        </form>
      )}
    </AuthShell>
  )
}

export function ResetPasswordPage() {
  const nav = useNavigate()
  const token = (() => {
    try {
      return new URLSearchParams(window.location.search).get('token') || ''
    } catch {
      return ''
    }
  })()
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!token) {
      setError('Missing reset token')
      return
    }
    setBusy(true)
    setError('')
    try {
      await api.resetPassword(token, password)
      void nav({ to: '/login' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Reset failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthShell>
      <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
        Choose a new password
      </h1>
      <form onSubmit={onSubmit} className="mt-6 space-y-4">
        <div>
          <label className="mb-1 block text-xs font-medium text-gray-700 dark:text-gray-300">New password</label>
          <div className="relative">
            <Lock className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-gray-400" />
            <input
              type="password"
              required
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className={inputClass}
            />
          </div>
        </div>
        {error && <p className="text-sm text-error-500">{error}</p>}
        <button
          type="submit"
          disabled={busy || !token}
          className="flex h-9 w-full items-center justify-center rounded-md bg-brand-500 text-sm font-medium text-white disabled:opacity-60"
        >
          {busy ? 'Saving…' : 'Update password'}
        </button>
      </form>
    </AuthShell>
  )
}
