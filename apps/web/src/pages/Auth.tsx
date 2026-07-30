import { useState, useEffect, type FormEvent, type ReactNode } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { Eye, EyeOff, Lock, Mail, User } from 'lucide-react'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'
import { appConfig } from '../config/app.config'
import { ThemeToggle } from '../components/theme/ThemeToggle'

const inputClass =
  'h-9 w-full rounded-md border border-gray-200 bg-white pl-9 pr-3 text-sm text-gray-900 shadow-sm transition placeholder:text-gray-400 focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20 focus:outline-none dark:border-gray-700 dark:bg-gray-900 dark:text-white'

function BrandPanel({ className = '' }: { className?: string }) {
  const { brand } = appConfig
  return (
    <div
      className={`relative flex flex-col items-center justify-center overflow-hidden bg-brand-950 px-8 py-12 text-center sm:px-10 ${className}`}
    >
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute -top-24 -left-24 h-72 w-72 rounded-full bg-brand-500/35 blur-3xl" />
        <div className="absolute -right-16 bottom-0 h-80 w-80 rounded-full bg-brand-600/25 blur-3xl" />
      </div>
      <div className="relative z-10 w-full max-w-md">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-brand-500 text-xl font-bold text-white shadow-lg shadow-brand-500/25 sm:h-16 sm:w-16 sm:text-2xl">
          {brand.letter}
        </div>
        <h2 className="mt-6 text-2xl font-semibold tracking-tight text-white sm:text-3xl">{brand.name}</h2>
        <p className="mt-3 text-sm leading-relaxed text-brand-100/90 sm:text-base">{brand.loginDescription}</p>
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
    <div className="relative grid min-h-dvh w-full lg:grid-cols-2">
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

  useEffect(() => {
    if (!loading && user) void nav({ to: '/dashboard' })
  }, [loading, user, nav])

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

  return (
    <AuthShell>
      <div className="mb-5 flex h-9 w-9 items-center justify-center rounded-lg bg-brand-500 text-sm font-bold text-white lg:hidden">
        {appConfig.brand.letter}
      </div>
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
              className="absolute top-1/2 right-3 -translate-y-1/2 rounded-md p-0.5 text-gray-500 hover:bg-gray-100 dark:hover:bg-white/10"
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
      <p className="mt-6 text-center text-sm text-gray-500">
        No account?{' '}
        <Link to="/register" className="font-medium text-brand-600 hover:text-brand-500">
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

  useEffect(() => {
    if (!loading && user) void nav({ to: '/dashboard' })
  }, [loading, user, nav])

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

  return (
    <AuthShell>
      <div className="mb-5 flex h-9 w-9 items-center justify-center rounded-lg bg-brand-500 text-sm font-bold text-white lg:hidden">
        {appConfig.brand.letter}
      </div>
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
      <p className="mt-6 text-center text-sm text-gray-500">
        Already have an account?{' '}
        <Link to="/login" className="font-medium text-brand-600 hover:text-brand-500">
          Sign in
        </Link>
      </p>
    </AuthShell>
  )
}
