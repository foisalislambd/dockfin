import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { api, fetchAllEnvironments, LAST_ENV_KEY } from '../lib/api'
import { Btn, Header, Input, Modal } from './Servers'

export function ProjectsPage() {
  const qc = useQueryClient()
  const projects = useQuery({ queryKey: ['projects'], queryFn: api.projects })
  const [show, setShow] = useState(false)
  const [name, setName] = useState('')
  const create = useMutation({
    mutationFn: () => api.createProject(name),
    onSuccess: (data) => {
      localStorage.setItem(LAST_ENV_KEY, data.environment.id)
      void qc.invalidateQueries({ queryKey: ['projects'] })
      void qc.invalidateQueries({ queryKey: ['all-environments'] })
      setShow(false)
      setName('')
    },
  })

  return (
    <div className="space-y-6">
      <Header
        title="Projects"
        subtitle="Group applications, databases, and services by environment."
        actions={
          <Btn primary onClick={() => setShow(true)}>
            New project
          </Btn>
        }
      />
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {(projects.data?.projects || []).map((p) => (
          <div key={p.id} className="rounded-xl border border-[var(--color-line)] bg-[var(--color-panel)]/60 p-5">
            <div className="font-medium">{p.name}</div>
            <div className="mt-1 text-sm text-[var(--color-muted)]">{p.description || 'No description'}</div>
          </div>
        ))}
      </div>
      {show && (
        <Modal title="Create project" onClose={() => setShow(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <Input label="Name" value={name} onChange={setName} />
            {create.error && <p className="text-sm text-[var(--color-danger)]">{create.error.message}</p>}
            <Btn primary type="submit">
              Create
            </Btn>
          </form>
        </Modal>
      )}
    </div>
  )
}

export function ApplicationsPage() {
  const qc = useQueryClient()
  const apps = useQuery({ queryKey: ['applications'], queryFn: () => api.applications() })
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments })
  const [show, setShow] = useState(false)
  const [form, setForm] = useState({
    name: '',
    environment_id: '',
    destination_id: '',
    build_pack: 'dockerimage',
    docker_registry_image_name: 'nginx',
    docker_registry_image_tag: 'alpine',
    ports_exposes: '80',
    fqdn: '',
    git_repository: '',
  })

  useEffect(() => {
    if (!show) return
    const saved = localStorage.getItem(LAST_ENV_KEY) || ''
    const list = envs.data || []
    const pick =
      (saved && list.find((e) => e.id === saved)?.id) ||
      list[0]?.id ||
      saved ||
      ''
    setForm((f) => ({
      ...f,
      environment_id: f.environment_id || pick,
      destination_id: f.destination_id || dests.data?.destinations?.[0]?.id || '',
    }))
  }, [show, envs.data, dests.data])

  const create = useMutation({
    mutationFn: () => api.createApplication(form),
    onSuccess: (app) => {
      if (form.environment_id) localStorage.setItem(LAST_ENV_KEY, form.environment_id)
      void qc.invalidateQueries({ queryKey: ['applications'] })
      setShow(false)
      setForm((f) => ({ ...f, name: '', fqdn: '' }))
      return app
    },
  })

  return (
    <div className="space-y-6">
      <Header
        title="Applications"
        subtitle="Dockerfile, compose, nixpacks, and image deploys."
        actions={
          <Btn primary onClick={() => setShow(true)}>
            New application
          </Btn>
        }
      />
      <div className="overflow-hidden rounded-xl border border-[var(--color-line)]">
        <table className="w-full text-left text-sm">
          <thead className="bg-[var(--color-panel)] text-[var(--color-muted)]">
            <tr>
              <th className="px-4 py-3">Name</th>
              <th className="px-4 py-3">Build</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3">Domain</th>
              <th className="px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {(apps.data?.applications || []).map((a) => (
              <tr key={a.id} className="border-t border-[var(--color-line)]">
                <td className="px-4 py-3">
                  <Link to="/applications/$appId" params={{ appId: a.id }} className="text-[var(--color-accent)] hover:underline">
                    {a.name}
                  </Link>
                </td>
                <td className="px-4 py-3 font-mono text-xs">{a.build_pack}</td>
                <td className="px-4 py-3">{a.status}</td>
                <td className="px-4 py-3">{a.fqdn || '—'}</td>
                <td className="px-4 py-3 space-x-3">
                  <Link to="/applications/$appId" params={{ appId: a.id }} className="text-[var(--color-muted)] hover:text-[var(--color-accent)]">
                    Open
                  </Link>
                  <button
                    type="button"
                    className="text-[var(--color-accent)]"
                    onClick={() =>
                      void api.deployApplication(a.id).then(() => qc.invalidateQueries({ queryKey: ['applications'] }))
                    }
                  >
                    Deploy
                  </button>
                </td>
              </tr>
            ))}
            {!apps.data?.applications?.length && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-[var(--color-muted)]">
                  No applications yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {show && (
        <Modal title="New application" onClose={() => setShow(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <Input label="Name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} />
            <label className="block text-sm">
              <span className="mb-1 block text-[var(--color-muted)]">Environment</span>
              <select
                required
                value={form.environment_id}
                onChange={(e) => setForm({ ...form, environment_id: e.target.value })}
                className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2"
              >
                <option value="">Select…</option>
                {(envs.data || []).map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.project_name} / {e.name}
                  </option>
                ))}
              </select>
              {!envs.data?.length && (
                <span className="mt-1 block text-xs text-[var(--color-muted)]">
                  Create a project first to get an environment.
                </span>
              )}
            </label>
            <label className="block text-sm">
              <span className="mb-1 block text-[var(--color-muted)]">Destination</span>
              <select
                required
                value={form.destination_id}
                onChange={(e) => setForm({ ...form, destination_id: e.target.value })}
                className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2"
              >
                <option value="">Select…</option>
                {(dests.data?.destinations || []).map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name} ({d.network})
                  </option>
                ))}
              </select>
            </label>
            <label className="block text-sm">
              <span className="mb-1 block text-[var(--color-muted)]">Build pack</span>
              <select
                value={form.build_pack}
                onChange={(e) => setForm({ ...form, build_pack: e.target.value })}
                className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2"
              >
                <option value="dockerimage">Docker Image</option>
                <option value="dockerfile">Dockerfile</option>
                <option value="dockercompose">Docker Compose</option>
                <option value="nixpacks">Nixpacks</option>
                <option value="static">Static</option>
              </select>
            </label>
            {form.build_pack === 'dockerimage' ? (
              <>
                <Input
                  label="Image"
                  value={form.docker_registry_image_name}
                  onChange={(v) => setForm({ ...form, docker_registry_image_name: v })}
                />
                <Input
                  label="Tag"
                  value={form.docker_registry_image_tag}
                  onChange={(v) => setForm({ ...form, docker_registry_image_tag: v })}
                />
              </>
            ) : (
              <Input
                label="Git repository"
                value={form.git_repository}
                onChange={(v) => setForm({ ...form, git_repository: v })}
              />
            )}
            <Input label="Port" value={form.ports_exposes} onChange={(v) => setForm({ ...form, ports_exposes: v })} />
            <Input label="FQDN (optional)" value={form.fqdn} onChange={(v) => setForm({ ...form, fqdn: v })} required={false} />
            {create.error && <p className="text-sm text-[var(--color-danger)]">{create.error.message}</p>}
            <Btn primary type="submit">
              Create
            </Btn>
          </form>
        </Modal>
      )}
    </div>
  )
}

export function DatabasesPage() {
  const qc = useQueryClient()
  const dbs = useQuery({ queryKey: ['databases'], queryFn: api.databases })
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments })
  const [show, setShow] = useState(false)
  const [createdPassword, setCreatedPassword] = useState<string | null>(null)
  const [form, setForm] = useState({
    name: '',
    engine: 'postgresql',
    environment_id: '',
    destination_id: '',
  })

  useEffect(() => {
    if (!show) return
    const saved = localStorage.getItem(LAST_ENV_KEY) || ''
    const list = envs.data || []
    const pick =
      (saved && list.find((e) => e.id === saved)?.id) ||
      list[0]?.id ||
      saved ||
      ''
    setForm((f) => ({
      ...f,
      environment_id: f.environment_id || pick,
      destination_id: f.destination_id || dests.data?.destinations?.[0]?.id || '',
    }))
  }, [show, envs.data, dests.data])

  const create = useMutation({
    mutationFn: () => api.createDatabase(form),
    onSuccess: (data) => {
      if (form.environment_id) localStorage.setItem(LAST_ENV_KEY, form.environment_id)
      void qc.invalidateQueries({ queryKey: ['databases'] })
      setCreatedPassword(data.password || null)
      setShow(false)
      setForm((f) => ({ ...f, name: '' }))
    },
  })

  return (
    <div className="space-y-6">
      <Header
        title="Databases"
        subtitle="Postgres, MySQL, MariaDB, MongoDB, Redis, KeyDB, Dragonfly, ClickHouse."
        actions={
          <Btn primary onClick={() => setShow(true)}>
            New database
          </Btn>
        }
      />

      {createdPassword && (
        <div className="rounded-xl border border-[var(--color-warn)]/50 bg-[var(--color-panel)]/80 p-4">
          <div className="text-sm font-medium text-[var(--color-warn)]">Save this password now — it won’t be shown again.</div>
          <code className="mt-2 block break-all font-mono text-sm">{createdPassword}</code>
          <button
            type="button"
            className="mt-3 text-sm text-[var(--color-muted)] hover:text-[var(--color-accent)]"
            onClick={() => setCreatedPassword(null)}
          >
            Dismiss
          </button>
        </div>
      )}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {(dbs.data?.databases || []).map((d) => (
          <div key={d.id} className="rounded-xl border border-[var(--color-line)] bg-[var(--color-panel)]/60 p-5">
            <div className="font-medium">{d.name}</div>
            <div className="mt-1 font-mono text-xs text-[var(--color-muted)]">{d.engine}</div>
            <div className="mt-3 text-sm">{d.status}</div>
            <div className="mt-4 flex gap-2">
              <Btn
                onClick={() =>
                  void api.startDatabase(d.id).then(() => qc.invalidateQueries({ queryKey: ['databases'] }))
                }
              >
                Start
              </Btn>
              <Btn
                onClick={() =>
                  void api.stopDatabase(d.id).then(() => qc.invalidateQueries({ queryKey: ['databases'] }))
                }
              >
                Stop
              </Btn>
            </div>
          </div>
        ))}
      </div>
      {show && (
        <Modal title="New database" onClose={() => setShow(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <Input label="Name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} />
            <label className="block text-sm">
              <span className="mb-1 block text-[var(--color-muted)]">Environment</span>
              <select
                required
                value={form.environment_id}
                onChange={(e) => setForm({ ...form, environment_id: e.target.value })}
                className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2"
              >
                <option value="">Select…</option>
                {(envs.data || []).map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.project_name} / {e.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block text-sm">
              <span className="mb-1 block text-[var(--color-muted)]">Destination</span>
              <select
                required
                value={form.destination_id}
                onChange={(e) => setForm({ ...form, destination_id: e.target.value })}
                className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2"
              >
                <option value="">Select…</option>
                {(dests.data?.destinations || []).map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name} ({d.network})
                  </option>
                ))}
              </select>
            </label>
            <label className="block text-sm">
              <span className="mb-1 block text-[var(--color-muted)]">Engine</span>
              <select
                value={form.engine}
                onChange={(e) => setForm({ ...form, engine: e.target.value })}
                className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2"
              >
                {['postgresql', 'mysql', 'mariadb', 'mongodb', 'redis', 'keydb', 'dragonfly', 'clickhouse'].map((e) => (
                  <option key={e} value={e}>
                    {e}
                  </option>
                ))}
              </select>
            </label>
            {create.error && <p className="text-sm text-[var(--color-danger)]">{create.error.message}</p>}
            <Btn primary type="submit">
              Create
            </Btn>
          </form>
        </Modal>
      )}
    </div>
  )
}

export function ServicesPage() {
  const qc = useQueryClient()
  const services = useQuery({ queryKey: ['services'], queryFn: api.services })
  const templates = useQuery({ queryKey: ['templates'], queryFn: api.templates })
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments })
  const [show, setShow] = useState(false)
  const [deployError, setDeployError] = useState('')
  const [form, setForm] = useState({
    name: '',
    environment_id: '',
    destination_id: '',
    template: 'uptime-kuma',
  })

  useEffect(() => {
    if (!show) return
    const saved = localStorage.getItem(LAST_ENV_KEY) || ''
    const list = envs.data || []
    const pick =
      (saved && list.find((e) => e.id === saved)?.id) ||
      list[0]?.id ||
      saved ||
      ''
    setForm((f) => ({
      ...f,
      environment_id: f.environment_id || pick,
      destination_id: f.destination_id || dests.data?.destinations?.[0]?.id || '',
      template: f.template || templates.data?.templates?.[0]?.type || 'uptime-kuma',
    }))
  }, [show, envs.data, dests.data, templates.data])

  const create = useMutation({
    mutationFn: () => api.createService(form),
    onSuccess: () => {
      if (form.environment_id) localStorage.setItem(LAST_ENV_KEY, form.environment_id)
      void qc.invalidateQueries({ queryKey: ['services'] })
      setShow(false)
    },
  })

  return (
    <div className="space-y-6">
      <Header
        title="Services"
        subtitle="One-click compose stacks and custom services."
        actions={
          <Btn primary onClick={() => setShow(true)}>
            New service
          </Btn>
        }
      />
      {deployError && <p className="text-sm text-[var(--color-danger)]">{deployError}</p>}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {(services.data?.services || []).map((s) => (
          <div key={s.id} className="rounded-xl border border-[var(--color-line)] bg-[var(--color-panel)]/60 p-5">
            <div className="font-medium">{s.name}</div>
            <div className="mt-1 text-sm text-[var(--color-muted)]">{s.service_type}</div>
            <div className="mt-3 text-sm">{s.status}</div>
            <div className="mt-4">
              <Btn
                primary
                onClick={() => {
                  setDeployError('')
                  void api
                    .deployService(s.id)
                    .then(() => qc.invalidateQueries({ queryKey: ['services'] }))
                    .catch((e: Error) => setDeployError(e.message))
                }}
              >
                Deploy
              </Btn>
            </div>
          </div>
        ))}
      </div>
      <div>
        <h2 className="mb-3 text-lg font-medium">Catalog</h2>
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          {(templates.data?.templates || []).map((t) => (
            <div key={t.type} className="rounded-lg border border-[var(--color-line)] px-3 py-3 text-sm">
              <div className="font-medium">{t.name}</div>
              <div className="text-[var(--color-muted)]">{t.description}</div>
            </div>
          ))}
        </div>
      </div>
      {show && (
        <Modal title="New service" onClose={() => setShow(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              create.mutate()
            }}
          >
            <Input label="Name" value={form.name} onChange={(v) => setForm({ ...form, name: v })} />
            <label className="block text-sm">
              <span className="mb-1 block text-[var(--color-muted)]">Environment</span>
              <select
                required
                value={form.environment_id}
                onChange={(e) => setForm({ ...form, environment_id: e.target.value })}
                className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2"
              >
                <option value="">Select…</option>
                {(envs.data || []).map((e) => (
                  <option key={e.id} value={e.id}>
                    {e.project_name} / {e.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block text-sm">
              <span className="mb-1 block text-[var(--color-muted)]">Destination</span>
              <select
                value={form.destination_id}
                onChange={(e) => setForm({ ...form, destination_id: e.target.value })}
                className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2"
              >
                <option value="">Optional…</option>
                {(dests.data?.destinations || []).map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name} ({d.network})
                  </option>
                ))}
              </select>
            </label>
            <label className="block text-sm">
              <span className="mb-1 block text-[var(--color-muted)]">Template</span>
              <select
                value={form.template}
                onChange={(e) => setForm({ ...form, template: e.target.value })}
                className="w-full rounded-lg border border-[var(--color-line)] bg-[var(--color-ink)] px-3 py-2"
              >
                {(templates.data?.templates || []).map((t) => (
                  <option key={t.type} value={t.type}>
                    {t.name}
                  </option>
                ))}
              </select>
            </label>
            {create.error && <p className="text-sm text-[var(--color-danger)]">{create.error.message}</p>}
            <Btn primary type="submit">
              Create
            </Btn>
          </form>
        </Modal>
      )}
    </div>
  )
}
