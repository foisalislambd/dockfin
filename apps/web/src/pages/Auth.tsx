import { useState, type FormEvent, type ReactNode } from 'react'
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

export function LoginPage() {
  const nav = useNavigate()
  const { refresh, user, loading } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await api.login(email, password)
      await refresh()
      void nav({ to: '/dashboard' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setBusy(false)
    }
  }

  // Wait for session check so logged-in users never flash the sign-in form
  if (loading) {
    return (
      <AuthShell>
        <AuthFormSkeleton />
      </AuthShell>
    )
  }
  if (user) return <Navigate to="/dashboard" />

  return (
    <AuthShell>
      <BrandLogo className="mb-5 h-10 w-10 rounded-lg lg:hidden" />
      <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
        Sign in
      </h1>
      <form onSubmit={onSubmit} className="mt-6 space-y-4">
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
          {busy ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
      <p className="mt-6 text-center text-sm text-gray-500 dark:text-gray-400">
        No account?{' '}
        <Link to="/register" className="font-medium text-brand-600 hover:text-brand-500 dark:text-brand-400 dark:hover:text-brand-300">
          Create one
        </Link>
      </p>
    </AuthShell>
  )
}

export function RegisterPage() {
  const nav = useNavigate()
  const { refresh, user, loading } = useAuth()
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
  if (loading) {
    return (
      <AuthShell>
        <AuthFormSkeleton />
      </AuthShell>
    )
  }
  if (user) return <Navigate to="/dashboard" />

  return (
    <AuthShell>
      <BrandLogo className="mb-5 h-10 w-10 rounded-lg lg:hidden" />
      <h1 className="text-xl font-semibold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
        Create account
      </h1>
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
