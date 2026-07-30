import { useState, useEffect } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'

function LoginPageInner() {
  const nav = useNavigate()
  const { refresh, user, loading } = useAuth()
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
    <AuthLayout title="Welcome back" subtitle="Sign in to your self-hosted control plane.">
      <form onSubmit={onSubmit} className="space-y-4">
        <Field label="Email" type="email" value={email} onChange={setEmail} />
        <Field label="Password" type="password" value={password} onChange={setPassword} />
        {error && <p className="text-sm text-[var(--color-danger)]">{error}</p>}
        <button
          type="submit"
          disabled={busy}
          className="w-full rounded-lg bg-[var(--color-accent)] px-4 py-2.5 font-medium text-[var(--color-ink)] transition hover:bg-[var(--color-accent-2)] disabled:opacity-60"
        >
          {busy ? 'Signing in…' : 'Sign in'}
        </button>
      </form>
      <p className="mt-6 text-sm text-[var(--color-muted)]">
        No account?{' '}
        <Link to="/register" className="text-[var(--color-accent)]">
          Create one
        </Link>
      </p>
    </AuthLayout>
  )
}

export function LoginPage() {
  return <LoginPageInner />
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
    <AuthLayout title="Create your Goolify" subtitle="Full open-source PaaS. No cloud lock-in.">
      <form onSubmit={onSubmit} className="space-y-4">
        <Field label="Name" value={name} onChange={setName} />
        <Field label="Email" type="email" value={email} onChange={setEmail} />
        <Field label="Password" type="password" value={password} onChange={setPassword} />
        {error && <p className="text-sm text-[var(--color-danger)]">{error}</p>}
        <button
          type="submit"
          disabled={busy}
          className="w-full rounded-lg bg-[var(--color-accent)] px-4 py-2.5 font-medium text-[var(--color-ink)] transition hover:bg-[var(--color-accent-2)] disabled:opacity-60"
        >
          {busy ? 'Creating…' : 'Create account'}
        </button>
      </form>
      <p className="mt-6 text-sm text-[var(--color-muted)]">
        Already have an account?{' '}
        <Link to="/login" className="text-[var(--color-accent)]">
          Sign in
        </Link>
      </p>
    </AuthLayout>
  )
}

function AuthLayout({
  title,
  subtitle,
  children,
}: {
  title: string
  subtitle: string
  children: React.ReactNode
}) {
  return (
    <div className="mx-auto grid min-h-screen max-w-6xl items-center gap-10 px-4 py-10 lg:grid-cols-2">
      <div>
        <p className="mb-3 text-sm uppercase tracking-[0.2em] text-[var(--color-accent)]">Goolify</p>
        <h1 className="max-w-md text-4xl font-semibold tracking-tight text-[var(--color-text)] md:text-5xl">
          Deploy on your servers.
        </h1>
        <p className="mt-4 max-w-md text-[var(--color-muted)]">
          A modern Coolify alternative — Go control plane, React dashboard, PostgreSQL. SSH + Docker,
          no vendor lock-in.
        </p>
      </div>
      <div className="rounded-2xl border border-[var(--color-line)] bg-[var(--color-panel)]/90 p-6 shadow-xl backdrop-blur">
        <h2 className="text-xl font-semibold">{title}</h2>
        <p className="mt-1 mb-6 text-sm text-[var(--color-muted)]">{subtitle}</p>
        {children}
      </div>
    </div>
  )
}

function Field({
  label,
  value,
  onChange,
  type = 'text',
}: {
  label: string
  value: string
  onChange: (v: string) => void
  type?: string
}) {
  return (
    <label className="block text-sm">
      <span className="mb-1.5 block text-[var(--color-muted)]">{label}</span>
      <input
        type={type}
        required
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2 outline-none ring-[var(--color-accent)] focus:ring-1"
      />
    </label>
  )
}
